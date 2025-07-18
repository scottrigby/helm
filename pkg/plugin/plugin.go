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
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

// Runtime interface defines the methods that all plugin runtimes must implement
type Runtime interface {
	Invoke(in *bytes.Buffer, out *bytes.Buffer) error
}

// RuntimeConfig interface defines the methods that all runtime configurations must implement
type RuntimeConfig interface {
	GetRuntimeType() string
	Validate() error
	CreateRuntime(pluginDir string, pluginName string) (Runtime, error)
}

// RuntimeConfigSubprocess represents configuration for subprocess runtime
type RuntimeConfigSubprocess struct {
	// PlatformCommand is the plugin command, with a platform selector and support for args.
	PlatformCommand []PlatformCommand `json:"platformCommand"`
	// Command is the plugin command, as a single string.
	// DEPRECATED: Use PlatformCommand instead. Remove in Helm 4.
	Command string `json:"command"`
	// ExtraArgs are additional arguments to pass to the plugin command
	ExtraArgs []string `json:"extraArgs"`
	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `json:"platformHooks"`
	// Hooks are commands that will run on plugin events, as a single string.
	// DEPRECATED: Use PlatformHooks instead. Remove in Helm 4.
	Hooks Hooks `json:"hooks"`
	// UseTunnel indicates that this command needs a tunnel.
	// DEPRECATED and unused, but retained for backwards compatibility. Remove in Helm 4.
	UseTunnel bool `json:"useTunnel"`
}

// RuntimeConfigWasm represents configuration for WASM runtime
type RuntimeConfigWasm struct {
	// WasmModule is the path to the WASM module file
	WasmModule string `json:"wasmModule"`
	// HostFunctions are the host functions to make available to the WASM module
	HostFunctions []string `json:"hostFunctions"`
	// MemorySettings configure WASM memory limits
	MemorySettings WasmMemorySettings `json:"memorySettings"`
	// AllowedHosts are the hosts that the WASM module is allowed to connect to
	AllowedHosts []string `json:"allowedHosts"`
	// AllowedPaths are the file system paths that the WASM module is allowed to access
	AllowedPaths []string `json:"allowedPaths"`
}

// WasmMemorySettings configure WASM memory limits
type WasmMemorySettings struct {
	InitialPages int `json:"initialPages"`
	MaxPages     int `json:"maxPages"`
}

// GetRuntimeType implementations
func (r *RuntimeConfigSubprocess) GetRuntimeType() string { return "subprocess" }
func (r *RuntimeConfigWasm) GetRuntimeType() string       { return "wasm" }

// Validate implementations for RuntimeConfig types
func (r *RuntimeConfigSubprocess) Validate() error {
	if len(r.PlatformCommand) > 0 && len(r.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set")
	}
	if len(r.PlatformHooks) > 0 && len(r.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set")
	}
	// At least one command must be specified
	if len(r.PlatformCommand) == 0 && len(r.Command) == 0 {
		return fmt.Errorf("either platformCommand or command must be specified")
	}
	return nil
}

func (r *RuntimeConfigWasm) Validate() error {
	if r.WasmModule == "" {
		return fmt.Errorf("wasmModule is required for WASM runtime")
	}
	if r.MemorySettings.InitialPages < 0 {
		return fmt.Errorf("initialPages must be non-negative")
	}
	if r.MemorySettings.MaxPages < 0 {
		return fmt.Errorf("maxPages must be non-negative")
	}
	if r.MemorySettings.MaxPages > 0 && r.MemorySettings.InitialPages > r.MemorySettings.MaxPages {
		return fmt.Errorf("initialPages cannot exceed maxPages")
	}
	return nil
}

// RuntimeSubprocess implements the Runtime interface for subprocess execution
type RuntimeSubprocess struct {
	config         *RuntimeConfigSubprocess
	pluginDir      string
	pluginName     string
	extraArgs      []string
	settings       *cli.EnvSettings
}

// SetExtraArgs sets the extra arguments for the subprocess runtime
func (r *RuntimeSubprocess) SetExtraArgs(args []string) {
	r.extraArgs = args
}

// SetSettings sets the environment settings for the subprocess runtime
func (r *RuntimeSubprocess) SetSettings(settings *cli.EnvSettings) {
	r.settings = settings
}

