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

type Metadata interface {
	GetAPIVersion() string
	GetName() string
	GetType() string
	GetConfig() Config
	GetRuntimeConfig() RuntimeConfig
}

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

func (m *MetadataLegacy) GetAPIVersion() string { return "legacy" }

func (m *MetadataLegacy) GetName() string { return m.Name }

func (m *MetadataLegacy) GetType() string {
	if len(m.Downloaders) > 0 {
		return "download"
	}
	return "cli"
}

func (m *MetadataLegacy) GetConfig() Config {
	switch m.GetType() {
	case "download":
		return &ConfigDownload{
			Downloaders: m.Downloaders,
		}
	case "cli":
		return &ConfigCLI{
			Usage:       "",            // Legacy plugins don't have Usage field for command syntax
			ShortHelp:   m.Usage,       // Map legacy usage to shortHelp
			LongHelp:    m.Description, // Map legacy description to longHelp
			IgnoreFlags: m.IgnoreFlags,
		}
	default:
		return nil
	}
}

func (m *MetadataLegacy) GetRuntimeConfig() RuntimeConfig {
	return &RuntimeConfigSubprocess{
		PlatformCommand: m.PlatformCommand,
		Command:         m.Command,
		PlatformHooks:   m.PlatformHooks,
		Hooks:           m.Hooks,
		UseTunnel:       m.UseTunnelDeprecated,
	}
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

func (m *MetadataV1) GetAPIVersion() string { return m.APIVersion }

func (m *MetadataV1) GetName() string { return m.Name }

func (m *MetadataV1) GetType() string { return m.Type }

func (m *MetadataV1) GetConfig() Config { return m.Config }

func (m *MetadataV1) GetRuntimeConfig() RuntimeConfig { return m.RuntimeConfig }
