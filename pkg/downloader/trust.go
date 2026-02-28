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
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"helm.sh/helm/v4/internal/artifacthub"
	"helm.sh/helm/v4/pkg/helmpath"
)

// TrustConfig manages trusted publishers configuration for chart-defined plugins.
type TrustConfig struct {
	// Publishers is a list of trusted publishers.
	Publishers []TrustedPublisher `yaml:"publishers,omitempty"`
}

// TrustedPublisher represents a publisher that has been trusted by the user.
type TrustedPublisher struct {
	// Name is the display name of the publisher.
	Name string `yaml:"name"`
	// ArtifactHubOrg is the ArtifactHub organization/repository name.
	ArtifactHubOrg string `yaml:"artifacthub_org,omitempty"`
	// Fingerprint is the signing key fingerprint (single key).
	Fingerprint string `yaml:"fingerprint,omitempty"`
	// Fingerprints is a list of signing key fingerprints (multiple keys).
	Fingerprints []string `yaml:"fingerprints,omitempty"`
}

// TrustConfigPath returns the path to the trusted publishers config file.
func TrustConfigPath() string {
	return filepath.Join(helmpath.ConfigPath(), "trusted-publishers.yaml")
}

// LoadTrustConfig loads the trusted publishers configuration from disk.
// Returns an empty config if the file doesn't exist.
func LoadTrustConfig() (*TrustConfig, error) {
	configPath := TrustConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrustConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read trust config: %w", err)
	}

	var config TrustConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse trust config: %w", err)
	}

	return &config, nil
}

// SaveTrustConfig saves the trusted publishers configuration to disk.
func SaveTrustConfig(config *TrustConfig) error {
	configPath := TrustConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal trust config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write trust config: %w", err)
	}

	return nil
}

// IsTrustedPublisher checks if the given publisher/fingerprint is trusted.
func (c *TrustConfig) IsTrustedPublisher(repoName, fingerprint string) bool {
	for _, p := range c.Publishers {
		// Check by ArtifactHub org
		if p.ArtifactHubOrg != "" && p.ArtifactHubOrg == repoName {
			return true
		}

		// Check by fingerprint
		if p.Fingerprint != "" && p.Fingerprint == fingerprint {
			return true
		}

		// Check fingerprints list
		for _, fp := range p.Fingerprints {
			if fp == fingerprint {
				return true
			}
		}
	}
	return false
}

// AddTrustedPublisher adds a new trusted publisher to the config.
// If the publisher already exists, it updates the fingerprint.
func (c *TrustConfig) AddTrustedPublisher(name, artifacthubOrg, fingerprint string) {
	// Check if publisher already exists
	for i, p := range c.Publishers {
		if p.ArtifactHubOrg == artifacthubOrg {
			// Update existing publisher
			if fingerprint != "" {
				// Add fingerprint if not already present
				if p.Fingerprint == "" && len(p.Fingerprints) == 0 {
					c.Publishers[i].Fingerprint = fingerprint
				} else if p.Fingerprint != fingerprint {
					// Convert single fingerprint to list and add new one
					if p.Fingerprint != "" {
						c.Publishers[i].Fingerprints = append([]string{p.Fingerprint}, p.Fingerprints...)
						c.Publishers[i].Fingerprint = ""
					}
					// Add new fingerprint if not already present
					found := false
					for _, fp := range c.Publishers[i].Fingerprints {
						if fp == fingerprint {
							found = true
							break
						}
					}
					if !found {
						c.Publishers[i].Fingerprints = append(c.Publishers[i].Fingerprints, fingerprint)
					}
				}
			}
			return
		}
	}

	// Add new publisher
	c.Publishers = append(c.Publishers, TrustedPublisher{
		Name:           name,
		ArtifactHubOrg: artifacthubOrg,
		Fingerprint:    fingerprint,
	})
}

// PluginTrustInfo contains trust-related information about a plugin.
type PluginTrustInfo struct {
	// Plugin name
	Name string
	// Plugin version
	Version string
	// Repository URL
	Repository string
	// ArtifactHub repository name (parsed from Repository)
	ArtifactHubRepoName string
	// Whether the plugin is signed
	Signed bool
	// Signature types (e.g., "cosign", "pgp")
	Signatures []string
	// Signing key fingerprint
	Fingerprint string
	// Publisher name
	PublisherName string
	// Whether the publisher is verified on ArtifactHub
	VerifiedPublisher bool
	// Whether the publisher is trusted in local config
	TrustedPublisher bool
	// Whether the plugin is official
	Official bool
	// ArtifactHub package data (may be nil if not found)
	Package *artifacthub.PluginPackage
}

