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

package v2

import "time"

// Dependency describes a chart upon which another chart depends.
//
// Dependencies can be used to express developer intent, or to capture the state
// of a chart.
type Dependency struct {
	// Name is the name of the dependency.
	//
	// This must mach the name in the dependency's Chart.yaml.
	Name string `json:"name" yaml:"name"`
	// Version is the version (range) of this chart.
	//
	// A lock file will always produce a single version, while a dependency
	// may contain a semantic version range.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// The URL to the repository.
	//
	// Appending `index.yaml` to this string should result in a URL that can be
	// used to fetch the repository index.
	Repository string `json:"repository" yaml:"repository"`
	// A yaml path that resolves to a boolean, used for enabling/disabling charts (e.g. subchart1.enabled )
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
	// Tags can be used to group charts for enabling/disabling together
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Enabled bool determines if chart should be loaded
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// ImportValues holds the mapping of source values to parent key to be imported. Each item can be a
	// string or pair of child/parent sublist items.
	ImportValues []interface{} `json:"import-values,omitempty" yaml:"import-values,omitempty"`
	// Alias usable alias to be used for the chart
	Alias string `json:"alias,omitempty" yaml:"alias,omitempty"`
}

// Validate checks for common problems with the dependency datastructure in
// the chart. This check must be done at load time before the dependency's charts are
// loaded.
func (d *Dependency) Validate() error {
	if d == nil {
		return ValidationError("dependencies must not contain empty or null nodes")
	}
	d.Name = sanitizeString(d.Name)
	d.Version = sanitizeString(d.Version)
	d.Repository = sanitizeString(d.Repository)
	d.Condition = sanitizeString(d.Condition)
	for i := range d.Tags {
		d.Tags[i] = sanitizeString(d.Tags[i])
	}
	if d.Alias != "" && !aliasNameFormat.MatchString(d.Alias) {
		return ValidationErrorf("dependency %q has disallowed characters in the alias", d.Name)
	}
	return nil
}

// PluginDependency describes a plugin required by a chart.
//
// Chart-defined plugins are downloaded and cached globally by version,
// then used when rendering or building chart dependencies.
type PluginDependency struct {
	// Name is the name of the plugin.
	Name string `json:"name" yaml:"name"`
	// Type is the plugin type (e.g., "render/v1", "getter/v1").
	Type string `json:"type" yaml:"type"`
	// Repository is the OCI URL where the plugin is stored.
	Repository string `json:"repository" yaml:"repository"`
	// Version is the version (range) of this plugin.
	//
	// A lock file will always produce a single version, while a dependency
	// may contain a semantic version range.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Digest is the SHA256 hash of the plugin tarball content.
	// This enables content-addressable caching in $HELM_CACHE_HOME/content/.
	// When present in Chart.lock, the downloader can retrieve cached tarballs
	// without re-downloading.
	Digest string `json:"digest,omitempty" yaml:"digest,omitempty"`
}

// Validate checks for common problems with the plugin dependency.
func (p *PluginDependency) Validate() error {
	if p == nil {
		return ValidationError("plugins must not contain empty or null nodes")
	}
	p.Name = sanitizeString(p.Name)
	p.Type = sanitizeString(p.Type)
	p.Repository = sanitizeString(p.Repository)
	p.Version = sanitizeString(p.Version)
	if p.Name == "" {
		return ValidationError("plugin name is required")
	}
	if p.Type == "" {
		return ValidationError("plugin type is required")
	}
	if p.Repository == "" {
		return ValidationError("plugin repository is required")
	}
	return nil
}

// GetName returns the plugin name.
func (p *PluginDependency) GetName() string { return p.Name }

// GetType returns the plugin type (e.g., "render/v1", "getter/v1").
func (p *PluginDependency) GetType() string { return p.Type }

// GetRepository returns the OCI repository URL.
func (p *PluginDependency) GetRepository() string { return p.Repository }

// GetVersion returns the plugin version.
func (p *PluginDependency) GetVersion() string { return p.Version }

// GetDigest returns the content digest for content-addressable caching.
func (p *PluginDependency) GetDigest() string { return p.Digest }

// Lock is a lock file for dependencies.
//
// It represents the state that the dependencies should be in.
type Lock struct {
	// Generated is the date the lock file was last generated.
	Generated time.Time `json:"generated"`
	// Digest is a hash of the dependencies in Chart.yaml.
	Digest string `json:"digest"`
	// Dependencies is the list of dependencies that this lock file has locked.
	Dependencies []*Dependency `json:"dependencies"`
	// Plugins is the list of plugins that this lock file has locked.
	Plugins []*PluginDependency `json:"plugins,omitempty"`
}
