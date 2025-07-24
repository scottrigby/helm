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
	"os"
	"path/filepath"
	"slices"
	"strings"

	"helm.sh/helm/v4/pkg/plugin/schema"
)

type pluginExec struct {
	command string
	argv    []string
	env     []string
}

func getProtocolDownloader(downloaders []SubprocessDownloaders, protocol string) *SubprocessDownloaders {
	for _, d := range downloaders {
		if slices.Contains(d.Protocols, protocol) {
			return &d
		}
	}

	return nil
}

func convertGetter(r *RuntimeSubprocess, input *Input) (pluginExec, error) {

	msg, ok := (input.Message).(*schema.GetterInputV1)
	if !ok {
		return pluginExec{}, fmt.Errorf("expected input type schema.GetterInputV1, got %T", input)
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("helm-plugin-%s-", r.plugin.Metadata.Name))
	if err != nil {
		return pluginExec{}, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	writeTempFile := func(name string, data []byte) (string, error) {
		if len(data) == 0 {
			return "", nil
		}

		tempFile := filepath.Join(tmpDir, name)
		err := os.WriteFile(tempFile, msg.Options.Cert, 0o640)
		if err != nil {
			return "", fmt.Errorf("failed to write temporary file: %w", err)
		}
		return tempFile, nil
	}

	certFile, err := writeTempFile("cert", msg.Options.Cert)
	if err != nil {
		return pluginExec{}, err
	}

	keyFile, err := writeTempFile("key", msg.Options.Cert)
	if err != nil {
		return pluginExec{}, err
	}

	caFile, err := writeTempFile("ca", msg.Options.Cert)
	if err != nil {
		return pluginExec{}, err
	}

	d := getProtocolDownloader(r.config.Downloaders, msg.Protocol)
	if d == nil {
		return pluginExec{}, fmt.Errorf("no downloader found for protocol %q", msg.Protocol)
	}

	commands := strings.Split(d.Command, " ")
	argv := append(
		commands[1:],
		certFile,
		keyFile,
		caFile,
		msg.Href)

	env := append(
		os.Environ(),
		fmt.Sprintf("HELM_PLUGIN_USERNAME=%s", msg.Options.Username),
		fmt.Sprintf("HELM_PLUGIN_PASSWORD=%s", msg.Options.Password),
		fmt.Sprintf("HELM_PLUGIN_PASS_CREDENTIALS_ALL=%t", msg.Options.PassCredentialsAll))

	return pluginExec{
		command: commands[0],
		argv:    argv,
		env:     env,
	}, nil
}

func convertCli(r *RuntimeSubprocess, input *Input) (pluginExec, error) {
	return pluginExec{}, nil
}

func convertInput(r *RuntimeSubprocess, input *Input) (pluginExec, error) {

	switch r.plugin.Metadata.Type {
	case "getter/v1":
		return convertGetter(r, input)
	case "cli/v1":
		return convertCli(r, input)
	}

	return pluginExec{}, fmt.Errorf("unsupported subprocess plugin type %q", r.plugin.Metadata.Type)
}

func convertOutput(buf *bytes.Buffer) *Output {
	return &Output{
		Message: schema.GetterOutputV1{
			Data: buf,
		},
	}
}