// RuntimeWasm implements the Runtime interface for WASM execution
type RuntimeWasm struct {
	config         *RuntimeConfigWasm
	pluginDir      string
	pluginName     string
	settings       *cli.EnvSettings
}

// CreateRuntime implementations for RuntimeConfig types
func (r *RuntimeConfigSubprocess) CreateRuntime(pluginDir string, pluginName string) (Runtime, error) {
	return &RuntimeSubprocess{
		config:    r,
		pluginDir: pluginDir,
		pluginName: pluginName,
		settings:  cli.New(),
	}, nil
}

func (r *RuntimeConfigWasm) CreateRuntime(pluginDir string, pluginName string) (Runtime, error) {
	return &RuntimeWasm{
		config:    r,
		pluginDir: pluginDir,
		pluginName: pluginName,
		settings:  cli.New(),
	}, nil
}

// Invoke implementations for Runtime types
func (r *RuntimeSubprocess) Invoke(in *bytes.Buffer, out *bytes.Buffer) error {
	// Setup plugin environment
	SetupPluginEnv(r.settings, r.pluginName, r.pluginDir)
	
	// Prepare command based on runtime configuration
	var extraArgsIn []string
	if r.config.IgnoreFlags {
		extraArgsIn = []string{}
	} else {
		extraArgsIn = r.extraArgs
	}
	
	cmds := r.config.PlatformCommand
	if len(cmds) == 0 && len(r.config.Command) > 0 {
		cmds = []PlatformCommand{{Command: r.config.Command}}
	}
	
	main, args, err := PrepareCommands(cmds, true, extraArgsIn)
	if err != nil {
		return fmt.Errorf("failed to prepare command: %w", err)
	}
	
	// Execute the command
	cmd := exec.Command(main, args...)
	cmd.Dir = r.pluginDir
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = out
	
	return cmd.Run()
}

func (r *RuntimeWasm) Invoke(in *bytes.Buffer, out *bytes.Buffer) error {
	// TODO: Implement WASM runtime execution
	// This will include:
	// - Loading the WASM module from r.config.WasmModule
	// - Setting up host functions from r.config.HostFunctions
	// - Configuring memory settings from r.config.MemorySettings
	// - Applying security constraints (AllowedHosts, AllowedPaths)
	// - Executing the WASM module with input from 'in' buffer
	// - Writing output to 'out' buffer
	return fmt.Errorf("WASM runtime not yet implemented")
}

// Config interface defines the methods that all plugin type configurations must implement
type Config interface {
	GetType() string
	Validate() error
}

// ConfigCLI represents the configuration for CLI plugins
type ConfigCLI struct {
	// Usage is the single-line usage text shown in help
	Usage string `json:"usage"`
	// Description is a long description shown in places like `helm help`
	Description string `json:"description"`
	// IgnoreFlags ignores any flags passed in from Helm
	IgnoreFlags bool `json:"ignoreFlags"`
}

// ConfigDownload represents the configuration for download plugins
type ConfigDownload struct {
	// Downloaders field is used if the plugin supply downloader mechanism
	// for special protocols.
	Downloaders []Downloaders `json:"downloaders"`
}

// ConfigPostrender represents the configuration for postrender plugins
type ConfigPostrender struct {
	// PostrenderArgs are arguments passed to the postrender command
	PostrenderArgs []string `json:"postrenderArgs"`
}

// GetType implementations for Config types
func (c *ConfigCLI) GetType() string        { return "cli" }
func (c *ConfigDownload) GetType() string   { return "download" }
func (c *ConfigPostrender) GetType() string { return "postrender" }

// Validate implementations for Config types
func (c *ConfigCLI) Validate() error {
	// Config validation for CLI plugins
	return nil
}

