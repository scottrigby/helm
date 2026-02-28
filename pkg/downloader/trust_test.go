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

package downloader

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArtifactHubRepoName(t *testing.T) {
	tests := []struct {
		name     string
		ociURL   string
		expected string
	}{
		{
			name:     "ghcr.io with plugins path",
			ociURL:   "oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/varsubst-render",
			expected: "ref-hip-chart-defined-plugins",
		},
		{
			name:     "ghcr.io short path",
			ociURL:   "oci://ghcr.io/owner/repo",
			expected: "repo",
		},
		{
			name:     "ghcr.io without plugins",
			ociURL:   "oci://ghcr.io/owner/myrepo/someplugin",
			expected: "myrepo",
		},
		{
			name:     "other registry with plugins path",
			ociURL:   "oci://registry.example.com/org/repo/plugins/myplugin",
			expected: "repo",
		},
		{
			name:     "without oci prefix",
			ociURL:   "ghcr.io/owner/repo/plugins/test",
			expected: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseArtifactHubRepoName(tt.ociURL)
			if result != tt.expected {
				t.Errorf("ParseArtifactHubRepoName(%q) = %q, want %q", tt.ociURL, result, tt.expected)
			}
		})
	}
}

func TestTrustConfigAddTrustedPublisher(t *testing.T) {
	config := &TrustConfig{}

	// Add first publisher
	config.AddTrustedPublisher("Test Publisher", "test-repo", "fingerprint1")
	if len(config.Publishers) != 1 {
		t.Fatalf("expected 1 publisher, got %d", len(config.Publishers))
	}
	if config.Publishers[0].Name != "Test Publisher" {
		t.Errorf("expected name 'Test Publisher', got %q", config.Publishers[0].Name)
	}
	if config.Publishers[0].Fingerprint != "fingerprint1" {
		t.Errorf("expected fingerprint 'fingerprint1', got %q", config.Publishers[0].Fingerprint)
	}

	// Add same publisher with different fingerprint
	config.AddTrustedPublisher("Test Publisher", "test-repo", "fingerprint2")
	if len(config.Publishers) != 1 {
		t.Fatalf("expected 1 publisher after update, got %d", len(config.Publishers))
	}
	if len(config.Publishers[0].Fingerprints) != 2 {
		t.Errorf("expected 2 fingerprints after update, got %d", len(config.Publishers[0].Fingerprints))
	}

	// Add different publisher
	config.AddTrustedPublisher("Another Publisher", "another-repo", "fingerprint3")
	if len(config.Publishers) != 2 {
		t.Fatalf("expected 2 publishers, got %d", len(config.Publishers))
	}
}

func TestTrustConfigIsTrustedPublisher(t *testing.T) {
	config := &TrustConfig{
		Publishers: []TrustedPublisher{
			{
				Name:           "Test Publisher",
				ArtifactHubOrg: "test-repo",
				Fingerprint:    "fingerprint1",
			},
			{
				Name:           "Multi Key Publisher",
				ArtifactHubOrg: "multi-repo",
				Fingerprints:   []string{"fp1", "fp2", "fp3"},
			},
		},
	}

	tests := []struct {
		name        string
		repoName    string
		fingerprint string
		expected    bool
	}{
		{
			name:        "trusted by repo name",
			repoName:    "test-repo",
			fingerprint: "",
			expected:    true,
		},
		{
			name:        "trusted by single fingerprint",
			repoName:    "unknown-repo",
			fingerprint: "fingerprint1",
			expected:    true,
		},
		{
			name:        "trusted by fingerprints list",
			repoName:    "unknown-repo",
			fingerprint: "fp2",
			expected:    true,
		},
		{
			name:        "not trusted - unknown repo and fingerprint",
			repoName:    "unknown-repo",
			fingerprint: "unknown-fp",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsTrustedPublisher(tt.repoName, tt.fingerprint)
			if result != tt.expected {
				t.Errorf("IsTrustedPublisher(%q, %q) = %v, want %v", tt.repoName, tt.fingerprint, result, tt.expected)
			}
		})
	}
}

func TestPromptForTrust(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TrustDecision
	}{
		{"yes lowercase", "y\n", TrustDecisionAllow},
		{"yes full", "yes\n", TrustDecisionAllow},
		{"trust publisher", "trust-publisher\n", TrustDecisionTrustPublisher},
		{"trust shorthand", "t\n", TrustDecisionTrustPublisher},
		{"trust alias", "trust\n", TrustDecisionTrustPublisher},
		{"no", "n\n", TrustDecisionDeny},
		{"empty", "\n", TrustDecisionDeny},
		{"random input", "something\n", TrustDecisionDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			in := strings.NewReader(tt.input)
			info := &PluginTrustInfo{
				Name:       "test-plugin",
				Version:    "1.0.0",
				Repository: "oci://example.com/plugins/test",
			}

			decision, err := PromptForTrust(out, in, info)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision != tt.expected {
				t.Errorf("PromptForTrust with input %q = %v, want %v", tt.input, decision, tt.expected)
			}
		})
	}
}

