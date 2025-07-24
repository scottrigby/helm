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

package subprocess

import (
	"fmt"
	"strings"
	"unicode"

	"helm.sh/helm/v4/pkg/plugin"
)

// Plugin represents a subprocess plugin
type Plugin struct {
	// Metadata is a parsed representation of a legacy plugin.yaml
	Metadata plugin.MetadataV1
	// Dir is the string path to the directory that holds the plugin.
	Dir string
}

// Interface implementations for PluginLegacy
func (p *Plugin) GetDir() string  { return p.Dir }
func (p *Plugin) GetName() string { return p.MetadataLegacy.Name }

// Legacy plugins can be either a downloader or a legacy-CLI plugin (we label them as legacy)
func (p *Plugin) GetType() string {
	if len(p.MetadataLegacy.Downloaders) > 0 {
		return "download"
	}
	return "cli"
}
func (p *Plugin) GetAPIVersion() string    { return "legacy" }
func (p *Plugin) GetRuntime() string       { return "subprocess" }
func (p *Plugin) GetMetadata() interface{} { return p.MetadataLegacy }

func (p *Plugin) GetRuntimeConfig() plugin.RuntimeConfig {
	return &RuntimeConfig{
		PlatformCommand: p.MetadataLegacy.PlatformCommand,
		Command:         p.MetadataLegacy.Command,
		PlatformHooks:   p.MetadataLegacy.PlatformHooks,
		Hooks:           p.MetadataLegacy.Hooks,
		UseTunnel:       p.MetadataLegacy.UseTunnelDeprecated,
	}
}

func (p *Plugin) GetConfig() plugin.Config {
	switch p.GetType() {
	case "download":
		downloaders := []plugin.Downloaders{}
		for _, d := range p.MetadataLegacy.Downloaders {
			downloaders = append(downloaders, plugin.Downloaders{
				Protocols: d.Protocols,
				Command:   d.Command,
			})

		}

		return &plugin.ConfigDownload{
			Downloaders: downloaders,
		}
	case "cli":
		return &plugin.ConfigCLI{
			Usage:       "",                           // Legacy plugins don't have Usage field for command syntax
			ShortHelp:   p.MetadataLegacy.Usage,       // Map legacy usage to shortHelp
			LongHelp:    p.MetadataLegacy.Description, // Map legacy description to longHelp
			IgnoreFlags: p.MetadataLegacy.IgnoreFlags,
		}
	default:
		// Return a basic CLI config as fallback
		return &plugin.ConfigCLI{
			Usage:       "",                           // Legacy plugins don't have Usage field for command syntax
			ShortHelp:   p.MetadataLegacy.Usage,       // Map legacy usage to shortHelp
			LongHelp:    p.MetadataLegacy.Description, // Map legacy description to longHelp
			IgnoreFlags: p.MetadataLegacy.IgnoreFlags,
		}
	}
}

func (p *Plugin) GetRuntimeInstance() (plugin.Runtime, error) {
	runtimeConfig := p.GetRuntimeConfig()
	return runtimeConfig.CreateRuntime(p.Dir, p.MetadataLegacy.Name)
}

//func (p *Plugin) PrepareCommand(extraArgs []string) (string, []string, error) {
//	var extraArgsIn []string
//
//	if !p.MetadataLegacy.IgnoreFlags {
//		extraArgsIn = extraArgs
//	}
//
//	cmds := p.MetadataLegacy.PlatformCommand
//	if len(cmds) == 0 && len(p.MetadataLegacy.Command) > 0 {
//		cmds = []PlatformCommand{{Command: p.MetadataLegacy.Command}}
//	}
//
//	return PrepareCommands(cmds, true, extraArgsIn)
//}

func (p *Plugin) PrepareCommand(extraArgs []string) (string, []string, error) {
	config := p.GetConfig()
	runtimeConfig := p.GetRuntimeConfig()

	// Only subprocess runtime uses PrepareCommand
	if subprocessConfig, ok := runtimeConfig.(*RuntimeConfig); ok {
		var extraArgsIn []string

		// For CLI plugins, check ignore flags
		if config.GetType() == "cli" {
			if cliConfig, ok := config.(*plugin.ConfigCLI); ok && cliConfig.IgnoreFlags {
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

// Validate validates a legacy plugin's metadata.
func (p *Plugin) Validate() error {
	if !plugin.ValidPluginName.MatchString(p.MetadataLegacy.Name) {
		return fmt.Errorf("invalid plugin name")
	}
	p.MetadataLegacy.Usage = sanitizeString(p.MetadataLegacy.Usage)

	if len(p.MetadataLegacy.PlatformCommand) > 0 && len(p.MetadataLegacy.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set")
	}

	if len(p.MetadataLegacy.PlatformHooks) > 0 && len(p.MetadataLegacy.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set")
	}

	// Validate downloader plugins
	if len(p.MetadataLegacy.Downloaders) > 0 {
		for i, downloader := range p.MetadataLegacy.Downloaders {
			if downloader.Command == "" {
				return fmt.Errorf("downloader %d has empty command", i)
			}
			if len(downloader.Protocols) == 0 {
				return fmt.Errorf("downloader %d has no protocols", i)
			}
			for j, protocol := range downloader.Protocols {
				if protocol == "" {
					return fmt.Errorf("downloader %d has empty protocol at index %d", i, j)
				}
			}
		}
	}

	return nil
}

// sanitizeString normalize spaces and removes non-printable characters.
func sanitizeString(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, str)
}
