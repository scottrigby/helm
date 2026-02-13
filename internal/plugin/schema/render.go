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

package schema

import (
	"fmt"
)

// ReleaseInfo contains release metadata passed to render plugins.
type ReleaseInfo struct {
	// Name is the release name
	Name string `json:"name"`
	// Namespace is the release namespace
	Namespace string `json:"namespace"`
	// Revision is the release revision number
	Revision int `json:"revision"`
	// IsInstall is true if this is an install operation
	IsInstall bool `json:"isInstall"`
	// IsUpgrade is true if this is an upgrade operation
	IsUpgrade bool `json:"isUpgrade"`
	// Service is the name of the rendering engine (always "Helm")
	Service string `json:"service"`
}

// ChartInfo contains chart metadata passed to render plugins.
type ChartInfo struct {
	// Name is the chart name
	Name string `json:"name"`
	// Version is the chart version
	Version string `json:"version"`
	// AppVersion is the application version
	AppVersion string `json:"appVersion,omitempty"`
	// Description is a one-sentence description
	Description string `json:"description,omitempty"`
	// Type is the chart type (application or library)
	Type string `json:"type,omitempty"`
	// IsRoot indicates if this is the root chart
	IsRoot bool `json:"isRoot"`
}

// SubchartInfo contains metadata about a subchart dependency.
type SubchartInfo struct {
	// Name is the subchart name
	Name string `json:"name"`
	// Version is the resolved version
	Version string `json:"version,omitempty"`
	// Enabled indicates if the subchart is enabled
	Enabled bool `json:"enabled"`
}

// CapabilitiesInfo contains Kubernetes cluster capabilities.
type CapabilitiesInfo struct {
	// KubeVersion is the Kubernetes version
	KubeVersion KubeVersionInfo `json:"kubeVersion"`
	// APIVersions are supported Kubernetes API versions
	APIVersions []string `json:"apiVersions"`
	// HelmVersion is the Helm version
	HelmVersion string `json:"helmVersion"`
}

// KubeVersionInfo contains Kubernetes version information.
type KubeVersionInfo struct {
	// Version is the full version string
	Version string `json:"version"`
	// Major is the major version number
	Major string `json:"major"`
	// Minor is the minor version number
	Minor string `json:"minor"`
}

// SourceFile represents a file in the chart that can be rendered or read.
type SourceFile struct {
	// Name is the file path relative to the chart root
	Name string `json:"name"`
	// Data is the file contents
	Data []byte `json:"data"`
}

// TemplateInfo contains information about the current template being rendered.
type TemplateInfo struct {
	// Name is the template file path
	Name string `json:"name"`
	// BasePath is the templates directory path
	BasePath string `json:"basePath"`
}

// InputMessageRenderV1 is the input message for render/v1 plugins.
// It contains all the Helm built-in objects needed for rendering.
type InputMessageRenderV1 struct {
	// Release contains release metadata
	Release ReleaseInfo `json:"release"`
	// Values contains merged values from values.yaml and --set flags
	Values map[string]interface{} `json:"values"`
	// Chart contains chart metadata
	Chart ChartInfo `json:"chart"`
	// Subcharts contains metadata about subchart dependencies
	Subcharts map[string]SubchartInfo `json:"subcharts"`
	// Files contains non-template files accessible to the chart
	Files []SourceFile `json:"files"`
	// Capabilities contains information about the Kubernetes cluster
	Capabilities CapabilitiesInfo `json:"capabilities"`
	// SourceFiles contains the files to be rendered by this plugin
	// These are the files matching the plugin's glob patterns
	SourceFiles []SourceFile `json:"sourceFiles"`
}

// RenderedFile represents a rendered output file.
type RenderedFile struct {
	// Name is the output file path
	Name string `json:"name"`
	// Content is the rendered content
	Content string `json:"content"`
}

// OutputMessageRenderV1 is the output message from render/v1 plugins.
type OutputMessageRenderV1 struct {
	// RenderedFiles contains the rendered Kubernetes manifests
	// Key is the file path, value is the rendered content
	RenderedFiles map[string]string `json:"renderedFiles"`
	// ModifiedSourceFiles contains source files to pass to subsequent plugins.
	// This enables plugin composition where one plugin can:
	// - Add new files for later plugins to process
	// - Modify existing files before passing to later plugins
	// - Remove files by not including them
	// If nil/empty, the original unmatched files are passed to the next plugin.
	ModifiedSourceFiles []SourceFile `json:"modifiedSourceFiles,omitempty"`
	// Errors contains any non-fatal errors encountered during rendering
	Errors []string `json:"errors,omitempty"`
}

// ConfigRenderV1 represents the configuration for render plugins.
type ConfigRenderV1 struct {
	// Patterns are glob patterns that define which files this plugin manages.
	// Files matching these patterns will be passed to the plugin for rendering.
	// Examples: ["*.pkl", "/templates/*.pkl"], ["/templates/*.yaml", "/templates/*.tpl"]
	Patterns []string `yaml:"patterns"`
}

func (c *ConfigRenderV1) Validate() error {
	if len(c.Patterns) == 0 {
		return fmt.Errorf("render plugin has no patterns")
	}
	for i, pattern := range c.Patterns {
		if pattern == "" {
			return fmt.Errorf("render plugin has empty pattern at index %d", i)
		}
	}
	return nil
}
