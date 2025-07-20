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
	"runtime"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/cli"
)

func TestRuntimeSubprocessInvoke(t *testing.T) {
	// Skip on Windows for now
	if runtime.GOOS == "windows" {
		t.Skip("Skipping subprocess test on Windows")
	}

	// Create a simple subprocess runtime
	config := &RuntimeConfigSubprocess{
		Command: "echo",
	}

	rt, err := config.CreateRuntime("/tmp", "test-plugin")
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}

	// Set extra args
	if subprocessRuntime, ok := rt.(*RuntimeSubprocess); ok {
		subprocessRuntime.SetExtraArgs([]string{"Hello", "World"})
		subprocessRuntime.SetSettings(cli.New())
	}

	// Invoke the runtime
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	err = rt.Invoke(in, out)
	if err != nil {
		t.Fatalf("Failed to invoke runtime: %v", err)
	}

	// Check output
	output := strings.TrimSpace(out.String())
	expected := "Hello World"
	if output != expected {
		t.Errorf("Expected output %q, got %q", expected, output)
	}
}

func TestRuntimeWasmNotImplemented(t *testing.T) {
	config := &RuntimeConfigWasm{
		WasmModule: "test.wasm",
	}

	rt, err := config.CreateRuntime("/tmp", "test-plugin")
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}

	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	err = rt.Invoke(in, out)
	if err == nil {
		t.Fatal("Expected error for unimplemented WASM runtime")
	}

	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

func TestInvokePlugin(t *testing.T) {
	// Skip on Windows for now
	if runtime.GOOS == "windows" {
		t.Skip("Skipping subprocess test on Windows")
	}

	// Create a test plugin
	plug := &PluginV1{
		MetadataV1: &MetadataV1{
			Name:       "test",
			Type:       "cli",
			APIVersion: "v1",
			Runtime:    "subprocess",
			Config: &ConfigCLI{
				IgnoreFlags: false,
			},
			RuntimeConfig: &RuntimeConfigSubprocess{
				Command: "echo",
			},
		},
		Dir: "/tmp",
	}

	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	opts := InvokeOptions{
		ExtraArgs: []string{"Plugin", "Test"},
		Settings:  cli.New(),
	}

	err := InvokePlugin(plug, in, out, opts)
	if err != nil {
		t.Fatalf("Failed to invoke plugin: %v", err)
	}

	// Check output
	output := strings.TrimSpace(out.String())
	expected := "Plugin Test"
	if output != expected {
		t.Errorf("Expected output %q, got %q", expected, output)
	}
}