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

	"sigs.k8s.io/yaml"
)

// Config interface defines the methods that all plugin type configurations must implement
type Config interface {
	GetType() string
	Validate() error
}

// ConfigCLI represents the configuration for CLI plugins
type ConfigCLI struct {
	// Usage is the single-line usage text shown in help
	// For recommended syntax, see [spf13/cobra.command.Command] Use field comment:
	// https://pkg.go.dev/github.com/spf13/cobra#Command
	Usage string `json:"usage"`
	// ShortHelp is the short description shown in the 'helm help' output
	ShortHelp string `json:"shortHelp"`
	// LongHelp is the long message shown in the 'helm help <this-command>' output
	LongHelp string `json:"longHelp"`
	// IgnoreFlags ignores any flags passed in from Helm
	IgnoreFlags bool `json:"ignoreFlags"`
}

// ConfigDownload represents the configuration for download plugins
type ConfigGetter struct {
	// Protocols are the list of URL schemes supported by this downloader
	Protocols []string `json:"protocols"`
}

// ConfigPostrender represents the configuration for postrender plugins
type ConfigPostrender struct {
	// PostrenderArgs are arguments passed to the postrender command
	// TODO: remove this field. it is not needed as args are passed from CLI to the plugin
	PostrenderArgs []string `json:"postrenderArgs"`
}

// GetType implementations for Config types
func (c *ConfigCLI) GetType() string        { return "cli" }
func (c *ConfigGetter) GetType() string     { return "download" }
func (c *ConfigPostrender) GetType() string { return "postrender" }

// Validate implementations for Config types
func (c *ConfigCLI) Validate() error {
	// Config validation for CLI plugins
	return nil
}

func (c *ConfigGetter) Validate() error {
	if len(c.Protocols) == 0 {
		return fmt.Errorf("getter has no protocols")
	}
	for i, protocol := range c.Protocols {
		if protocol == "" {
			return fmt.Errorf("getter has empty protocol at index %d", i)
		}
	}

	return nil
}

func (c *ConfigPostrender) Validate() error {
	// Config validation for postrender plugins
	return nil
}

// unmarshalConfigCLI unmarshals a config map into a ConfigCLI struct
func unmarshalConfigCLI(configData map[string]interface{}) (*ConfigCLI, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigCLI
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// unmarshalConfigGetter unmarshals a config map into a ConfigGetter struct
func unmarshalConfigGetter(configData map[string]interface{}) (*ConfigGetter, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigGetter
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// unmarshalConfigPostrender unmarshals a config map into a ConfigPostrender struct
func unmarshalConfigPostrender(configData map[string]interface{}) (*ConfigPostrender, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigPostrender
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
