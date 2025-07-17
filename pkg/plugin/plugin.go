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

package plugin // import "helm.sh/helm/v4/pkg/plugin"

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
)

const PluginFileName = "plugin.yaml"

// Downloaders represents the plugins capability if it can retrieve
// charts from special sources
type Downloaders struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `json:"protocols"`
	// Command is the executable path with which the plugin performs
	// the actual download for the corresponding Protocols
	Command string `json:"command"`
}

// PlatformCommand represents a command for a particular operating system and architecture
type PlatformCommand struct {
	OperatingSystem string   `json:"os"`
	Architecture    string   `json:"arch"`
	Command         string   `json:"command"`
	Args            []string `json:"args"`
}

// Plugin interface defines the common methods that all plugin versions must implement
type Plugin interface {
	GetDir() string
	GetName() string
	GetType() string
	GetAPIVersion() string
	GetMetadata() interface{}
	Validate() error
	PrepareCommand(extraArgs []string) (string, []string, error)
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
	Hooks Hooks

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

	// Type of plugin (eg, subcommand, downloader, postrender)
	Type string `json:"type"`

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
	Hooks Hooks

	// Downloaders field is used if the plugin supply downloader mechanism
	// for special protocols.
	Downloaders []Downloaders `json:"downloaders"`

	// UseTunnelDeprecated indicates that this command needs a tunnel.
	// DEPRECATED and unused, but retained for backwards compatibility with Helm 2 plugins. Remove in Helm 4
	UseTunnelDeprecated bool `json:"useTunnel,omitempty"`
}

// PluginLegacy represents a legacy plugin
type PluginLegacy struct {
	// MetadataLegacy is a parsed representation of a plugin.yaml
	MetadataLegacy *MetadataLegacy
	// Dir is the string path to the directory that holds the plugin.
	Dir string
}

// PluginV1 represents a V1 plugin
type PluginV1 struct {
	// MetadataV1 is a parsed representation of a plugin.yaml
	MetadataV1 *MetadataV1
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
func (p *PluginLegacy) GetMetadata() interface{} { return p.MetadataLegacy }

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

// Interface implementations for PluginV1
func (p *PluginV1) GetDir() string           { return p.Dir }
func (p *PluginV1) GetName() string          { return p.MetadataV1.Name }
func (p *PluginV1) GetType() string          { return p.MetadataV1.Type }
func (p *PluginV1) GetAPIVersion() string    { return p.MetadataV1.APIVersion }
func (p *PluginV1) GetMetadata() interface{} { return p.MetadataV1 }

func (p *PluginV1) PrepareCommand(extraArgs []string) (string, []string, error) {
	var extraArgsIn []string

	if !p.MetadataV1.IgnoreFlags {
		extraArgsIn = extraArgs
	}

	cmds := p.MetadataV1.PlatformCommand
	if len(cmds) == 0 && len(p.MetadataV1.Command) > 0 {
		cmds = []PlatformCommand{{Command: p.MetadataV1.Command}}
	}

	return PrepareCommands(cmds, true, extraArgsIn)
}

func (p *PluginV1) Validate() error {
	if !validPluginName.MatchString(p.MetadataV1.Name) {
		return fmt.Errorf("invalid plugin name")
	}

	if p.MetadataV1.APIVersion != "v1" {
		return fmt.Errorf("V1 plugin must have apiVersion: v1")
	}

	if p.MetadataV1.Type == "" {
		return fmt.Errorf("V1 plugin must have a type field")
	}

	p.MetadataV1.Usage = sanitizeString(p.MetadataV1.Usage)

	// Validate downloader plugins
	if len(p.MetadataV1.Downloaders) > 0 {
		for i, downloader := range p.MetadataV1.Downloaders {
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

// Returns command and args strings based on the following rules in priority order:
// - From the PlatformCommand where OS and Arch match the current platform
// - From the PlatformCommand where OS matches the current platform and Arch is empty/unspecified
// - From the PlatformCommand where OS is empty/unspecified and Arch matches the current platform
// - From the PlatformCommand where OS and Arch are both empty/unspecified
// - Return nil, nil
func getPlatformCommand(cmds []PlatformCommand) ([]string, []string) {
	var command, args []string
	found := false
	foundOs := false

	eq := strings.EqualFold
	for _, c := range cmds {
		if eq(c.OperatingSystem, runtime.GOOS) && eq(c.Architecture, runtime.GOARCH) {
			// Return early for an exact match
			return strings.Split(c.Command, " "), c.Args
		}

		if (len(c.OperatingSystem) > 0 && !eq(c.OperatingSystem, runtime.GOOS)) || len(c.Architecture) > 0 {
			// Skip if OS is not empty and doesn't match or if arch is set as a set arch requires an OS match
			continue
		}

		if !foundOs && len(c.OperatingSystem) > 0 && eq(c.OperatingSystem, runtime.GOOS) {
			// First OS match with empty arch, can only be overridden by a direct match
			command = strings.Split(c.Command, " ")
			args = c.Args
			found = true
			foundOs = true
		} else if !found {
			// First empty match, can be overridden by a direct match or an OS match
			command = strings.Split(c.Command, " ")
			args = c.Args
			found = true
		}
	}

	return command, args
}

// PrepareCommands takes a []Plugin.PlatformCommand
// and prepares the command and arguments for execution.
//
// It merges extraArgs into any arguments supplied in the plugin. It
// returns the main command and an args array.
//
// The result is suitable to pass to exec.Command.
func PrepareCommands(cmds []PlatformCommand, expandArgs bool, extraArgs []string) (string, []string, error) {
	cmdParts, args := getPlatformCommand(cmds)
	if len(cmdParts) == 0 || cmdParts[0] == "" {
		return "", nil, fmt.Errorf("no plugin command is applicable")
	}

	main := os.ExpandEnv(cmdParts[0])
	baseArgs := []string{}
	if len(cmdParts) > 1 {
		for _, cmdPart := range cmdParts[1:] {
			if expandArgs {
				baseArgs = append(baseArgs, os.ExpandEnv(cmdPart))
			} else {
				baseArgs = append(baseArgs, cmdPart)
			}
		}
	}

	for _, arg := range args {
		if expandArgs {
			baseArgs = append(baseArgs, os.ExpandEnv(arg))
		} else {
			baseArgs = append(baseArgs, arg)
		}
	}

	if len(extraArgs) > 0 {
		baseArgs = append(baseArgs, extraArgs...)
	}

	return main, baseArgs, nil
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")

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

// LoadDir loads a plugin from the given directory.
func LoadDir(dirname string) (Plugin, error) {
	pluginfile := filepath.Join(dirname, PluginFileName)
	data, err := os.ReadFile(pluginfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin at %q: %w", pluginfile, err)
	}

	// First, try to detect the API version
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse plugin at %q: %w", pluginfile, err)
	}

	// Check if APIVersion is present and equals "v1"
	if apiVersion, ok := raw["apiVersion"].(string); ok && apiVersion == "v1" {
		// Load as V1 plugin
		plug := &PluginV1{Dir: dirname}
		if err := yaml.UnmarshalStrict(data, &plug.MetadataV1); err != nil {
			return nil, fmt.Errorf("failed to load V1 plugin at %q: %w", pluginfile, err)
		}
		return plug, plug.Validate()
	} else {
		// Load as legacy plugin
		plug := &PluginLegacy{Dir: dirname}
		if err := yaml.UnmarshalStrict(data, &plug.MetadataLegacy); err != nil {
			return nil, fmt.Errorf("failed to load legacy plugin at %q: %w", pluginfile, err)
		}
		return plug, plug.Validate()
	}
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
