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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/openpgp"           //nolint
	"golang.org/x/crypto/openpgp/clearsign" //nolint
	"golang.org/x/mod/sumdb/dirhash"

	"helm.sh/helm/v4/pkg/provenance"
)

// VerifyPlugin verifies a plugin (tarball or directory) against a signature.
//
// This function verifies that a plugin has a valid provenance file
// and that the provenance file is signed by a trusted entity.
// It supports both plugin tarballs and installed plugin directories.
func VerifyPlugin(pluginPath, keyring string) (*provenance.Verification, error) {
	// Verify the plugin path exists
	fi, err := os.Stat(pluginPath)
	if err != nil {
		return nil, err
	}

	// Look for provenance file
	provFile := pluginPath + ".prov"
	if _, err := os.Stat(provFile); err != nil {
		return nil, fmt.Errorf("could not find provenance file %s: %w", provFile, err)
	}

	// Create signatory from keyring
	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, err
	}

	// Handle directories and tarballs differently
	if fi.IsDir() {
		return verifyPluginDirectory(pluginPath, provFile, sig)
	}

	// For files, verify it's a tarball
	if !isTarball(pluginPath) {
		return nil, errors.New("plugin file must be a gzipped tarball (.tar.gz or .tgz)")
	}

	return verifyPluginTarball(pluginPath, provFile, sig)
}

// verifySignature verifies the signature in a provenance file
func verifySignature(provPath string, sig *provenance.Signatory) (*clearsign.Block, *provenance.Verification, error) {
	// Read the provenance file
	provData, err := os.ReadFile(provPath)
	if err != nil {
		return nil, nil, err
	}

	// Decode the clearsign block
	block, _ := clearsign.Decode(provData)
	if block == nil {
		return nil, nil, errors.New("provenance file does not contain a valid signature block")
	}

	// Verify the signature
	signer, err := openpgp.CheckDetachedSignature(
		sig.KeyRing,
		bytes.NewBuffer(block.Bytes),
		block.ArmoredSignature.Body,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	// Create verification result
	ver := &provenance.Verification{
		SignedBy: signer,
	}

	return block, ver, nil
}

// getPluginName extracts the plugin name from metadata
func getPluginName(metadata interface{}) string {
	switch m := metadata.(type) {
	case *MetadataV1:
		return m.Name
	case *MetadataLegacy:
		return m.Name
	default:
		return ""
	}
}

// verifyPluginTarball verifies a plugin tarball against its signature
func verifyPluginTarball(pluginPath, provPath string, sig *provenance.Signatory) (*provenance.Verification, error) {
	// For tarballs, we only verify the signature itself
	// The actual content verification happens when verifying the installed directory
	block, ver, err := verifySignature(provPath, sig)
	if err != nil {
		return nil, err
	}

	// The signed content should just be the directory hash
	// We can't verify it against the tarball, but we store it for info
	ver.FileHash = strings.TrimSpace(string(block.Plaintext))
	ver.FileName = filepath.Base(pluginPath)

	return ver, nil
}

// verifyPluginDirectory verifies an installed plugin directory against its signature
func verifyPluginDirectory(pluginPath, provPath string, sig *provenance.Signatory) (*provenance.Verification, error) {
	// Verify the signature and get the signed content
	block, ver, err := verifySignature(provPath, sig)
	if err != nil {
		return nil, err
	}

	// The signed content should just be the hash
	expectedHash := strings.TrimSpace(string(block.Plaintext))

	// Verify directory hash
	actualHash, err := dirhash.HashDir(pluginPath, "", dirhash.DefaultHash)
	if err != nil {
		return ver, fmt.Errorf("failed to hash directory: %w", err)
	}

	if expectedHash != actualHash {
		return ver, fmt.Errorf("directory hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	ver.FileHash = actualHash

	// Load the plugin to get its name
	plugin, err := LoadDir(pluginPath)
	if err != nil {
		// Even if we can't load the plugin, the hash verification passed
		ver.FileName = filepath.Base(pluginPath)
	} else {
		// Set the file name to the plugin name
		m := plugin.Metadata()
		name := getPluginName(m.Name)
		if name != "" {
			ver.FileName = name
		} else {
			ver.FileName = filepath.Base(pluginPath)
		}
	}

	return ver, nil
}

// isTarball checks if a file has a tarball extension
func isTarball(filename string) bool {
	return filepath.Ext(filename) == ".gz" || filepath.Ext(filename) == ".tgz"
}
