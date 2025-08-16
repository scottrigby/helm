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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeekAPIVersion(t *testing.T) {
	testCases := map[string]struct {
		data     []byte
		expected string
	}{
		"v1": {
			data: []byte(`---
apiVersion: v1
name: "test-plugin"
`),
			expected: "v1",
		},
		"legacy": { // No apiVersion field
			data: []byte(`---
name: "test-plugin"
`),
			expected: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			version, err := peekAPIVersion(bytes.NewReader(tc.data))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, version)
		})
	}

	// invalid yaml
	{
		data := []byte(`bad yaml`)
		_, err := peekAPIVersion(bytes.NewReader(data))
		assert.Error(t, err)
	}
}

func TestLoadDir(t *testing.T) {
	dirname := "testdata/plugdir/good/hello"

	expect := Metadata{
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

	plug, err := LoadDir(dirname)
	require.NoError(t, err, "error loading plugin from %s", dirname)

	assert.Equal(t, dirname, plug.Dir())
	assert.EqualValues(t, expect, plug.Metadata())
}

func TestLoadDirDuplicateEntries(t *testing.T) {
	dirname := "testdata/plugdir/bad/duplicate-entries"
	if _, err := LoadDir(dirname); err == nil {
		t.Errorf("successfully loaded plugin with duplicate entries when it should've failed")
	}
}

func TestLoadDirGetter(t *testing.T) {
	dirname := "testdata/plugdir/good/getter"

	expect := Metadata{
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

	plug, err := LoadDir(dirname)
	require.NoError(t, err)
	assert.Equal(t, dirname, plug.Dir())
	assert.Equal(t, expect, plug.Metadata())
}

func TestPostRenderer(t *testing.T) {
	dirname := "testdata/plugdir/good/postrenderer"

	expect := Metadata{
		Name:       "postrenderer",
		Version:    "1.2.3",
		Type:       "postrenderer/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config: &ConfigPostrenderer{
			PostrendererArgs: []string{},
		},
		RuntimeConfig: &RuntimeConfigSubprocess{
			PlatformCommand: []PlatformCommand{
				{
					Command: "${HELM_PLUGIN_DIR}/sed-test.sh",
				},
			},
		},
	}

	plug, err := LoadDir(dirname)
	require.NoError(t, err)
	assert.Equal(t, dirname, plug.Dir())
	assert.Equal(t, expect, plug.Metadata())
}

func TestDetectDuplicates(t *testing.T) {
	plugs := []Plugin{
		mockSubprocessCLIPlugin(t, "foo"),
		mockSubprocessCLIPlugin(t, "bar"),
	}
	if err := detectDuplicates(plugs); err != nil {
		t.Error("no duplicates in the first set")
	}
	plugs = append(plugs, mockSubprocessCLIPlugin(t, "foo"))
	if err := detectDuplicates(plugs); err == nil {
		t.Error("duplicates in the second set")
	}
}

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
	assert.Equal(t, "echo", plugs[0].Metadata().Name)
	assert.Equal(t, "getter", plugs[1].Metadata().Name)
	assert.Equal(t, "hello", plugs[2].Metadata().Name)
	assert.Equal(t, "postrenderer", plugs[3].Metadata().Name)
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
