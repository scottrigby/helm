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
	"strings"
	"unicode"
)

func (m *MetadataLegacy) Validate() error {
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin name")
	}
	m.Usage = sanitizeString(m.Usage)

	if len(m.PlatformCommand) > 0 && len(m.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set")
	}

	if len(m.PlatformHooks) > 0 && len(m.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set")
	}

	// Validate downloader plugins
	for i, downloader := range m.Downloaders {
		if downloader.Command == "" {
			return fmt.Errorf("downloader %d has empty command", i)
		}
		if len(downloader.Protocols) == 0 {
			return fmt.Errorf("downloader %d has no protocols", i)
		}
		for j, protocol := range downloader.Protocols {
			if protocol == "" {
				return fmt.Errorf("downloader %d has empty protocol at index %d", i, j)
			}
		}
	}

	return nil
}

// sanitizeString normalize spaces and removes non-printable characters.
func sanitizeString(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, str)
}
