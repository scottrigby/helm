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

package getter

import (
	"bytes"
	"fmt"
	"time"
)

// Transport defines the interface for fetching artifacts from various protocols.
// This unifies the getter and transport abstractions for better architecture.
type Transport interface {
	Get(url string, options ...TransportOption) (*bytes.Buffer, error)
}

// ClientProvider is implemented by transports that need custom clients.
type ClientProvider interface {
	GetClient() interface{}
	SetClient(client interface{})
}

// RepositoryProvider is implemented by transports that use repository indexes.
type RepositoryProvider interface {
	GetRepositoryConfig() string
	SetRepositoryConfig(path string)
	GetRepositoryCache() string
	SetRepositoryCache(path string)
}

// CacheProvider is implemented by transports that need custom caching behavior.
type CacheProvider interface {
	GetCacheConfig() CacheConfig
	SetCacheConfig(config CacheConfig)
}

// CacheConfig represents transport-specific caching configuration.
type CacheConfig struct {
	Enabled   bool
	Directory string
	MaxSize   int64
	TTL       int64
}

// TransportProviders is a collection of Transport implementations indexed by scheme.
type TransportProviders map[string]Transport

// ByScheme returns the Transport that handles the given scheme.
func (p TransportProviders) ByScheme(scheme string) (Transport, error) {
	transport, ok := p[scheme]
	if !ok {
		return nil, fmt.Errorf("no transport registered for scheme %q", scheme)
	}
	return transport, nil
}

// TransportOption is a functional option for configuring Transport behavior.
type TransportOption func(*TransportOptions)

// TransportOptions contains configuration options for Transport operations.
type TransportOptions struct {
	// Common options that all transports might use
	Username           string
	Password           string
	UserAgent          string
	AcceptHeader       string
	Timeout            int64
	InsecureSkipTLS    bool
	CertFile           string
	KeyFile            string
	CAFile             string
	PlainHTTP          bool
	PassCredentialsAll bool
	ArtifactType       string
	Version            string
	URL                string

	// Protocol-specific options can be added via custom option functions
	Custom map[string]interface{}
}

// Common transport option constructors
func WithTransportBasicAuth(username, password string) TransportOption {
	return func(opts *TransportOptions) {
		opts.Username = username
		opts.Password = password
	}
}

func WithTransportUserAgent(agent string) TransportOption {
	return func(opts *TransportOptions) {
		opts.UserAgent = agent
	}
}

func WithTransportAcceptHeader(header string) TransportOption {
	return func(opts *TransportOptions) {
		opts.AcceptHeader = header
	}
}

func WithTransportTLS(certFile, keyFile, caFile string, insecureSkipTLS bool) TransportOption {
	return func(opts *TransportOptions) {
		opts.CertFile = certFile
		opts.KeyFile = keyFile
		opts.CAFile = caFile
		opts.InsecureSkipTLS = insecureSkipTLS
	}
}

func WithTransportPlainHTTP() TransportOption {
	return func(opts *TransportOptions) {
		opts.PlainHTTP = true
	}
}

func WithTransportPassCredentialsAll(passCredentialsAll bool) TransportOption {
	return func(opts *TransportOptions) {
		opts.PassCredentialsAll = passCredentialsAll
	}
}

func WithTransportArtifactType(artifactType string) TransportOption {
	return func(opts *TransportOptions) {
		opts.ArtifactType = artifactType
	}
}

func WithTransportVersion(version string) TransportOption {
	return func(opts *TransportOptions) {
		opts.Version = version
	}
}

func WithTransportURL(url string) TransportOption {
	return func(opts *TransportOptions) {
		opts.URL = url
	}
}

func WithTransportTimeout(timeout time.Duration) TransportOption {
	return func(opts *TransportOptions) {
		opts.Timeout = int64(timeout)
	}
}

func WithTransportCustomOption(key string, value interface{}) TransportOption {
	return func(opts *TransportOptions) {
		if opts.Custom == nil {
			opts.Custom = make(map[string]interface{})
		}
		opts.Custom[key] = value
	}
}

// ConvertTransportOptionsToGetterOptions converts TransportOptions to equivalent getter.Options
func ConvertTransportOptionsToGetterOptions(transportOpts ...TransportOption) []Option {
	// Apply transport options to a temporary TransportOptions struct to extract values
	opts := &TransportOptions{}
	for _, opt := range transportOpts {
		opt(opts)
	}

	var getterOpts []Option

	// Convert each field to equivalent getter option
	if opts.Username != "" && opts.Password != "" {
		getterOpts = append(getterOpts, WithBasicAuth(opts.Username, opts.Password))
	}
	if opts.UserAgent != "" {
		getterOpts = append(getterOpts, WithUserAgent(opts.UserAgent))
	}
	if opts.AcceptHeader != "" {
		getterOpts = append(getterOpts, WithAcceptHeader(opts.AcceptHeader))
	}
	if opts.CertFile != "" || opts.KeyFile != "" || opts.CAFile != "" {
		getterOpts = append(getterOpts, WithTLSClientConfig(opts.CertFile, opts.KeyFile, opts.CAFile))
	}
	if opts.InsecureSkipTLS {
		getterOpts = append(getterOpts, WithInsecureSkipVerifyTLS(opts.InsecureSkipTLS))
	}
	if opts.PlainHTTP {
		getterOpts = append(getterOpts, WithPlainHTTP(opts.PlainHTTP))
	}
	if opts.PassCredentialsAll {
		getterOpts = append(getterOpts, WithPassCredentialsAll(opts.PassCredentialsAll))
	}
	if opts.URL != "" {
		getterOpts = append(getterOpts, WithURL(opts.URL))
	}
	if opts.Timeout > 0 {
		getterOpts = append(getterOpts, WithTimeout(time.Duration(opts.Timeout)))
	}

	return getterOpts
}