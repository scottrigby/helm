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

package transport

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"helm.sh/helm/v4/pkg/registry"
)

// OCITransport handles OCI registry requests.
type OCITransport struct {
	client *registry.Client
}

// NewOCITransport creates a new OCI transport.
func NewOCITransport(options ...Option) (Transport, error) {
	opts := &Options{}
	for _, opt := range options {
		opt(opts)
	}

	var clientOpts []registry.ClientOption
	if opts.PlainHTTP {
		clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
	}

	client, err := registry.NewClient(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client: %w", err)
	}

	return &OCITransport{
		client: client,
	}, nil
}

// GetClient returns the registry client.
func (o *OCITransport) GetClient() interface{} {
	return o.client
}

// SetClient sets the registry client.
func (o *OCITransport) SetClient(client interface{}) {
	if registryClient, ok := client.(*registry.Client); ok {
		o.client = registryClient
	}
}

// Get fetches data from an OCI registry.
func (o *OCITransport) Get(url string, options ...Option) (*bytes.Buffer, error) {
	opts := &Options{}
	for _, opt := range options {
		opt(opts)
	}

	ref := strings.TrimPrefix(url, fmt.Sprintf("%s://", registry.OCIScheme))

	if version := opts.Version; version != "" && !strings.Contains(path.Base(ref), ":") {
		ref = fmt.Sprintf("%s:%s", ref, version)
	}

	// Check if this is a provenance file request
	requestingProv := strings.HasSuffix(ref, ".prov")
	if requestingProv {
		ref = strings.TrimSuffix(ref, ".prov")
	}

	// Handle different artifact types
	switch opts.ArtifactType {
	case "plugin":
		return o.getPlugin(ref, requestingProv)
	default: // chart or unspecified
		return o.getChart(ref, requestingProv)
	}
}

// getChart fetches a chart from OCI registry.
func (o *OCITransport) getChart(ref string, requestingProv bool) (*bytes.Buffer, error) {
	var pullOpts []registry.PullOption
	if requestingProv {
		pullOpts = append(pullOpts,
			registry.PullOptWithChart(false),
			registry.PullOptWithProv(true))
	}

	result, err := o.client.Pull(ref, pullOpts...)
	if err != nil {
		return nil, err
	}

	if requestingProv {
		return bytes.NewBuffer(result.Prov.Data), nil
	}
	return bytes.NewBuffer(result.Chart.Data), nil
}

// getPlugin fetches a plugin from OCI registry.
func (o *OCITransport) getPlugin(ref string, requestingProv bool) (*bytes.Buffer, error) {
	// Extract plugin name from the reference
	parts := strings.Split(ref, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid OCI reference: %s", ref)
	}
	lastPart := parts[len(parts)-1]
	pluginName := lastPart
	if idx := strings.LastIndex(lastPart, ":"); idx > 0 {
		pluginName = lastPart[:idx]
	}
	if idx := strings.LastIndex(lastPart, "@"); idx > 0 {
		pluginName = lastPart[:idx]
	}

	var pullOpts []registry.PluginPullOption
	if requestingProv {
		pullOpts = append(pullOpts, registry.PullPluginOptWithProv(true))
	}

	result, err := o.client.PullPlugin(ref, pluginName, pullOpts...)
	if err != nil {
		return nil, err
	}

	if requestingProv {
		return bytes.NewBuffer(result.Prov.Data), nil
	}
	return bytes.NewBuffer(result.PluginData), nil
}
