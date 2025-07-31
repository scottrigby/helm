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
func (p *PluginLegacy) GetDir() string     { return p.Dir }
func (p *PluginLegacy) Metadata() Metadata { return p.MetadataLegacy }

func (p *PluginLegacy) Runtime() (Runtime, error) {
	runtimeConfig := p.Metadata().GetRuntimeConfig()
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
