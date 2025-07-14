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
	"helm.sh/helm/v4/pkg/cli"
	"io"
	"os"
	"os/exec"
)

// TODO this should be a plugin name instead of binary path
// should we still allow postrender args? If so, how would that work with a postrender Wasm plugin?
// for now, pre-Wasm work, we could still draw the command from the plugin's plugin.yaml file with minimal changes here
type execRender struct {
	plugin   *Plugin
	args     []string
	settings *cli.EnvSettings
}

// NewExec returns a PostRenderer implementation that calls the provided postrender type plugin.
// It returns an error if the plugin cannot be found.
func NewExec(settings *cli.EnvSettings, pluginName string, args ...string) (PostRenderer, error) {
	p, err := FindPlugin(pluginName, settings.PluginsDirectory, "postrender")
	if err != nil {
		return nil, err
	}
	return &execRender{p, args, settings}, nil
}

// Run the configured binary for the post render
func (p *execRender) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// this part from [cmd.loadPlugins]
	// needed to get the correct args, which can be defined both in plugin.yaml and additional CLI command args
	SetupPluginEnv(p.settings, p.plugin.Metadata.Name, p.plugin.Dir)
	main, argv, err := p.plugin.PrepareCommand(p.args)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return nil, fmt.Errorf("plugin %q exited with error", p.plugin.Metadata.Name)
	}

	// this part modified from [CallPluginExec]
	env := os.Environ()
	for k, v := range p.settings.EnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	mainCmdExp := os.ExpandEnv(main)
	cmd := exec.Command(mainCmdExp, argv...)
	cmd.Env = env

	// slightly modified original below
	//cmd := exec.Command(p.binaryPath, p.args...)
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
		return nil, fmt.Errorf("error while running command %s. error output:\n%s: %w", p.plugin.Metadata.Name, stderr.String(), err)
	}

	// If the binary returned almost nothing, it's likely that it didn't
	// successfully render anything
	if len(bytes.TrimSpace(postRendered.Bytes())) == 0 {
		return nil, fmt.Errorf("post-renderer %q produced empty output", p.plugin.Metadata.Name)
	}

	return postRendered, nil
}
