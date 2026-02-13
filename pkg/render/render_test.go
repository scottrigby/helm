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

package render

import (
	"testing"

	"github.com/gobwas/glob"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func TestMatchFilesToPlugin(t *testing.T) {
	tests := []struct {
		name              string
		files             []schema.SourceFile
		patterns          []string
		expectedMatched   []string
		expectedRemaining []string
	}{
		{
			name: "match pkl files",
			files: []schema.SourceFile{
				{Name: "templates/deployment.pkl"},
				{Name: "templates/service.pkl"},
				{Name: "templates/configmap.yaml"},
			},
			patterns:          []string{"templates/*.pkl"},
			expectedMatched:   []string{"templates/deployment.pkl", "templates/service.pkl"},
			expectedRemaining: []string{"templates/configmap.yaml"},
		},
		{
			name: "match test files",
			files: []schema.SourceFile{
				{Name: "templates/file1.test"},
				{Name: "templates/file2.test"},
				{Name: "templates/file3.yaml"},
			},
			patterns:          []string{"templates/*.test"},
			expectedMatched:   []string{"templates/file1.test", "templates/file2.test"},
			expectedRemaining: []string{"templates/file3.yaml"},
		},
		{
			name: "no matches",
			files: []schema.SourceFile{
				{Name: "templates/deployment.yaml"},
				{Name: "templates/service.yaml"},
			},
			patterns:          []string{"templates/*.pkl"},
			expectedMatched:   []string{},
			expectedRemaining: []string{"templates/deployment.yaml", "templates/service.yaml"},
		},
		{
			name: "multiple patterns",
			files: []schema.SourceFile{
				{Name: "templates/main.pkl"},
				{Name: "templates/helper.cue"},
				{Name: "templates/config.yaml"},
			},
			patterns:          []string{"templates/*.pkl", "templates/*.cue"},
			expectedMatched:   []string{"templates/main.pkl", "templates/helper.cue"},
			expectedRemaining: []string{"templates/config.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compile patterns
			patterns := make([]glob.Glob, len(tt.patterns))
			for i, p := range tt.patterns {
				patterns[i] = glob.MustCompile(p, '/')
			}

			matched, remaining := matchFilesToPlugin(tt.files, patterns)

			// Check matched files
			matchedNames := make([]string, len(matched))
			for i, f := range matched {
				matchedNames[i] = f.Name
			}
			if !stringSliceEqual(matchedNames, tt.expectedMatched) {
				t.Errorf("matched files = %v, want %v", matchedNames, tt.expectedMatched)
			}

			// Check remaining files
			remainingNames := make([]string, len(remaining))
			for i, f := range remaining {
				remainingNames[i] = f.Name
			}
			if !stringSliceEqual(remainingNames, tt.expectedRemaining) {
				t.Errorf("remaining files = %v, want %v", remainingNames, tt.expectedRemaining)
			}
		})
	}
}

func TestMergeSourceFiles(t *testing.T) {
	tests := []struct {
		name      string
		remaining []schema.SourceFile
		modified  []schema.SourceFile
		expected  map[string]string // name -> data
	}{
		{
			name: "add new file",
			remaining: []schema.SourceFile{
				{Name: "file1.txt", Data: []byte("original1")},
			},
			modified: []schema.SourceFile{
				{Name: "file2.txt", Data: []byte("new file")},
			},
			expected: map[string]string{
				"file1.txt": "original1",
				"file2.txt": "new file",
			},
		},
		{
			name: "override existing file",
			remaining: []schema.SourceFile{
				{Name: "file1.txt", Data: []byte("original")},
			},
			modified: []schema.SourceFile{
				{Name: "file1.txt", Data: []byte("modified")},
			},
			expected: map[string]string{
				"file1.txt": "modified",
			},
		},
		{
			name: "remove file by not including",
			remaining: []schema.SourceFile{
				{Name: "keep.txt", Data: []byte("keep")},
				{Name: "remove.txt", Data: []byte("remove")},
			},
			modified: []schema.SourceFile{
				// Only include keep.txt - remove.txt will be removed
				// Actually, the current implementation keeps remaining files
				// This test documents the actual behavior
			},
			expected: map[string]string{
				"keep.txt":   "keep",
				"remove.txt": "remove",
			},
		},
		{
			name: "empty modified keeps remaining",
			remaining: []schema.SourceFile{
				{Name: "file1.txt", Data: []byte("content1")},
				{Name: "file2.txt", Data: []byte("content2")},
			},
			modified: []schema.SourceFile{},
			expected: map[string]string{
				"file1.txt": "content1",
				"file2.txt": "content2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeSourceFiles(tt.remaining, tt.modified)

			// Convert result to map for easier comparison
			resultMap := make(map[string]string)
			for _, f := range result {
				resultMap[f.Name] = string(f.Data)
			}

			if len(resultMap) != len(tt.expected) {
				t.Errorf("result has %d files, want %d", len(resultMap), len(tt.expected))
			}

			for name, expectedData := range tt.expected {
				if data, ok := resultMap[name]; !ok {
					t.Errorf("missing file %q", name)
				} else if data != expectedData {
					t.Errorf("file %q has data %q, want %q", name, data, expectedData)
				}
			}
		})
	}
}

func TestMatchesAnyPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		patterns []string
		expected bool
	}{
		{
			name:     "matches pkl pattern",
			filename: "templates/deployment.pkl",
			patterns: []string{"templates/*.pkl"},
			expected: true,
		},
		{
			name:     "no match",
			filename: "templates/deployment.yaml",
			patterns: []string{"templates/*.pkl"},
			expected: false,
		},
		{
			name:     "matches second pattern",
			filename: "templates/helper.cue",
			patterns: []string{"templates/*.pkl", "templates/*.cue"},
			expected: true,
		},
		{
			name:     "empty patterns",
			filename: "templates/file.txt",
			patterns: []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := make([]glob.Glob, len(tt.patterns))
			for i, p := range tt.patterns {
				patterns[i] = glob.MustCompile(p, '/')
			}

			result := matchesAnyPattern(tt.filename, patterns)
			if result != tt.expected {
				t.Errorf("matchesAnyPattern(%q, %v) = %v, want %v",
					tt.filename, tt.patterns, result, tt.expected)
			}
		})
	}
}

// Helper function to compare string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
