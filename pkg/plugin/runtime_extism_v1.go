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
	"encoding/json"
	"fmt"
	"github.com/tetratelabs/wazero"
	"helm.sh/helm/v4/pkg/plugin/schema"
	"io"
	"log/slog"
	"path/filepath"

	extism "github.com/extism/go-sdk"
	"sigs.k8s.io/yaml"
)

// RuntimeConfigExtismV1 represents configuration for WASM runtime
type RuntimeConfigExtismV1 struct {
	MaxPages             uint32            `json:"maxPages,omitempty"`
	MaxHttpResponseBytes int64             `json:"maxHttpResponseBytes,omitempty"`
	MaxVarBytes          int64             `json:"maxVarBytes,omitempty"`
	Config               map[string]string `json:"config,omitempty"`
	AllowedHosts         []string          `json:"allowedHosts,omitempty"`
	AllowedPaths         map[string]string `json:"allowedPaths,omitempty"`
	Timeout              uint64            `json:"timeout,omitempty"`
	HostFunctions        []string          `json:"hostFunctions,omitempty"`
	EntryFuncName        string            `json:"entryFuncName,omitempty"` // The name of entry function name to call in the plugin. Defaults to "helm_plugin_main".
}

func (r *RuntimeConfigExtismV1) GetType() string { return "extism/v1" }

func (r *RuntimeConfigExtismV1) Validate() error {
	if r.MaxPages < 0 {
		return fmt.Errorf("maxPages must be non-negative")
	}
	if r.MaxHttpResponseBytes < 0 {
		return fmt.Errorf("maxHttpResponseBytes must be non-negative")
	}
	if r.MaxVarBytes < 0 {
		return fmt.Errorf("maxVarBytes must be non-negative")
	}
	if r.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	// TODO should we add some warning if certain allowedPaths are requested by the plugin?
	// 	(eg, anything not in the HELM env var paths?)
	return nil
}

// RuntimeExtismV1 implements the Runtime interface for WASM execution
type RuntimeExtismV1 struct {
	config           *RuntimeConfigExtismV1
	pluginDir        string
	pluginName       string
	pluginType       string
	manifest         extism.Manifest
	hostFunctions    []extism.HostFunction
	CompliationCache wazero.CompilationCache
}

type ExtismV1Registry struct {
	AllowedHostFunctions []extism.HostFunction
}

// TODO who should define these? other parts of the Helm codebase? Or should these be from funcs per plugin type for Wasm runtime?
// TODO add actual host functions
func getExtismV1Registry() ExtismV1Registry {
	var registry ExtismV1Registry
	// TODO replace this example drawn from extism.NewHostFunctionWithStack function comment (also fixed their typo)
	mult := extism.NewHostFunctionWithStack(
		"mult",
		func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			a := extism.DecodeI32(stack[0])
			b := extism.DecodeI32(stack[1])

			stack[0] = extism.EncodeI32(a * b)
		},
		[]extism.ValueType{extism.ValueTypeI64, extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
	registry.AllowedHostFunctions = append(registry.AllowedHostFunctions, mult)
	return registry
}

func (r *RuntimeConfigExtismV1) CreateRuntime(pluginDir string, pluginName string, pluginType string) (Runtime, error) {
	wasmFile := filepath.Join(pluginDir, "plugin.wasm")
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmFile,
				Name: wasmFile,
			},
		},
		Memory: &extism.ManifestMemory{
			MaxPages:             r.MaxPages,
			MaxHttpResponseBytes: r.MaxHttpResponseBytes,
			MaxVarBytes:          r.MaxVarBytes,
		},
		Config:       r.Config,
		AllowedHosts: r.AllowedHosts,
		AllowedPaths: r.AllowedPaths,
		Timeout:      r.Timeout,
	}

	hostFunctions := make([]extism.HostFunction, 0, len(r.HostFunctions))
	registry := getExtismV1Registry()
	for _, fnName := range r.HostFunctions {
		found := false
		for _, allowedFn := range registry.AllowedHostFunctions {
			if allowedFn.Name == fnName {
				hostFunctions = append(hostFunctions, allowedFn)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("plugin requested host function %q not allowed", fnName)
		}
	}

	return &RuntimeExtismV1{
		config:        r,
		pluginDir:     pluginDir,
		pluginName:    pluginName,
		pluginType:    pluginType,
		manifest:      manifest,
		hostFunctions: hostFunctions,
	}, nil
}

