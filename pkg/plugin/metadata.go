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

import "fmt"

// MetadataLegacy is the legacy plugin.yaml format
type MetadataLegacy struct {
	// Name is the name of the plugin
	Name string `yaml:"name"`

	// Version is a SemVer 2 version of the plugin.
	Version string `yaml:"version"`

	// Usage is the single-line usage text shown in help
	Usage string `yaml:"usage"`

	// Description is a long description shown in places like `helm help`
	Description string `yaml:"description"`

	// PlatformCommand is the plugin command, with a platform selector and support for args.
	PlatformCommand []PlatformCommand `yaml:"platformCommand"`

	// Command is the plugin command, as a single string.
	// DEPRECATED: Use PlatformCommand instead. Remove in Helm 4.
	Command string `yaml:"command"`

	// IgnoreFlags ignores any flags passed in from Helm
	IgnoreFlags bool `yaml:"ignoreFlags"`

	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `yaml:"platformHooks"`

	// Hooks are commands that will run on plugin events, as a single string.
	// DEPRECATED: Use PlatformHooks instead. Remove in Helm 4.
	Hooks Hooks `yaml:"hooks"`

	// Downloaders field is used if the plugin supply downloader mechanism
	// for special protocols.
	Downloaders []Downloaders `yaml:"downloaders"`

	// UseTunnelDeprecated indicates that this command needs a tunnel.
	// DEPRECATED and unused, but retained for backwards compatibility with Helm 2 plugins. Remove in Helm 4
	UseTunnelDeprecated bool `yaml:"useTunnel,omitempty"`
}

// MetadataLegacy is the APIVersion V1 plugin.yaml format
type MetadataV1 struct {
	// APIVersion specifies the plugin API version
	APIVersion string `yaml:"apiVersion"`

	// Name is the name of the plugin
	Name string `yaml:"name"`

	// Type of plugin (eg, cli/v1, getter/v1, postrenderer/v1)
	Type string `yaml:"type"`

	// Runtime specifies the runtime type (subprocess, wasm)
	Runtime string `yaml:"runtime"`

	// Version is a SemVer 2 version of the plugin.
	Version string `yaml:"version"`

	// SourceURL is the URL where this plugin can be found
	SourceURL string `yaml:"sourceURL,omitempty"`

	// Config contains the type-specific configuration for this plugin
	Config map[string]any `yaml:"config"`

	// RuntimeConfig contains the runtime-specific configuration
	RuntimeConfig map[string]any `yaml:"runtimeConfig"`
}

// Metadata of a plugin, converted from the "on-disk" legacy or v1 (yaml) formats
// Specifically, Config and RuntimeConfig are converted to their respective types based on the plugin type and runtime
type Metadata struct {
	// APIVersion specifies the plugin API version
	APIVersion string

	// Name is the name of the plugin
	Name string

	// Type of plugin (eg, cli/v1, getter/v1, postrenderer/v1)
	Type string

	// Runtime specifies the runtime type (subprocess, wasm)
	Runtime string

	// Version is the SemVer 2 version of the plugin.
	Version string

	// SourceURL is the URL where this plugin can be found
	SourceURL string

	// Config contains the type-specific configuration for this plugin
	Config Config

	// RuntimeConfig contains the runtime-specific configuration
	RuntimeConfig RuntimeConfig
}

func (m Metadata) Validate() error {
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("invalid name")
	}

	if m.APIVersion == "" {
		return fmt.Errorf("empty APIVersion")
	}

	if m.Type == "" {
		return fmt.Errorf("empty type field")
	}

	if m.Runtime == "" {
		return fmt.Errorf("empty runtime field")
	}

	if m.Config == nil {
		return fmt.Errorf("missing config field")
	}

	if m.RuntimeConfig == nil {
		return fmt.Errorf("missing runtimeConfig field")
	}

	// Validate the config itself
	if err := m.Config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Validate the runtime config itself
	if err := m.RuntimeConfig.Validate(); err != nil {
		return fmt.Errorf("runtime config validation failed: %w", err)
	}

	return nil
}

func fromMetadataLegacy(m MetadataLegacy) *Metadata {
	pluginType := "cli/v1"

	if len(m.Downloaders) > 0 {
		pluginType = "getter/v1"
	}

	return &Metadata{
		APIVersion:    "legacy",
		Name:          m.Name,
		Version:       m.Version,
		Type:          pluginType,
		Runtime:       "subprocess",
		Config:        buildLegacyConfig(m, pluginType),
		RuntimeConfig: buildLegacyRuntimeConfig(m),
	}
}

func buildLegacyConfig(m MetadataLegacy, pluginType string) Config {
	switch pluginType {
	case "getter/v1":
		var protocols []string
		for _, d := range m.Downloaders {
			protocols = append(protocols, d.Protocols...)
		}
		return &ConfigGetter{
			Protocols: protocols,
		}
	case "cli/v1":
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

func buildLegacyRuntimeConfig(m MetadataLegacy) RuntimeConfig {
	return &RuntimeConfigSubprocess{
		PlatformCommand: m.PlatformCommand,
		Command:         m.Command,
		PlatformHooks:   m.PlatformHooks,
		Hooks:           m.Hooks,
	}
}

func fromMetadataV1(m MetadataV1) (*Metadata, error) {

	config, err := convertMetadataConfig(m.Type, m.Config)
	if err != nil {
		return nil, err
	}

	runtimeConfig, err := convertMetdataRuntimeConfig(m.Runtime, m.RuntimeConfig)
	if err != nil {
		return nil, err
	}

	return &Metadata{
		APIVersion:    m.APIVersion,
		Name:          m.Name,
		Type:          m.Type,
		Runtime:       m.Runtime,
		Version:       m.Version,
		SourceURL:     m.SourceURL,
		Config:        config,
		RuntimeConfig: runtimeConfig,
	}, nil
}

func convertMetadataConfig(pluginType string, configRaw map[string]any) (Config, error) {
	var err error
	var config Config

	switch pluginType {
	case "cli/v1":
		config, err = unmarshalConfigCLI(configRaw)
	case "getter/v1":
		config, err = unmarshalConfigGetter(configRaw)
	case "postrenderer/v1":
		config, err = unmarshalConfigPostrenderer(configRaw)
	default:
		return nil, fmt.Errorf("unsupported plugin type: %s", pluginType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config for %s plugin type: %w", pluginType, err)
	}

	return config, nil
}

func convertMetdataRuntimeConfig(runtimeType string, runtimeConfigRaw map[string]any) (RuntimeConfig, error) {
	var runtimeConfig RuntimeConfig
	var err error

	switch runtimeType {
	case "subprocess":
		runtimeConfig, err = unmarshalRuntimeConfigSubprocess(runtimeConfigRaw)
	default:
		return nil, fmt.Errorf("unsupported plugin runtime type: %q", runtimeType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal runtimeConfig for %s runtime: %w", runtimeType, err)
	}
	return runtimeConfig, nil
}
