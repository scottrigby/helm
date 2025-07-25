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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/openpgp/clearsign" //nolint
	"golang.org/x/crypto/openpgp/packet"    //nolint
	"golang.org/x/mod/sumdb/dirhash"

	"helm.sh/helm/v4/pkg/provenance"
)

// SignPlugin signs a plugin tarball using the directory hash of the source.
//
// This is used when packaging and signing a plugin, where we have both the source
// directory and the resulting tarball. It creates a signature that includes only
// the directory hash, allowing verification of the installed plugin directory later.
func SignPlugin(tarballPath, sourceDir string, signer *provenance.Signatory) (string, error) {
	if signer.Entity == nil {
		return "", errors.New("private key not found")
	}
	if signer.Entity.PrivateKey == nil {
		return "", errors.New("provided key is not a private key")
	}

	// Verify both paths exist
	if _, err := os.Stat(tarballPath); err != nil {
		return "", fmt.Errorf("tarball not found: %w", err)
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return "", fmt.Errorf("source directory not found: %w", err)
	}

	return signPluginTarball(tarballPath, sourceDir, signer)
}

// signPluginTarball signs a plugin tarball
func signPluginTarball(tarballPath, sourceDir string, signer *provenance.Signatory) (string, error) {
	// Create the message block for signing
	messageBlock, err := createPluginMessageBlock(tarballPath, sourceDir)
	if err != nil {
		return "", err
	}

	// Sign the message block directly
	out := bytes.NewBuffer(nil)

	// Use the same PGP config as the provenance package
	pgpConfig := packet.Config{
		DefaultHash: crypto.SHA512,
	}

	// Create the clearsign encoder
	w, err := clearsign.Encode(out, signer.Entity.PrivateKey, &pgpConfig)
	if err != nil {
		return "", err
	}

	// Write our message block
	if _, err := io.Copy(w, messageBlock); err != nil {
		return "", fmt.Errorf("failed to write message block: %w", err)
	}

	// Close to perform the actual signing
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to sign message block: %w", err)
	}

	return out.String(), nil
}

// createPluginMessageBlock creates the content that will be signed
func createPluginMessageBlock(tarballPath, sourceDir string) (*bytes.Buffer, error) {
	if sourceDir == "" {
		return nil, errors.New("source directory is required for signing")
	}

	// Compute directory hash of the source
	hash, err := dirhash.HashDir(sourceDir, "", dirhash.DefaultHash)
	if err != nil {
		return nil, fmt.Errorf("failed to hash directory: %w", err)
	}

	// Create a simple provenance with just the hash
	return bytes.NewBufferString(hash), nil
}

// extractPlugin extracts plugin metadata from a tarball
func extractPlugin(tarballPath string) (Plugin, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Look for plugin.yaml
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Check if this is plugin.yaml (at root or in a subdirectory)
		if filepath.Base(header.Name) == PluginFileName {
			// Read the plugin.yaml content
			data := make([]byte, header.Size)
			if _, err := io.ReadFull(tr, data); err != nil {
				return nil, err
			}

			// Use the Load function to properly parse the plugin
			// We pass an empty dirname since this is from a tarball
			return Load(data, "")
		}
	}

	return nil, errors.New("no plugin.yaml found in tarball")
}

// CreatePluginTarball creates a gzipped tarball from a plugin directory
func CreatePluginTarball(sourceDir string, w io.Writer) error {
	gzw := gzip.NewWriter(w)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Get the base directory name
	baseDir := filepath.Base(sourceDir)

	// Walk the directory tree
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Update the name to be relative to the source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Include the base directory name in the tarball
		header.Name = filepath.Join(baseDir, relPath)

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a regular file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})
}
