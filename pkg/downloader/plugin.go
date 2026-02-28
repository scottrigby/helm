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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"

	"helm.sh/helm/v4/internal/artifacthub"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

// PluginDownloader handles downloading chart-defined plugins to the content cache.
type PluginDownloader struct {
	// Out is used to print warnings and notifications.
	Out io.Writer
	// In is used to read user input for trust prompts.
	// If nil, prompts are skipped and plugins are rejected unless auto-trusted.
	In io.Reader
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

	// ArtifactHubEndpoint is the ArtifactHub API URL for plugin discovery.
	// Defaults to https://artifacthub.io if empty.
	ArtifactHubEndpoint string

	// VerifyPlugins enables plugin verification via ArtifactHub.
	// When enabled, plugins are looked up on ArtifactHub to check signatures
	// and publisher verification status.
	VerifyPlugins bool

	// TrustUnsigned allows downloading unsigned plugins without prompting.
	// Use with caution - this bypasses signature verification.
	TrustUnsigned bool

	// TrustConfig is the trusted publishers configuration.
	// If nil, it will be loaded from disk when needed.
	TrustConfig *TrustConfig

	// AutoApprove skips all trust prompts and allows all plugins.
	// This is intended for CI environments where interactive prompts are not possible.
	AutoApprove bool

	// artifactHubClient is the lazily-initialized ArtifactHub client.
	artifactHubClient *artifacthub.Client
}

// NewPluginDownloader creates a new PluginDownloader with default settings.
func NewPluginDownloader(out io.Writer, getters getter.Providers) *PluginDownloader {
	return &PluginDownloader{
		Out:          out,
		Getters:      getters,
		ContentCache: helmpath.CachePath("content"),
	}
}

// DownloadResult holds the download result for a plugin.
type DownloadResult struct {
	Name    string
	Version string
	Digest  string
}

// DownloadAll downloads all plugins from the given list to the content cache.
// It returns a slice of DownloadResults containing the digest for each plugin.
func (d *PluginDownloader) DownloadAll(plugins []chart.PluginDependency) ([]DownloadResult, error) {
	results := make([]DownloadResult, 0, len(plugins))
	for _, p := range plugins {
		digest, err := d.Download(p)
		if err != nil {
			return nil, fmt.Errorf("failed to download plugin %q: %w", p.GetName(), err)
		}
		results = append(results, DownloadResult{
			Name:    p.GetName(),
			Version: p.GetVersion(),
			Digest:  digest,
		})
	}
	return results, nil
}

// Download downloads a single plugin to the content-addressed cache.
// Plugin tarballs are stored at $HELM_CACHE_HOME/content/{digest}.plugin
// and loaded directly from the archive at render time (no extraction needed).
// Returns the computed SHA256 digest of the plugin tarball.
func (d *PluginDownloader) Download(p chart.PluginDependency) (string, error) {
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
				return p.GetDigest(), nil
			}
		}
	}

	// Verify plugin trust before downloading
	if d.VerifyPlugins {
		trustInfo := d.getPluginTrustInfo(p)
		DisplayPluginSignatureStatus(d.Out, trustInfo)

		// Check if plugin is trusted
		if err := d.checkPluginTrust(trustInfo); err != nil {
			return "", err
		}
	}

	// Download from OCI registry
	fmt.Fprintf(d.Out, "Downloading plugin %s version %s from %s\n", p.GetName(), p.GetVersion(), p.GetRepository())

	g, err := d.getOCIGetter()
	if err != nil {
		return "", fmt.Errorf("failed to get OCI getter: %w", err)
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
		return "", fmt.Errorf("failed to download plugin from %s: %w", ociRef, err)
	}
	pluginData := data.Bytes()

	// Compute SHA256 digest and store in content cache
	digest32 := sha256.Sum256(pluginData)
	digestStr := hex.EncodeToString(digest32[:])
	slog.Debug("computed plugin digest", "name", p.GetName(), "version", p.GetVersion(), "digest", digestStr)

	// Store in content cache
	cachePath, err := d.Cache.Put(digest32, bytes.NewReader(pluginData), CachePlugin)
	if err != nil {
		return "", fmt.Errorf("failed to cache plugin tarball: %w", err)
	}

	slog.Debug("stored plugin in content cache", "name", p.GetName(), "version", p.GetVersion(), "digest", digestStr, "path", cachePath)
	fmt.Fprintf(d.Out, "Plugin %s version %s cached (%s)\n", p.GetName(), p.GetVersion(), digestStr[:12])
	return digestStr, nil
}

// getPluginTrustInfo retrieves trust information for a plugin.
func (d *PluginDownloader) getPluginTrustInfo(p chart.PluginDependency) *PluginTrustInfo {
	info := &PluginTrustInfo{
		Name:                p.GetName(),
		Version:             p.GetVersion(),
		Repository:          p.GetRepository(),
		ArtifactHubRepoName: ParseArtifactHubRepoName(p.GetRepository()),
	}

	// Try to look up plugin on ArtifactHub
	if info.ArtifactHubRepoName != "" {
		pkg, err := d.LookupPluginInfo(info.ArtifactHubRepoName, p.GetName(), p.GetVersion())
		if err == nil && pkg != nil {
			info.Package = pkg
			info.Signed = pkg.Signed
			info.Signatures = pkg.Signatures
			if pkg.SignKey != nil {
				info.Fingerprint = pkg.SignKey.Fingerprint
			}
			if pkg.Repository != nil {
				info.PublisherName = pkg.Repository.DisplayName
				if info.PublisherName == "" {
					info.PublisherName = pkg.Repository.Name
				}
				info.VerifiedPublisher = pkg.Repository.VerifiedPublisher
				info.Official = pkg.Repository.Official
			}
		} else {
			slog.Debug("failed to lookup plugin on ArtifactHub", "plugin", p.GetName(), "error", err)
		}
	}

	// Check if publisher is trusted locally
	if d.TrustConfig != nil && info.ArtifactHubRepoName != "" {
		info.TrustedPublisher = d.TrustConfig.IsTrustedPublisher(info.ArtifactHubRepoName, info.Fingerprint)
	}

	return info
}

