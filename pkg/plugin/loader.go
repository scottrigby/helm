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

// LoadDir loads a plugin from the given directory.
func LoadDir(dirname string) (Plugin, error) {
	pluginfile := filepath.Join(dirname, PluginFileName)
	data, err := os.ReadFile(pluginfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin at %q: %w", pluginfile, err)
	}

	// First, try to detect the API version
	var raw map[string]interface{}
	if err := yaml.UnmarshalStrict(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse plugin at %q: %w", pluginfile, err)
	}

	// Check if APIVersion is present
	apiVersion, ok := raw["apiVersion"].(string)
	if !ok || apiVersion == "" {
		apiVersion = "legacy"
	}

	switch apiVersion {
	case "v1":
		// Load as V1 plugin with new structure
		plug := &V1{Dir: dirname}

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
			return nil, fmt.Errorf("failed to load V1 plugin metadata at %q: %w", pluginfile, err)
		}

		// Default runtime to subprocess if not specified
		if tempMeta.Runtime == "" {
			tempMeta.Runtime = "subprocess"
		}

		// Default type to cli if not specified
		if tempMeta.Type == "" {
			tempMeta.Type = "cli/v1"
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
			case "cli/v1":
				config, err = unmarshalConfigCLI(configData)
			case "getter/v1":
				config, err = unmarshalConfigGetter(configData)
			case "postrenderer/v1":
				config, err = unmarshalConfigPostrenderer(configData)
			default:
				return nil, fmt.Errorf("unsupported plugin type: %s", tempMeta.Type)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal config for %s plugin at %q: %w", tempMeta.Type, pluginfile, err)
			}

			plug.MetadataV1.Config = config
		} else {
			// Create default config based on plugin type
			var config Config
			switch tempMeta.Type {
			case "cli/v1":
				config = &ConfigCLI{}
			case "getter/v1":
				config = &ConfigGetter{}
			case "postrenderer/v1":
				config = &ConfigPostrenderer{}
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
				return nil, fmt.Errorf("failed to unmarshal runtimeConfig for %s runtime at %q: %w", tempMeta.Runtime, pluginfile, err)
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
	case "legacy":
		// Load as legacy plugin
		plug := &Legacy{Dir: dirname}
		if err := yaml.UnmarshalStrict(data, &plug.MetadataLegacy); err != nil {
			return nil, fmt.Errorf("failed to load legacy plugin at %q: %w", pluginfile, err)
		}
		return plug, plug.Validate()
	default:
		return nil, fmt.Errorf("unsupported apiVersion %q in plugin at %q", apiVersion, pluginfile)
	}
}

// LoadAll loads all plugins found beneath the base directory.
//
// This scans only one directory level.
func LoadAll(basedir string) ([]Plugin, error) {
	var plugins []Plugin
	// We want basedir/*/plugin.yaml
	scanpath := filepath.Join(basedir, "*", PluginFileName)
	matches, err := filepath.Glob(scanpath)
	if err != nil {
		return nil, fmt.Errorf("failed to search for plugins in %q: %w", scanpath, err)
	}

	// empty dir should load
	if len(matches) == 0 {
		return plugins, nil
	}

	for _, yamlFile := range matches {
		dir := filepath.Dir(yamlFile)
		p, err := LoadDir(dir)
		if err != nil {
			return plugins, err
		}
		plugins = append(plugins, p)
	}
	return plugins, detectDuplicates(plugins)
}

// findFunc is a function that finds plugins in a directory
type findFunc func(pluginsDir string) ([]Plugin, error)

// filterFunc is a function that filters plugins
type filterFunc func(Plugin) bool

// FindPlugins returns a list of plugins that match the descriptor
func FindPlugins(pluginsDirs []string, descriptor Descriptor) ([]Plugin, error) {
	return findPlugins(pluginsDirs, LoadAll, makeDescriptorFilter(descriptor))
}

// findPlugins is the internal implementation that uses the find and filter functions
func findPlugins(pluginsDirs []string, findFunc findFunc, filterFunc filterFunc) ([]Plugin, error) {
	var found []Plugin
	for _, pluginsDir := range pluginsDirs {
		ps, err := findFunc(pluginsDir)

		if err != nil {
			return nil, err
		}

		for _, p := range ps {
			if filterFunc(p) {
				found = append(found, p)
			}
		}

	}

	return found, nil
}

// makeDescriptorFilter creates a filter function from a descriptor
// Additional plugin filter criteria we wish to support can be added here
func makeDescriptorFilter(descriptor Descriptor) filterFunc {
	return func(p Plugin) bool {
		// If name is specified, it must match
		if descriptor.Name != "" && p.Metadata().GetName() != descriptor.Name {
			return false

		}
		// If type is specified, it must match
		if descriptor.Type != "" && p.Metadata().GetType() != descriptor.Type {
			return false
		}
		return true
	}
}

// FindPlugin returns a single plugin that matches the descriptor
func FindPlugin(dirs []string, descriptor Descriptor) (Plugin, error) {
	plugins, err := FindPlugins(dirs, descriptor)
	if err != nil {
		return nil, err
	}

	if len(plugins) > 0 {
		return plugins[0], nil
	}

	return nil, fmt.Errorf("plugin: %+v not found", descriptor)
}

func detectDuplicates(plugs []Plugin) error {
	names := map[string]string{}

	for _, plug := range plugs {
		if oldpath, ok := names[plug.Metadata().GetName()]; ok {
			return fmt.Errorf(
				"two plugins claim the name %q at %q and %q",
				plug.Metadata().GetName(),
				oldpath,
				plug.GetDir(),
			)
		}
		names[plug.Metadata().GetName()] = plug.GetDir()
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
