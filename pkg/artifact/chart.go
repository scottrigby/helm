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

// Chart-specific artifact downloading capabilities.
package artifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"helm.sh/helm/v4/internal/fileutil"
	ifs "helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/transport"
)

// ChartDownloader handles downloading charts with chart-specific features.
type ChartDownloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// Getter collection for the operation
	Getters getter.Providers
	// Options provide parameters to be passed along to the Getter being initialized.
	Options          []getter.Option
	RegistryClient   *registry.Client
	RepositoryConfig string
	RepositoryCache  string

	// ContentCache is the location where Cache stores its files by default
	// In previous versions of Helm the charts were put in the RepositoryCache. The
	// repositories and charts are stored in 2 difference caches.
	ContentCache string

	// Cache specifies the cache implementation to use.
	Cache Cache

	// Internal unified downloader
	downloader *Downloader
}

// DownloadTo retrieves a chart. Depending on the settings, it may also download a provenance file.
func (c *ChartDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	if c.Cache == nil {
		if c.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		c.Cache = &DiskCache{Root: c.ContentCache}
	}

	hash, u, err := c.ResolveChartVersion(ref, version)
	if err != nil {
		return "", nil, err
	}

	// Use the transport bridge to ensure options are properly applied
	transports := c.convertGettersToTransports()
	transport, err := transports.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	// Check the cache for the content. Otherwise download it.
	var data *bytes.Buffer
	var found bool
	var digest []byte
	var digest32 [sha256.Size]byte
	if hash != "" {
		// if there is a hash, populate the other formats
		digest, err = hex.DecodeString(hash)
		if err != nil {
			return "", nil, err
		}
		copy(digest32[:], digest)
		if pth, err := c.Cache.Get(digest32, CacheArtifact); err == nil {
			fdata, err := os.ReadFile(pth)
			if err == nil {
				found = true
				data = bytes.NewBuffer(fdata)
			}
		}
	}

	if !found {
		data, err = transport.Get(u.String())
		if err != nil {
			return "", nil, err
		}
	}

	name := filepath.Base(u.Path)
	if u.Scheme == registry.OCIScheme {
		idx := strings.LastIndexByte(name, ':')
		name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
	}

	destfile := filepath.Join(dest, name)
	if err := fileutil.AtomicWriteFile(destfile, data, 0644); err != nil {
		return destfile, nil, err
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if c.Verify > VerifyNever {
		found = false
		var body *bytes.Buffer
		if hash != "" {
			if pth, err := c.Cache.Get(digest32, CacheProv); err == nil {
				fdata, err := os.ReadFile(pth)
				if err == nil {
					found = true
					body = bytes.NewBuffer(fdata)
				}
			}
		}
		if !found {
			body, err = transport.Get(u.String() + ".prov")
			if err != nil {
				if c.Verify == VerifyAlways {
					return destfile, ver, fmt.Errorf("failed to fetch provenance %q", u.String()+".prov")
				}
				fmt.Fprintf(c.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
				return destfile, ver, nil
			}
		}
		provfile := destfile + ".prov"
		if err := fileutil.AtomicWriteFile(provfile, body, 0644); err != nil {
			return destfile, nil, err
		}

		if c.Verify != VerifyLater {
			ver, err = VerifyChart(destfile, destfile+".prov", c.Keyring)
			if err != nil {
				return destfile, ver, err
			}
		}
	}
	return destfile, ver, nil
}

// DownloadToCache downloads a chart to the cache.
//
// TODO: This method doesn't call DownloadTo directly due to legacy complexity from the original
// chart downloader's intricate cache management patterns that weren't easily unified. Unlike
// PluginDownloader which has the simpler pattern (DownloadToCache calls DownloadTo), this
// implementation maintains separate logic for:
// - Content-addressable cache paths vs user-specified destinations
// - Complex verification involving temporary files for cached content
// - Different performance optimizations for cache hits vs direct downloads
//
// Investigate whether we can simplify and unify these patterns to match PluginDownloader's
// cleaner approach where DownloadToCache(ref, version) calls DownloadTo(ref, version, cache).
func (c *ChartDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	if c.Cache == nil {
		if c.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		c.Cache = &DiskCache{Root: c.ContentCache}
	}

	digestString, u, err := c.ResolveChartVersion(ref, version)
	if err != nil {
		return "", nil, err
	}

	// Use the transport bridge to ensure options are properly applied
	transports := c.convertGettersToTransports()
	transport, err := transports.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	// Check the cache for the file
	digest, err := hex.DecodeString(digestString)
	if err != nil {
		return "", nil, err
	}
	var digest32 [sha256.Size]byte
	copy(digest32[:], digest)

	var pth string
	// only fetch from the cache if we have a digest
	if len(digest) > 0 {
		pth, err = c.Cache.Get(digest32, CacheArtifact)
		if err == nil {
			// Found in cache, check for verification
			ver := &provenance.Verification{}
			if c.Verify > VerifyNever {
				ppth, err := c.Cache.Get(digest32, CacheProv)
				if err == nil && c.Verify != VerifyLater {
					name := filepath.Base(u.Path)
					if u.Scheme == registry.OCIScheme {
						idx := strings.LastIndexByte(name, ':')
						name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
					}
					tmpdir := filepath.Dir(filepath.Join(c.ContentCache, "tmp"))
					if err := os.MkdirAll(tmpdir, 0755); err != nil {
						return pth, ver, err
					}
					tmpfile := filepath.Join(tmpdir, name)
					if err := ifs.CopyFile(pth, tmpfile); err != nil {
						return pth, ver, err
					}
					defer os.RemoveAll(tmpfile)
					ver, err = VerifyChart(tmpfile, ppth, c.Keyring)
					if err != nil {
						return pth, ver, err
					}
				}
			}
			return pth, ver, nil
		}
	}
	if len(digest) == 0 || err != nil {
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}

		// Get file not in the cache
		data, gerr := transport.Get(u.String())
		if gerr != nil {
			return "", nil, gerr
		}

		// Generate the digest
		if len(digest) == 0 {
			digest32 = sha256.Sum256(data.Bytes())
		}

		pth, err = c.Cache.Put(digest32, data, CacheArtifact)
		if err != nil {
			return "", nil, err
		}
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if c.Verify > VerifyNever {
		ppth, err := c.Cache.Get(digest32, CacheProv)
		if err != nil {
			if !os.IsNotExist(err) {
				return pth, ver, err
			}

			body, err := transport.Get(u.String() + ".prov")
			if err != nil {
				if c.Verify == VerifyAlways {
					return pth, ver, fmt.Errorf("failed to fetch provenance %q", u.String()+".prov")
				}
				fmt.Fprintf(c.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
				return pth, ver, nil
			}

			ppth, err = c.Cache.Put(digest32, body, CacheProv)
			if err != nil {
				return "", nil, err
			}
		}

		if c.Verify != VerifyLater {
			name := filepath.Base(u.Path)
			if u.Scheme == registry.OCIScheme {
				idx := strings.LastIndexByte(name, ':')
				name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
			}

			tmpdir := filepath.Dir(filepath.Join(c.ContentCache, "tmp"))
			if err := os.MkdirAll(tmpdir, 0755); err != nil {
				return pth, ver, err
			}
			tmpfile := filepath.Join(tmpdir, name)
			err = ifs.CopyFile(pth, tmpfile)
			if err != nil {
				return pth, ver, err
			}
			defer os.RemoveAll(tmpfile)

			ver, err = VerifyChart(tmpfile, ppth, c.Keyring)
			if err != nil {
				return pth, ver, err
			}
		}
	}
	return pth, ver, nil
}

// ResolveChartVersion resolves a chart reference to a URL.
// This delegates to the unified artifact downloader.
func (c *ChartDownloader) ResolveChartVersion(ref, version string) (string, *url.URL, error) {
	// Initialize the internal downloader if needed
	if c.downloader == nil {
		transports := c.convertGettersToTransports()
		c.downloader = &Downloader{
			Transports: transports,
			Options:    make([]transport.Option, 0), // Initialize options
		}
		c.downloader.SetRepositoryConfig(c.RepositoryConfig, c.RepositoryCache)
		if c.RegistryClient != nil {
			c.downloader.SetRegistryClient(c.RegistryClient)
		}
	}

	hash, url, err := c.downloader.ResolveArtifactVersion(ref, version, TypeChart)

	// For backward compatibility, convert transport options back to getter options
	// so that legacy tests that check c.Options continue to work
	if err == nil && len(c.downloader.Options) > 0 {
		bridge := &getterTransportBridge{}
		convertedOpts := bridge.convertTransportOptsToGetterOpts(c.downloader.Options...)
		c.Options = append(c.Options, convertedOpts...)
	}

	return hash, url, err
}

// NewChartDownloader creates a new ChartDownloader.
func NewChartDownloader() *ChartDownloader {
	return &ChartDownloader{}
}

// VerifyChart takes a path to a chart archive and a keyring, and verifies the chart.
//
// It assumes that a chart archive file is accompanied by a provenance file whose
// name is the archive file name plus the ".prov" extension.
func VerifyChart(path, provfile, keyring string) (*provenance.Verification, error) {
	// For now, error out if it's not a tar file.
	switch fi, err := os.Stat(path); {
	case err != nil:
		return nil, err
	case fi.IsDir():
		return nil, errors.New("unpacked charts cannot be verified")
	case !isTar(path):
		return nil, errors.New("chart must be a tgz file")
	}

	if keyring == "" {
		keyring = defaultKeyring()
	}

	if _, err := os.Stat(provfile); err != nil {
		return nil, fmt.Errorf("could not load provenance file %s: %w", provfile, err)
	}

	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load keyring: %w", err)
	}

	// Read archive and provenance files
	archiveData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read chart archive: %w", err)
	}
	provData, err := os.ReadFile(provfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	return sig.Verify(archiveData, provData, filepath.Base(path))
}

// isTar tests whether the given file is a tar file.
//
// Currently, this simply checks extension, since a subsequent function will
// untar the file and validate its binary format.
func isTar(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".tgz")
}

func defaultKeyring() string {
	return os.ExpandEnv("$PGP_KEYRING")
}

// convertGettersToTransports converts getter.Providers to transport.Providers.
// This is a temporary bridge function until we fully migrate to the transport system.
func (c *ChartDownloader) convertGettersToTransports() transport.Providers {
	transports := make(transport.Providers)

	if c.Getters != nil {
		// For each provider, create a transport bridge for each scheme it supports
		for _, provider := range c.Getters {
			for _, scheme := range provider.Schemes {
				transports[scheme] = &getterTransportBridge{
					provider: provider,
					options:  c.Options,
				}
			}
		}
	}

	return transports
}

// getterTransportBridge is a temporary bridge that wraps a getter provider as a transport.
type getterTransportBridge struct {
	provider getter.Provider
	options  []getter.Option
}

// Get implements the transport.Transport interface by delegating to the getter.
func (gtb *getterTransportBridge) Get(url string, transportOpts ...transport.Option) (*bytes.Buffer, error) {
	// Create a getter instance from the provider using base options plus URL
	allOptions := make([]getter.Option, len(gtb.options))
	copy(allOptions, gtb.options)

	// Add URL option to ensure basic auth works correctly
	allOptions = append(allOptions, getter.WithURL(url))

	g, err := gtb.provider.New(allOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create getter: %w", err)
	}

	// Convert transport options to getter options
	var additionalOpts []getter.Option
	if len(transportOpts) > 0 {
		additionalOpts = gtb.convertTransportOptsToGetterOpts(transportOpts...)
	}

	return g.Get(url, additionalOpts...)
}

// convertTransportOptsToGetterOpts converts transport.Options to equivalent getter.Options
func (gtb *getterTransportBridge) convertTransportOptsToGetterOpts(transportOpts ...transport.Option) []getter.Option {
	// Apply transport options to a temporary Options struct to extract values
	opts := &transport.Options{}
	for _, opt := range transportOpts {
		opt(opts)
	}

	var getterOpts []getter.Option

	// Convert each field to equivalent getter option
	if opts.Username != "" && opts.Password != "" {
		getterOpts = append(getterOpts, getter.WithBasicAuth(opts.Username, opts.Password))
	}
	if opts.UserAgent != "" {
		getterOpts = append(getterOpts, getter.WithUserAgent(opts.UserAgent))
	}
	if opts.AcceptHeader != "" {
		getterOpts = append(getterOpts, getter.WithAcceptHeader(opts.AcceptHeader))
	}
	if opts.CertFile != "" || opts.KeyFile != "" || opts.CAFile != "" {
		getterOpts = append(getterOpts, getter.WithTLSClientConfig(opts.CertFile, opts.KeyFile, opts.CAFile))
	}
	if opts.InsecureSkipTLS {
		getterOpts = append(getterOpts, getter.WithInsecureSkipVerifyTLS(opts.InsecureSkipTLS))
	}
	if opts.PlainHTTP {
		getterOpts = append(getterOpts, getter.WithPlainHTTP(opts.PlainHTTP))
	}
	if opts.PassCredentialsAll {
		getterOpts = append(getterOpts, getter.WithPassCredentialsAll(opts.PassCredentialsAll))
	}
	if opts.URL != "" {
		getterOpts = append(getterOpts, getter.WithURL(opts.URL))
	}
	if opts.Timeout > 0 {
		getterOpts = append(getterOpts, getter.WithTimeout(time.Duration(opts.Timeout)))
	}

	return getterOpts
}
