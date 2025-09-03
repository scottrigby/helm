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

package transport

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"helm.sh/helm/v4/internal/tlsutil"
)

// HTTPTransport handles HTTP and HTTPS requests.
type HTTPTransport struct {
	client *http.Client
}

// NewHTTPTransport creates a new HTTP transport.
func NewHTTPTransport(options ...Option) (Transport, error) {
	opts := &Options{}
	for _, opt := range options {
		opt(opts)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	// Configure TLS if specified
	if opts.CertFile != "" || opts.KeyFile != "" || opts.CAFile != "" || opts.InsecureSkipTLS {
		tlsConfig, err := tlsutil.NewTLSConfig(
			tlsutil.WithInsecureSkipVerify(opts.InsecureSkipTLS),
			tlsutil.WithCertKeyPairFiles(opts.CertFile, opts.KeyFile),
			tlsutil.WithCAFile(opts.CAFile),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		transport.TLSClientConfig = tlsConfig
	}

	timeout := 30 * time.Second
	if opts.Timeout > 0 {
		timeout = time.Duration(opts.Timeout) * time.Second
	}

	return &HTTPTransport{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}, nil
}

// Get fetches data from an HTTP URL.
func (h *HTTPTransport) Get(url string, options ...Option) (*bytes.Buffer, error) {
	opts := &Options{}
	for _, opt := range options {
		opt(opts)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}
	if opts.AcceptHeader != "" {
		req.Header.Set("Accept", opts.AcceptHeader)
	}

	// Set authentication
	if opts.Username != "" && opts.Password != "" {
		req.SetBasicAuth(opts.Username, opts.Password)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(data), nil
}