func (c *ConfigDownload) Validate() error {
	if len(c.Downloaders) > 0 {
		for i, downloader := range c.Downloaders {
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

func (c *ConfigPostrender) Validate() error {
	// Config validation for postrender plugins
	return nil
}

// Plugin interface defines the common methods that all plugin versions must implement
type Plugin interface {
	GetDir() string
	GetName() string
	GetType() string
	GetAPIVersion() string
	GetRuntime() string
	GetMetadata() interface{}
	GetConfig() Config
	GetRuntimeConfig() RuntimeConfig
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
	Hooks Hooks `json:"hooks"`

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

	// Type of plugin (eg, cli, download, postrender)
	Type string `json:"type"`

	// Runtime specifies the runtime type (subprocess, wasm)
	Runtime string `json:"runtime"`

	// Version is a SemVer 2 version of the plugin.
	Version string `json:"version"`

	// Config contains the type-specific configuration for this plugin
	Config Config `json:"config"`

	// RuntimeConfig contains the runtime-specific configuration
	RuntimeConfig RuntimeConfig `json:"runtimeConfig"`
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
			Usage:       p.MetadataLegacy.Usage,
			Description: p.MetadataLegacy.Description,
			IgnoreFlags: p.MetadataLegacy.IgnoreFlags,
		}
	default:
		// Return a basic CLI config as fallback
		return &ConfigCLI{
			Usage:       p.MetadataLegacy.Usage,
			Description: p.MetadataLegacy.Description,
			IgnoreFlags: p.MetadataLegacy.IgnoreFlags,
		}
	}
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

// Interface implementations for PluginV1
func (p *PluginV1) GetDir() string           { return p.Dir }
func (p *PluginV1) GetName() string          { return p.MetadataV1.Name }
func (p *PluginV1) GetType() string          { return p.MetadataV1.Type }
func (p *PluginV1) GetAPIVersion() string    { return p.MetadataV1.APIVersion }
func (p *PluginV1) GetRuntime() string       { return p.MetadataV1.Runtime }
func (p *PluginV1) GetMetadata() interface{} { return p.MetadataV1 }
func (p *PluginV1) GetConfig() Config        { return p.MetadataV1.Config }
func (p *PluginV1) GetRuntimeConfig() RuntimeConfig { return p.MetadataV1.RuntimeConfig }

func (p *PluginV1) PrepareCommand(extraArgs []string) (string, []string, error) {
	config := p.GetConfig()
	runtimeConfig := p.GetRuntimeConfig()
	
	// Only subprocess runtime uses PrepareCommand
	if subprocessConfig, ok := runtimeConfig.(*RuntimeConfigSubprocess); ok {
		var extraArgsIn []string
		
		// For CLI plugins, check ignore flags
		if config.GetType() == "cli" {
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
		// Load as V1 plugin with new structure
		plug := &PluginV1{Dir: dirname}

		// First, unmarshal the base metadata without the config and runtimeConfig fields
		tempMeta := &struct {
			APIVersion string `json:"apiVersion"`
			Name       string `json:"name"`
			Type       string `json:"type"`
			Runtime    string `json:"runtime"`
			Version    string `json:"version"`
		}{}

		if err := yaml.UnmarshalStrict(data, tempMeta); err != nil {
			return nil, fmt.Errorf("failed to load V1 plugin metadata at %q: %w", pluginfile, err)
		}

		// Default runtime to subprocess if not specified
		if tempMeta.Runtime == "" {
			tempMeta.Runtime = "subprocess"
		}

		// Create the MetadataV1 struct with base fields
		plug.MetadataV1 = &MetadataV1{
			APIVersion: tempMeta.APIVersion,
			Name:       tempMeta.Name,
			Type:       tempMeta.Type,
			Runtime:    tempMeta.Runtime,
			Version:    tempMeta.Version,
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
				return nil, fmt.Errorf("failed to unmarshal config for %s plugin at %q: %w", tempMeta.Type, pluginfile, err)
			}

			plug.MetadataV1.Config = config
		} else {
			// Backward compatibility: create config from legacy fields
			var config Config
			var err error

			switch tempMeta.Type {
			case "cli":
				config, err = createConfigCLIFromLegacy(raw)
			case "download":
				config, err = createConfigDownloadFromLegacy(raw)
			case "postrender":
				config, err = createConfigPostrenderFromLegacy(raw)
			default:
				return nil, fmt.Errorf("unsupported plugin type: %s", tempMeta.Type)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to create config from legacy fields for %s plugin at %q: %w", tempMeta.Type, pluginfile, err)
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
			// Backward compatibility: create runtimeConfig from legacy fields
			var runtimeConfig RuntimeConfig
			var err error

			switch tempMeta.Runtime {
			case "subprocess":
				runtimeConfig, err = createRuntimeConfigSubprocessFromLegacy(raw)
			case "wasm":
				return nil, fmt.Errorf("WASM runtime not supported in legacy format")
			default:
				return nil, fmt.Errorf("unsupported runtime type: %s", tempMeta.Runtime)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to create runtimeConfig from legacy fields for %s runtime at %q: %w", tempMeta.Runtime, pluginfile, err)
			}

			plug.MetadataV1.RuntimeConfig = runtimeConfig
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

// unmarshalConfigCLI unmarshals a config map into a ConfigCLI struct
func unmarshalConfigCLI(configData map[string]interface{}) (*ConfigCLI, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigCLI
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// unmarshalRuntimeConfigSubprocess unmarshals a runtime config map into a RuntimeConfigSubprocess struct
func unmarshalRuntimeConfigSubprocess(runtimeData map[string]interface{}) (*RuntimeConfigSubprocess, error) {
	data, err := yaml.Marshal(runtimeData)
	if err != nil {
		return nil, err
	}

	var config RuntimeConfigSubprocess
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// unmarshalRuntimeConfigWasm unmarshals a runtime config map into a RuntimeConfigWasm struct
func unmarshalRuntimeConfigWasm(runtimeData map[string]interface{}) (*RuntimeConfigWasm, error) {
	data, err := yaml.Marshal(runtimeData)
	if err != nil {
		return nil, err
	}

	var config RuntimeConfigWasm
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// unmarshalConfigDownload unmarshals a config map into a ConfigDownload struct
func unmarshalConfigDownload(configData map[string]interface{}) (*ConfigDownload, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigDownload
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// unmarshalConfigPostrender unmarshals a config map into a ConfigPostrender struct
func unmarshalConfigPostrender(configData map[string]interface{}) (*ConfigPostrender, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigPostrender
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// createConfigCLIFromLegacy creates a ConfigCLI from legacy plugin fields
func createConfigCLIFromLegacy(raw map[string]interface{}) (*ConfigCLI, error) {
	config := &ConfigCLI{}
	
	if usage, ok := raw["usage"].(string); ok {
		config.Usage = usage
	}
	if description, ok := raw["description"].(string); ok {
		config.Description = description
	}
	if ignoreFlags, ok := raw["ignoreFlags"].(bool); ok {
		config.IgnoreFlags = ignoreFlags
	}

	return config, nil
}

// createConfigDownloadFromLegacy creates a ConfigDownload from legacy plugin fields
func createConfigDownloadFromLegacy(raw map[string]interface{}) (*ConfigDownload, error) {
	config := &ConfigDownload{}
	
	// Create downloaders
	if downloadersRaw, ok := raw["downloaders"]; ok {
		downloadersData, err := yaml.Marshal(downloadersRaw)
		if err != nil {
			return nil, err
		}
		if err := yaml.UnmarshalStrict(downloadersData, &config.Downloaders); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// createConfigPostrenderFromLegacy creates a ConfigPostrender from legacy plugin fields
func createConfigPostrenderFromLegacy(raw map[string]interface{}) (*ConfigPostrender, error) {
	config := &ConfigPostrender{}
	
	// No specific fields to extract from legacy for postrender
	return config, nil
}

// createRuntimeConfigSubprocessFromLegacy creates a RuntimeConfigSubprocess from legacy plugin fields
func createRuntimeConfigSubprocessFromLegacy(raw map[string]interface{}) (*RuntimeConfigSubprocess, error) {
	legacyFields := map[string]interface{}{
		"platformCommand": raw["platformCommand"],
		"command":         raw["command"],
		"extraArgs":       raw["extraArgs"],
		"platformHooks":   raw["platformHooks"],
		"hooks":           raw["hooks"],
		"useTunnel":       raw["useTunnel"],
	}

	data, err := yaml.Marshal(legacyFields)
	if err != nil {
		return nil, err
	}

	var runtimeConfig RuntimeConfigSubprocess
	if err := yaml.UnmarshalStrict(data, &runtimeConfig); err != nil {
		return nil, err
	}

	return &runtimeConfig, nil
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
