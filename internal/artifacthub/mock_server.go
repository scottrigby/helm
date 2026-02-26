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

package artifacthub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// MockServer provides a mock ArtifactHub API for testing.
type MockServer struct {
	*httptest.Server

	mu      sync.RWMutex
	plugins map[string]*PluginPackage // key: "repoName/pluginName/version"
	sigKeys map[string][]byte         // key: URL, value: key content
}

// NewMockServer creates a new mock ArtifactHub server.
func NewMockServer() *MockServer {
	s := &MockServer{
		plugins: make(map[string]*PluginPackage),
		sigKeys: make(map[string][]byte),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm-plugin/", s.handlePlugin)
	mux.HandleFunc("/api/v1/packages/search", s.handleSearch)
	mux.HandleFunc("/keys/", s.handleKeys)

	s.Server = httptest.NewServer(mux)
	return s
}

// AddPlugin registers a plugin package for the mock server.
func (s *MockServer) AddPlugin(repoName, pluginName, version string, pkg *PluginPackage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := repoName + "/" + pluginName + "/" + version
	s.plugins[key] = pkg

	// Also store without version for "latest" lookups
	latestKey := repoName + "/" + pluginName
	if existing, ok := s.plugins[latestKey]; !ok || pkg.Version > existing.Version {
		s.plugins[latestKey] = pkg
	}
}

// AddSigningKey registers a signing key that can be fetched via URL.
func (s *MockServer) AddSigningKey(keyURL string, keyContent []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sigKeys[keyURL] = keyContent
}

func (s *MockServer) handlePlugin(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse path: /api/v1/packages/helm-plugin/{repo}/{name}[/{version}]
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/packages/helm-plugin/")
	parts := strings.Split(path, "/")

	var key string
	switch len(parts) {
	case 2:
		// Latest version: repo/name
		key = parts[0] + "/" + parts[1]
	case 3:
		// Specific version: repo/name/version
		key = parts[0] + "/" + parts[1] + "/" + parts[2]
	default:
		http.NotFound(w, r)
		return
	}

	pkg, ok := s.plugins[key]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pkg)
}

func (s *MockServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := r.URL.Query()

	// Check kind filter
	kindStr := query.Get("kind")
	if kindStr != "" && kindStr != strconv.Itoa(int(KindHelmPlugin)) {
		// Not searching for plugins
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"packages": []interface{}{},
		})
		return
	}

	searchQuery := strings.ToLower(query.Get("ts_query_web"))
	verifiedOnly := query.Get("verified_publisher") == "true"
	officialOnly := query.Get("official") == "true"

	var results []PluginPackage
	seen := make(map[string]bool)

	for key, pkg := range s.plugins {
		// Skip versioned entries (only include latest)
		parts := strings.Split(key, "/")
		if len(parts) == 3 {
			continue
		}

		// Deduplicate
		if seen[pkg.PackageID] {
			continue
		}

		// Apply filters
		if searchQuery != "" {
			match := strings.Contains(strings.ToLower(pkg.Name), searchQuery) ||
				strings.Contains(strings.ToLower(pkg.Description), searchQuery)
			if !match {
				continue
			}
		}
		if verifiedOnly && (pkg.Repository == nil || !pkg.Repository.VerifiedPublisher) {
			continue
		}
		if officialOnly && (pkg.Repository == nil || !pkg.Repository.Official) {
			continue
		}

		results = append(results, *pkg)
		seen[pkg.PackageID] = true
	}

	// Apply pagination
	offset := 0
	if o := query.Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}
	limit := 20
	if l := query.Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	totalCount := len(results)
	if offset >= len(results) {
		results = nil
	} else {
		end := offset + limit
		if end > len(results) {
			end = len(results)
		}
		results = results[offset:end]
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Pagination-Total-Count", strconv.Itoa(totalCount))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"packages": results,
	})
}

func (s *MockServer) handleKeys(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try to find the key by full URL
	keyURL := s.URL + r.URL.Path
	keyContent, ok := s.sigKeys[keyURL]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/pgp-keys")
	w.Write(keyContent)
}

