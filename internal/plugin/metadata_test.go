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
	"fmt"
	"testing"
)

func TestValidatePluginData(t *testing.T) {

	// A mock plugin with no commands
	mockNoCommand := mockSubprocessCLIPlugin(t, "foo")
	mockNoCommand.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommands: []PlatformCommand{},
		PlatformHooks:    map[string][]PlatformCommand{},
	}

	// A mock plugin with legacy commands
	mockLegacyCommand := mockSubprocessCLIPlugin(t, "foo")
	mockLegacyCommand.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommands: []PlatformCommand{
			{
				Command: "echo \"mock plugin\"",
			},
		},
		PlatformHooks: map[string][]PlatformCommand{
			Install: {
				PlatformCommand{
					Command: "echo installing...",
				},
			},
		},
	}

	for i, item := range []struct {
		pass bool
		plug Plugin
	}{
		{true, mockSubprocessCLIPlugin(t, "abcdefghijklmnopqrstuvwxyz0123456789_-ABC")},
		{true, mockSubprocessCLIPlugin(t, "foo-bar-FOO-BAR_1234")},
		{false, mockSubprocessCLIPlugin(t, "foo -bar")},
		{false, mockSubprocessCLIPlugin(t, "$foo -bar")}, // Test leading chars
		{false, mockSubprocessCLIPlugin(t, "foo -bar ")}, // Test trailing chars
		{false, mockSubprocessCLIPlugin(t, "foo\nbar")},  // Test newline
		{true, mockNoCommand},     // Test no command metadata works
		{true, mockLegacyCommand}, // Test legacy command metadata works
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			err := item.plug.Metadata().Validate()
			if item.pass && err != nil {
				t.Errorf("failed to validate case %d: %s", i, err)
			} else if !item.pass && err == nil {
				t.Errorf("expected case %d to fail", i)
			}
		})
	}
}
