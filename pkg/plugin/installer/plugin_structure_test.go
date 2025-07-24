package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPluginRoot(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) error
		expectRoot  string
		expectError bool
	}{
		{
			name: "plugin.yaml at root",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("name: test"), 0644)
			},
			expectRoot:  ".",
			expectError: false,
		},
		{
			name: "plugin.yaml in subdirectory",
			setup: func(dir string) error {
				subdir := filepath.Join(dir, "my-plugin")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(subdir, "plugin.yaml"), []byte("name: test"), 0644)
			},
			expectRoot:  "my-plugin",
			expectError: false,
		},
		{
			name: "no plugin.yaml",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0644)
			},
			expectRoot:  "",
			expectError: true,
		},
		{
			name: "plugin.yaml in nested subdirectory (should not find)",
			setup: func(dir string) error {
				subdir := filepath.Join(dir, "outer", "inner")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(subdir, "plugin.yaml"), []byte("name: test"), 0644)
			},
			expectRoot:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			root, err := detectPluginRoot(dir)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				expectedPath := dir
				if tt.expectRoot != "." {
					expectedPath = filepath.Join(dir, tt.expectRoot)
				}
				if root != expectedPath {
					t.Errorf("Expected root %s but got %s", expectedPath, root)
				}
			}
		})
	}
}

func TestValidatePluginName(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(dir string) error
		pluginRoot   string
		expectedName string
		expectError  bool
	}{
		{
			name: "matching directory and plugin name",
			setup: func(dir string) error {
				subdir := filepath.Join(dir, "my-plugin")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					return err
				}
				yaml := `name: my-plugin
version: 1.0.0
usage: test
description: test`
				return os.WriteFile(filepath.Join(subdir, "plugin.yaml"), []byte(yaml), 0644)
			},
			pluginRoot:   "my-plugin",
			expectedName: "my-plugin",
			expectError:  false,
		},
		{
			name: "different directory and plugin name",
			setup: func(dir string) error {
				subdir := filepath.Join(dir, "wrong-name")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					return err
				}
				yaml := `name: my-plugin
version: 1.0.0
usage: test
description: test`
				return os.WriteFile(filepath.Join(subdir, "plugin.yaml"), []byte(yaml), 0644)
			},
			pluginRoot:   "wrong-name",
			expectedName: "wrong-name",
			expectError:  false, // Currently we don't error on mismatch
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			pluginRoot := filepath.Join(dir, tt.pluginRoot)
			err := validatePluginName(pluginRoot, tt.expectedName)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
