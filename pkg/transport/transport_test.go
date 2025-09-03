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
	"testing"
)

// MockTransport implements Transport for testing.
type MockTransport struct {
	data   []byte
	err    error
	config map[string]interface{}
}

func (m *MockTransport) Get(_ string, _ ...Option) (*bytes.Buffer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return bytes.NewBuffer(m.data), nil
}

func (m *MockTransport) GetConfig() map[string]interface{} {
	return m.config
}

func (m *MockTransport) SetConfig(config map[string]interface{}) {
	m.config = config
}

func TestProviders_ByScheme(t *testing.T) {
	providers := Providers{
		"http": &MockTransport{data: []byte("http data")},
		"oci":  &MockTransport{data: []byte("oci data")},
	}

	// Test existing scheme
	transport, err := providers.ByScheme("http")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if transport == nil {
		t.Error("Expected transport, got nil")
	}

	// Test missing scheme
	_, err = providers.ByScheme("missing")
	if err == nil {
		t.Error("Expected error for missing scheme")
	}
}

func TestOptions(t *testing.T) {
	opts := &Options{}

	// Test basic auth option
	WithBasicAuth("user", "pass")(opts)
	if opts.Username != "user" || opts.Password != "pass" {
		t.Errorf("Expected user/pass, got %s/%s", opts.Username, opts.Password)
	}

	// Test user agent option
	WithUserAgent("test-agent")(opts)
	if opts.UserAgent != "test-agent" {
		t.Errorf("Expected test-agent, got %s", opts.UserAgent)
	}

	// Test artifact type option
	WithArtifactType("plugin")(opts)
	if opts.ArtifactType != "plugin" {
		t.Errorf("Expected plugin, got %s", opts.ArtifactType)
	}

	// Test custom option
	WithCustomOption("key", "value")(opts)
	if opts.Custom["key"] != "value" {
		t.Errorf("Expected value, got %v", opts.Custom["key"])
	}
}

func TestTransportInterfaces(t *testing.T) {
	transport := &MockTransport{}

	// Test that transport can be configured
	if configProvider, ok := interface{}(transport).(interface {
		GetConfig() map[string]interface{}
		SetConfig(config map[string]interface{})
	}); ok {
		config := map[string]interface{}{"test": "value"}
		configProvider.SetConfig(config)
		result := configProvider.GetConfig()
		if result["test"] != "value" {
			t.Errorf("Expected value, got %v", result["test"])
		}
	} else {
		t.Error("Transport should support configuration")
	}
}
