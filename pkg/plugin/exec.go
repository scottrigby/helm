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
	"os"
	"os/exec"
	"syscall"

	"helm.sh/helm/v4/pkg/cli"
)

// execRender implements PostRenderer. represents a postrender type plugin and args
// TODO: rename to PostRendererWithEnv, because this is not subprocess/exec specific, but does contain Helm Client env settings
type execRender struct {
	plugin   Plugin
	args     []string
	settings *cli.EnvSettings
}

// NewExec returns a PostRenderer implementation that calls the provided postrender type plugin.
// It returns an error if the plugin cannot be found.
// TODO: rename to NewPostRendererWithEnv
func NewExec(settings *cli.EnvSettings, pluginName string, args ...string) (PostRenderer, error) {
	p, err := FindPlugin(pluginName, settings.PluginsDirectory, "postrender")
	if err != nil {
		return nil, err
	}
	return &execRender{p, args, settings}, nil
}

// Run the configured binary for the post render
// TODO: consolidate with methods in pkg/plugin/runtime_subprocess.go
func (p *execRender) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// this part from [cmd.loadPlugins]
	// needed to get the correct args, which can be defined both in plugin.yaml and additional CLI command args
	SetupPluginEnv(p.settings, p.plugin.GetName(), p.plugin.GetDir())
	main, argv, err := p.plugin.PrepareCommand(p.args)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return nil, fmt.Errorf("plugin %q exited with error", p.plugin.GetName())
	}

	// this part modified from [cmd.callPluginExecutable]
	env := os.Environ()
	for k, v := range p.settings.EnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	mainCmdExp := os.ExpandEnv(main)
	cmd := exec.Command(mainCmdExp, argv...)
	cmd.Env = env

	// slightly modified original below
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	var postRendered = &bytes.Buffer{}
	var stderr = &bytes.Buffer{}
	cmd.Stdout = postRendered
	cmd.Stderr = stderr

	go func() {
		defer stdin.Close()
		io.Copy(stdin, renderedManifests)
	}()
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error while running command %s. error output:\n%s: %w", p.plugin.GetName(), stderr.String(), err)
	}

	// If the binary returned almost nothing, it's likely that it didn't
	// successfully render anything
	if len(bytes.TrimSpace(postRendered.Bytes())) == 0 {
		return nil, fmt.Errorf("post-renderer %q produced empty output", p.plugin.GetName())
	}

	return postRendered, nil
}

// ExecPluginWithEnv executes a plugin command with custom environment and I/O streams
func ExecPluginWithEnv(pluginName string, main string, argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
			return &ExecError{
				Err:        fmt.Errorf("plugin %q exited with error", pluginName),
				PluginName: pluginName,
				Code:       status.ExitStatus(),
			}
		}
		return err
	}
	return nil
}

// ExecError is returned when a plugin exits with a non-zero status code
type ExecError struct {
	Err        error
	PluginName string
	Code       int
}

// Error implements the error interface
func (e *ExecError) Error() string {
	return e.Err.Error()
}

// ExecHook executes a plugin hook command
func ExecHook(pluginName string, event string, main string, argv []string) error {
	prog := exec.Command(main, argv...)
	prog.Stdout, prog.Stderr = os.Stdout, os.Stderr

	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return fmt.Errorf("plugin %s hook for %q exited with error", event, pluginName)
		}
		return err
	}
	return nil
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
