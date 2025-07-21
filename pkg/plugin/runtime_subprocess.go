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

	"os/exec"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
)

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

// GetRuntimeType implementation for RuntimeConfig
func (r *RuntimeConfigSubprocess) GetRuntimeType() string { return "subprocess" }

// Validate implementation for RuntimeConfig
func (r *RuntimeConfigSubprocess) Validate() error {
	if len(r.PlatformCommand) > 0 && len(r.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set")
	}
	if len(r.PlatformHooks) > 0 && len(r.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set")
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

// CreateRuntime implementation for RuntimeConfig
func (r *RuntimeConfigSubprocess) CreateRuntime(pluginDir string, pluginName string) (Runtime, error) {
	return &RuntimeSubprocess{
		config:     r,
		pluginDir:  pluginDir,
		pluginName: pluginName,
		settings:   cli.New(),
	}, nil
}

// Invoke implementation for RuntimeConfig
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

// InvokeWithEnv implementation for RuntimeConfig
func (r *RuntimeSubprocess) InvokeWithEnv(stdin io.Reader, stdout, stderr io.Writer, env []string) error {
	// Prepare command based on runtime configuration
	cmds := r.config.PlatformCommand
	if len(cmds) == 0 && len(r.config.Command) > 0 {
		cmds = []PlatformCommand{{Command: r.config.Command}}
	}

	main, args, err := PrepareCommands(cmds, true, r.extraArgs)
	if err != nil {
		return fmt.Errorf("failed to prepare command: %w", err)
	}

	// Use the ExecPluginWithEnv function
	return ExecPluginWithEnv(r.pluginName, main, args, env, stdin, stdout, stderr)
}

// InvokeHook implementation for RuntimeConfig
func (r *RuntimeSubprocess) InvokeHook(event string) error {
	// Get hook commands for the event
	var cmds []PlatformCommand
	expandArgs := true

	cmds = r.config.PlatformHooks[event]
	if len(cmds) == 0 && len(r.config.Hooks) > 0 {
		cmd := r.config.Hooks[event]
		if len(cmd) > 0 {
			cmds = []PlatformCommand{{Command: "sh", Args: []string{"-c", cmd}}}
			expandArgs = false
		}
	}

	// If no hook commands are defined, just return successfully
	if len(cmds) == 0 {
		return nil
	}

	main, argv, err := PrepareCommands(cmds, expandArgs, []string{})
	if err != nil {
		return err
	}

	// Use the ExecHook function
	return ExecHook(r.pluginName, event, main, argv)
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
