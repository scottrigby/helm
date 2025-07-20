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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/plugin"
)

const pluginHelp = `
Manage client-side Helm plugins.
`

func newPluginCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "install, list, or uninstall Helm plugins",
		Long:  pluginHelp,
	}
	cmd.AddCommand(
		newPluginInstallCmd(out),
		newPluginListCmd(out),
		newPluginUninstallCmd(out),
		newPluginUpdateCmd(out),
	)
	return cmd
}

// runHook will execute a plugin hook.
func runHook(p plugin.Plugin, event string) error {
	var cmds []plugin.PlatformCommand
	expandArgs := true
	metadata := p.GetMetadata()
	switch meta := metadata.(type) {
	case *plugin.MetadataLegacy:
		cmds = meta.PlatformHooks[event]
		if len(cmds) == 0 && len(meta.Hooks) > 0 {
			cmd := meta.Hooks[event]
			if len(cmd) > 0 {
				cmds = []plugin.PlatformCommand{{Command: "sh", Args: []string{"-c", cmd}}}
				expandArgs = false
			}
		}
	case *plugin.MetadataV1:
		// V1 plugins store hooks in runtime config, not directly in metadata
		runtimeConfig := p.GetRuntimeConfig()
		if runtimeConfig != nil {
			if subprocessConfig, ok := runtimeConfig.(*plugin.RuntimeConfigSubprocess); ok {
				cmds = subprocessConfig.PlatformHooks[event]
				if len(cmds) == 0 && len(subprocessConfig.Hooks) > 0 {
					cmd := subprocessConfig.Hooks[event]
					if len(cmd) > 0 {
						cmds = []plugin.PlatformCommand{{Command: "sh", Args: []string{"-c", cmd}}}
						expandArgs = false
					}
				}
			}
		}
	default:
		return fmt.Errorf("unsupported plugin metadata type for hook execution")
	}

	// If no hook commands are defined, just return successfully
	if len(cmds) == 0 {
		return nil
	}

	// Prepare the command
	main, argv, err := plugin.PrepareCommands(cmds, expandArgs, []string{})
	if err != nil {
		return err
	}

	// Create a temporary runtime config for the hook command
	tempRuntimeConfig := &plugin.RuntimeConfigSubprocess{
		Command: main,
	}

	tempRuntime, err := tempRuntimeConfig.CreateRuntime(p.GetDir(), p.GetName())
	if err != nil {
		return fmt.Errorf("failed to create runtime for hook: %w", err)
	}

	if subprocessRuntime, ok := tempRuntime.(*plugin.RuntimeSubprocess); ok {
		subprocessRuntime.SetSettings(settings)
		subprocessRuntime.SetExtraArgs(argv)
	}

	slog.Debug("running hook", "event", event, "command", main, "args", argv)

	// Run the hook with no input
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	if err := tempRuntime.Invoke(in, out); err != nil {
		// Write any output to stdout/stderr
		os.Stdout.Write(out.Bytes())
		return fmt.Errorf("plugin %s hook for %q exited with error: %w", event, p.GetName(), err)
	}

	// Write successful output
	os.Stdout.Write(out.Bytes())
	return nil
}
