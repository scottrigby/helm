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

// Package postrender contains an interface that can be implemented for custom
// post-renderers and implementations for different runtime types
package plugin

import (
	"bytes"
	"fmt"

	"helm.sh/helm/v4/pkg/cli"
)

// PostRenderer is the interface for any type that can render a set of Kubernetes
// manifests into a different form. For example, a PostRenderer might take a
// set of Helm-generated manifests and render them into one or more Kustomize
// configurations.
//
// A PostRenderer shall not attempt to perform any templating on the input, as it
// has already been templated by Helm. The manifest output of a PostRenderer will
// be sent to Kubernetes during installation or upgrade.
type PostRenderer interface {
	// Run expects a single buffer filled with Helm rendered manifests. It
	// expects the modified results to be returned on a separate buffer or an
	// error if there was an issue or failure while running the post render step
	Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error)
}

// NewPostRenderer creates a PostRenderer based on the plugin's runtime type
func NewPostRenderer(settings *cli.EnvSettings, pluginName string, args ...string) (PostRenderer, error) {
	p, err := FindPlugin(pluginName, settings.PluginsDirectory, "postrender")
	if err != nil {
		return nil, err
	}

	// Verify this is a postrender plugin
	config := p.GetConfig()
	if _, ok := config.(*ConfigPostrender); !ok {
		return nil, fmt.Errorf("plugin %s is not a postrender plugin", pluginName)
	}

	// Get runtime instance to determine implementation
	runtime, err := p.GetRuntimeInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime instance: %w", err)
	}

	switch runtime.(type) {
	case *RuntimeSubprocess:
		// Use execRender for subprocess runtime
		return &execRender{p, args, settings}, nil
	case *RuntimeWasm:
		// Return WASM implementation when available
		return &wasmRender{p, args, settings}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime type for postrender plugin %s", pluginName)
	}
}

// wasmRender implements PostRenderer for WASM runtime
// TODO: implement WASM postrender functionality
type wasmRender struct {
	plugin   Plugin
	args     []string
	settings *cli.EnvSettings
}

// Run implements PostRenderer for WASM runtime
func (w *wasmRender) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	return nil, fmt.Errorf("WASM postrender not yet implemented")
}
