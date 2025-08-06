package extismv1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero"
	"helm.sh/helm/v4/pkg/plugin"
	"helm.sh/helm/v4/pkg/plugin/schema"
)

type Runtime struct {
	HostFunctions    map[string]extism.HostFunction
	CompliationCache wazero.CompilationCache
}

type RuntimeConfig struct {
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

func (r *Runtime) CreatePlugin(metadata plugin.MetadataV1, path string) (*Plugin, error) {

	rc, ok := metadata.RuntimeConfig.(*RuntimeConfig)
	if !ok {
		fmt.Sprintf("invalid extism/v1 plugin runtime config type: %T", metadata.RuntimeConfig)
	}

	wasmFile := filepath.Join(path, "plugin.wasm")
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmFile,
				Name: wasmFile,
			},
		},
		Memory: &extism.ManifestMemory{
			MaxPages:             rc.MaxPages,
			MaxHttpResponseBytes: rc.MaxHttpResponseBytes,
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

	return &Plugin{
		metadata:      metadata,
		dir:           path,
		manifest:      manifest,
		hostFunctions: hostFunctions,
	}, nil
}

type Plugin struct {
	metadata      plugin.MetadataV1
	dir           string
	manifest      extism.Manifest
	hostFunctions []extism.HostFunction
	rc            *RuntimeConfig
	r             *Runtime
}

var _ plugin.Plugin = &Plugin{}

func (p *Plugin) Metadata() plugin.MetadataV1 {
	return p.metadata
}

func (p *Plugin) Dir() string {
	return p.dir
}

func (p *Plugin) Invoke(ctx context.Context, input *plugin.Input) (*plugin.Output, error) {

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
	// mv = mc.WithEnv()

	config := extism.PluginConfig{
		ModuleConfig:  mc,
		RuntimeConfig: wazero.NewRuntimeConfig().WithCloseOnContextDone(true).WithCompilationCache(r.CompliationCache),
		EnableWasi:    true,
		//EnableHttpResponseHeaders: true,
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
	})

	inputData, err := json.Marshal(input.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to json marshel plugin input message: %T: %w", input.Message, err)
	}

	entryFuncName := p.rc.EntryFuncName
	if entryFuncName == "" {
		entryFuncName = "helm_plugin_main"
	}

	exitCode, outputData, err := pe.Call(entryFuncName, inputData)
	if err != nil {
		return nil, fmt.Errorf("plugin %q failed to invoke: %w", p.metadata.Name, err)
	}

	if exitCode != 0 {
		return nil, &plugin.Error{
			Code: int(exitCode),
		}
	}

	outputMessage := makeOutputMessage(p.metadata.Type)

	if err := json.Unmarshal(outputData, outputMessage); err != nil {
		return nil, fmt.Errorf("failed to json marshel plugin output message: %T: %w", outputMessage, err)
	}

	output := &plugin.Output{
		Message: outputMessage,
	}

	return output, nil
}

func makeOutputMessage(pluginType string) any {
	switch pluginType {
	case "getter/v1":
		return &schema.GetterOutputV1{}
	case "cli/v1":
		return &schema.CLIOutputV1{}
	case "post/v1":
		return &schema.CLIOutputV1{}
	}

	return nil
}
