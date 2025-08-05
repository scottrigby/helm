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
	Type() string
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

// ConfigGetter represents the configuration for download plugins
type ConfigGetter struct {
	// Protocols are the list of URL schemes supported by this downloader
	Protocols []string `json:"protocols"`
}

// ConfigPostrenderer represents the configuration for postrenderer plugins
type ConfigPostrenderer struct {
	// PostrendererArgs are arguments passed to the post-renderer plugin
	// TODO: remove this field. it is not needed as args are passed from CLI to the plugin
	PostrendererArgs []string `json:"postrendererArgs"`
}

func (c *ConfigCLI) Type() string          { return "cli/v1" }
func (c *ConfigGetter) Type() string       { return "getter/v1" }
func (c *ConfigPostrenderer) Type() string { return "postrenderer/v1" }

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

func (c *ConfigPostrenderer) Validate() error {
	// Config validation for postrenderer plugins
	return nil
}

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

func unmarshalConfigPostrenderer(configData map[string]interface{}) (*ConfigPostrenderer, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config ConfigPostrenderer
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
