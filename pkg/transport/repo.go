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

// RepoTransport handles repository-based requests.
type RepoTransport struct {
	repositoryConfig string
	repositoryCache  string
	httpTransport    Transport
}

// NewRepoTransport creates a new repository transport.
func NewRepoTransport(options ...Option) (Transport, error) {
	httpTransport, err := NewHTTPTransport(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP transport for repository: %w", err)
	}

	return &RepoTransport{
		httpTransport: httpTransport,
	}, nil
}

// GetRepositoryConfig returns the repository configuration path.
func (r *RepoTransport) GetRepositoryConfig() string {
	return r.repositoryConfig
}

// SetRepositoryConfig sets the repository configuration path.
func (r *RepoTransport) SetRepositoryConfig(path string) {
	r.repositoryConfig = path
}

// GetRepositoryCache returns the repository cache path.
func (r *RepoTransport) GetRepositoryCache() string {
	return r.repositoryCache
}

// SetRepositoryCache sets the repository cache path.
func (r *RepoTransport) SetRepositoryCache(path string) {
	r.repositoryCache = path
}

// Get fetches data using repository resolution.
func (r *RepoTransport) Get(ref string, options ...Option) (*bytes.Buffer, error) {
	// Use the underlying HTTP transport since repository resolution
	// is handled by the downloader that uses this transport
	return r.httpTransport.Get(ref, options...)
}
