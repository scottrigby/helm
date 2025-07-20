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
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"

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
	plugin.SetupPluginEnv(settings, p.GetName(), p.GetDir())

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

	main, argv, err := plugin.PrepareCommands(cmds, expandArgs, []string{})
	if err != nil {
		return err
	}

	prog := exec.Command(main, argv...)

	slog.Debug("running hook", "event", event, "program", prog)

	prog.Stdout, prog.Stderr = os.Stdout, os.Stderr
	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return fmt.Errorf("plugin %s hook for %q exited with error", event, p.GetName())
		}
		return err
	}
	return nil
}
