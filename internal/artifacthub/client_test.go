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
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClient_GetPlugin(t *testing.T) {
	server := NewMockServerWithTestData()
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	tests := []struct {
		name       string
		repoName   string
		pluginName string
		version    string
		wantErr    error
		wantName   string
		wantSigned bool
	}{
		{
			name:       "unsigned test plugin",
			repoName:   "test-plugins",
			pluginName: "test-render",
			version:    "0.1.0",
			wantName:   "test-render",
			wantSigned: false,
		},
		{
			name:       "signed plugin",
			repoName:   "verified-plugins",
			pluginName: "signed-plugin",
			version:    "1.0.0",
			wantName:   "signed-plugin",
			wantSigned: true,
		},
		{
			name:       "community unsigned plugin",
			repoName:   "community-plugins",
			pluginName: "unsigned-plugin",
			version:    "0.5.0",
			wantName:   "unsigned-plugin",
			wantSigned: false,
		},
		{
			name:       "plugin not found",
			repoName:   "nonexistent",
			pluginName: "nonexistent",
			version:    "1.0.0",
			wantErr:    ErrPluginNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := client.GetPlugin(ctx, tt.repoName, tt.pluginName, tt.version)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GetPlugin() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetPlugin() unexpected error: %v", err)
			}

			if pkg.Name != tt.wantName {
				t.Errorf("GetPlugin() name = %v, want %v", pkg.Name, tt.wantName)
			}

			if pkg.Signed != tt.wantSigned {
				t.Errorf("GetPlugin() signed = %v, want %v", pkg.Signed, tt.wantSigned)
			}
		})
	}
}

func TestClient_GetPluginLatest(t *testing.T) {
	server := NewMockServerWithTestData()
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	pkg, err := client.GetPluginLatest(ctx, "test-plugins", "test-render")
	if err != nil {
		t.Fatalf("GetPluginLatest() unexpected error: %v", err)
	}

	if pkg.Name != "test-render" {
		t.Errorf("GetPluginLatest() name = %v, want test-render", pkg.Name)
	}

	if pkg.Version != "0.1.0" {
		t.Errorf("GetPluginLatest() version = %v, want 0.1.0", pkg.Version)
	}
}

func TestClient_GetSigningKey(t *testing.T) {
	server := NewMockServerWithTestData()
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	tests := []struct {
		name            string
		repoName        string
		pluginName      string
		version         string
		wantErr         error
		wantFingerprint string
	}{
		{
			name:       "unsigned test plugin",
			repoName:   "test-plugins",
			pluginName: "test-render",
			version:    "0.1.0",
			wantErr:    ErrNotSigned,
		},
		{
			name:       "unsigned community plugin",
			repoName:   "community-plugins",
			pluginName: "unsigned-plugin",
			version:    "0.5.0",
			wantErr:    ErrNotSigned,
		},
		{
			name:       "plugin not found",
			repoName:   "nonexistent",
			pluginName: "nonexistent",
			version:    "1.0.0",
			wantErr:    ErrPluginNotFound,
		},
		{
			name:            "signed plugin",
			repoName:        "verified-plugins",
			pluginName:      "signed-plugin",
			version:         "1.0.0",
			wantFingerprint: "ABCD1234EFGH5678IJKL9012MNOP3456QRST7890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := client.GetSigningKey(ctx, tt.repoName, tt.pluginName, tt.version)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GetSigningKey() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetSigningKey() unexpected error: %v", err)
			}

			if key.Fingerprint != tt.wantFingerprint {
				t.Errorf("GetSigningKey() fingerprint = %v, want %v", key.Fingerprint, tt.wantFingerprint)
			}
		})
	}
}

func TestClient_SearchPlugins(t *testing.T) {
	server := NewMockServerWithTestData()
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	tests := []struct {
		name      string
		opts      SearchPluginsOptions
		wantCount int
	}{
		{
			name:      "search all plugins",
			opts:      SearchPluginsOptions{},
			wantCount: 3, // All test plugins
		},
		{
			name: "search by query - render",
			opts: SearchPluginsOptions{
				Query: "render",
			},
			wantCount: 1, // test-render
		},
		{
			name: "search by query - retrieval",
			opts: SearchPluginsOptions{
				Query: "retrieval",
			},
			wantCount: 1, // signed-plugin (matches description "key retrieval")
		},
		{
			name: "search verified publishers only",
			opts: SearchPluginsOptions{
				VerifiedPublisher: true,
			},
			wantCount: 1, // Only signed-plugin is from verified publisher
		},
		{
			name: "search official only",
			opts: SearchPluginsOptions{
				Official: true,
			},
			wantCount: 0, // No official plugins in test data
		},
		{
			name: "search with limit",
			opts: SearchPluginsOptions{
				Limit: 2,
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.SearchPlugins(ctx, tt.opts)
			if err != nil {
				t.Fatalf("SearchPlugins() unexpected error: %v", err)
			}

			if len(result.Packages) != tt.wantCount {
				t.Errorf("SearchPlugins() got %d packages, want %d", len(result.Packages), tt.wantCount)
			}
		})
	}
}

func TestPluginData(t *testing.T) {
	server := NewMockServerWithTestData()
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	pkg, err := client.GetPlugin(ctx, "test-plugins", "test-render", "0.1.0")
	if err != nil {
		t.Fatalf("GetPlugin() unexpected error: %v", err)
	}

	if pkg.Data == nil {
		t.Fatal("GetPlugin() data is nil")
	}

	if pkg.Data.PluginType != "render/v1" {
		t.Errorf("PluginType = %v, want render/v1", pkg.Data.PluginType)
	}

	if pkg.Data.Runtime != "wasm" {
		t.Errorf("Runtime = %v, want wasm", pkg.Data.Runtime)
	}

	if pkg.Data.HelmVersionConstraint != ">=4.0.0" {
		t.Errorf("HelmVersionConstraint = %v, want >=4.0.0", pkg.Data.HelmVersionConstraint)
	}
}

func TestFetchSigningKey(t *testing.T) {
	server := NewMockServerWithTestData()
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	// Get the signed plugin to find its key URL
	pkg, err := client.GetPlugin(ctx, "verified-plugins", "signed-plugin", "1.0.0")
	if err != nil {
		t.Fatalf("GetPlugin() unexpected error: %v", err)
	}

	if pkg.SignKey == nil {
		t.Fatal("Expected SignKey to be present")
	}

	// The mock server uses HTTP, but FetchSigningKey requires HTTPS for security
	// This verifies the HTTPS requirement is enforced
	_, err = client.FetchSigningKey(ctx, pkg.SignKey.URL)
	if err == nil {
		t.Error("FetchSigningKey() should reject HTTP URLs")
	}

	// Verify the error message mentions HTTPS requirement
	if err != nil && !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("FetchSigningKey() error should mention HTTPS: %v", err)
	}
}

func TestFetchSigningKey_HTTPS(t *testing.T) {
	// Test that HTTPS URLs are accepted (we can't actually test
	// fetching without a real HTTPS server, but we verify the
	// URL validation logic)
	client := NewClient()
	ctx := context.Background()

	// Invalid URL should error
	_, err := client.FetchSigningKey(ctx, "not-a-url")
	if err == nil {
		t.Error("FetchSigningKey() should reject invalid URLs")
	}

	// HTTP URL should be rejected
	_, err = client.FetchSigningKey(ctx, "http://example.com/key.asc")
	if err == nil {
		t.Error("FetchSigningKey() should reject HTTP URLs")
	}
}
