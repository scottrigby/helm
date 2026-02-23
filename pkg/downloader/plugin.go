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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

// PluginDownloader handles downloading chart-defined plugins to the content cache.
type PluginDownloader struct {
	// Out is used to print warnings and notifications.
	Out io.Writer
	// Getters collection for the operation
	Getters getter.Providers
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
		ContentCache: helmpath.CachePath("content"),
	}
}

// DownloadAll downloads all plugins from the given list to the content cache.
func (d *PluginDownloader) DownloadAll(plugins []chart.PluginDependency) error {
	for _, p := range plugins {
		if err := d.Download(p); err != nil {
			return fmt.Errorf("failed to download plugin %q: %w", p.GetName(), err)
		}
	}
	return nil
}

// Download downloads a single plugin to the content-addressed cache.
// Plugin tarballs are stored at $HELM_CACHE_HOME/content/{digest}.plugin
// and loaded directly from the archive at render time (no extraction needed).
func (d *PluginDownloader) Download(p chart.PluginDependency) error {
	// Initialize cache if not set
	if d.Cache == nil {
		if d.ContentCache == "" {
			d.ContentCache = helmpath.CachePath("content")
		}
		d.Cache = &DiskCache{Root: d.ContentCache}
		slog.Debug("set up default plugin downloader cache", "path", d.ContentCache)
	}

	// Check if plugin is already in content cache using digest
	if p.GetDigest() != "" {
		digestBytes, err := hex.DecodeString(p.GetDigest())
		if err == nil && len(digestBytes) == sha256.Size {
			var digest32 [sha256.Size]byte
			copy(digest32[:], digestBytes)
			if _, err := d.Cache.Get(digest32, CachePlugin); err == nil {
				slog.Debug("plugin already in content cache", "name", p.GetName(), "version", p.GetVersion(), "digest", p.GetDigest())
				return nil
			}
		}
	}

	// Download from OCI registry
	fmt.Fprintf(d.Out, "Downloading plugin %s version %s from %s\n", p.GetName(), p.GetVersion(), p.GetRepository())

	g, err := d.getOCIGetter()
	if err != nil {
		return fmt.Errorf("failed to get OCI getter: %w", err)
	}

	// Construct the full OCI reference with version tag
	ociRef := fmt.Sprintf("%s:%s", p.GetRepository(), p.GetVersion())

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
	pluginData := data.Bytes()

	// Compute SHA256 digest and store in content cache
	digest32 := sha256.Sum256(pluginData)
	digestStr := hex.EncodeToString(digest32[:])
	slog.Debug("computed plugin digest", "name", p.GetName(), "version", p.GetVersion(), "digest", digestStr)

	// Store in content cache
	cachePath, err := d.Cache.Put(digest32, bytes.NewReader(pluginData), CachePlugin)
	if err != nil {
		return fmt.Errorf("failed to cache plugin tarball: %w", err)
	}

	slog.Debug("stored plugin in content cache", "name", p.GetName(), "version", p.GetVersion(), "digest", digestStr, "path", cachePath)
	fmt.Fprintf(d.Out, "Plugin %s version %s cached (%s)\n", p.GetName(), p.GetVersion(), digestStr[:12])
	return nil
}

// getOCIGetter returns an OCI getter from the providers.
func (d *PluginDownloader) getOCIGetter() (getter.Getter, error) {
	return d.Getters.ByScheme("oci")
}
