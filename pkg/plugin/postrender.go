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
	"fmt"

	"helm.sh/helm/v4/pkg/cli"
)

// PostRenderer is an interface different plugin runtimes
// it may be also be used without the factory for custom post-renderers
type PostRenderer interface {
	// Run expects a single buffer filled with Helm rendered manifests. It
	// expects the modified results to be returned on a separate buffer or an
	// error if there was an issue or failure while running the post render step
	Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error)
}

// NewPostRenderer creates a PostRenderer that uses the plugin's runtime
func NewPostRenderer(settings *cli.EnvSettings, pluginName string, args ...string) (PostRenderer, error) {
	p, err := FindPlugin(pluginName, settings.PluginsDirectory, "postrender")
	if err != nil {
		return nil, err
	}

	// Verify this is a postrender plugin
	config := p.Metadata().Config
	if _, ok := config.(*ConfigPostrender); !ok {
		return nil, fmt.Errorf("plugin %s is not a postrender plugin", pluginName)
	}

	return &runtimePostRenderer{
		plugin:   p,
		args:     args,
		settings: settings,
	}, nil
}

// runtimePostRenderer implements PostRenderer by delegating to the plugin's runtime
type runtimePostRenderer struct {
	plugin   Plugin
	args     []string
	settings *cli.EnvSettings
}

// Run implements PostRenderer by using the plugin's runtime
func (r *runtimePostRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// For subprocess runtime, configure settings
	if subprocessRuntime, ok := r.plugin.(*RuntimeSubprocess); ok {
		subprocessRuntime.SetSettings(r.settings)
	}

	// Use the runtime's Postrender method
	// return r.plugin.PostRender(renderedManifests, r.args) TODO: need to fix this
	return nil, nil
}
