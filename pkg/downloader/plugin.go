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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

// PluginDownloader handles downloading chart-defined plugins to the versioned plugin cache.
type PluginDownloader struct {
	// Out is used to print warnings and notifications.
	Out io.Writer
	// Getters collection for the operation
	Getters getter.Providers
	// PluginsDir is the base plugins directory
	PluginsDir string
	// PlainHTTP enables plain HTTP for OCI registries (for local/insecure registries)
	PlainHTTP bool

	// ContentCache is the location where the content-addressed cache stores plugin tarballs.
	// Defaults to $HELM_CACHE_HOME/content/
	ContentCache string

	// Cache specifies the cache implementation to use for storing plugin tarballs.
	// If nil, a default DiskCache at ContentCache will be used.
	Cache Cache
}

// NewPluginDownloader creates a new PluginDownloader with default settings.
func NewPluginDownloader(out io.Writer, getters getter.Providers) *PluginDownloader {
	return &PluginDownloader{
		Out:          out,
		Getters:      getters,
		PluginsDir:   helmpath.PluginsDir(),
		ContentCache: helmpath.CachePath("content"),
	}
}

// DownloadAll downloads all plugins from the given list.
// Plugins are downloaded to versioned paths: {PluginsDir}/{name}/{version}/
func (d *PluginDownloader) DownloadAll(plugins []*chart.PluginDependency) error {
	for _, p := range plugins {
		if err := d.Download(p); err != nil {
			return fmt.Errorf("failed to download plugin %q: %w", p.Name, err)
		}
	}
	return nil
}

// VersionsSubdir is the subdirectory name for downloaded plugin versions.
// This keeps downloaded versions separate from globally installed plugins.
const VersionsSubdir = "versions"

// Download downloads a single plugin to the versioned plugin cache.
// Plugin tarballs are stored in the content-addressed cache at $HELM_CACHE_HOME/content/
// for efficient storage and deduplication. The extracted plugin is placed at
// $PLUGINS_DIR/versions/<name>/<version>/ for execution.
func (d *PluginDownloader) Download(p *chart.PluginDependency) error {
	// Initialize cache if not set
	if d.Cache == nil {
		if d.ContentCache == "" {
			d.ContentCache = helmpath.CachePath("content")
		}
		d.Cache = &DiskCache{Root: d.ContentCache}
		slog.Debug("set up default plugin downloader cache", "path", d.ContentCache)
	}

	// Compute the versioned plugin path: $PLUGINS_DIR/versions/<name>/<version>/
	// This is separate from installed plugins at $PLUGINS_DIR/<name>/
	pluginPath := filepath.Join(d.PluginsDir, VersionsSubdir, p.Name, p.Version)

	// Check if already extracted and available
	if isPluginDir(pluginPath) {
		slog.Debug("plugin already extracted", "name", p.Name, "version", p.Version, "path", pluginPath)
		return nil
	}

	// Try to get plugin data from content cache using digest if available
	var pluginData []byte
	var digest32 [sha256.Size]byte
	var fromCache bool

	if p.Digest != "" {
		digestBytes, err := hex.DecodeString(p.Digest)
		if err == nil && len(digestBytes) == sha256.Size {
			copy(digest32[:], digestBytes)
			if cachedPath, err := d.Cache.Get(digest32, CachePlugin); err == nil {
				// Found in cache, read the tarball
				pluginData, err = os.ReadFile(cachedPath)
				if err == nil {
					fromCache = true
					slog.Debug("plugin found in content cache", "name", p.Name, "version", p.Version, "digest", p.Digest)
				}
			}
		}
	}

	// If not in cache, download from OCI registry
	if !fromCache {
		fmt.Fprintf(d.Out, "Downloading plugin %s version %s from %s\n", p.Name, p.Version, p.Repository)

		// Get the OCI getter
		g, err := d.getOCIGetter()
		if err != nil {
			return fmt.Errorf("failed to get OCI getter: %w", err)
		}

		// Construct the full OCI reference with version tag
		ociRef := fmt.Sprintf("%s:%s", p.Repository, p.Version)

		// Build getter options
		getterOpts := []getter.Option{
			getter.WithArtifactType("plugin"),
		}
		if d.PlainHTTP {
			getterOpts = append(getterOpts, getter.WithPlainHTTP(true))
		}

		// Download the plugin
		data, err := g.Get(ociRef, getterOpts...)
		if err != nil {
			return fmt.Errorf("failed to download plugin from %s: %w", ociRef, err)
		}
		pluginData = data.Bytes()

		// Compute SHA256 digest and store in content cache
		digest32 = sha256.Sum256(pluginData)
		digestStr := hex.EncodeToString(digest32[:])
		slog.Debug("computed plugin digest", "name", p.Name, "version", p.Version, "digest", digestStr)

		// Store in content cache
		if _, err := d.Cache.Put(digest32, bytes.NewReader(pluginData), CachePlugin); err != nil {
			// Log but don't fail - we can still proceed with extraction
			slog.Warn("failed to cache plugin tarball", "name", p.Name, "error", err)
		} else {
			slog.Debug("stored plugin in content cache", "name", p.Name, "version", p.Version, "digest", digestStr)
		}
	}

	// Create a temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "helm-plugin-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract the plugin tarball
	if err := extractTarGz(bytes.NewReader(pluginData), tmpDir); err != nil {
		return fmt.Errorf("failed to extract plugin: %w", err)
	}

	// Find the plugin directory (may be in root or subdirectory)
	srcDir, err := findPluginDir(tmpDir)
	if err != nil {
		return err
	}

	// Create the versioned plugin directory
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy to final destination
	slog.Debug("installing plugin", "source", srcDir, "destination", pluginPath)
	if err := fs.CopyDir(srcDir, pluginPath); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	fmt.Fprintf(d.Out, "Plugin %s version %s installed to %s\n", p.Name, p.Version, pluginPath)
	return nil
}

// getOCIGetter returns an OCI getter from the providers.
func (d *PluginDownloader) getOCIGetter() (getter.Getter, error) {
	// Use ByScheme to get the correct getter for OCI URLs
	return d.Getters.ByScheme("oci")
}

// isPluginDir checks if a directory contains a valid plugin.
func isPluginDir(dir string) bool {
	pluginFile := filepath.Join(dir, plugin.PluginFileName)
	_, err := os.Stat(pluginFile)
	return err == nil
}

// findPluginDir locates the plugin directory within an extracted archive.
// The plugin.yaml may be at the root or in a subdirectory.
func findPluginDir(dir string) (string, error) {
	// Check root first
	if isPluginDir(dir) {
		return dir, nil
	}

	// Check subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			if isPluginDir(subDir) {
				return subDir, nil
			}
		}
	}

	return "", fmt.Errorf("no plugin.yaml found in downloaded archive")
}

// extractTarGz extracts a gzipped tar archive to the specified directory.
func extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(dest, header.Name)

		// Ensure the target is within the destination directory
		if !isInsideDir(target, dest) {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to copy file contents: %w", err)
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
		}
	}

	return nil
}

// isInsideDir checks if a path is inside a directory.
func isInsideDir(path, dir string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && !hasParentRef(rel)
}

// hasParentRef checks if a path contains parent directory references.
func hasParentRef(path string) bool {
	for _, part := range filepath.SplitList(path) {
		if part == ".." {
			return true
		}
	}
	return false
}
