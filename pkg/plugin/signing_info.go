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

	"golang.org/x/mod/sumdb/dirhash"

	"golang.org/x/crypto/openpgp/clearsign" //nolint

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

	if !validateProvenanceHash(blockContent, pluginDir) {
		return &SigningInfo{
			Status:   "mismatched provenance",
			IsSigned: false,
		}, nil
	}

	// We have a provenance file that is valid for this plugin
	// Without a keyring, we can't verify the signature, but we know:
	// 1. A .prov file exists
	// 2. It's a valid clearsigned document (cryptographically signed)
	// 3. The hash in .prov exactly matches the plugin directory hash
	return &SigningInfo{
		Status:   "signed",
		IsSigned: true,
	}, nil
}

func validateProvenanceHash(blockContent, pluginDir string) bool {
	// Verify the directory hash is correct
	expectedHash, _ := dirhash.HashDir(pluginDir, "", dirhash.DefaultHash)

	// Extract the hash from the signed message
	// The hash should be the only content between the headers
	return strings.Contains(blockContent, expectedHash)
}

// GetSigningInfoForPlugins returns signing info for multiple plugins
func GetSigningInfoForPlugins(plugins []Plugin) map[string]*SigningInfo {
	result := make(map[string]*SigningInfo)

	for _, p := range plugins {
		m := p.Metadata()

		info, err := GetPluginSigningInfo(m.Name)
		if err != nil {
			// If there's an error, treat as unsigned
			result[m.Name] = &SigningInfo{
				Status:   "unknown",
				IsSigned: false,
			}
		} else {
			result[m.Name] = info
		}
	}

	return result
}
