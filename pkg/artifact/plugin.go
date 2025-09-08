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

// Plugin-specific artifact downloading capabilities.
package artifact

import (
	"io"
	"net/url"

	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/transport"
)

// PluginDownloader handles downloading plugins with plugin-specific features.
type PluginDownloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// RegistryClient for OCI plugin downloads
	RegistryClient *registry.Client

	// ContentCache is the location where Cache stores its files by default
	ContentCache string

	// Cache specifies the cache implementation to use.
	Cache Cache

	// Internal unified downloader
	downloader *Downloader
}

// DownloadTo downloads a plugin to the specified destination.
func (p *PluginDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	// Initialize the internal downloader if needed
	if p.downloader == nil {
		p.downloader = &Downloader{
			Verify:       p.Verify,
			Keyring:      p.Keyring,
			Transports:   make(transport.Providers), // TODO: populate with plugin-appropriate transports
			Cache:        p.Cache,
			ContentCache: p.ContentCache,
		}
		if p.RegistryClient != nil {
			p.downloader.SetRegistryClient(p.RegistryClient)
		}
	}

	return p.downloader.Download(ref, version, dest, TypePlugin)
}

// DownloadToCache downloads a plugin to the cache.
func (p *PluginDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	return p.DownloadTo(ref, version, p.ContentCache)
}

// ResolvePluginVersion resolves a plugin reference to a URL.
// This delegates to the unified artifact downloader.
func (p *PluginDownloader) ResolvePluginVersion(ref, version string) (string, *url.URL, error) {
	// Initialize the internal downloader if needed
	if p.downloader == nil {
		p.downloader = &Downloader{}
		if p.RegistryClient != nil {
			p.downloader.SetRegistryClient(p.RegistryClient)
		}
	}

	return p.downloader.ResolveArtifactVersion(ref, version, TypePlugin)
}

// NewPluginDownloader creates a new PluginDownloader.
func NewPluginDownloader() *PluginDownloader {
	return &PluginDownloader{}
}
