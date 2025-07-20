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
	"bytes"
	"fmt"
	"os/exec"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
)

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
	config     *RuntimeConfigSubprocess
	pluginDir  string
	pluginName string
	extraArgs  []string
	settings   *cli.EnvSettings
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
	config     *RuntimeConfigWasm
	pluginDir  string
	pluginName string
	settings   *cli.EnvSettings
}

// CreateRuntime implementations for RuntimeConfig types
func (r *RuntimeConfigSubprocess) CreateRuntime(pluginDir string, pluginName string) (Runtime, error) {
	return &RuntimeSubprocess{
		config:     r,
		pluginDir:  pluginDir,
		pluginName: pluginName,
		settings:   cli.New(),
	}, nil
}

func (r *RuntimeConfigWasm) CreateRuntime(pluginDir string, pluginName string) (Runtime, error) {
	return &RuntimeWasm{
		config:     r,
		pluginDir:  pluginDir,
		pluginName: pluginName,
		settings:   cli.New(),
	}, nil
}

// Invoke implementations for Runtime types
func (r *RuntimeSubprocess) Invoke(in *bytes.Buffer, out *bytes.Buffer) error {
	// Setup plugin environment
	SetupPluginEnv(r.settings, r.pluginName, r.pluginDir)

	// Prepare command based on runtime configuration
	// Note: IgnoreFlags is handled at the plugin level, not runtime level
	extraArgsIn := r.extraArgs

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
