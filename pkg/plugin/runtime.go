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
	"context"
	"io"
)

// Runtime interface defines the methods that all plugin runtimes must implement
type Runtime interface {
	invoke(ctx context.Context, input *Input) (*Output, error)
	invokeHook(event string) error
	invokeWithEnv(main string, argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}

// RuntimeConfig interface defines the methods that all runtime configurations must implement
type RuntimeConfig interface {
	Type() string
	Validate() error
	CreateRuntime(pluginDir string, pluginName string, pluginType string) (Runtime, error)
}
