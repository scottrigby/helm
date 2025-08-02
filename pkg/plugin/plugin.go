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
	"bytes"
	"context"
	"io"

	"helm.sh/helm/v4/pkg/cli"
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

// Plugin interface defines the common methods that all plugin versions must implement
type Plugin interface {
	GetDir() string
	Metadata() Metadata
	Invoke(ctx context.Context, input *Input) (*Output, error)
	InvokeWithEnv(main string, argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
	InvokeHook(event string) error
	Postrender(renderedManifests *bytes.Buffer, args []string, extraArgs []string, settings *cli.EnvSettings) (*bytes.Buffer, error)
}

// Input defines the input message and parameters to be passed to the plugin
type Input struct {
	// Message represents the type-elided value to be passed to the plugin.
	// The plugin is expected to interpret the message according to its type/version
	// The message object must be JSON-serializable
	Message any

	// Optional: Reader to be consumed plugin's "stdin"
	Stdin io.Reader

	// Optional: Writers to consume the plugin's "stdout" and "stderr"
	Stdout, Stderr io.Writer

	// Env represents the environment as a list of "key=value" strings
	// see os.Environ
	Env []string
}

// Output defines the output message and parameters the passed from the plugin
type Output struct {
	// Message represents the type-elided value passed from the plugin
	// The invoker is expected to interpret the message according to the plugins type/version
	Message any
}
