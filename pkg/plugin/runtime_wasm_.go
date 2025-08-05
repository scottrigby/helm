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
	"context"
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
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

func (r *RuntimeConfigWasm) GetType() string { return "wasm" }

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
	pluginType string
}

func (r *RuntimeConfigWasm) CreateRuntime(pluginDir string, pluginName string, pluginType string) (Runtime, error) {
	return &RuntimeWasm{
		config:     r,
		pluginDir:  pluginDir,
		pluginName: pluginName,
		pluginType: pluginType,
	}, nil
}

// Invoke implementation for Runtime
func (r *RuntimeWasm) invoke(_ context.Context, _ *Input) (*Output, error) {
	// TODO: Implement WASM runtime execution
	// This will include:
	// - Loading the WASM module from r.config.WasmModule
	// - Setting up host functions from r.config.HostFunctions
	// - Configuring memory settings from r.config.MemorySettings
	// - Applying security constraints (AllowedHosts, AllowedPaths)
	// - Executing the WASM module with environment from 'env'
	// - Reading input from 'stdin' and writing output to 'stdout'/'stderr'
	return nil, fmt.Errorf("WASM runtime not yet implemented")
}

func (r *RuntimeWasm) invokeWithEnv(_ string, _ []string, _ []string, _ io.Reader, _, _ io.Writer) error {
	return fmt.Errorf("WASM runtime not yet implemented")
}

func (r *RuntimeWasm) invokeHook(_ string) error {
	return fmt.Errorf("WASM runtime not yet implemented")
}

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
