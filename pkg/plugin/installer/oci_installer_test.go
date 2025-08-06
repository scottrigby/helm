package installer

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/plugin/cache"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func Test_Install_Happy(t *testing.T) {
	shasum := ""
	var content []byte
	contentSize := 0
	uploadSessionId := "c6ce3ba4-788f-4e10-93ed-ff77d35c6851"

	// https://github.com/opencontainers/distribution-spec/blob/main/spec.md
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusNotFound)
		} else if r.Method == "POST" && r.URL.Path == "/v2/test/blobs/uploads/" {
			w.Header().Set("Location", "/v2/test/blobs/uploads/"+uploadSessionId)
			w.WriteHeader(http.StatusAccepted)
		} else if r.Method == "PUT" && r.URL.Path == "/v2/test/blobs/uploads/"+uploadSessionId {
			w.Header().Set("Location", "/v2/test/blobs/sha256:irrelevant")
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/v2/test/manifests/sha256:") {
			content = make([]byte, r.ContentLength)
			r.Body.Read(content)
			h := sha256.New()
			h.Write(content)
			shasumBuilder := strings.Builder{}
			fmt.Fprintf(&shasumBuilder, "%x", h.Sum(nil))
			shasum = shasumBuilder.String()
			contentSize = len(content)
			w.Header().Set("Location", "/v2/test/manifests/sha256:"+shasum)
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == "GET" && r.URL.Path == "/v2/test/manifests/sha256:"+shasum {
			w.Header().Set("Content-Length", strconv.Itoa(contentSize))
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", "sha256:"+shasum)
			_, err := fmt.Fprint(w, string(content))
			if err != nil {
				t.Errorf("%s", err)
			}
		} else if r.Method == "PUT" && r.URL.Path == "/v2/test/manifests/0.1.0" {
			w.Header().Set("Docker-Content-Digest", "sha256:"+shasum)
			w.Header().Set("Content-Length", strconv.Itoa(contentSize))
			w.Header().Set("Location", "/v2/test/manifests/sha256:"+shasum)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	settings := cli.New()
	pluginName := "jesse_is_cool"

	key, err := cache.Key("jesse_is_cool")
	if err != nil {
		t.Fail()
	}

	u, _ := url.ParseRequestURI(srv.URL)

	// example:   oci://localhost:9283
	ociReplacedUrl := strings.Replace(u.String(), "http", "oci", 1)
	ociHostName := strings.Replace(ociReplacedUrl, "oci://", "", 1)

	repository := &remote.Repository{
		Client: &auth.Client{
			Client: srv.Client(),
		},
		//   oci://localhost:9283/jesse_plugin:tag_one
		Reference: registry.Reference{
			Registry:   ociHostName,
			Repository: "jesse_plugin",
			Reference:  "tag_one",
		},
		PlainHTTP: true,
	}

	ociInstaller := OCIInstaller{
		CacheDir:   helmpath.CachePath("plugins", key),
		base:       newBase(ociReplacedUrl + "/jesse_plugin:tag_one"),
		PluginName: pluginName,
		repository: repository,
		settings:   settings,
	}

	err = ociInstaller.Install()
	if err != nil {
		t.Errorf("Test failed %s", err)
	}

}
