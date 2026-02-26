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

package artifacthub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	// DefaultBaseURL is the production ArtifactHub API endpoint.
	DefaultBaseURL = "https://artifacthub.io"

	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second
)

// Common errors returned by the client.
var (
	ErrPluginNotFound = errors.New("plugin not found on ArtifactHub")
	ErrNoSigningKey   = errors.New("plugin has no signing key configured")
	ErrNotSigned      = errors.New("plugin is not signed")
)

// Client provides access to the ArtifactHub API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient creates a new ArtifactHub client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetPlugin retrieves a specific plugin package by repository name, plugin name, and version.
func (c *Client) GetPlugin(ctx context.Context, repoName, pluginName, version string) (*PluginPackage, error) {
	path := fmt.Sprintf("/api/v1/packages/helm-plugin/%s/%s/%s",
		url.PathEscape(repoName),
		url.PathEscape(pluginName),
		url.PathEscape(version))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPluginNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var pkg PluginPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &pkg, nil
}

// GetPluginLatest retrieves the latest version of a plugin.
func (c *Client) GetPluginLatest(ctx context.Context, repoName, pluginName string) (*PluginPackage, error) {
	path := fmt.Sprintf("/api/v1/packages/helm-plugin/%s/%s",
		url.PathEscape(repoName),
		url.PathEscape(pluginName))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPluginNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var pkg PluginPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &pkg, nil
}

// GetSigningKey retrieves the signing key for a specific plugin version.
// Returns ErrPluginNotFound if the plugin doesn't exist on ArtifactHub.
// Returns ErrNoSigningKey if the plugin exists but has no signing key.
// Returns ErrNotSigned if the plugin is not signed.
func (c *Client) GetSigningKey(ctx context.Context, repoName, pluginName, version string) (*SignKey, error) {
	pkg, err := c.GetPlugin(ctx, repoName, pluginName, version)
	if err != nil {
		return nil, err
	}

	if !pkg.Signed {
		return nil, ErrNotSigned
	}

	if pkg.SignKey == nil {
		return nil, ErrNoSigningKey
	}

	return pkg.SignKey, nil
}

// SearchPluginsOptions configures plugin search.
type SearchPluginsOptions struct {
	Query             string
	VerifiedPublisher bool
	Official          bool
	CNCF              bool
	Limit             int
	Offset            int
}

// SearchPlugins searches for Helm plugins matching the given criteria.
func (c *Client) SearchPlugins(ctx context.Context, opts SearchPluginsOptions) (*SearchResult, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/packages/search")
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	q := u.Query()
	q.Set("kind", strconv.Itoa(int(KindHelmPlugin)))
	if opts.Query != "" {
		q.Set("ts_query_web", opts.Query)
	}
	if opts.VerifiedPublisher {
		q.Set("verified_publisher", "true")
	}
	if opts.Official {
		q.Set("official", "true")
	}
	if opts.CNCF {
		q.Set("cncf", "true")
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", strconv.Itoa(opts.Offset))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp struct {
		Packages []PluginPackage `json:"packages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	result := &SearchResult{
		Packages: searchResp.Packages,
	}

	// Parse total count from header
	if tc := resp.Header.Get("Pagination-Total-Count"); tc != "" {
		if count, err := strconv.Atoi(tc); err == nil {
			result.TotalCount = count
		}
	}

	return result, nil
}

// FetchSigningKey downloads the signing key from the given URL.
func (c *Client) FetchSigningKey(ctx context.Context, keyURL string) ([]byte, error) {
	// Validate URL scheme
	u, err := url.Parse(keyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid key URL: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("signing key URL must use HTTPS, got: %s", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching signing key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch signing key: status %d", resp.StatusCode)
	}

	// Limit key size to 1MB
	const maxKeySize = 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxKeySize+1)
	keyData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading signing key: %w", err)
	}
	if len(keyData) > maxKeySize {
		return nil, fmt.Errorf("signing key exceeds maximum size of %d bytes", maxKeySize)
	}

	return keyData, nil
}
