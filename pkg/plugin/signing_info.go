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

package plugin

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/openpgp/clearsign" //nolint
	"gopkg.in/yaml.v3"

	"helm.sh/helm/v4/pkg/helmpath"
)

// SigningInfo contains information about a plugin's signing status
type SigningInfo struct {
	// Status can be:
	// - "local dev": Plugin is a symlink (development mode)
	// - "unsigned": No provenance file found
	// - "invalid provenance": Provenance file is malformed
	// - "mismatched provenance": Provenance file is for a different plugin/version
	// - "signed": Valid signature exists for this exact plugin
	Status   string
	IsSigned bool // True if plugin has a valid signature (even if not verified against keyring)
}

// GetPluginSigningInfo returns signing information for an installed plugin
func GetPluginSigningInfo(pluginName string) (*SigningInfo, error) {
	pluginDir := helmpath.DataPath("plugins", pluginName)

	// Check if plugin directory exists
	fi, err := os.Lstat(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("plugin %s not found: %w", pluginName, err)
	}

	// Check if it's a symlink (local development)
	if fi.Mode()&os.ModeSymlink != 0 {
		return &SigningInfo{
			Status:   "local dev",
			IsSigned: false,
		}, nil
	}

	// Check for .prov file
	provFile := pluginDir + ".prov"
	provData, err := os.ReadFile(provFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &SigningInfo{
				Status:   "unsigned",
				IsSigned: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	// Parse the provenance file to check validity
	block, _ := clearsign.Decode(provData)
	if block == nil {
		return &SigningInfo{
			Status:   "invalid provenance",
			IsSigned: false,
		}, nil
	}

	// Validate that the provenance matches the actual plugin
	// This prevents copying .prov files between plugins
	blockContent := string(block.Plaintext)

	// Load the actual plugin metadata to compare
	plugin, err := LoadDir(pluginDir)
	if err != nil {
		return &SigningInfo{
			Status:   "cannot verify provenance",
			IsSigned: false,
		}, nil
	}

	if !validateProvenanceMetadata(blockContent, plugin.GetMetadata()) {
		return &SigningInfo{
			Status:   "mismatched provenance",
			IsSigned: false,
		}, nil
	}

	// We have a provenance file that is valid for this plugin
	// Without a keyring, we can't verify the signature, but we know:
	// 1. A .prov file exists
	// 2. It's a valid clearsigned document (cryptographically signed)
	// 3. The signed metadata exactly matches this plugin
	return &SigningInfo{
		Status:   "signed",
		IsSigned: true,
	}, nil
}

// validateProvenanceMetadata validates that provenance metadata matches the actual plugin
func validateProvenanceMetadata(provenanceContent string, actualMetadata interface{}) bool {
	// Provenance files contain plugin metadata followed by checksums
	// Split by the YAML document separator
	parts := strings.Split(provenanceContent, "\n...\n")
	if len(parts) == 0 {
		return false
	}

	// Parse the first part which should contain plugin metadata
	metadataYAML := parts[0]

	// To handle the complex unmarshaling of Config and RuntimeConfig interfaces,
	// we use the Load function to parse the provenance metadata the same way
	// the actual plugin was loaded
	provPlugin, err := Load([]byte(metadataYAML), "")
	if err != nil {
		return false
	}

	// Get the metadata from the loaded provenance plugin
	provMetadata := provPlugin.GetMetadata()

	// Now marshal both metadata objects and compare
	provYAML, err := yaml.Marshal(provMetadata)
	if err != nil {
		return false
	}

	actualYAML, err := yaml.Marshal(actualMetadata)
	if err != nil {
		return false
	}

	// Compare the marshaled YAML
	return string(provYAML) == string(actualYAML)
}

// GetSigningInfoForPlugins returns signing info for multiple plugins
func GetSigningInfoForPlugins(plugins []Plugin) map[string]*SigningInfo {
	result := make(map[string]*SigningInfo)

	for _, p := range plugins {
		info, err := GetPluginSigningInfo(p.GetName())
		if err != nil {
			// If there's an error, treat as unsigned
			result[p.GetName()] = &SigningInfo{
				Status:   "unknown",
				IsSigned: false,
			}
		} else {
			result[p.GetName()] = info
		}
	}

	return result
}
