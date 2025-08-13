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
	"regexp"
)

func (m *MetadataV1) Validate() error {
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin `name`")
	}

	if m.APIVersion != "v1" {
		return fmt.Errorf("invalid `apiVersion`: %q", m.APIVersion)
	}

	if m.Type == "" {
		return fmt.Errorf("`type` missing")
	}

	if m.Runtime == "" {
		return fmt.Errorf("`runtime` missing")
	}

	return nil
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")