// checkPluginTrust checks if a plugin should be trusted and prompts the user if needed.
func (d *PluginDownloader) checkPluginTrust(info *PluginTrustInfo) error {
	// Auto-approve mode - allow everything
	if d.AutoApprove {
		slog.Debug("auto-approving plugin", "plugin", info.Name)
		return nil
	}

	// Trusted publisher - allow without prompt
	if info.TrustedPublisher {
		slog.Debug("plugin from trusted publisher", "plugin", info.Name, "publisher", info.PublisherName)
		return nil
	}

	// Verified publisher with signature - allow without prompt
	if info.VerifiedPublisher && info.Signed {
		slog.Debug("plugin signed by verified publisher", "plugin", info.Name, "publisher", info.PublisherName)
		return nil
	}

	// Unsigned plugin with TrustUnsigned flag - allow
	if !info.Signed && d.TrustUnsigned {
		slog.Debug("allowing unsigned plugin with --trust-unsigned", "plugin", info.Name)
		return nil
	}

	// Interactive mode - prompt for trust
	if d.In != nil {
		decision, err := PromptForTrust(d.Out, d.In, info)
		if err != nil {
			return fmt.Errorf("failed to prompt for trust: %w", err)
		}

		switch decision {
		case TrustDecisionAllow:
			return nil
		case TrustDecisionTrustPublisher:
			// Save to trusted publishers config
			if err := d.addTrustedPublisher(info); err != nil {
				fmt.Fprintf(d.Out, "Warning: failed to save trusted publisher: %v\n", err)
			} else {
				fmt.Fprintf(d.Out, "Added %s to trusted publishers\n", info.PublisherName)
			}
			return nil
		case TrustDecisionDeny:
			return fmt.Errorf("plugin %s v%s not trusted by user", info.Name, info.Version)
		}
	}

	// Non-interactive mode without auto-approve - reject unsigned/unverified plugins
	if !info.Signed {
		return fmt.Errorf("plugin %s v%s is unsigned; use --trust-unsigned to allow", info.Name, info.Version)
	}

	if !info.VerifiedPublisher && !info.TrustedPublisher {
		return fmt.Errorf("plugin %s v%s is from unverified publisher; use interactive mode or add to trusted publishers", info.Name, info.Version)
	}

	return nil
}

// addTrustedPublisher adds a publisher to the trusted publishers config.
func (d *PluginDownloader) addTrustedPublisher(info *PluginTrustInfo) error {
	// Load current config
	config, err := LoadTrustConfig()
	if err != nil {
		return err
	}

	// Add the publisher
	config.AddTrustedPublisher(info.PublisherName, info.ArtifactHubRepoName, info.Fingerprint)

	// Save config
	return SaveTrustConfig(config)
}

// getOCIGetter returns an OCI getter from the providers.
func (d *PluginDownloader) getOCIGetter() (getter.Getter, error) {
	return d.Getters.ByScheme("oci")
}

// getArtifactHubClient returns a lazily-initialized ArtifactHub client.
func (d *PluginDownloader) getArtifactHubClient() *artifacthub.Client {
	if d.artifactHubClient == nil {
		opts := []artifacthub.ClientOption{}
		if d.ArtifactHubEndpoint != "" {
			opts = append(opts, artifacthub.WithBaseURL(d.ArtifactHubEndpoint))
		}
		d.artifactHubClient = artifacthub.NewClient(opts...)
	}
	return d.artifactHubClient
}

// LookupPluginInfo queries ArtifactHub for plugin metadata.
// This can be used for plugin discovery and signing key retrieval.
// repoName is the ArtifactHub repository name (e.g., "ref-hip-chart-defined-plugins").
func (d *PluginDownloader) LookupPluginInfo(repoName, pluginName, version string) (*artifacthub.PluginPackage, error) {
	client := d.getArtifactHubClient()
	ctx := context.Background()

	pkg, err := client.GetPlugin(ctx, repoName, pluginName, version)
	if err != nil {
		slog.Debug("ArtifactHub lookup failed", "repo", repoName, "plugin", pluginName, "version", version, "error", err)
		return nil, err
	}

	slog.Debug("ArtifactHub lookup succeeded", "repo", repoName, "plugin", pluginName, "version", version, "signed", pkg.Signed)
	return pkg, nil
}

// GetSigningKey retrieves the signing key for a plugin from ArtifactHub.
// Returns the signing key metadata, or an error if the plugin is not signed or not found.
func (d *PluginDownloader) GetSigningKey(repoName, pluginName, version string) (*artifacthub.SignKey, error) {
	client := d.getArtifactHubClient()
	ctx := context.Background()

	signKey, err := client.GetSigningKey(ctx, repoName, pluginName, version)
	if err != nil {
		return nil, err
	}

	return signKey, nil
}
