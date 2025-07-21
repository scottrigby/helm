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
)

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

// execHook executes a plugin hook command
func execHook(pluginName string, event string, main string, argv []string) error {
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
