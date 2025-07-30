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

package plugin // import "helm.sh/helm/v4/pkg/plugin"

import (
	"context"
	"io"
)

const PluginFileName = "plugin.yaml"

// Downloaders represents the plugins capability if it can retrieve
// charts from special sources
type Downloaders struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `json:"protocols"`
	// Command is the executable path with which the plugin performs
	// the actual download for the corresponding Protocols
	Command string `json:"command"`
}

// Input defined the input message and parameters to be passed to the plugin
type Input struct {
	// Message represents the type-elided value to be passed to the plugin
	// The plugin is expected to interpret the message according to its type/version
	// The message object must be JSON-serializable
	Message any

	// Optional: Reader to be consumed plugin's "stdin"
	Stdin io.Reader

	// Optional: Writers to consume the plugin's "stdout" and "stderr"
	Stdout, Stderr io.Writer
}

// Input defined the output message and parameters the passed from the plugin
type Output struct {
	// Message represents the type-elided value passed from the plugin
	// The invoker is expected to interpret the message according to the plugins type/version
	Message any
}

// Plugin defines the "invokable" interface for a plugin, as well a getter for the plugin's describing manifest
// The invoke method can be thought of request/response message passing between the plugin invoker and the plugin itself
type Plugin interface {
	Metadata() MetadataV1
	Dir() string
	Invoke(ctx context.Context, input *Input) (*Output, error)
}
