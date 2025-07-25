package plugin

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"testing"
)

func TestExtractPlugin(t *testing.T) {
	tests := []struct {
		name       string
		pluginYAML string
		wantType   string
		wantAPI    string
	}{
		{
			name: "v1 plugin",
			pluginYAML: `apiVersion: v1
name: test-v1
version: 1.0.0
type: cli
runtime: subprocess
`,
			wantType: "*plugin.PluginV1",
			wantAPI:  "v1",
		},
		{
			name: "legacy plugin",
			pluginYAML: `name: test-legacy
version: 1.0.0
usage: "Test plugin"
description: "A test plugin"
command: "$HELM_PLUGIN_DIR/test.sh"
`,
			wantType: "*plugin.PluginLegacy",
			wantAPI:  "legacy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary tarball
			tmpFile, err := os.CreateTemp("", "plugin-*.tar.gz")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			// Create tarball with plugin.yaml
			gzw := gzip.NewWriter(tmpFile)
			tw := tar.NewWriter(gzw)

			// Add plugin.yaml
			hdr := &tar.Header{
				Name: "test-plugin/plugin.yaml",
				Mode: 0644,
				Size: int64(len(tt.pluginYAML)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(tt.pluginYAML)); err != nil {
				t.Fatal(err)
			}

			// Close writers
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gzw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := tmpFile.Close(); err != nil {
				t.Fatal(err)
			}

			// Test extractPlugin
			plugin, err := extractPlugin(tmpFile.Name())
			if err != nil {
				t.Fatalf("extractPlugin failed: %v", err)
			}

			// Check type
			gotType := fmt.Sprintf("%T", plugin)
			if gotType != tt.wantType {
				t.Errorf("got plugin type %s, want %s", gotType, tt.wantType)
			}

			// Check API version
			if got := plugin.GetAPIVersion(); got != tt.wantAPI {
				t.Errorf("got API version %s, want %s", got, tt.wantAPI)
			}
		})
	}
}
