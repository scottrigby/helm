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

package plugin // import "helm.sh/helm/v4/pkg/plugin"

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
)

// TODO add tests for both legacy and v1 plugins
func TestPrepareCommand(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	p := &PluginV1{
		Dir: "/tmp", // Unused
		MetadataV1: &MetadataV1{
			Name:       "test",
			Type:       "cli/v1",
			APIVersion: "v1",
			Runtime:    "subprocess",
			Config: &ConfigCLI{
				IgnoreFlags: false,
			},
			RuntimeConfig: &RuntimeConfigSubprocess{
				Command: "echo \"error\"",
				PlatformCommand: []PlatformCommand{
					{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
				},
			},
		},
	}

	if subprocessConfig, ok := p.MetadataV1.RuntimeConfig.(*RuntimeConfigSubprocess); ok {
		cmds := subprocessConfig.PlatformCommand
		cmd, args, err := PrepareCommands(cmds, true, []string{})
		if err != nil {
			t.Fatal(err)
		}
		if cmd != cmdMain {
			t.Fatalf("Expected %q, got %q", cmdMain, cmd)
		}
		if !reflect.DeepEqual(args, cmdArgs) {
			t.Fatalf("Expected %v, got %v", cmdArgs, args)
		}
	}
}

func TestPrepareCommandExtraArgs(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}
	extraArgs := []string{"--debug", "--foo", "bar"}

	p := &PluginV1{
		Dir: "/tmp", // Unused
		MetadataV1: &MetadataV1{
			Name:       "test",
			Type:       "cli/v1",
			APIVersion: "v1",
			Runtime:    "subprocess",
			Config: &ConfigCLI{
				IgnoreFlags: false,
			},
			RuntimeConfig: &RuntimeConfigSubprocess{
				Command: "echo \"error\"",
				PlatformCommand: []PlatformCommand{
					{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
					{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				},
			},
		},
	}

	expectedArgs := append(cmdArgs, extraArgs...)

	// extra args are expected when ignoreFlags is unset or false
	if cliConfig, ok := p.Metadata().GetConfig().(*ConfigCLI); ok {
		if cliConfig.IgnoreFlags {
			extraArgs = []string{}
		}
	}
	if subprocessConfig, ok := p.MetadataV1.RuntimeConfig.(*RuntimeConfigSubprocess); ok {
		cmds := subprocessConfig.PlatformCommand
		cmd, args, err := PrepareCommands(cmds, true, extraArgs)
		if err != nil {
			t.Fatal(err)
		}
		if cmd != cmdMain {
			t.Fatalf("Expected %q, got %q", cmdMain, cmd)
		}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Fatalf("Expected %v, got %v", expectedArgs, args)
		}
	}
}

func TestPrepareCommandExtraArgsIgnored(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}
	extraArgs := []string{"--debug", "--foo", "bar"}

	p := &PluginV1{
		Dir: "/tmp", // Unused
		MetadataV1: &MetadataV1{
			Name:       "test",
			Type:       "cli/v1",
			APIVersion: "v1",
			Runtime:    "subprocess",
			Config: &ConfigCLI{
				IgnoreFlags: true,
			},
			RuntimeConfig: &RuntimeConfigSubprocess{
				Command: "echo \"error\"",
				PlatformCommand: []PlatformCommand{
					{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
					{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
					{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				},
			},
		},
	}

	// no extra args if ignoreFlags is set
	if cliConfig, ok := p.Metadata().GetConfig().(*ConfigCLI); ok {
		if cliConfig.IgnoreFlags {
			extraArgs = []string{}
		}
	}
	if subprocessConfig, ok := p.Metadata().GetRuntimeConfig().(*RuntimeConfigSubprocess); ok {
		cmds := subprocessConfig.PlatformCommand
		cmd, args, err := PrepareCommands(cmds, true, extraArgs)
		if err != nil {
			t.Fatal(err)
		}
		if cmd != cmdMain {
			t.Fatalf("Expected %q, got %q", cmdMain, cmd)
		}
		if !reflect.DeepEqual(args, cmdArgs) {
			t.Fatalf("Expected %v, got %v", cmdArgs, args)
		}
	}
}

func TestPrepareCommands(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	cmd, args, err := PrepareCommands(cmds, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandsExtraArgs(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}
	extraArgs := []string{"--debug", "--foo", "bar"}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	expectedArgs := append(cmdArgs, extraArgs...)

	cmd, args, err := PrepareCommands(cmds, true, extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("Expected %v, got %v", expectedArgs, args)
	}
}

func TestPrepareCommandsNoArch(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	cmd, args, err := PrepareCommands(cmds, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandsNoOsNoArch(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: "", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	cmd, args, err := PrepareCommands(cmds, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandsNoMatch(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: "no-os", Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
	}

	if _, _, err := PrepareCommands(cmds, true, []string{}); err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandsNoCommands(t *testing.T) {
	cmds := []PlatformCommand{}

	if _, _, err := PrepareCommands(cmds, true, []string{}); err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandsExpand(t *testing.T) {
	t.Setenv("TEST", "test")
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"${TEST}\""}
	cmds := []PlatformCommand{
		{OperatingSystem: "", Architecture: "", Command: cmdMain, Args: cmdArgs},
	}

	expectedArgs := []string{"-c", "echo \"test\""}

	cmd, args, err := PrepareCommands(cmds, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("Expected %v, got %v", expectedArgs, args)
	}
}

func TestPrepareCommandsNoExpand(t *testing.T) {
	t.Setenv("TEST", "test")
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"${TEST}\""}
	cmds := []PlatformCommand{
		{OperatingSystem: "", Architecture: "", Command: cmdMain, Args: cmdArgs},
	}

	cmd, args, err := PrepareCommands(cmds, false, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestLoadDir(t *testing.T) {
	dirname := "testdata/plugdir/good/hello"
	plug, err := LoadDir(dirname)
	if err != nil {
		t.Fatalf("error loading Hello plugin: %s", err)
	}

	if plug.GetDir() != dirname {
		t.Fatalf("Expected dir %q, got %q", dirname, plug.GetDir())
	}

	expect := &MetadataV1{
		Name:       "hello",
		Version:    "0.1.0",
		Type:       "cli/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config: &ConfigCLI{
			Usage:       "hello [params]...",
			ShortHelp:   "echo hello message",
			LongHelp:    "description",
			IgnoreFlags: true,
		},
		RuntimeConfig: &RuntimeConfigSubprocess{
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "${HELM_PLUGIN_DIR}/hello.sh"}},
				{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "${HELM_PLUGIN_DIR}/hello.ps1"}},
			},
			PlatformHooks: map[string][]PlatformCommand{
				Install: {
					{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
					{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"installing...\""}},
				},
			},
		},
	}

	assert.Equal(t, expect, plug.Metadata())
}

func TestLoadDirDuplicateEntries(t *testing.T) {
	dirname := "testdata/plugdir/bad/duplicate-entries"
	if _, err := LoadDir(dirname); err == nil {
		t.Errorf("successfully loaded plugin with duplicate entries when it should've failed")
	}
}

func TestGetter(t *testing.T) {
	dirname := "testdata/plugdir/good/getter"
	plug, err := LoadDir(dirname)
	assert.NoError(t, err)

	expect := &MetadataV1{
		Name:       "getter",
		Version:    "1.2.3",
		Type:       "getter/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config: &ConfigGetter{
			Protocols: []string{"myprotocol", "myprotocols"},
		},
		RuntimeConfig: &RuntimeConfigSubprocess{
			ProtocolCommands: []SubprocessProtocolCommand{
				{
					Protocols: []string{"myprotocol", "myprotocols"},
					Command:   "echo getter",
				},
			},
		},
	}

	assert.Equal(t, dirname, plug.GetDir())
	assert.Equal(t, expect, plug.Metadata())
}

func TestPostRenderer(t *testing.T) {
	dirname := "testdata/plugdir/good/postrender"
	plug, err := LoadDir(dirname)
	assert.NoError(t, err)

	expect := &MetadataV1{
		Name:       "postrender",
		Version:    "1.2.3",
		Type:       "postrender/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config: &ConfigPostrender{
			PostrenderArgs: []string{},
		},
		RuntimeConfig: &RuntimeConfigSubprocess{
			PlatformCommand: []PlatformCommand{
				{
					Command: "${HELM_PLUGIN_DIR}/sed-test.sh",
				},
			},
		},
	}

	assert.Equal(t, dirname, plug.GetDir())
	assert.Equal(t, expect, plug.Metadata())
}

//func TestNewPostRenderPluginRunWithNoOutput(t *testing.T) {
//	if runtime.GOOS == "windows" {
//		// the actual Run test uses a basic sed example, so skip this test on windows
//		t.Skip("skipping on windows")
//	}
//	is := assert.New(t)
//	s := cli.New()
//	s.PluginsDirectory = "testdata/plugdir/good"
//	name := "postrender"
//	base := filepath.Join(s.PluginsDirectory, name)
//	SetupPluginEnv(s, name, base)
//
//	renderer, err := postrender.NewPostRenderPlugin(s, name, "")
//	require.NoError(t, err)
//
//	_, err = renderer.Run(bytes.NewBufferString(""))
//	is.Error(err)
//}
//
//func TestNewPostRenderPluginWithOneArgsRun(t *testing.T) {
//	if runtime.GOOS == "windows" {
//		// the actual Run test uses a basic sed example, so skip this test on windows
//		t.Skip("skipping on windows")
//	}
//	is := assert.New(t)
//	s := cli.New()
//	s.PluginsDirectory = "testdata/plugdir/good"
//	name := "postrender"
//	base := filepath.Join(s.PluginsDirectory, name)
//	SetupPluginEnv(s, name, base)
//
//	renderer, err := postrender.NewPostRenderPlugin(s, name, "ARG1")
//	require.NoError(t, err)
//
//	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
//	is.NoError(err)
//	is.Contains(output.String(), "ARG1")
//}
//
//func TestNewPostRenderPluginWithTwoArgsRun(t *testing.T) {
//	if runtime.GOOS == "windows" {
//		// the actual Run test uses a basic sed example, so skip this test on windows
//		t.Skip("skipping on windows")
//	}
//	is := assert.New(t)
//	s := cli.New()
//	s.PluginsDirectory = "testdata/plugdir/good"
//	name := "postrender"
//	base := filepath.Join(s.PluginsDirectory, name)
//	SetupPluginEnv(s, name, base)
//
//	renderer, err := postrender.NewPostRenderPlugin(s, name, "ARG1", "ARG2")
//	require.NoError(t, err)
//
//	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
//	is.NoError(err)
//	is.Contains(output.String(), "ARG1 ARG2")
//}

func TestLoadAll(t *testing.T) {
	// Verify that empty dir loads:
	{
		plugs, err := LoadAll("testdata")
		require.NoError(t, err)
		assert.Len(t, plugs, 0)
	}

	basedir := "testdata/plugdir/good"
	plugs, err := LoadAll(basedir)
	require.NoError(t, err)

	assert.Len(t, plugs, 4)
	assert.Equal(t, "echo", plugs[0].Metadata().GetName())
	assert.Equal(t, "getter", plugs[1].Metadata().GetName())
	assert.Equal(t, "hello", plugs[2].Metadata().GetName())
	assert.Equal(t, "postrender", plugs[3].Metadata().GetName())
}

func TestFindPlugins(t *testing.T) {
	cases := []struct {
		name     string
		plugdirs string
		expected int
	}{
		{
			name:     "plugdirs is empty",
			plugdirs: "",
			expected: 0,
		},
		{
			name:     "plugdirs isn't dir",
			plugdirs: "./plugin_test.go",
			expected: 0,
		},
		{
			name:     "plugdirs doesn't have plugin",
			plugdirs: ".",
			expected: 0,
		},
		{
			name:     "normal",
			plugdirs: "./testdata/plugdir/good",
			expected: 4,
		},
	}
	for _, c := range cases {
		t.Run(t.Name(), func(t *testing.T) {
			plugin, _ := LoadAll(c.plugdirs)
			if len(plugin) != c.expected {
				t.Errorf("expected: %v, got: %v", c.expected, len(plugin))
			}
		})
	}
}

func TestSetupEnv(t *testing.T) {
	name := "pequod"
	base := filepath.Join("testdata/helmhome/helm/plugins", name)

	s := cli.New()
	s.PluginsDirectory = "testdata/helmhome/helm/plugins"

	SetupPluginEnv(s, name, base)
	for _, tt := range []struct {
		name, expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}

func TestSetupEnvWithSpace(t *testing.T) {
	name := "sureshdsk"
	base := filepath.Join("testdata/helm home/helm/plugins", name)

	s := cli.New()
	s.PluginsDirectory = "testdata/helm home/helm/plugins"

	SetupPluginEnv(s, name, base)
	for _, tt := range []struct {
		name, expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}

func TestValidatePluginData(t *testing.T) {
	// A mock plugin missing any metadata.
	mockMissingMeta := &PluginV1{
		Dir: "no-such-dir",
	}

	// A mock plugin with no commands
	mockNoCommand := mockPlugin("foo")
	mockNoCommand.MetadataV1.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{},
		PlatformHooks:   map[string][]PlatformCommand{},
	}

	// A mock plugin with legacy commands
	mockLegacyCommand := mockPlugin("foo")
	mockLegacyCommand.MetadataV1.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{},
		Command:         "echo \"mock plugin\"",
		PlatformHooks:   map[string][]PlatformCommand{},
		Hooks: map[string]string{
			Install: "echo installing...",
		},
	}

	// A mock plugin with a command also set
	mockWithCommand := mockPlugin("foo")
	mockWithCommand.MetadataV1.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{
			{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
		},
		Command: "echo \"mock plugin\"",
	}

	// A mock plugin with a hooks also set
	mockWithHooks := mockPlugin("foo")
	mockWithHooks.MetadataV1.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{
			{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
		},
		PlatformHooks: map[string][]PlatformCommand{
			Install: {
				{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
			},
		},
		Hooks: map[string]string{
			Install: "echo installing...",
		},
	}

	for i, item := range []struct {
		pass bool
		plug *PluginV1
	}{
		{true, mockPlugin("abcdefghijklmnopqrstuvwxyz0123456789_-ABC")},
		{true, mockPlugin("foo-bar-FOO-BAR_1234")},
		{false, mockPlugin("foo -bar")},
		{false, mockPlugin("$foo -bar")}, // Test leading chars
		{false, mockPlugin("foo -bar ")}, // Test trailing chars
		{false, mockPlugin("foo\nbar")},  // Test newline
		{false, mockMissingMeta},         // Test if the metadata section missing
		{true, mockNoCommand},            // Test no command metadata works
		{true, mockLegacyCommand},        // Test legacy command metadata works
		{false, mockWithCommand},         // Test platformCommand and command both set fails
		{false, mockWithHooks},           // Test platformHooks and hooks both set fails
	} {
		err := item.plug.Validate()
		if item.pass && err != nil {
			t.Errorf("failed to validate case %d: %s", i, err)
		} else if !item.pass && err == nil {
			t.Errorf("expected case %d to fail", i)
		}
	}
}

func TestDetectDuplicates(t *testing.T) {
	plugs := []Plugin{
		mockPlugin("foo"),
		mockPlugin("bar"),
	}
	if err := detectDuplicates(plugs); err != nil {
		t.Error("no duplicates in the first set")
	}
	plugs = append(plugs, mockPlugin("foo"))
	if err := detectDuplicates(plugs); err == nil {
		t.Error("duplicates in the second set")
	}
}

func mockPlugin(name string) *PluginV1 {
	return &PluginV1{
		MetadataV1: &MetadataV1{
			Name:       name,
			Version:    "v0.1.2",
			Type:       "cli/v1",
			APIVersion: "v1",
			Runtime:    "subprocess",
			Config: &ConfigCLI{
				Usage:       "Mock plugin",
				ShortHelp:   "Mock plugin",
				LongHelp:    "Mock plugin for testing",
				IgnoreFlags: false,
			},
			RuntimeConfig: &RuntimeConfigSubprocess{
				PlatformCommand: []PlatformCommand{
					{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
					{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"mock plugin\""}},
				},
				PlatformHooks: map[string][]PlatformCommand{
					Install: {
						{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
						{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"installing...\""}},
					},
				},
			},
		},
		Dir: "no-such-dir",
	}
}
