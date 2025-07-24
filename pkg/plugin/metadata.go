package plugin

// Metadata describes a V1 plugin (APIVersion: v1)
type MetadataV1 struct {
	// APIVersion specifies the plugin API version
	APIVersion string `json:"apiVersion"`

	// Name is the name of the plugin
	Name string `json:"name"`

	// Type of plugin (eg, cli, download, postrender)
	Type string `json:"type"`

	// Runtime specifies the runtime type (subprocess, wasm)
	Runtime string `json:"runtime"`

	// Version is a SemVer 2 version of the plugin.
	Version string `json:"version"`

	// SourceURL is the URL where this plugin can be found
	SourceURL string `json:"sourceURL,omitempty"`

	// Config contains the type-specific configuration for this plugin
	Config Config `json:"config"`

	// RuntimeConfig contains the runtime-specific configuration
	RuntimeConfig RuntimeConfig `json:"runtimeConfig"`
}
