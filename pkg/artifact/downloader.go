/*
Copyright The Helm Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package artifact

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/fileutil"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/transport"
)

// Type represents the type of artifact being downloaded.
type Type string

const (
	TypeChart  Type = "chart"
	TypePlugin Type = "plugin"
	// Future artifact types beyond charts and plugins can be added here
)

// VerificationStrategy describes a strategy for determining whether to verify an artifact.
type VerificationStrategy int

const (
	// VerifyNever will skip all verification of an artifact.
	VerifyNever VerificationStrategy = iota
	// VerifyIfPossible will attempt a verification, it will not error if verification
	// data is missing. But it will not stop processing if verification fails.
	VerifyIfPossible
	// VerifyAlways will always attempt a verification, and will fail if the
	// verification fails.
	VerifyAlways
	// VerifyLater will fetch verification data, but not do any verification.
	// This is to accommodate the case where another step of the process will
	// perform verification.
	VerifyLater
)

// Cache defines the interface for artifact caching.
type Cache interface {
	Get(digest [32]byte, cacheType CacheType) (string, error)
	Put(digest [32]byte, data *bytes.Buffer, cacheType CacheType) (string, error)
}

// CacheType indicates what type of cached item is being stored.
type CacheType int

const (
	CacheArtifact CacheType = iota
	CacheProv
)

// ErrNoOwnerRepo indicates that a given artifact URL can't be found in any repos.
var ErrNoOwnerRepo = errors.New("could not find a repo containing the given URL")

// Downloader handles downloading artifacts with verification and caching.
type Downloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// Transports provide protocol handling for different URL schemes.
	Transports transport.Providers
	// Options provide parameters to be passed along to Transports.
	Options []transport.Option
	// ContentCache is the location where Cache stores its files by default.
	ContentCache string
	// Cache specifies the cache implementation to use.
	Cache Cache

	// Configuration for repository-based transports
	repositoryConfig string
	repositoryCache  string
	// Configuration for OCI-based transports
	registryClient *registry.Client
}

// Download retrieves an artifact. Depending on the settings, it may also download a provenance file.
//
// If Verify is set to VerifyNever, the verification will be nil.
// If Verify is set to VerifyIfPossible, this will return a verification (or nil on failure), and print a warning on failure.
// If Verify is set to VerifyAlways, this will return a verification or an error if the verification fails.
// If Verify is set to VerifyLater, this will download the prov file (if it exists), but not verify it.
//
// Returns a string path to the location where the file was downloaded and a verification
// (if provenance was verified), or an error if something bad happened.
func (d *Downloader) Download(ref, version, dest string, artifactType Type) (string, *provenance.Verification, error) {
	if d.Cache == nil {
		if d.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		d.Cache = &DiskCache{Root: d.ContentCache}
		slog.Debug("set up default downloader cache")
	}

	hash, u, err := d.ResolveArtifactVersion(ref, version, artifactType)
	if err != nil {
		return "", nil, err
	}

	t, err := d.Transports.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	// Configure transport based on its capabilities
	if err := d.configureTransport(t); err != nil {
		return "", nil, err
	}

	// Check the cache for the content. Otherwise download it.
	var data *bytes.Buffer
	var found bool
	var digest []byte
	var digest32 [32]byte
	if hash != "" {
		// if there is a hash, populate the other formats
		digest, err = hex.DecodeString(hash)
		if err != nil {
			return "", nil, err
		}
		copy(digest32[:], digest)
		if pth, err := d.Cache.Get(digest32, CacheArtifact); err == nil {
			fdata, err := os.ReadFile(pth)
			if err == nil {
				found = true
				data = bytes.NewBuffer(fdata)
				slog.Debug("found artifact in cache", "id", hash, "type", artifactType)
			}
		}
	}

	if !found {
		opts := append(d.Options, transport.WithArtifactType(string(artifactType)))
		data, err = t.Get(u.String(), opts...)
		if err != nil {
			return "", nil, err
		}
	}

	name := d.generateArtifactName(u, artifactType)
	destfile := filepath.Join(dest, name)
	if err := fileutil.AtomicWriteFile(destfile, data, 0644); err != nil {
		return destfile, nil, err
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if d.Verify > VerifyNever {
		found = false
		var body *bytes.Buffer
		if hash != "" {
			if pth, err := d.Cache.Get(digest32, CacheProv); err == nil {
				fdata, err := os.ReadFile(pth)
				if err == nil {
					found = true
					body = bytes.NewBuffer(fdata)
					slog.Debug("found provenance in cache", "id", hash, "type", artifactType)
				}
			}
		}
		if !found {
			opts := append(d.Options, transport.WithArtifactType(string(artifactType)))
			body, err = t.Get(u.String()+".prov", opts...)
			if err != nil {
				if d.Verify == VerifyAlways {
					return destfile, ver, fmt.Errorf("failed to fetch provenance %q", u.String()+".prov")
				}
				fmt.Fprintf(d.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
				return destfile, ver, nil
			}
		}
		provfile := destfile + ".prov"
		if err := fileutil.AtomicWriteFile(provfile, body, 0644); err != nil {
			return destfile, nil, err
		}

		if d.Verify != VerifyLater {
			ver, err = d.VerifyArtifact(destfile, destfile+".prov")
			if err != nil {
				return destfile, ver, err
			}
		}
	}
	return destfile, ver, nil
}

// SetRepositoryConfig sets the repository configuration for repository-based transports.
func (d *Downloader) SetRepositoryConfig(config, cache string) {
	d.repositoryConfig = config
	d.repositoryCache = cache
}

// SetRegistryClient sets the registry client for OCI-based transports.
func (d *Downloader) SetRegistryClient(client *registry.Client) {
	d.registryClient = client
}

// configureTransport configures a transport based on its supported interfaces.
func (d *Downloader) configureTransport(t transport.Transport) error {
	// Configure repository-based transports
	if repoProvider, ok := t.(transport.RepositoryProvider); ok {
		if d.repositoryConfig != "" {
			repoProvider.SetRepositoryConfig(d.repositoryConfig)
		}
		if d.repositoryCache != "" {
			repoProvider.SetRepositoryCache(d.repositoryCache)
		}
	}

	// Configure client-based transports (like OCI)
	if clientProvider, ok := t.(transport.ClientProvider); ok {
		if d.registryClient != nil {
			clientProvider.SetClient(d.registryClient)
		}
	}

	return nil
}

// generateArtifactName generates the filename for the downloaded artifact.
func (d *Downloader) generateArtifactName(u *url.URL, artifactType Type) string {
	name := filepath.Base(u.Path)

	// Handle OCI references
	if u.Scheme == registry.OCIScheme {
		idx := strings.LastIndexByte(name, ':')
		if idx >= 0 {
			name = fmt.Sprintf("%s-%s", name[:idx], name[idx+1:])
		}
	}

	// Add appropriate extension based on artifact type
	switch artifactType {
	case TypeChart:
		if !strings.HasSuffix(name, ".tgz") {
			name += ".tgz"
		}
	case TypePlugin:
		if !strings.HasSuffix(name, ".tgz") {
			name += ".tgz"
		}
	default:
		// Default to tgz for unknown types
		if !strings.HasSuffix(name, ".tgz") {
			name += ".tgz"
		}
	}

	return name
}

// ResolveArtifactVersion resolves an artifact reference to a URL.
//
// It returns:
// - A hash of the content if available
// - The URL for downloading the artifact
// - An error if resolution fails
//
// A reference may be an HTTP URL, an OCI reference URL, a 'reponame/artifactname'
// reference, or a local path.
//
// A version is a SemVer string (1.2.3-beta.1+f334a6789).
func (d *Downloader) ResolveArtifactVersion(ref, version string, artifactType Type) (string, *url.URL, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return "", nil, fmt.Errorf("invalid artifact URL format: %s", ref)
	}

	// Handle OCI references - supported for all artifact types
	if registry.IsOCI(u.String()) {
		if d.registryClient == nil {
			// In testing or when registry client is not configured,
			// treat OCI URLs as direct URLs to allow testing
			return "", u, nil
		}

		digest, OCIref, err := d.registryClient.ValidateReference(ref, version, u)
		return digest, OCIref, err
	}

	// Handle direct URLs (http/https/file) - supported for all artifact types
	if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
		// For direct URLs, version is typically ignored since URLs are specific
		// But we could validate that the URL contains the expected version string
		return "", u, nil
	}

	// Handle repository-based references (reponame/artifactname)
	switch artifactType {
	case TypeChart:
		// Chart repository resolution requires pkg/getter which would create import cycles
		// Use pkg/downloader.ChartDownloader for backward compatibility
		return "", nil, fmt.Errorf("repository-based chart resolution should use pkg/downloader.ChartDownloader for backward compatibility")

	case TypePlugin:
		// Plugins use modern OCI-based distribution, not repository-based like charts
		// Repository-based plugin distribution is not planned due to scalability issues
		// with the chart repository index model
		return "", nil, fmt.Errorf("repository-based plugin distribution is not supported - use OCI references instead (oci://registry/plugin:version)")


	default:
		return "", nil, fmt.Errorf("unknown artifact type: %s", artifactType)
	}
}

// VerifyArtifact verifies an artifact using its provenance file.
func (d *Downloader) VerifyArtifact(artifactPath, provPath string) (*provenance.Verification, error) {
	// Check if artifact exists and is valid format
	switch fi, err := os.Stat(artifactPath); {
	case err != nil:
		return nil, err
	case fi.IsDir():
		return nil, errors.New("unpacked artifacts cannot be verified")
	case !strings.HasSuffix(artifactPath, ".tgz"):
		return nil, errors.New("artifact must be a tgz file")
	}

	if _, err := os.Stat(provPath); err != nil {
		return nil, fmt.Errorf("could not load provenance file %s: %w", provPath, err)
	}

	sig, err := provenance.NewFromKeyring(d.Keyring, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load keyring: %w", err)
	}

	// Read artifact and provenance files
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact: %w", err)
	}
	provData, err := os.ReadFile(provPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	return sig.Verify(artifactData, provData, filepath.Base(artifactPath))
}

// DiskCache is a simple disk-based cache implementation.
type DiskCache struct {
	Root string
}

func (c *DiskCache) Get(digest [32]byte, cacheType CacheType) (string, error) {
	subdir := "artifacts"
	if cacheType == CacheProv {
		subdir = "provenance"
	}
	path := filepath.Join(c.Root, subdir, hex.EncodeToString(digest[:]))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

func (c *DiskCache) Put(digest [32]byte, data *bytes.Buffer, cacheType CacheType) (string, error) {
	subdir := "artifacts"
	if cacheType == CacheProv {
		subdir = "provenance"
	}
	dir := filepath.Join(c.Root, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, hex.EncodeToString(digest[:]))
	return path, fileutil.AtomicWriteFile(path, data, 0644)
}
