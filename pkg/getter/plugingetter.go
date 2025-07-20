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

package getter

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/plugin"
)

// collectPlugins scans for getter plugins.
// This will load plugins according to the cli.
func collectPlugins(settings *cli.EnvSettings) (Providers, error) {
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory, "download")
	if err != nil {
		return nil, err
	}
	var result Providers
	for _, p := range plugins {
		// Get downloaders based on API version
		var downloaders []plugin.Downloaders
		switch p.GetAPIVersion() {
		case "legacy":
			if metadata, ok := p.GetMetadata().(*plugin.MetadataLegacy); ok {
				downloaders = metadata.Downloaders
			}
		case "v1":
			if metadata, ok := p.GetMetadata().(*plugin.MetadataV1); ok {
				if config, ok := metadata.Config.(*plugin.ConfigDownload); ok {
					downloaders = config.Downloaders
				}
			}
		}

		for _, downloader := range downloaders {
			result = append(result, Provider{
				Schemes: downloader.Protocols,
				New: NewPluginGetter(
					downloader.Command,
					settings,
					p.GetName(),
					p.GetDir(),
				),
			})
		}
	}
	return result, nil
}

// pluginGetter is a generic type to invoke custom downloaders,
// implemented in plugins.
type pluginGetter struct {
	command  string
	settings *cli.EnvSettings
	name     string
	base     string
	opts     options
}

// Get runs downloader plugin command
func (p *pluginGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&p.opts)
	}

	// Create a temporary runtime config for the downloader command
	commands := strings.Split(p.command, " ")
	argv := append(commands[1:], p.opts.certFile, p.opts.keyFile, p.opts.caFile, href)

	tempRuntimeConfig := &plugin.RuntimeConfigSubprocess{
		Command: filepath.Join(p.base, commands[0]),
	}

	tempRuntime, err := tempRuntimeConfig.CreateRuntime(p.base, p.name)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}

	if subprocessRuntime, ok := tempRuntime.(*plugin.RuntimeSubprocess); ok {
		subprocessRuntime.SetSettings(p.settings)
		subprocessRuntime.SetExtraArgs(argv)

		// Setup plugin-specific env vars
		envVars := make(map[string]string)
		envVars["HELM_PLUGIN_USERNAME"] = p.opts.username
		envVars["HELM_PLUGIN_PASSWORD"] = p.opts.password
		envVars["HELM_PLUGIN_PASS_CREDENTIALS_ALL"] = fmt.Sprintf("%t", p.opts.passCredentialsAll)
		subprocessRuntime.SetEnvVars(envVars)
	}

	buf := bytes.NewBuffer(nil)
	in := bytes.NewBuffer(nil)

	if err := tempRuntime.Invoke(in, buf); err != nil {
		return nil, fmt.Errorf("plugin %q exited with error: %w", p.command, err)
	}
	return buf, nil
}

// NewPluginGetter constructs a valid plugin getter
func NewPluginGetter(command string, settings *cli.EnvSettings, name, base string) Constructor {
	return func(options ...Option) (Getter, error) {
		result := &pluginGetter{
			command:  command,
			settings: settings,
			name:     name,
			base:     base,
		}
		for _, opt := range options {
			opt(&result.opts)
		}
		return result, nil
	}
}