// TrustDecision represents the user's decision about trusting a plugin.
type TrustDecision int

const (
	// TrustDecisionDeny rejects the plugin.
	TrustDecisionDeny TrustDecision = iota
	// TrustDecisionAllow allows this specific plugin version.
	TrustDecisionAllow
	// TrustDecisionTrustPublisher trusts all plugins from this publisher.
	TrustDecisionTrustPublisher
)

// PromptForTrust prompts the user to trust a plugin.
// Returns the user's trust decision.
func PromptForTrust(out io.Writer, in io.Reader, info *PluginTrustInfo) (TrustDecision, error) {
	// Display plugin information
	fmt.Fprintf(out, "\nPlugin verification for %s v%s:\n", info.Name, info.Version)
	fmt.Fprintf(out, "  Repository: %s\n", info.Repository)

	if info.Package != nil {
		if info.PublisherName != "" {
			verifiedStr := ""
			if info.VerifiedPublisher {
				verifiedStr = " (verified on ArtifactHub)"
			}
			fmt.Fprintf(out, "  Publisher: %s%s\n", info.PublisherName, verifiedStr)
		}

		if info.Official {
			fmt.Fprintf(out, "  Status: Official\n")
		}

		if info.Signed {
			fmt.Fprintf(out, "  Signed: Yes (%s)\n", strings.Join(info.Signatures, ", "))
			if info.Fingerprint != "" {
				fmt.Fprintf(out, "  Key fingerprint: %s\n", formatFingerprint(info.Fingerprint))
			}
		} else {
			fmt.Fprintf(out, "  Signed: No\n")
		}
	} else {
		fmt.Fprintf(out, "  Publisher: Unknown (not registered on ArtifactHub)\n")
		fmt.Fprintf(out, "  Signed: Unknown\n")
	}

	fmt.Fprintln(out)

	// Prompt for decision
	fmt.Fprintf(out, "Trust this plugin? [y/N/trust-publisher]: ")

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil {
		return TrustDecisionDeny, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "y", "yes":
		return TrustDecisionAllow, nil
	case "trust-publisher", "trust", "t":
		return TrustDecisionTrustPublisher, nil
	default:
		return TrustDecisionDeny, nil
	}
}

// formatFingerprint formats a fingerprint for display (groups of 4 chars).
func formatFingerprint(fp string) string {
	fp = strings.ToUpper(strings.ReplaceAll(fp, " ", ""))
	if len(fp) <= 8 {
		return fp
	}

	var parts []string
	for i := 0; i < len(fp); i += 4 {
		end := i + 4
		if end > len(fp) {
			end = len(fp)
		}
		parts = append(parts, fp[i:end])
	}
	return strings.Join(parts, " ")
}

// DisplayPluginSignatureStatus displays the signature status of a plugin.
// This is used during `helm dependency update` to show trust information.
func DisplayPluginSignatureStatus(out io.Writer, info *PluginTrustInfo) {
	statusIcon := "⚠"
	statusText := "unsigned"

	if info.Signed {
		if info.VerifiedPublisher || info.TrustedPublisher {
			statusIcon = "✓"
			statusText = "signed (trusted)"
		} else {
			statusIcon = "?"
			statusText = "signed (unverified)"
		}
	}

	fmt.Fprintf(out, "  %s %s v%s - %s\n", statusIcon, info.Name, info.Version, statusText)

	if info.Package != nil && info.PublisherName != "" {
		publisherStatus := ""
		if info.VerifiedPublisher {
			publisherStatus = " [verified]"
		}
		if info.TrustedPublisher {
			publisherStatus += " [trusted]"
		}
		fmt.Fprintf(out, "    Publisher: %s%s\n", info.PublisherName, publisherStatus)
	}
}

// ParseArtifactHubRepoName extracts the ArtifactHub repository name from an OCI URL.
// For example: "oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/varsubst-render"
// returns "ref-hip-chart-defined-plugins".
func ParseArtifactHubRepoName(ociURL string) string {
	// Remove oci:// prefix
	url := strings.TrimPrefix(ociURL, "oci://")

	// Split by /
	parts := strings.Split(url, "/")

	// For ghcr.io/owner/repo/... pattern, return repo
	if len(parts) >= 3 && strings.Contains(parts[0], "ghcr.io") {
		return parts[2]
	}

	// For other registries, try to find the repo name
	// This is a heuristic and may need adjustment for different registry structures
	if len(parts) >= 2 {
		// Return the second-to-last component before "plugins"
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == "plugins" && i > 0 {
				return parts[i-1]
			}
		}
		// Fallback: return the second component
		return parts[1]
	}

	return ""
}
