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
	"regexp"
)

// PluginV1 represents a V1 plugin
type PluginV1 struct {
	// MetadataV1 is a parsed representation of a plugin.yaml
	Metadata MetadataV1
	// Dir is the string path to the directory that holds the plugin.
	Dir string
}

func (p *PluginV1) GetRuntimeInstance() (Runtime, error) {
	if p.Metadata.RuntimeConfig == nil {
		return nil, fmt.Errorf("plugin has no runtime configuration")
	}
	return p.Metadata.RuntimeConfig.CreateRuntime(p)
}

func (p *PluginV1) PrepareCommand(extraArgs []string) (string, []string, error) {
	config := p.Metadata.Config
	runtimeConfig := p.Metadata.RuntimeConfig

	// Only subprocess runtime uses PrepareCommand
	if subprocessConfig, ok := runtimeConfig.(*RuntimeConfigSubprocess); ok {
		var extraArgsIn []string

		// For CLI plugins, check ignore flags
		if config.GetType() == "cli/v1" {
			if cliConfig, ok := config.(*ConfigCLI); ok && cliConfig.IgnoreFlags {
				extraArgsIn = []string{}
			} else {
				extraArgsIn = extraArgs
			}
		} else {
			extraArgsIn = extraArgs
		}

		cmds := subprocessConfig.PlatformCommand
		if len(cmds) == 0 && len(subprocessConfig.Command) > 0 {
			cmds = []PlatformCommand{{Command: subprocessConfig.Command}}
		}

		return PrepareCommands(cmds, true, extraArgsIn)
	}

	return "", nil, fmt.Errorf("PrepareCommand only supported for subprocess runtime")
}

func (p *PluginV1) Validate() error {

	if !validPluginName.MatchString(p.Metadata.Name) {
		return fmt.Errorf("invalid name")
	}

	if p.Metadata.APIVersion != "v1" {
		return fmt.Errorf("invalid apiVersion: %q", p.Metadata.APIVersion)
	}

	if p.Metadata.Type == "" {
		return fmt.Errorf("empty type field")
	}

	if p.Metadata.Runtime == "" {
		return fmt.Errorf("empty runtime field")
	}

	if p.Metadata.Config == nil {
		return fmt.Errorf("missing config field")
	}

	if p.Metadata.RuntimeConfig == nil {
		return fmt.Errorf("missing runtimeConfig field")
	}

	// Validate that config type matches plugin type
	if p.Metadata.Config.GetType() != p.Metadata.Type {
		return fmt.Errorf("config type %s does not match plugin type %s", p.Metadata.Config.GetType(), p.Metadata.Type)
	}

	// Validate that runtime config type matches runtime type
	if p.Metadata.RuntimeConfig.GetRuntimeType() != p.Metadata.Runtime {
		return fmt.Errorf("runtime config type %s does not match runtime %s", p.Metadata.RuntimeConfig.GetRuntimeType(), p.Metadata.Runtime)
	}

	// Validate the config itself
	if err := p.Metadata.Config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Validate the runtime config itself
	if err := p.Metadata.RuntimeConfig.Validate(); err != nil {
		return fmt.Errorf("runtime config validation failed: %w", err)
	}

	return nil
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")
