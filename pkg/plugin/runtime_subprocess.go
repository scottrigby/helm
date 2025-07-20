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

// InvokeOptions contains options for invoking a plugin
type InvokeOptions struct {
	ExtraArgs []string
	Settings  *cli.EnvSettings
	EnvVars   map[string]string
}

// InvokePlugin executes a plugin with the given options
func InvokePlugin(plugin Plugin, in *bytes.Buffer, out *bytes.Buffer, opts InvokeOptions) error {
	runtime, err := plugin.GetRuntimeInstance()
	if err != nil {
		return fmt.Errorf("failed to get runtime instance: %w", err)
	}

	// Set settings and extra args on runtime if it's a subprocess runtime
	if subprocessRuntime, ok := runtime.(*RuntimeSubprocess); ok {
		if opts.Settings != nil {
			subprocessRuntime.SetSettings(opts.Settings)
		}

		// Handle flag ignoring for CLI plugins
		config := plugin.GetConfig()
		if config.GetType() == "cli" {
			if cliConfig, ok := config.(*ConfigCLI); ok && cliConfig.IgnoreFlags {
				subprocessRuntime.SetExtraArgs([]string{})
			} else {
				subprocessRuntime.SetExtraArgs(opts.ExtraArgs)
			}
		} else {
			subprocessRuntime.SetExtraArgs(opts.ExtraArgs)
		}

		// Set additional environment variables
		if opts.EnvVars != nil {
			subprocessRuntime.SetEnvVars(opts.EnvVars)
		}
	}

	return runtime.Invoke(in, out)
}

// InvokePluginCommand executes a specific command within a plugin's context
// This is used for dynamic completion and other special cases where we need to run
// a different executable than the plugin's main command
func InvokePluginCommand(plugin Plugin, command string, args []string, settings *cli.EnvSettings, in *bytes.Buffer, out *bytes.Buffer) error {
	// This only works with subprocess plugins
	if plugin.GetRuntime() != "subprocess" {
		return fmt.Errorf("plugin runtime %q does not support custom commands", plugin.GetRuntime())
	}

	// Create a temporary runtime config for the specific command
	tempRuntimeConfig := &RuntimeConfigSubprocess{
		Command: command,
	}

	tempRuntime, err := tempRuntimeConfig.CreateRuntime(plugin.GetDir(), plugin.GetName())
	if err != nil {
		return fmt.Errorf("failed to create runtime for command: %w", err)
	}

	if subprocessRuntime, ok := tempRuntime.(*RuntimeSubprocess); ok {
		subprocessRuntime.SetSettings(settings)
		subprocessRuntime.SetExtraArgs(args)
	}

	return tempRuntime.Invoke(in, out)
}

// PluginError represents an error from plugin execution
type PluginError struct {
	error
	Code int
}

// InvokePluginWithExit executes a plugin and returns exit code information
func InvokePluginWithExit(plugin Plugin, in *bytes.Buffer, out *bytes.Buffer, opts InvokeOptions) error {
	err := InvokePlugin(plugin, in, out, opts)
	if err != nil {
		// Check if it's an exit error and wrap it
		if eerr, ok := err.(*exec.ExitError); ok {
			status := eerr.Sys().(syscall.WaitStatus)
			return PluginError{
				error: fmt.Errorf("plugin %q exited with error", plugin.GetName()),
				Code:  status.ExitStatus(),
			}
		}
		return err
	}
	return nil
}

// SetEnvVars adds environment variables to the subprocess runtime
func (r *RuntimeSubprocess) SetEnvVars(envVars map[string]string) {
	if r.envVars == nil {
		r.envVars = make(map[string]string)
	}
	for k, v := range envVars {
		r.envVars[k] = v
	}
}

