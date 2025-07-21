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
	"io"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
)

// this filename underscore suffix is a workaround to Go treating files ending
// with _wasm.go as having an implicit build constraint for WebAssembly (GOARCH=wasm)
// ref https://pkg.go.dev/cmd/go#hdr-Build_constraints

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

// GetRuntimeType implementation for RuntimeConfig
func (r *RuntimeConfigWasm) GetRuntimeType() string { return "wasm" }

// Validate implementation for RuntimeConfig
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

// RuntimeWasm implements the Runtime interface for WASM execution
type RuntimeWasm struct {
	config     *RuntimeConfigWasm
	pluginDir  string
	pluginName string
	settings   *cli.EnvSettings
}

// CreateRuntime implementation for RuntimeConfig
func (r *RuntimeConfigWasm) CreateRuntime(pluginDir string, pluginName string) (Runtime, error) {
	return &RuntimeWasm{
		config:     r,
		pluginDir:  pluginDir,
		pluginName: pluginName,
		settings:   cli.New(),
	}, nil
}

// Invoke implementation for Runtime
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

// InvokeWithEnv implementation for RuntimeWasm (not yet implemented)
func (r *RuntimeWasm) InvokeWithEnv(stdin io.Reader, stdout, stderr io.Writer, env []string) error {
	return fmt.Errorf("WASM runtime not yet implemented")
}

// InvokeHook implementation for RuntimeWasm (not yet implemented)
func (r *RuntimeWasm) InvokeHook(event string) error {
	return fmt.Errorf("WASM runtime not yet implemented")
}

// Postrender implementation for RuntimeWasm
func (r *RuntimeWasm) Postrender(renderedManifests *bytes.Buffer, args []string) (*bytes.Buffer, error) {
	return nil, fmt.Errorf("WASM postrender not yet implemented")
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
