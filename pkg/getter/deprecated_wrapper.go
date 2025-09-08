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

// DEPRECATED: This file provides backward compatibility wrappers.
// Use helm.sh/helm/v4/pkg/transport instead.

package getter

import (
	"helm.sh/helm/v4/pkg/transport"
)

// TransportGetter is DEPRECATED.
// Deprecated: Use transport.Transport instead.
type TransportGetter = transport.Transport

// TransportProviders is DEPRECATED.
// Deprecated: Use transport.Providers instead.
type TransportProviders = transport.Providers

// TransportOption is DEPRECATED.
// Deprecated: Use transport.Option instead.
type TransportOption = transport.Option

// NewHTTPTransportGetter creates a new HTTP getter.
// Deprecated: Use transport.NewHTTPTransport instead.
func NewHTTPTransportGetter(options ...TransportOption) (TransportGetter, error) {
	return transport.NewHTTPTransport(options...)
}

// NewOCITransportGetter creates a new OCI getter.
// Deprecated: Use transport.NewOCITransport instead.
func NewOCITransportGetter(options ...TransportOption) (TransportGetter, error) {
	return transport.NewOCITransport(options...)
}

// AllTransports creates a Providers collection with all built-in getters.
// Deprecated: Use transport package equivalents instead.
func AllTransports() TransportProviders {
	return TransportProviders{
		"http":  mustCreateTransport(transport.NewHTTPTransport),
		"https": mustCreateTransport(transport.NewHTTPTransport),
		"oci":   mustCreateTransport(transport.NewOCITransport),
		"repo":  mustCreateTransport(transport.NewRepoTransport),
	}
}

// mustCreateTransport is a helper that panics on transport creation errors.
// This maintains backward compatibility with the original All() function.
func mustCreateTransport(factory func(...transport.Option) (transport.Transport, error)) transport.Transport {
	t, err := factory()
	if err != nil {
		panic(err)
	}
	return t
}

// Backward compatibility option constructors
var (
	// WithTransportBasicAuth is DEPRECATED.
	// Deprecated: Use transport.WithBasicAuth instead.
	WithTransportBasicAuth = transport.WithBasicAuth

	// WithTransportUserAgent is DEPRECATED.
	// Deprecated: Use transport.WithUserAgent instead.
	WithTransportUserAgent = transport.WithUserAgent

	// WithTransportAcceptHeader is DEPRECATED.
	// Deprecated: Use transport.WithAcceptHeader instead.
	WithTransportAcceptHeader = transport.WithAcceptHeader

	// WithTransportTLS is DEPRECATED.
	// Deprecated: Use transport.WithTLS instead.
	WithTransportTLS = transport.WithTLS

	// WithTransportPlainHTTP is DEPRECATED.
	// Deprecated: Use transport.WithPlainHTTP instead.
	WithTransportPlainHTTP = transport.WithPlainHTTP
)
