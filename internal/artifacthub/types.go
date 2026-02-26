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

// Package artifacthub provides a client for interacting with ArtifactHub API
// to retrieve plugin metadata and signing keys for verification.
package artifacthub

// RepositoryKind represents the type of repository in ArtifactHub.
type RepositoryKind int

const (
	// KindHelm represents Helm chart repositories.
	KindHelm RepositoryKind = 0
	// KindHelmPlugin represents Helm plugin repositories (v3 and v4).
	KindHelmPlugin RepositoryKind = 6
)

// SignKey represents a signing key for package verification.
type SignKey struct {
	// Fingerprint is the GPG key fingerprint (40 hex characters).
	Fingerprint string `json:"fingerprint"`
	// URL is the location where the public key can be downloaded.
	URL string `json:"url"`
}

// Repository represents an ArtifactHub repository.
type Repository struct {
	RepositoryID      string         `json:"repository_id"`
	Kind              RepositoryKind `json:"kind"`
	Name              string         `json:"name"`
	DisplayName       string         `json:"display_name,omitempty"`
	URL               string         `json:"url"`
	VerifiedPublisher bool           `json:"verified_publisher"`
	Official          bool           `json:"official"`
	CNCF              bool           `json:"cncf,omitempty"`
}

// Maintainer represents a package maintainer.
type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Link represents a URL associated with a package.
type Link struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// PluginData contains Helm plugin-specific metadata.
type PluginData struct {
	// PluginType is the plugin API type (e.g., "render/v1", "getter/v1").
	PluginType string `json:"plugin_type"`

	// Runtime is the plugin execution runtime ("wasm" or "native").
	Runtime string `json:"runtime"`

	// HelmVersionConstraint specifies compatible Helm versions.
	HelmVersionConstraint string `json:"helm_version_constraint,omitempty"`

	// Platforms lists supported OS/arch combinations.
	// For wasm plugins, this is typically ["any"].
	Platforms []string `json:"platforms,omitempty"`

	// FilePatterns lists file patterns this plugin handles (for render/v1).
	FilePatterns []string `json:"file_patterns,omitempty"`
}

// PluginPackage represents a Helm plugin package from ArtifactHub.
type PluginPackage struct {
	PackageID      string       `json:"package_id"`
	Name           string       `json:"name"`
	NormalizedName string       `json:"normalized_name,omitempty"`
	DisplayName    string       `json:"display_name,omitempty"`
	Description    string       `json:"description"`
	Version        string       `json:"version"`
	AppVersion     string       `json:"app_version,omitempty"`
	License        string       `json:"license,omitempty"`
	Deprecated     bool         `json:"deprecated"`
	Signed         bool         `json:"signed"`
	Signatures     []string     `json:"signatures,omitempty"`
	SignKey        *SignKey     `json:"sign_key,omitempty"`
	ContentURL     string       `json:"content_url"`
	Digest         string       `json:"digest,omitempty"`
	TS             int64        `json:"ts"`
	Repository     *Repository  `json:"repository"`
	Data           *PluginData  `json:"data,omitempty"`
	Readme         string       `json:"readme,omitempty"`
	Maintainers    []Maintainer `json:"maintainers,omitempty"`
	Links          []Link       `json:"links,omitempty"`
	Keywords       []string     `json:"keywords,omitempty"`
}

// SearchResult represents the result of a package search.
type SearchResult struct {
	Packages   []PluginPackage `json:"packages"`
	TotalCount int             `json:"-"` // From Pagination-Total-Count header
}
