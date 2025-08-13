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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"

	"helm.sh/helm/v4/pkg/provenance"
)

func TestSignPlugin(t *testing.T) {
	// Create a test plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plugin.yaml file
	pluginYAML := `apiVersion: v1
name: test-plugin
version: 1.0.0
description: A test plugin
command: echo`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a tarball
	tarballPath := filepath.Join(tempDir, "test-plugin.tgz")
	tarFile, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreatePluginTarball(pluginDir, tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Create a test key for signing
	keyring := "../cmd/testdata/helm-test-key.secret"
	signer, err := provenance.NewFromKeyring(keyring, "helm-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.DecryptKey(func(s string) ([]byte, error) {
		return []byte(""), nil
	}); err != nil {
		t.Fatal(err)
	}

	// Sign the plugin with source directory
	sig, err := SignPlugin(pluginDir, signer)
	if err != nil {
		t.Fatalf("failed to sign plugin: %v", err)
	}

	// Verify the signature contains the expected content
	if !strings.Contains(sig, "-----BEGIN PGP SIGNED MESSAGE-----") {
		t.Error("signature does not contain PGP header")
	}

	// Verify the directory hash is correct
	expectedHash, err := dirhash.HashDir(pluginDir, "", dirhash.DefaultHash)
	if err != nil {
		t.Fatal(err)
	}
	// Extract the hash from the signed message
	// The hash should be the only content between the headers
	if !strings.Contains(sig, expectedHash) {
		t.Errorf("signature does not contain expected directory hash: %s", expectedHash)
	}
}
