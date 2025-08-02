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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/plugin/schema"
)

// SubprocessProtocolCommand maps a given protocol to the getter command used to retrieve artifacts for that protcol
type SubprocessProtocolCommand struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `json:"protocols"`
	// Command is the executable path with which the plugin performs
	// the actual download for the corresponding Protocols
	Command string `json:"command"`
}

// RuntimeConfigSubprocess represents configuration for subprocess runtime
type RuntimeConfigSubprocess struct {
	// PlatformCommand is a list containing a plugin command, with a platform selector and support for args.
	// TODO rename to "PlatformCommands" plural to match other plural field names
	PlatformCommand []PlatformCommand `json:"platformCommand"`
	// Command is the plugin command, as a single string.
	// DEPRECATED: Use PlatformCommand instead. Remove in Helm 4.
	Command string `json:"command"`
	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `json:"platformHooks"`
	// Hooks are commands that will run on plugin events, as a single string.
	// DEPRECATED: Use PlatformHooks instead. Remove in Helm 4.
	Hooks Hooks `json:"hooks"`
	// ProtocolCommands field is used if the plugin supply downloader mechanism
	// for special protocols.
	// (This is a compatibility handover from the old plugin downloader mechanism, which was extended to support multiple
	// protocols in a given plugin)
	ProtocolCommands []SubprocessProtocolCommand `json:"protocolCommands,omitempty"`
	// UseTunnel indicates that this command needs a tunnel.
	// DEPRECATED and unused, but retained for backwards compatibility. Remove in Helm 4.
	UseTunnel bool `json:"useTunnel"`
}

func (r *RuntimeConfigSubprocess) Type() string { return "subprocess" }

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
	pluginType string
}

// CreateRuntime implementation for RuntimeConfig
func (r *RuntimeConfigSubprocess) CreateRuntime(pluginDir string, pluginName string, pluginType string) (Runtime, error) {
	return &RuntimeSubprocess{
		config:     r,
		pluginDir:  pluginDir,
		pluginName: pluginName,
		pluginType: pluginType,
	}, nil
}

func (r *RuntimeSubprocess) invoke(_ context.Context, input *Input) (*Output, error) {
	// TODO should we find a better way to do this?
	// TODO add postrender message schema and case here
	switch input.Message.(type) {
	case schema.CLIInputV1:
		return r.runCLI(input)
	case schema.InputMessageGetterV1:
		return r.runGetter(input)
		//return runGetter(r, input)
	default:
		return nil, fmt.Errorf("unsupported subprocess plugin type %q", r.pluginType)
	}
}

// InvokeWithEnv executes a plugin command with custom environment and I/O streams
// This method allows execution with different command/args than the plugin's default
func (r *RuntimeSubprocess) invokeWithEnv(main string, argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	mainCmdExp := os.ExpandEnv(main)
	prog := exec.Command(mainCmdExp, argv...)
	prog.Env = env
	prog.Stdin = stdin
	prog.Stdout = stdout
	prog.Stderr = stderr

	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			status := eerr.Sys().(syscall.WaitStatus)
			return &Error{
				Err:        fmt.Errorf("plugin %q exited with error", r.pluginName),
				PluginName: r.pluginName,
				Code:       status.ExitStatus(),
			}
		}
		return err
	}
	return nil
}

func (r *RuntimeSubprocess) invokeHook(event string) error {
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

	prog := exec.Command(main, argv...)
	prog.Stdout, prog.Stderr = os.Stdout, os.Stderr

	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return fmt.Errorf("plugin %s hook for %q exited with error", event, r.pluginName)
		}
		return err
	}
	return nil
}

func (r *RuntimeSubprocess) postrender(renderedManifests *bytes.Buffer, args []string, extraArgs []string, settings *cli.EnvSettings) (*bytes.Buffer, error) {
	// Setup plugin environment
	SetupPluginEnv(settings, r.pluginName, r.pluginDir)

	// Prepare command with the provided args
	originalExtraArgs := extraArgs
	extraArgs = args
	defer func() { extraArgs = originalExtraArgs }()

	cmds := r.config.PlatformCommand
	if len(cmds) == 0 && len(r.config.Command) > 0 {
		cmds = []PlatformCommand{{Command: r.config.Command}}
	}

	main, argv, err := PrepareCommands(cmds, true, extraArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare command: %w", err)
	}

	// Execute the postrender command
	mainCmdExp := os.ExpandEnv(main)
	cmd := exec.Command(mainCmdExp, argv...)

	// Set up environment
	env := os.Environ()
	for k, v := range settings.EnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Set up stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	var postRendered bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &postRendered
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Write input to stdin
	go func() {
		defer stdin.Close()
		io.Copy(stdin, renderedManifests)
	}()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error while running postrender %s. error output:\n%s: %w", r.pluginName, stderr.String(), err)
	}

	// Check for empty output
	if len(bytes.TrimSpace(postRendered.Bytes())) == 0 {
		return nil, fmt.Errorf("post-renderer %q produced empty output", r.pluginName)
	}

	return &postRendered, nil
}

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

// TODO decide the best way to handle this code
// right now we implement status and error return in 3 slightly different ways in this file
// then replace the other three with a call to this func
func executeCmd(prog *exec.Cmd, pluginName string) error {
	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return &Error{
				Err:  fmt.Errorf("plugin %q exited with error", pluginName),
				Code: eerr.ExitCode(),
			}
		}

		return &Error{
			Err: err,
		}
	}

	return nil
}

func (r *RuntimeSubprocess) runCLI(input *Input) (*Output, error) {
	if _, ok := input.Message.(schema.CLIInputV1); !ok {
		return nil, fmt.Errorf("plugin %q input message does not implement CLIInputV1", r.pluginName)
	}

	extraArgs := input.Message.(schema.CLIInputV1).ExtraArgs

	cmds := r.config.PlatformCommand
	if len(cmds) == 0 && len(r.config.Command) > 0 {
		cmds = []PlatformCommand{{Command: r.config.Command}}
	}

	command, args, err := PrepareCommands(cmds, true, extraArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare plugin command: %w", err)
	}

	// TODO can we use invokeWithEnv here?
	//prog := exec.Command(
	//	command,
	//	args...)
	////prog.Env = pluginExec.env
	//prog.Stdin = input.Stdin
	//prog.Stdout = input.Stdout
	//prog.Stderr = input.Stderr
	//if err := executeCmd(prog, r.pluginName); err != nil {
	//	return nil, err
	//}

	err2 := r.invokeWithEnv(command, args, input.Env, input.Stdin, input.Stdout, input.Stderr)
	if err2 != nil {
		return nil, err2
	}

	return &Output{
		Message: &schema.CLIOutputV1{},
	}, nil
}

// ExecDownloader executes a plugin downloader command with custom environment
func ExecDownloader(base string, command string, argv []string, env []string) (*bytes.Buffer, error) {
	prog := exec.Command(command, argv...)
	prog.Env = env

	buf := bytes.NewBuffer(nil)
	prog.Stdout = buf
	prog.Stderr = os.Stderr

	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return nil, fmt.Errorf("plugin %q exited with error", command)
		}
		return nil, err
	}
	return buf, nil
}
