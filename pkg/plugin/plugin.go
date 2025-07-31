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
	GetRuntimeInstance() (Runtime, error)
	Validate() error
	PrepareCommand(extraArgs []string) (string, []string, error)
}

// Error is returned when a plugin exits with a non-zero status code
type Error struct {
	Err        error
	PluginName string
	Code       int
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Err.Error()
}
