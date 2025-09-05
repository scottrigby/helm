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

package artifact

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"helm.sh/helm/v4/pkg/transport"
)

// MockTransport implements transport.Transport for testing.
type MockTransport struct {
	data map[string][]byte
	err  error
}

func (m *MockTransport) Get(url string, _ ...transport.Option) (*bytes.Buffer, error) {
	if m.err != nil {
		return nil, m.err
	}

	data, exists := m.data[url]
	if !exists {
		data = []byte("mock data")
	}

	return bytes.NewBuffer(data), nil
}

// MockCache implements Cache for testing.
type MockCache struct {
	data map[string][]byte
}

func (c *MockCache) Get(digest [sha256.Size]byte, cacheType CacheType) (string, error) {
	// Use cache type in key to simulate different cache directories
	key := fmt.Sprintf("%s-%d", string(digest[:]), cacheType)
	if data, exists := c.data[key]; exists {
		// Create temporary file with appropriate extension
		pattern := "cache-*.tgz"
		if cacheType == CacheProv {
			pattern = "cache-*.prov"
		}
		tmpfile, err := os.CreateTemp("", pattern)
		if err != nil {
			return "", err
		}
		defer tmpfile.Close()

		_, err = tmpfile.Write(data)
		return tmpfile.Name(), err
	}
	return "", os.ErrNotExist
}

func (c *MockCache) Put(digest [sha256.Size]byte, data io.Reader, cacheType CacheType) (string, error) {
	// Use cache type in key to simulate different cache directories
	key := fmt.Sprintf("%s-%d", string(digest[:]), cacheType)

	// Read data into buffer
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, data); err != nil {
		return "", err
	}
	c.data[key] = buf.Bytes()

	// Create temporary file with appropriate extension
	pattern := "cache-*.tgz"
	if cacheType == CacheProv {
		pattern = "cache-*.prov"
	}
	tmpfile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	_, err = tmpfile.Write(buf.Bytes())
	return tmpfile.Name(), err
}

func TestDownloader_Download(t *testing.T) {
	// Create temporary directory for test
	tmpdir := t.TempDir()

	// Create mock transport
	mockTransport := &MockTransport{
		data: map[string][]byte{
			"http://example.com/chart.tgz": []byte("chart data"),
			"oci://example.com/plugin:v1":  []byte("plugin data"),
		},
	}

	// Create downloader with mock registry client
	downloader := &Downloader{
		Transports: transport.Providers{
			"http": mockTransport,
			"oci":  mockTransport,
		},
		Cache:        &MockCache{data: make(map[string][]byte)},
		ContentCache: tmpdir,
		Verify:       VerifyNever, // Skip verification for this test
	}

	// Test chart download
	chartPath, verification, err := downloader.Download("http://example.com/chart.tgz", "", tmpdir, TypeChart)
	if err != nil {
		t.Errorf("Expected no error downloading chart, got %v", err)
	}
	if chartPath == "" {
		t.Error("Expected chart path, got empty string")
	}
	if verification == nil {
		t.Error("Expected verification object, got nil")
	}

	// Verify file was created
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		t.Errorf("Expected chart file to exist at %s", chartPath)
	}

	// Test plugin download
	pluginPath, _, err := downloader.Download("oci://example.com/plugin:v1", "", tmpdir, TypePlugin)
	if err != nil {
		t.Errorf("Expected no error downloading plugin, got %v", err)
	}
	if pluginPath == "" {
		t.Error("Expected plugin path, got empty string")
	}

	// Verify file was created
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Errorf("Expected plugin file to exist at %s", pluginPath)
	}
}

func TestChartDownloader_Compatibility(t *testing.T) {
	// Test that ChartDownloader wrapper works
	chartDownloader := NewChartDownloader()

	if chartDownloader.Downloader == nil {
		t.Error("Expected ChartDownloader to have embedded Downloader")
	}

	// Test method call (this will fail without proper setup, but shows the API works)
	tmpdir := t.TempDir()
	mockTransport := &MockTransport{
		data: map[string][]byte{
			"http://example.com/chart.tgz": []byte("chart data"),
		},
	}

	chartDownloader.Transports = transport.Providers{
		"http": mockTransport,
	}
	chartDownloader.Cache = &MockCache{data: make(map[string][]byte)}
	chartDownloader.ContentCache = tmpdir
	chartDownloader.Verify = VerifyNever

	path, verification, err := chartDownloader.DownloadTo("http://example.com/chart.tgz", "", tmpdir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if path == "" {
		t.Error("Expected path, got empty string")
	}
	if verification == nil {
		t.Error("Expected verification object, got nil")
	}
}

func TestPluginDownloader_Compatibility(t *testing.T) {
	// Test that PluginDownloader wrapper works
	pluginDownloader := NewPluginDownloader()

	if pluginDownloader.Downloader == nil {
		t.Error("Expected PluginDownloader to have embedded Downloader")
	}

	// Test method call
	tmpdir := t.TempDir()
	mockTransport := &MockTransport{
		data: map[string][]byte{
			"http://example.com/plugin.tgz": []byte("plugin data"),
		},
	}

	pluginDownloader.Transports = transport.Providers{
		"http": mockTransport,
	}
	pluginDownloader.Cache = &MockCache{data: make(map[string][]byte)}
	pluginDownloader.ContentCache = tmpdir
	pluginDownloader.Verify = VerifyNever

	path, verification, err := pluginDownloader.DownloadTo("http://example.com/plugin.tgz", "", tmpdir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if path == "" {
		t.Error("Expected path, got empty string")
	}
	if verification == nil {
		t.Error("Expected verification object, got nil")
	}
}

// Cache tests moved to cache_test.go for better organization
