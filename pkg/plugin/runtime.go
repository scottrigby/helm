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
	"io"
)

// Runtime interface defines the methods that all plugin runtimes must implement
type Runtime interface {
	Invoke(stdin io.Reader, stdout, stderr io.Writer, env []string) error
	InvokeHook(event string) error
	// Postrender executes the plugin as a post-renderer with rendered manifests
	// This method should only be called when the plugin type is "postrender"
	Postrender(renderedManifests *bytes.Buffer, args []string) (*bytes.Buffer, error)
}

// RuntimeConfig interface defines the methods that all runtime configurations must implement
type RuntimeConfig interface {
	GetRuntimeType() string
	Validate() error
	CreateRuntime(pluginDir string, pluginName string) (Runtime, error)
}