// Update RuntimeSubprocess struct in runtime.go to include envVars field
// This is a helper function to execute the command with proper environment setup
func (r *RuntimeSubprocess) executeCommand(main string, args []string, in *bytes.Buffer, out *bytes.Buffer, stderr io.Writer) error {
	cmd := exec.Command(main, args...)
	cmd.Dir = r.pluginDir
	cmd.Stdin = in
	cmd.Stdout = out
	if stderr != nil {
		cmd.Stderr = stderr
	} else {
		cmd.Stderr = out
	}

	// Setup environment
	env := os.Environ()
	if r.settings != nil {
		envVars := r.settings.EnvVars()
		// Add plugin-specific environment variables
		envVars["HELM_PLUGIN_NAME"] = r.pluginName
		envVars["HELM_PLUGIN_DIR"] = r.pluginDir

		for k, v := range envVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if r.envVars != nil {
		for k, v := range r.envVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.Env = env

	return cmd.Run()
}

// execRender struct for postrender plugin execution
type execRender struct {
	plugin   Plugin
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
	postRendered := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Get postrender args from config
	var extraArgs []string
	if config := p.plugin.GetConfig(); config != nil {
		if postrenderConfig, ok := config.(*ConfigPostrender); ok {
			extraArgs = append(postrenderConfig.PostrenderArgs, p.args...)
		} else {
			extraArgs = p.args
		}
	} else {
		extraArgs = p.args
	}

	opts := InvokeOptions{
		ExtraArgs: extraArgs,
		Settings:  p.settings,
	}

	// Use a custom invoke for postrender to handle stderr separately
	runtime, err := p.plugin.GetRuntimeInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime instance: %w", err)
	}

	// Set settings and extra args on runtime if it's a subprocess runtime
	if subprocessRuntime, ok := runtime.(*RuntimeSubprocess); ok {
		subprocessRuntime.SetSettings(opts.Settings)
		subprocessRuntime.SetExtraArgs(opts.ExtraArgs)

		// Temporarily set plugin environment variables for PrepareCommands to expand
		oldPluginName := os.Getenv("HELM_PLUGIN_NAME")
		oldPluginDir := os.Getenv("HELM_PLUGIN_DIR")

		os.Setenv("HELM_PLUGIN_NAME", subprocessRuntime.pluginName)
		os.Setenv("HELM_PLUGIN_DIR", subprocessRuntime.pluginDir)

		// Execute with separate stderr handling
		main, args, err := PrepareCommands(subprocessRuntime.config.PlatformCommand, true, opts.ExtraArgs)
		if err != nil {
			if len(subprocessRuntime.config.Command) > 0 {
				main, args, err = PrepareCommands([]PlatformCommand{{Command: subprocessRuntime.config.Command}}, true, opts.ExtraArgs)
				if err != nil {
					// Restore before returning
					if oldPluginName != "" {
						os.Setenv("HELM_PLUGIN_NAME", oldPluginName)
					} else {
						os.Unsetenv("HELM_PLUGIN_NAME")
					}
					if oldPluginDir != "" {
						os.Setenv("HELM_PLUGIN_DIR", oldPluginDir)
					} else {
						os.Unsetenv("HELM_PLUGIN_DIR")
					}
					return nil, fmt.Errorf("failed to prepare command: %w", err)
				}
			} else {
				// Restore before returning
				if oldPluginName != "" {
					os.Setenv("HELM_PLUGIN_NAME", oldPluginName)
				} else {
					os.Unsetenv("HELM_PLUGIN_NAME")
				}
				if oldPluginDir != "" {
					os.Setenv("HELM_PLUGIN_DIR", oldPluginDir)
				} else {
					os.Unsetenv("HELM_PLUGIN_DIR")
				}
				return nil, fmt.Errorf("failed to prepare command: %w", err)
			}
		}

		// Restore original values
		if oldPluginName != "" {
			os.Setenv("HELM_PLUGIN_NAME", oldPluginName)
		} else {
			os.Unsetenv("HELM_PLUGIN_NAME")
		}
		if oldPluginDir != "" {
			os.Setenv("HELM_PLUGIN_DIR", oldPluginDir)
		} else {
			os.Unsetenv("HELM_PLUGIN_DIR")
		}
		err = subprocessRuntime.executeCommand(main, args, renderedManifests, postRendered, stderr)
		if err != nil {
			return nil, fmt.Errorf("error while running command %s. error output:\n%s: %w", p.plugin.GetName(), stderr.String(), err)
		}
	} else {
		// For non-subprocess runtimes, just use the standard invoke
		err = runtime.Invoke(renderedManifests, postRendered)
		if err != nil {
			return nil, fmt.Errorf("error while running plugin %s: %w", p.plugin.GetName(), err)
		}
	}

	// If the binary returned almost nothing, it's likely that it didn't
	// successfully render anything
	if len(bytes.TrimSpace(postRendered.Bytes())) == 0 {
		return nil, fmt.Errorf("post-renderer %q produced empty output", p.plugin.GetName())
	}

	return postRendered, nil
}
