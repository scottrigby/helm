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
	MetadataV1 *MetadataV1
	// Dir is the string path to the directory that holds the plugin.
	Dir string
}

// Interface implementations for PluginV1
func (p *PluginV1) GetDir() string     { return p.Dir }
func (p *PluginV1) Metadata() Metadata { return p.MetadataV1 }

func (p *PluginV1) Runtime() (Runtime, error) {
	if p.MetadataV1.RuntimeConfig == nil {
		return nil, fmt.Errorf("plugin has no runtime configuration")
	}
	return p.MetadataV1.RuntimeConfig.CreateRuntime(p.Dir, p.MetadataV1.Name)
}

// TODO call this from other packages instead of PrepareCommands() directly, so that ignore flags logic isn't lost
// it was a mistake that I left that out
func (p *PluginV1) PrepareCommand(extraArgs []string) (string, []string, error) {
	// Only subprocess runtime uses PrepareCommand
	if subprocessConfig, ok := p.MetadataV1.RuntimeConfig.(*RuntimeConfigSubprocess); ok {
		var extraArgsIn []string

		// For CLI plugins, check ignore flags
		if p.MetadataV1.Config.GetType() == "cli" {
			if cliConfig, ok := p.MetadataV1.Config.(*ConfigCLI); ok && cliConfig.IgnoreFlags {
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
	if p.MetadataV1 == nil {
		return fmt.Errorf("plugin metadata is missing")
	}

	if !validPluginName.MatchString(p.MetadataV1.Name) {
		return fmt.Errorf("invalid plugin name")
	}

	if p.MetadataV1.APIVersion != "v1" {
		return fmt.Errorf("v1 plugin must have apiVersion: v1")
	}

	if p.MetadataV1.Type == "" {
		return fmt.Errorf("v1 plugin must have a type field")
	}

	if p.MetadataV1.Runtime == "" {
		return fmt.Errorf("v1 plugin must have a runtime field")
	}

	if p.MetadataV1.Config == nil {
		return fmt.Errorf("v1 plugin must have a config field")
	}

	if p.MetadataV1.RuntimeConfig == nil {
		return fmt.Errorf("v1 plugin must have a runtimeConfig field")
	}

	// Validate that config type matches plugin type
	if p.MetadataV1.Config.GetType() != p.MetadataV1.Type {
		return fmt.Errorf("config type %s does not match plugin type %s", p.MetadataV1.Config.GetType(), p.MetadataV1.Type)
	}

	// Validate that runtime config type matches runtime type
	if p.MetadataV1.RuntimeConfig.GetRuntimeType() != p.MetadataV1.Runtime {
		return fmt.Errorf("runtime config type %s does not match runtime %s", p.MetadataV1.RuntimeConfig.GetRuntimeType(), p.MetadataV1.Runtime)
	}

	// Validate the config itself
	if err := p.MetadataV1.Config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Validate the runtime config itself
	if err := p.MetadataV1.RuntimeConfig.Validate(); err != nil {
		return fmt.Errorf("runtime config validation failed: %w", err)
	}

	return nil
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")
