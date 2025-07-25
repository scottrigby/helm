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

	"helm.sh/helm/v4/pkg/provenance"
)

const testKeyFile = "../cmd/testdata/helm-test-key.secret"
const testPubFile = "../cmd/testdata/helm-test-key.pub"

func TestVerifyPlugin(t *testing.T) {
	// Create a test plugin and sign it
	tempDir := t.TempDir()

	// Create plugin directory
	pluginDir := filepath.Join(tempDir, "verify-test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	pluginYAML := `name: verify-test-plugin
version: 1.0.0
usage: "Test plugin for verification"
description: "A test plugin"
command: "$HELM_PLUGIN_DIR/run.sh"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tarball
	tarballPath := filepath.Join(tempDir, "verify-test-plugin.tar.gz")
	tarFile, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := CreatePluginTarball(pluginDir, tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Sign the plugin with source directory
	signer, err := provenance.NewFromKeyring(testKeyFile, "helm-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.DecryptKey(func(s string) ([]byte, error) {
		return []byte(""), nil
	}); err != nil {
		t.Fatal(err)
	}

	sig, err := SignPlugin(tarballPath, pluginDir, signer)
	if err != nil {
		t.Fatal(err)
	}

	// Write the signature to .prov file
	provFile := tarballPath + ".prov"
	if err := os.WriteFile(provFile, []byte(sig), 0644); err != nil {
		t.Fatal(err)
	}

	// Now verify the plugin
	verification, err := VerifyPlugin(tarballPath, testPubFile)
	if err != nil {
		t.Fatalf("Failed to verify plugin: %v", err)
	}

	// Check verification results
	if verification.SignedBy == nil {
		t.Error("SignedBy is nil")
	}

	if verification.FileName != "verify-test-plugin.tar.gz" {
		t.Errorf("Expected filename 'verify-test-plugin.tar.gz', got %s", verification.FileName)
	}

	if verification.FileHash == "" {
		t.Error("FileHash is empty")
	}
}

func TestVerifyPluginBadSignature(t *testing.T) {
	tempDir := t.TempDir()

	// Create a plugin tarball
	pluginDir := filepath.Join(tempDir, "bad-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	pluginYAML := `name: bad-plugin
version: 1.0.0
usage: "Bad plugin"
description: "A plugin with bad signature"
command: "echo"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	tarballPath := filepath.Join(tempDir, "bad-plugin.tar.gz")
	tarFile, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := CreatePluginTarball(pluginDir, tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Create a bad signature (just some text)
	badSig := `-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

This is not a real signature
-----BEGIN PGP SIGNATURE-----

InvalidSignatureData

-----END PGP SIGNATURE-----`

	provFile := tarballPath + ".prov"
	if err := os.WriteFile(provFile, []byte(badSig), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to verify - should fail
	_, err = VerifyPlugin(tarballPath, testPubFile)
	if err == nil {
		t.Error("Expected verification to fail with bad signature")
	}
}

func TestVerifyPluginMissingProvenance(t *testing.T) {
	tempDir := t.TempDir()
	tarballPath := filepath.Join(tempDir, "no-prov.tar.gz")

	// Create a minimal tarball
	if err := os.WriteFile(tarballPath, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to verify without .prov file
	_, err := VerifyPlugin(tarballPath, testPubFile)
	if err == nil {
		t.Error("Expected verification to fail without provenance file")
	}
}

func TestVerifyPluginDirectory(t *testing.T) {
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

	// Create a tarball first (since we can only sign tarballs)
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
	signer, err := provenance.NewFromKeyring(testKeyFile, "helm-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.DecryptKey(func(s string) ([]byte, error) {
		return []byte(""), nil
	}); err != nil {
		t.Fatal(err)
	}

	// Sign the tarball with source directory
	sig, err := SignPlugin(tarballPath, pluginDir, signer)
	if err != nil {
		t.Fatalf("failed to sign plugin: %v", err)
	}

	// Write the provenance file next to the plugin directory
	provFile := pluginDir + ".prov"
	if err := os.WriteFile(provFile, []byte(sig), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify the plugin directory
	ver, err := VerifyPlugin(pluginDir, testPubFile)
	if err != nil {
		t.Fatalf("failed to verify plugin directory: %v", err)
	}

	// Check verification results
	if ver.SignedBy == nil {
		t.Error("verification result missing SignedBy")
	}
	if ver.FileHash == "" {
		t.Error("verification result missing FileHash")
	}
	if ver.FileName != "test-plugin" {
		t.Errorf("unexpected FileName: got %s, want test-plugin", ver.FileName)
	}

	// Modify the plugin directory and verify it fails
	if err := os.WriteFile(filepath.Join(pluginDir, "newfile.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = VerifyPlugin(pluginDir, testPubFile)
	if err == nil {
		t.Error("expected verification to fail after modifying directory")
	}
	if err != nil && !strings.Contains(err.Error(), "hash mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}
