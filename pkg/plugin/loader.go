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

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
)

// Load loads a plugin from raw YAML data.
// dirname is optional and can be empty if the plugin is not loaded from a directory.
func Load(data []byte, dirname string) (Plugin, error) {
	// First, try to detect the API version
	var raw map[string]interface{}
	if err := yaml.UnmarshalStrict(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse plugin: %w", err)
	}

	// Check if APIVersion is present
	if apiVersion, ok := raw["apiVersion"].(string); ok {
		if apiVersion == "v1" {
			// Load as V1 plugin with new structure
			plug := &PluginV1{Dir: dirname}

			// First, unmarshal the base metadata without the config and runtimeConfig fields
			tempMeta := &struct {
				APIVersion string `json:"apiVersion"`
				Name       string `json:"name"`
				Type       string `json:"type"`
				Runtime    string `json:"runtime"`
				Version    string `json:"version"`
				SourceURL  string `json:"sourceURL,omitempty"`
			}{}

			if err := yaml.Unmarshal(data, tempMeta); err != nil {
				return nil, fmt.Errorf("failed to load V1 plugin metadata: %w", err)
			}

			// Default runtime to subprocess if not specified
			if tempMeta.Runtime == "" {
				tempMeta.Runtime = "subprocess"
			}

			// Default type to cli if not specified
			if tempMeta.Type == "" {
				tempMeta.Type = "cli"
			}

			// Create the MetadataV1 struct with base fields
			plug.MetadataV1 = &MetadataV1{
				APIVersion: tempMeta.APIVersion,
				Name:       tempMeta.Name,
				Type:       tempMeta.Type,
				Runtime:    tempMeta.Runtime,
				Version:    tempMeta.Version,
				SourceURL:  tempMeta.SourceURL,
			}

			// Extract the config section based on plugin type
			if configData, ok := raw["config"].(map[string]interface{}); ok {
				var config Config
				var err error

				switch tempMeta.Type {
				case "cli":
					config, err = unmarshalConfigCLI(configData)
				case "download":
					config, err = unmarshalConfigDownload(configData)
				case "postrender":
					config, err = unmarshalConfigPostrender(configData)
				default:
					return nil, fmt.Errorf("unsupported plugin type: %s", tempMeta.Type)
				}

				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal config for %s plugin: %w", tempMeta.Type, err)
				}

				plug.MetadataV1.Config = config
			} else {
				// Create default config based on plugin type
				var config Config
				switch tempMeta.Type {
				case "cli":
					config = &ConfigCLI{}
				case "download":
					config = &ConfigDownload{}
				case "postrender":
					config = &ConfigPostrender{}
				default:
					return nil, fmt.Errorf("unsupported plugin type: %s", tempMeta.Type)
				}
				plug.MetadataV1.Config = config
			}

			// Extract the runtimeConfig section based on runtime type
			if runtimeConfigData, ok := raw["runtimeConfig"].(map[string]interface{}); ok {
				var runtimeConfig RuntimeConfig
				var err error

				switch tempMeta.Runtime {
				case "subprocess":
					runtimeConfig, err = unmarshalRuntimeConfigSubprocess(runtimeConfigData)
				case "wasm":
					runtimeConfig, err = unmarshalRuntimeConfigWasm(runtimeConfigData)
				default:
					return nil, fmt.Errorf("unsupported runtime type: %s", tempMeta.Runtime)
				}

				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal runtimeConfig for %s runtime: %w", tempMeta.Runtime, err)
				}

				plug.MetadataV1.RuntimeConfig = runtimeConfig
			} else {
				// Create default runtimeConfig based on runtime type
				var runtimeConfig RuntimeConfig
				switch tempMeta.Runtime {
				case "subprocess":
					runtimeConfig = &RuntimeConfigSubprocess{}
				case "wasm":
					runtimeConfig = &RuntimeConfigWasm{}
				default:
					return nil, fmt.Errorf("unsupported runtime type: %s", tempMeta.Runtime)
				}
				plug.MetadataV1.RuntimeConfig = runtimeConfig
			}

			return plug, plug.Validate()
		} else {
			// Unsupported apiVersion
			return nil, fmt.Errorf("unsupported apiVersion %q in plugin", apiVersion)
		}
	} else {
		// Load as legacy plugin
		plug := &PluginLegacy{Dir: dirname}
		if err := yaml.UnmarshalStrict(data, &plug.MetadataLegacy); err != nil {
			return nil, fmt.Errorf("failed to load legacy plugin: %w", err)
		}
		return plug, plug.Validate()
	}
}

// LoadDir loads a plugin from the given directory.
func LoadDir(dirname string) (Plugin, error) {
	pluginfile := filepath.Join(dirname, PluginFileName)
	data, err := os.ReadFile(pluginfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin at %q: %w", pluginfile, err)
	}

	plugin, err := Load(data, dirname)
	if err != nil {
		return nil, fmt.Errorf("plugin at %q: %w", pluginfile, err)
	}
	return plugin, nil
}

// LoadAll loads all plugins found beneath the base directory.
//
// This scans only one directory level.
func LoadAll(basedir, pluginType string) ([]Plugin, error) {
	plugins := []Plugin{}
	// We want basedir/*/plugin.yaml
	scanpath := filepath.Join(basedir, "*", PluginFileName)
	matches, err := filepath.Glob(scanpath)
	if err != nil {
		return plugins, fmt.Errorf("failed to find plugins in %q: %w", scanpath, err)
	}

	if matches == nil {
		return plugins, nil
	}

	for _, yaml := range matches {
		dir := filepath.Dir(yaml)
		p, err := LoadDir(dir)
		if err != nil {
			return plugins, err
		}
		if pluginType == "" || p.GetType() == pluginType {
			plugins = append(plugins, p)
		}
	}
	return plugins, detectDuplicates(plugins)
}

// FindPlugins returns a list of YAML files that describe plugins.
func FindPlugins(plugdirs string, pluginType string) ([]Plugin, error) {
	found := []Plugin{}
	// Let's get all UNIXy and allow path separators
	for _, p := range filepath.SplitList(plugdirs) {
		matches, err := LoadAll(p, pluginType)
		if err != nil {
			return matches, err
		}
		found = append(found, matches...)
	}
	return found, nil
}

// FindPlugin returns a plugin by name and optionally by type
// pluginType can be an empty string for any type
// TODO disambiguate from [cmd.findPlugin] or merge with this public func?
func FindPlugin(name, plugdirs, pluginType string) (Plugin, error) {
	plugins, _ := FindPlugins(plugdirs, pluginType)
	for _, p := range plugins {
		if p.GetName() == name {
			return p, nil
		}
	}
	err := fmt.Errorf("plugin: %s not found", name)
	return nil, err
}

func detectDuplicates(plugs []Plugin) error {
	names := map[string]string{}

	for _, plug := range plugs {
		if oldpath, ok := names[plug.GetName()]; ok {
			return fmt.Errorf(
				"two plugins claim the name %q at %q and %q",
				plug.GetName(),
				oldpath,
				plug.GetDir(),
			)
		}
		names[plug.GetName()] = plug.GetDir()
	}

	return nil
}

// SetupPluginEnv prepares os.Env for plugins. It operates on os.Env because
// the plugin subsystem itself needs access to the environment variables
// created here.
func SetupPluginEnv(settings *cli.EnvSettings, name, base string) {
	env := settings.EnvVars()
	env["HELM_PLUGIN_NAME"] = name
	env["HELM_PLUGIN_DIR"] = base
	for key, val := range env {
		os.Setenv(key, val)
	}
}
