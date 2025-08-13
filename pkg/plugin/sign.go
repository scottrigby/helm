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

// SignPlugin signs a plugin using the directory hash of the source.
//
// This is used when packaging and signing a plugin from a source directory
// It creates a signature that includes only the directory hash, allowing
// verification of the installed plugin directory later.
func SignPlugin(sourceDir string, signer *provenance.Signatory) (string, error) {
	if signer.Entity == nil {
		return "", errors.New("private key not found")
	}
	if signer.Entity.PrivateKey == nil {
		return "", errors.New("provided key is not a private key")
	}

	// Verify path exists
	if _, err := os.Stat(sourceDir); err != nil {
		return "", fmt.Errorf("source directory not found: %w", err)
	}

	// Create the message block for signing
	messageBlock, err := createPluginMessageBlock(sourceDir)
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
func createPluginMessageBlock(sourceDir string) (*bytes.Buffer, error) {
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
