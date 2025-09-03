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
)

// Transport defines the interface for fetching artifacts from various protocols.
type Transport interface {
	Get(url string, options ...Option) (*bytes.Buffer, error)
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

// Providers is a collection of Transport implementations indexed by scheme.
type Providers map[string]Transport

// ByScheme returns the Transport that handles the given scheme.
func (p Providers) ByScheme(scheme string) (Transport, error) {
	transport, ok := p[scheme]
	if !ok {
		return nil, fmt.Errorf("no transport registered for scheme %q", scheme)
	}
	return transport, nil
}

// Option is a functional option for configuring Transport behavior.
type Option func(*Options)

// Options contains configuration options for Transport operations.
type Options struct {
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

// Common option constructors
func WithBasicAuth(username, password string) Option {
	return func(opts *Options) {
		opts.Username = username
		opts.Password = password
	}
}

func WithUserAgent(agent string) Option {
	return func(opts *Options) {
		opts.UserAgent = agent
	}
}

func WithAcceptHeader(header string) Option {
	return func(opts *Options) {
		opts.AcceptHeader = header
	}
}

func WithTLS(certFile, keyFile, caFile string, insecureSkipTLS bool) Option {
	return func(opts *Options) {
		opts.CertFile = certFile
		opts.KeyFile = keyFile
		opts.CAFile = caFile
		opts.InsecureSkipTLS = insecureSkipTLS
	}
}

func WithPlainHTTP() Option {
	return func(opts *Options) {
		opts.PlainHTTP = true
	}
}

func WithArtifactType(artifactType string) Option {
	return func(opts *Options) {
		opts.ArtifactType = artifactType
	}
}

func WithVersion(version string) Option {
	return func(opts *Options) {
		opts.Version = version
	}
}

func WithURL(url string) Option {
	return func(opts *Options) {
		opts.URL = url
	}
}

func WithCustomOption(key string, value interface{}) Option {
	return func(opts *Options) {
		if opts.Custom == nil {
			opts.Custom = make(map[string]interface{})
		}
		opts.Custom[key] = value
	}
}