func TestDisplayPluginSignatureStatus(t *testing.T) {
	tests := []struct {
		name     string
		info     *PluginTrustInfo
		contains []string
	}{
		{
			name: "unsigned plugin",
			info: &PluginTrustInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
				Signed:  false,
			},
			contains: []string{"⚠", "test-plugin", "1.0.0", "unsigned"},
		},
		{
			name: "signed trusted plugin",
			info: &PluginTrustInfo{
				Name:              "test-plugin",
				Version:           "1.0.0",
				Signed:            true,
				VerifiedPublisher: true,
			},
			contains: []string{"✓", "test-plugin", "signed (trusted)"},
		},
		{
			name: "signed unverified plugin",
			info: &PluginTrustInfo{
				Name:              "test-plugin",
				Version:           "1.0.0",
				Signed:            true,
				VerifiedPublisher: false,
			},
			contains: []string{"?", "test-plugin", "signed (unverified)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			DisplayPluginSignatureStatus(out, tt.info)
			output := out.String()
			for _, s := range tt.contains {
				if !strings.Contains(output, s) {
					t.Errorf("output %q does not contain %q", output, s)
				}
			}
		})
	}
}

func TestFormatFingerprint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ABCD1234EFGH5678", "ABCD 1234 EFGH 5678"},
		{"abcd1234", "ABCD1234"},           // <= 8 chars, no grouping
		{"AB CD EF", "ABCDEF"},             // <= 8 after removing spaces
		{"short", "SHORT"},                 // <= 8 chars, just uppercase
		{"ABC", "ABC"},                     // <= 8 chars
		{"123456789012", "1234 5678 9012"}, // > 8 chars, grouped
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatFingerprint(tt.input)
			if result != tt.expected {
				t.Errorf("formatFingerprint(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadSaveTrustConfig(t *testing.T) {
	// Create a temp directory for test config
	tmpDir := t.TempDir()
	origConfigPath := os.Getenv("HELM_CONFIG_HOME")
	os.Setenv("HELM_CONFIG_HOME", tmpDir)
	defer os.Setenv("HELM_CONFIG_HOME", origConfigPath)

	// Test loading non-existent config returns empty
	config, err := LoadTrustConfig()
	if err != nil {
		t.Fatalf("LoadTrustConfig failed: %v", err)
	}
	if len(config.Publishers) != 0 {
		t.Errorf("expected empty publishers, got %d", len(config.Publishers))
	}

	// Add a publisher and save
	config.AddTrustedPublisher("Test Publisher", "test-repo", "fingerprint123")
	err = SaveTrustConfig(config)
	if err != nil {
		t.Fatalf("SaveTrustConfig failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, "trusted-publishers.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Reload and verify
	loaded, err := LoadTrustConfig()
	if err != nil {
		t.Fatalf("LoadTrustConfig after save failed: %v", err)
	}
	if len(loaded.Publishers) != 1 {
		t.Fatalf("expected 1 publisher after reload, got %d", len(loaded.Publishers))
	}
	if loaded.Publishers[0].Name != "Test Publisher" {
		t.Errorf("expected name 'Test Publisher', got %q", loaded.Publishers[0].Name)
	}
	if loaded.Publishers[0].ArtifactHubOrg != "test-repo" {
		t.Errorf("expected org 'test-repo', got %q", loaded.Publishers[0].ArtifactHubOrg)
	}
	if loaded.Publishers[0].Fingerprint != "fingerprint123" {
		t.Errorf("expected fingerprint 'fingerprint123', got %q", loaded.Publishers[0].Fingerprint)
	}
}

func TestPluginDownloaderCheckPluginTrust(t *testing.T) {
	tests := []struct {
		name          string
		downloader    *PluginDownloader
		info          *PluginTrustInfo
		expectError   bool
		errorContains string
	}{
		{
			name: "auto-approve allows everything",
			downloader: &PluginDownloader{
				AutoApprove: true,
			},
			info: &PluginTrustInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
				Signed:  false,
			},
			expectError: false,
		},
		{
			name:       "trusted publisher allows",
			downloader: &PluginDownloader{},
			info: &PluginTrustInfo{
				Name:             "test-plugin",
				Version:          "1.0.0",
				TrustedPublisher: true,
			},
			expectError: false,
		},
		{
			name:       "verified publisher with signature allows",
			downloader: &PluginDownloader{},
			info: &PluginTrustInfo{
				Name:              "test-plugin",
				Version:           "1.0.0",
				Signed:            true,
				VerifiedPublisher: true,
			},
			expectError: false,
		},
		{
			name: "trust-unsigned allows unsigned plugins",
			downloader: &PluginDownloader{
				TrustUnsigned: true,
			},
			info: &PluginTrustInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
				Signed:  false,
			},
			expectError: false,
		},
		{
			name:       "unsigned plugin without flags rejected",
			downloader: &PluginDownloader{},
			info: &PluginTrustInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
				Signed:  false,
			},
			expectError:   true,
			errorContains: "unsigned",
		},
		{
			name:       "unverified publisher rejected",
			downloader: &PluginDownloader{},
			info: &PluginTrustInfo{
				Name:              "test-plugin",
				Version:           "1.0.0",
				Signed:            true,
				VerifiedPublisher: false,
				TrustedPublisher:  false,
			},
			expectError:   true,
			errorContains: "unverified publisher",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.downloader.checkPluginTrust(tt.info)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
