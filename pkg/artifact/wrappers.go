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

package artifact

import (
	"errors"

	"helm.sh/helm/v4/pkg/provenance"
)

// ChartDownloader provides backward-compatible chart downloading functionality.
type ChartDownloader struct {
	*Downloader
}

// NewChartDownloader creates a new ChartDownloader.
func NewChartDownloader() *ChartDownloader {
	return &ChartDownloader{
		Downloader: &Downloader{},
	}
}

// DownloadTo downloads a chart to the specified destination.
// This method maintains backward compatibility with the original ChartDownloader API.
func (c *ChartDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	return c.Download(ref, version, dest, TypeChart)
}

// DownloadToCache downloads a chart to the cache.
func (c *ChartDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	if c.ContentCache == "" {
		return "", nil, errors.New("content cache must be set")
	}
	return c.Download(ref, version, c.ContentCache, TypeChart)
}

// PluginDownloader provides plugin downloading functionality.
type PluginDownloader struct {
	*Downloader
}

// NewPluginDownloader creates a new PluginDownloader.
func NewPluginDownloader() *PluginDownloader {
	return &PluginDownloader{
		Downloader: &Downloader{},
	}
}

// DownloadTo downloads a plugin to the specified destination.
func (p *PluginDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	return p.Download(ref, version, dest, TypePlugin)
}

// DownloadToCache downloads a plugin to the cache.
func (p *PluginDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	if p.ContentCache == "" {
		return "", nil, errors.New("content cache must be set")
	}
	return p.Download(ref, version, p.ContentCache, TypePlugin)
}
