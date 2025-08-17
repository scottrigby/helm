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
	"log/slog"
	"path/filepath"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero"

	"helm.sh/helm/v4/internal/plugin/schema"
)

type RuntimeConfigExtismV1 struct {
	MaxPages             uint32            `yaml:"maxPages,omitempty"`
	MaxHTTPResponseBytes int64             `yaml:"maxHttpResponseBytes,omitempty"`
	MaxVarBytes          int64             `yaml:"maxVarBytes,omitempty"`
	Config               map[string]string `yaml:"config,omitempty"`
	AllowedHosts         []string          `yaml:"allowedHosts,omitempty"`
	AllowedPaths         map[string]string `yaml:"allowedPaths,omitempty"`
	Timeout              uint64            `yaml:"timeout,omitempty"`
	HostFunctions        []string          `yaml:"hostFunctions,omitempty"`
	EntryFuncName        string            `yaml:"entryFuncName,omitempty"` // The name of entry function name to call in the plugin. Defaults to "helm_plugin_main".
}

var _ RuntimeConfig = (*RuntimeConfigExtismV1)(nil)

func (r *RuntimeConfigExtismV1) Validate() error {
	// TODO
	return nil
}

type RuntimeExtismV1 struct {
	HostFunctions    map[string]extism.HostFunction
	CompliationCache wazero.CompilationCache
}

var _ Runtime = (*RuntimeExtismV1)(nil)

func (r *RuntimeExtismV1) CreatePlugin(pluginDir string, metadata *Metadata) (Plugin, error) {

	rc, ok := metadata.RuntimeConfig.(*RuntimeConfigExtismV1)
	if !ok {
		return nil, fmt.Errorf("invalid extism/v1 plugin runtime config type: %T", metadata.RuntimeConfig)
	}

	wasmFile := filepath.Join(pluginDir, "plugin.wasm")
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmFile,
				Name: wasmFile,
			},
		},
		Memory: &extism.ManifestMemory{
			MaxPages:             rc.MaxPages,
			MaxHttpResponseBytes: rc.MaxHTTPResponseBytes,
			MaxVarBytes:          rc.MaxVarBytes,
		},
		Config:       rc.Config,
		AllowedHosts: rc.AllowedHosts,
		AllowedPaths: rc.AllowedPaths,
		Timeout:      rc.Timeout,
	}

	hostFunctions := make([]extism.HostFunction, len(rc.HostFunctions))
	for _, fnName := range rc.HostFunctions {
		fn, ok := r.HostFunctions[fnName]
		if !ok {
			return nil, fmt.Errorf("plugin requested host function %q not found", rc.HostFunctions)
		}

		hostFunctions = append(hostFunctions, fn)
	}

	return &ExtismV1PluginRuntime{
		metadata:      *metadata,
		dir:           pluginDir,
		manifest:      manifest,
		hostFunctions: hostFunctions,
	}, nil
}

type ExtismV1PluginRuntime struct {
	metadata      Metadata
	dir           string
	manifest      extism.Manifest
	hostFunctions []extism.HostFunction
	rc            RuntimeConfigExtismV1
	r             RuntimeExtismV1
}

var _ Plugin = (*ExtismV1PluginRuntime)(nil)

func (p *ExtismV1PluginRuntime) Metadata() Metadata {
	return p.metadata
}

func (p *ExtismV1PluginRuntime) Dir() string {
	return p.dir
}

func (p *ExtismV1PluginRuntime) Invoke(ctx context.Context, input *Input) (*Output, error) {

	mc := wazero.NewModuleConfig().
		WithSysWalltime()
	if input.Stdin != nil {
		mc = mc.
			WithStdin(input.Stdin)
	}
	if input.Stdout != nil {
		mc = mc.
			WithStdout(input.Stdout)
	}
	if input.Stderr != nil {
		mc = mc.
			WithStderr(input.Stderr)
	}
	// mc = mc.WithEnv()

	config := extism.PluginConfig{
		ModuleConfig:              mc,
		RuntimeConfig:             wazero.NewRuntimeConfig().WithCloseOnContextDone(true).WithCompilationCache(p.r.CompliationCache),
		EnableWasi:                true,
		EnableHttpResponseHeaders: true,
		//ObserveAdapter: ,
		//ObserveOptions: &observe.Options{},
	}

	pe, err := extism.NewPlugin(ctx, p.manifest, config, p.hostFunctions)
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
		slog.Debug(s, slog.String("level", logLevel.String()), slog.String("plugin", p.metadata.Name))
		fmt.Printf("%s [%s] %s", logLevel.String(), p.metadata.Name, s)
	})

	inputData, err := json.Marshal(input.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to json marshel plugin input message: %T: %w", input.Message, err)
	}

	slog.Debug("plugin input", slog.String("plugin", p.metadata.Name), slog.String("inputData", string(inputData)))

	entryFuncName := p.rc.EntryFuncName
	if entryFuncName == "" {
		entryFuncName = "helm_plugin_main"
	}

	exitCode, outputData, err := pe.Call(entryFuncName, inputData)
	if err != nil {
		return nil, fmt.Errorf("plugin %q failed to invoke: %w", p.metadata.Name, err)
	}

	if exitCode != 0 {
		return nil, &InvokeExecError{
			Code: int(exitCode),
		}
	}

	slog.Debug("plugin output", slog.String("plugin", p.metadata.Name), slog.Int("exitCode", int(exitCode)), slog.String("outputData", string(outputData)))

	outputMessage := makeOutputMessage(p.metadata.Type)

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
