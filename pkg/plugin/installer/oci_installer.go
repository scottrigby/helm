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

package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"

	"helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/plugin/cache"
	"helm.sh/helm/v4/pkg/registry"
)

// OCIInstaller installs plugins from OCI registries
type OCIInstaller struct {
	CacheDir   string
	PluginName string
	base
	repository *remote.Repository
	settings   *cli.EnvSettings
}

// NewOCIInstaller creates a new OCIInstaller
func NewOCIInstaller(source string) (*OCIInstaller, error) {
	ref := strings.TrimPrefix(source, fmt.Sprintf("%s://", registry.OCIScheme))

	// Extract plugin name from OCI reference
	// e.g., "ghcr.io/user/plugin-name:v1.0.0" -> "plugin-name"
	parts := strings.Split(ref, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid OCI reference: %s", source)
	}
	lastPart := parts[len(parts)-1]
	pluginName := lastPart
	if idx := strings.LastIndex(lastPart, ":"); idx > 0 {
		pluginName = lastPart[:idx]
	}
	if idx := strings.LastIndex(lastPart, "@"); idx > 0 {
		pluginName = lastPart[:idx]
	}

	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	settings := cli.New()

	i := &OCIInstaller{
		CacheDir:   helmpath.CachePath("plugins", key),
		PluginName: pluginName,
		base:       newBase(source),
		settings:   settings,
	}
	return i, nil
}

// Install downloads and installs a plugin from OCI registry
// Implements Installer.
func (i *OCIInstaller) Install() error {
	ref := strings.TrimPrefix(i.Source, fmt.Sprintf("%s://", registry.OCIScheme))

	// Pull the OCI artifact
	slog.Debug("pulling OCI plugin", "ref", ref)

	// Create memory store for the pull operation
	memoryStore := memory.New()

	// Create repository
	var repository *remote.Repository
	if i.repository == nil {
		repository, err := remote.NewRepository(ref)
		if err != nil {
			return err
		}

		// Configure authentication using Docker config
		dockerStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
		if err != nil {
			// If docker config is not available, continue without auth
			slog.Debug("unable to load docker config", "error", err)
		} else {
			// Create auth client with docker credentials
			authClient := &auth.Client{
				Credential: credentials.Credential(dockerStore),
			}
			repository.Client = authClient
		}

		// Set PlainHTTP to false for secure registries
		repository.PlainHTTP = false
	} else {
		repository = i.repository
	}

	ctx := context.Background()

	// Copy the artifact from registry to memory store
	manifest, err := oras.Copy(ctx, repository, ref, memoryStore, "", oras.CopyOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull plugin from %s: %w", ref, err)
	}

	// Fetch the manifest
	manifestData, err := content.FetchAll(ctx, memoryStore, manifest)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	// Parse manifest to get layers
	var imageManifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &imageManifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Create cache directory
	if err := os.MkdirAll(i.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Extract each layer to the cache directory
	// Only support compressed tar archives to preserve file permissions
	for _, layer := range imageManifest.Layers {
		layerData, err := content.FetchAll(ctx, memoryStore, layer)
		if err != nil {
			return fmt.Errorf("failed to fetch layer %s: %w", layer.Digest, err)
		}

		// Check if this is a gzip compressed file
		if len(layerData) < 2 || layerData[0] != 0x1f || layerData[1] != 0x8b {
			return fmt.Errorf("layer %s is not a gzip compressed archive", layer.Digest)
		}

		// Extract as gzipped tar
		if err := extractTarGz(bytes.NewReader(layerData), i.CacheDir); err != nil {
			return fmt.Errorf("failed to extract layer %s: %w", layer.Digest, err)
		}
	}

	// Verify plugin.yaml exists - check root and subdirectories
	pluginDir := i.CacheDir
	if !isPlugin(pluginDir) {
		// Check if plugin.yaml is in a subdirectory
		entries, err := os.ReadDir(i.CacheDir)
		if err != nil {
			return err
		}

		foundPluginDir := ""
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(i.CacheDir, entry.Name())
				if isPlugin(subDir) {
					foundPluginDir = subDir
					break
				}
			}
		}

		if foundPluginDir == "" {
			return ErrMissingMetadata
		}

		// Use the subdirectory as the plugin directory
		pluginDir = foundPluginDir
	}

	// Copy from cache to final destination
	src, err := filepath.Abs(pluginDir)
	if err != nil {
		return err
	}

	slog.Debug("copying", "source", src, "path", i.Path())
	return fs.CopyDir(src, i.Path())
}

// Update updates a plugin by reinstalling it
func (i *OCIInstaller) Update() error {
	// For OCI, update means removing the old version and installing the new one
	if err := os.RemoveAll(i.Path()); err != nil {
		return err
	}
	return i.Install()
}

// Path is where the plugin will be installed
func (i OCIInstaller) Path() string {
	if i.Source == "" {
		return ""
	}
	return helmpath.DataPath("plugins", i.PluginName)
}

// extractTarGz extracts a gzipped tar archive to a directory
func extractTarGz(r io.Reader, targetDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return extractTar(gzr, targetDir)
}

// extractTar extracts a tar archive to a directory
func extractTar(r io.Reader, targetDir string) error {
	tarReader := tar.NewReader(r)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path, err := cleanJoin(targetDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			// Skip these
			continue
		default:
			return fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
