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
	"context"
	"fmt"
	"io"
	"regexp"

	"helm.sh/helm/v4/pkg/cli"
)

// PluginV1 represents a V1 plugin
type PluginV1 struct {
	// MetadataV1 is a parsed representation of a plugin.yaml
	MetadataV1 *MetadataV1
	// Dir is the string path to the directory that holds the plugin.
	Dir string
}

func (p *PluginV1) Invoke(ctx context.Context, input *Input) (*Output, error) {
	r, err := p.Runtime()
	if err != nil {
		return nil, err
	}
	return r.invoke(ctx, input)
}

func (p *PluginV1) InvokeWithEnv(main string, argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	r, err := p.Runtime()
	if err != nil {
		return err
	}
	return r.invokeWithEnv(main, argv, env, stdin, stdout, stderr)
}

func (p *PluginV1) InvokeHook(event string) error {
	r, err := p.Runtime()
	if err != nil {
		return err
	}
	return r.invokeHook(event)
}

func (p *PluginV1) Postrender(renderedManifests *bytes.Buffer, args []string, extraArgs []string, settings *cli.EnvSettings) (*bytes.Buffer, error) {
	r, err := p.Runtime()
	if err != nil {
		return nil, err
	}
	return r.postrender(renderedManifests, args, extraArgs, settings)
}

func (p *PluginV1) GetDir() string     { return p.Dir }
func (p *PluginV1) Metadata() Metadata { return p.MetadataV1 }

func (p *PluginV1) Runtime() (Runtime, error) {
	if p.MetadataV1.RuntimeConfig == nil {
		return nil, fmt.Errorf("plugin has no runtime configuration")
	}
	return p.MetadataV1.RuntimeConfig.CreateRuntime(p.GetDir(), p.Metadata().GetName(), p.Metadata().GetType())
}

// TODO move Metadata-specific validation to Metadata interface implementations
func (p *PluginV1) Validate() error {
	if p.MetadataV1 == nil {
		return fmt.Errorf("plugin metadata is missing")
	}

	if !validPluginName.MatchString(p.MetadataV1.Name) {
		return fmt.Errorf("invalid name")
	}

	if p.MetadataV1.APIVersion != "v1" {
		return fmt.Errorf("invalid apiVersion: %q", p.MetadataV1.APIVersion)
	}

	if p.MetadataV1.Type == "" {
		return fmt.Errorf("empty type field")
	}

	if p.MetadataV1.Runtime == "" {
		return fmt.Errorf("empty runtime field")
	}

	if p.MetadataV1.Config == nil {
		return fmt.Errorf("missing config field")
	}

	if p.MetadataV1.RuntimeConfig == nil {
		return fmt.Errorf("missing runtimeConfig field")
	}

	// Validate that config type matches plugin type
	if p.MetadataV1.Config.Type() != p.MetadataV1.Type {
		return fmt.Errorf("config type %s does not match plugin type %s", p.MetadataV1.Config.Type(), p.MetadataV1.Type)
	}

	// Validate that runtime config type matches runtime type
	if p.MetadataV1.RuntimeConfig.Type() != p.MetadataV1.Runtime {
		return fmt.Errorf("runtime config type %s does not match runtime %s", p.MetadataV1.RuntimeConfig.Type(), p.MetadataV1.Runtime)
	}

	// Validate the config itself
	if err := p.MetadataV1.Config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Validate the runtime config itself
	if err := p.MetadataV1.RuntimeConfig.Validate(); err != nil {
		return fmt.Errorf("runtime config validation failed: %w", err)
	}

	return nil
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")