// Invoke implementation for Runtime
func (r *RuntimeExtismV1) invoke(ctx context.Context, input *Input) (*Output, error) {
	// TODO: Implement WASM runtime execution
	// This will include:
	// - Loading the WASM module from r.config.WasmModule
	// - Setting up host functions from r.config.HostFunctions
	// - Configuring memory settings from r.config.MemorySettings
	// - Applying security constraints (AllowedHosts, AllowedPaths)
	// - Executing the WASM module with environment from 'env'
	// - Reading input from 'stdin' and writing output to 'stdout'/'stderr'

	mc := wazero.NewModuleConfig().WithSysWalltime()
	if input.Stdin != nil {
		mc = mc.WithStdin(input.Stdin)
	}
	if input.Stdout != nil {
		mc = mc.WithStdout(input.Stdout)
	}
	if input.Stderr != nil {
		mc = mc.WithStderr(input.Stderr)
	}
	// mv = mc.WithEnv()

	config := extism.PluginConfig{
		ModuleConfig:  mc,
		RuntimeConfig: wazero.NewRuntimeConfig().WithCloseOnContextDone(true).WithCompilationCache(r.CompliationCache),
		EnableWasi:    true,
		//EnableHttpResponseHeaders: true,
		//ObserveAdapter: ,
		//ObserveOptions: &observe.Options{},
	}

	pe, err := extism.NewPlugin(ctx, r.manifest, config, r.hostFunctions)
	if err != nil {
		return nil, fmt.Errorf("failed to create existing plugin: %w", err)
	}

	pe.SetLogger(func(logLevel extism.LogLevel, s string) {
		//slogLevel := slog.LevelInfo
		//switch logLevel {
		//case extism.LogLevelDebug:
		//	slogLevel = slog.LevelDebug
		//case extism.LogLevelInfo:
		//	slogLevel = slog.LevelInfo
		//case extism.LogLevelWarn:
		//	slogLevel = slog.LevelWarn
		//case extism.LogLevelError:
		//	slogLevel = slog.LevelError
		//case extism.LogLevelFatal:
		//	slogLevel = slog.LevelError
		//}
		//slog.Log(context.Background(), slogLevel, s, slog.String("plugin", metadata.Name))
		slog.Debug(s, slog.String("level", logLevel.String()), slog.String("plugin", r.pluginName))
	})

	inputData, err := json.Marshal(input.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to json marshel plugin input message: %T: %w", input.Message, err)
	}

	entryFuncName := r.config.EntryFuncName
	if entryFuncName == "" {
		entryFuncName = "helm_plugin_main"
	}

	exitCode, outputData, err := pe.Call(entryFuncName, inputData)
	if err != nil {
		return nil, fmt.Errorf("plugin %q failed to invoke: %w", r.pluginName, err)
	}

	if exitCode != 0 {
		return nil, &Error{
			Code: int(exitCode),
		}
	}

	outputMessage := makeOutputMessage(r.pluginType)

	if err := json.Unmarshal(outputData, outputMessage); err != nil {
		return nil, fmt.Errorf("failed to json marshel plugin output message: %T: %w", outputMessage, err)
	}

	output := &Output{
		Message: outputMessage,
	}

	return output, nil
}

func makeOutputMessage(pluginType string) any {
	switch pluginType {
	case "getter/v1":
		return &schema.OutputMessageGetterV1{}
	case "cli/v1":
		return &schema.OutputMessageCLIV1{}
	case "postrenderer/v1":
		return &schema.OutputMessagePostRendererV1{}
	}
	return nil
}

func (r *RuntimeExtismV1) invokeWithEnv(_ string, _ []string, _ []string, _ io.Reader, _, _ io.Writer) error {
	return fmt.Errorf("WASM runtime not yet implemented")
}

func (r *RuntimeExtismV1) invokeHook(_ string) error {
	return fmt.Errorf("WASM runtime not yet implemented")
}

func unmarshalRuntimeConfigWasm(runtimeData map[string]interface{}) (*RuntimeConfigExtismV1, error) {
	data, err := yaml.Marshal(runtimeData)
	if err != nil {
		return nil, err
	}

	var config RuntimeConfigExtismV1
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
