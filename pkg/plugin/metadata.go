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

package plugin

// MetadataLegacy describes a legacy plugin (no APIVersion field)
type MetadataLegacy struct {
	// Name is the name of the plugin
	Name string `json:"name"`

	// Version is a SemVer 2 version of the plugin.
	Version string `json:"version"`

	// Usage is the single-line usage text shown in help
	Usage string `json:"usage"`

	// Description is a long description shown in places like `helm help`
	Description string `json:"description"`

	// PlatformCommand is the plugin command, with a platform selector and support for args.
	PlatformCommand []PlatformCommand `json:"platformCommand"`

	// Command is the plugin command, as a single string.
	// DEPRECATED: Use PlatformCommand instead. Remove in Helm 4.
	Command string `json:"command"`

	// IgnoreFlags ignores any flags passed in from Helm
	IgnoreFlags bool `json:"ignoreFlags"`

	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `json:"platformHooks"`

	// Hooks are commands that will run on plugin events, as a single string.
	// DEPRECATED: Use PlatformHooks instead. Remove in Helm 4.
	Hooks Hooks `json:"hooks"`

	// Downloaders field is used if the plugin supply downloader mechanism
	// for special protocols.
	Downloaders []Downloaders `json:"downloaders"`

	// UseTunnelDeprecated indicates that this command needs a tunnel.
	// DEPRECATED and unused, but retained for backwards compatibility with Helm 2 plugins. Remove in Helm 4
	UseTunnelDeprecated bool `json:"useTunnel,omitempty"`
}

// MetadataV1 describes a V1 plugin (APIVersion: v1)
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

func ConvertMetadataLegacy(m MetadataLegacy) MetadataV1 {
	pluginType := "cli"

	var config Config
	if len(m.Downloaders) > 0 {
		pluginType = "getter"

		config = &ConfigDownload{
			Downloaders: m.Downloaders,
		}
	}

	runtimeConfig := &RuntimeConfigSubprocess{
		PlatformCommand: m.PlatformCommand,
		Command:         m.Command,
		PlatformHooks:   m.PlatformHooks,
		Hooks:           m.Hooks,
		UseTunnel:       m.UseTunnelDeprecated,
	}

	return MetadataV1{
		APIVersion: "legacy",
		Name:       m.Name,
		Version:    m.Version,
		// Description:  m.Description,
		Type:          pluginType,
		Runtime:       "subprocess",
		Config:        config,
		RuntimeConfig: runtimeConfig,
	}
}
