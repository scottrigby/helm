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
	"strings"
	"unicode"
)

// PluginLegacy represents a legacy plugin
type PluginLegacy struct {
	// MetadataLegacy is a parsed representation of a plugin.yaml
	MetadataLegacy *MetadataLegacy
	// Dir is the string path to the directory that holds the plugin.
	Dir string
}

// Interface implementations for PluginLegacy
func (p *PluginLegacy) GetDir() string  { return p.Dir }
func (p *PluginLegacy) GetName() string { return p.MetadataLegacy.Name }

// Legacy plugins can be either a downloader or a legacy-CLI plugin (we label them as legacy)
func (p *PluginLegacy) GetType() string {
	if len(p.MetadataLegacy.Downloaders) > 0 {
		return "download"
	}
	return "cli"
}
func (p *PluginLegacy) GetAPIVersion() string    { return "legacy" }
func (p *PluginLegacy) GetRuntime() string       { return "subprocess" }
func (p *PluginLegacy) GetMetadata() interface{} { return p.MetadataLegacy }

func (p *PluginLegacy) GetRuntimeConfig() RuntimeConfig {
	return &RuntimeConfigSubprocess{
		PlatformCommand: p.MetadataLegacy.PlatformCommand,
		Command:         p.MetadataLegacy.Command,
		PlatformHooks:   p.MetadataLegacy.PlatformHooks,
		Hooks:           p.MetadataLegacy.Hooks,
		UseTunnel:       p.MetadataLegacy.UseTunnelDeprecated,
	}
}

func (p *PluginLegacy) GetConfig() Config {
	switch p.GetType() {
	case "download":
		return &ConfigDownload{
			Downloaders: p.MetadataLegacy.Downloaders,
		}
	case "cli":
		return &ConfigCLI{
			Usage:       "", // Legacy plugins don't have Usage field for command syntax
			ShortHelp:   p.MetadataLegacy.Usage, // Map legacy usage to shortHelp
			LongHelp:    p.MetadataLegacy.Description, // Map legacy description to longHelp
			IgnoreFlags: p.MetadataLegacy.IgnoreFlags,
		}
	default:
		// Return a basic CLI config as fallback
		return &ConfigCLI{
			Usage:       "", // Legacy plugins don't have Usage field for command syntax
			ShortHelp:   p.MetadataLegacy.Usage, // Map legacy usage to shortHelp
			LongHelp:    p.MetadataLegacy.Description, // Map legacy description to longHelp
			IgnoreFlags: p.MetadataLegacy.IgnoreFlags,
		}
	}
}

func (p *PluginLegacy) GetRuntimeInstance() (Runtime, error) {
	runtimeConfig := p.GetRuntimeConfig()
	return runtimeConfig.CreateRuntime(p.Dir, p.MetadataLegacy.Name)
}

func (p *PluginLegacy) PrepareCommand(extraArgs []string) (string, []string, error) {
	var extraArgsIn []string

	if !p.MetadataLegacy.IgnoreFlags {
		extraArgsIn = extraArgs
	}

	cmds := p.MetadataLegacy.PlatformCommand
	if len(cmds) == 0 && len(p.MetadataLegacy.Command) > 0 {
		cmds = []PlatformCommand{{Command: p.MetadataLegacy.Command}}
	}

	return PrepareCommands(cmds, true, extraArgsIn)
}

// Validate validates a legacy plugin's metadata.
func (p *PluginLegacy) Validate() error {
	if !validPluginName.MatchString(p.MetadataLegacy.Name) {
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
