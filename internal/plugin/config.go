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
	"go.yaml.in/yaml/v3"
)

// Config represents the assertable type for each plugin type's config.
// It is expected to type assert (cast) the a Config to its expected underlying type (schema.ConfigCLIV1, schema.ConfigGetterV1, etc).
type Config interface {
	Validate() error
}

func remarshalConfig[T Config](configData map[string]any) (Config, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}