// NewMockServerWithTestData creates a mock server with generic test data
// suitable for unit testing. Test plugins use placeholder OCI URLs.
func NewMockServerWithTestData() *MockServer {
	s := NewMockServer()

	// Generic test plugin - render/v1 Wasm plugin
	testRenderPlugin := &PluginPackage{
		PackageID:   "test-render-plugin-id",
		Name:        "test-render",
		DisplayName: "Test Render Plugin",
		Description: "A test render/v1 plugin for unit testing",
		Version:     "0.1.0",
		License:     "Apache-2.0",
		Signed:      false,
		ContentURL:  "oci://test-registry/plugins/test-render:0.1.0",
		TS:          1708531200,
		Data: &PluginData{
			PluginType:            "render/v1",
			Runtime:               "wasm",
			HelmVersionConstraint: ">=4.0.0",
			Platforms:             []string{"any"},
			FilePatterns:          []string{"*.yaml", "*.yml"},
		},
		Repository: &Repository{
			RepositoryID:      "test-repo-id",
			Kind:              KindHelmPlugin,
			Name:              "test-plugins",
			DisplayName:       "Test Plugins Repository",
			URL:               "oci://test-registry/plugins",
			VerifiedPublisher: false,
			Official:          false,
		},
		Keywords: []string{"helm", "helm-plugin", "test"},
	}
	s.AddPlugin("test-plugins", "test-render", "0.1.0", testRenderPlugin)

	// Signed plugin for testing verification flow
	signedPlugin := &PluginPackage{
		PackageID:   "signed-plugin-id",
		Name:        "signed-plugin",
		DisplayName: "Signed Plugin",
		Description: "A signed plugin for testing key retrieval",
		Version:     "1.0.0",
		License:     "Apache-2.0",
		Signed:      true,
		Signatures:  []string{"pgp"},
		SignKey: &SignKey{
			Fingerprint: "ABCD1234EFGH5678IJKL9012MNOP3456QRST7890",
			URL:         "", // Will be set dynamically with server URL
		},
		ContentURL: "oci://test-registry/plugins/signed-plugin:1.0.0",
		TS:         1708444800,
		Data: &PluginData{
			PluginType:            "render/v1",
			Runtime:               "wasm",
			HelmVersionConstraint: ">=4.0.0",
		},
		Repository: &Repository{
			RepositoryID:      "verified-repo-id",
			Kind:              KindHelmPlugin,
			Name:              "verified-plugins",
			DisplayName:       "Verified Plugins",
			URL:               "oci://test-registry/verified-plugins",
			VerifiedPublisher: true,
			Official:          false,
		},
		Keywords: []string{"helm", "helm-plugin", "signed"},
	}

	// Update signing key URL to use mock server URL
	signedPlugin.SignKey.URL = s.URL + "/keys/signed-plugin.asc"
	s.AddPlugin("verified-plugins", "signed-plugin", "1.0.0", signedPlugin)

	// Add mock signing key
	s.AddSigningKey(s.URL+"/keys/signed-plugin.asc", []byte(`-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: Test Key v1.0

mQINBGXYZ...test-key-data...
-----END PGP PUBLIC KEY BLOCK-----
`))

	// Unsigned plugin (explicitly marked)
	unsignedPlugin := &PluginPackage{
		PackageID:   "unsigned-plugin-id",
		Name:        "unsigned-plugin",
		DisplayName: "Unsigned Plugin",
		Description: "An unsigned plugin for testing unsigned flow",
		Version:     "0.5.0",
		License:     "MIT",
		Signed:      false,
		ContentURL:  "oci://test-registry/plugins/unsigned-plugin:0.5.0",
		TS:          1708358400,
		Data: &PluginData{
			PluginType:            "render/v1",
			Runtime:               "wasm",
			HelmVersionConstraint: ">=4.0.0",
		},
		Repository: &Repository{
			RepositoryID:      "community-repo-id",
			Kind:              KindHelmPlugin,
			Name:              "community-plugins",
			DisplayName:       "Community Plugins",
			URL:               "oci://test-registry/community-plugins",
			VerifiedPublisher: false,
			Official:          false,
		},
		Keywords: []string{"helm", "helm-plugin", "community"},
	}
	s.AddPlugin("community-plugins", "unsigned-plugin", "0.5.0", unsignedPlugin)

	return s
}
