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

// Package render provides support for invoking render/v1 plugins to render
// chart templates using alternative rendering engines.
package render

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gobwas/glob"
	"github.com/tetratelabs/wazero"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/schema"
	ci "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
)

// PluginRenderer renders chart files using render/v1 plugins.
type PluginRenderer struct {
	// ContentCachePath is the path to the content cache directory.
	// Plugins are loaded from cached archives using their digest from Chart.lock.
	ContentCachePath string

	// SDK options for customizing plugin loading behavior

	// CompilationCache allows providing a custom Wasm compilation cache.
	// If nil, a disk-based cache at $HELM_CACHE_HOME/wazero-build/ is used.
	// For non-writable filesystems, use wazero.NewCompilationCache() for in-memory.
	CompilationCache wazero.CompilationCache

	// PreloadedPlugins allows providing pre-loaded plugin archive data as raw bytes.
	// Key is the plugin digest (SHA256 hex string from Chart.lock).
	// Value is the raw tarball bytes (gzipped tar archive containing plugin.yaml and plugin.wasm).
	// Useful for non-writable filesystems where plugins are bundled with the application.
	PreloadedPlugins map[string][]byte
}

// ReleaseInfo contains release metadata passed to render plugins.
// This is the public SDK type that mirrors the internal schema type.
type ReleaseInfo struct {
	// Name is the release name
	Name string
	// Namespace is the release namespace
	Namespace string
	// Revision is the release revision number
	Revision int
	// IsInstall is true if this is an install operation
	IsInstall bool
	// IsUpgrade is true if this is an upgrade operation
	IsUpgrade bool
	// Service is the name of the rendering engine (always "Helm")
	Service string
}

// Context contains all the Helm built-in objects needed for rendering.
type Context struct {
	// Release contains release metadata
	Release ReleaseInfo
	// Values contains merged values
	Values map[string]interface{}
	// Capabilities contains cluster capabilities
	Capabilities *common.Capabilities
}

// FileAssignment tracks which plugin manages which files.
type FileAssignment struct {
	// PluginName is the name of the plugin managing this file
	PluginName string
	// Pattern is the glob pattern that matched this file
	Pattern string
	// Specificity is the pattern length (for most-specific-wins resolution)
	Specificity int
}

// Render invokes render plugins to render the chart's template files.
// It processes plugins in the order defined in Chart.yaml, allowing
// earlier plugins to modify files that later plugins may process.
func (r *PluginRenderer) Render(
	ctx context.Context,
	chart ci.Charter,
	renderCtx *Context,
) (map[string]string, error) {
	accessor, err := ci.NewAccessor(chart)
	if err != nil {
		return nil, fmt.Errorf("failed to access chart: %w", err)
	}

	// Get plugins from the accessor
	plugins := accessor.Plugins()

	// If no plugins defined, return empty (caller should use default engine)
	if len(plugins) == 0 {
		return nil, nil
	}

	// Build initial source files from chart
	sourceFiles := buildSourceFiles(accessor)

	// Build chart info from accessor methods
	chartInfo := buildChartInfoFromAccessor(accessor)

	// Build subcharts info
	subchartsInfo := buildSubchartsInfo(accessor)

	// Build capabilities info
	capabilitiesInfo := buildCapabilitiesInfo(renderCtx.Capabilities)

	// Build non-template files
	files := buildFiles(accessor.Files())

	// Process each plugin in order
	rendered := make(map[string]string)
	for _, pluginDep := range plugins {
		// Skip non-render plugins
		if pluginDep.GetType() != "render/v1" {
			continue
		}

		// Load the plugin
		p, err := r.loadPlugin(pluginDep)
		if err != nil {
			return nil, fmt.Errorf("failed to load render plugin %q: %w", pluginDep.GetName(), err)
		}

		// Get plugin's glob patterns from config
		patterns, err := r.getPluginPatterns(p)
		if err != nil {
			return nil, fmt.Errorf("failed to get patterns for render plugin %q: %w", pluginDep.GetName(), err)
		}

		// Match files to this plugin
		matchedFiles, remainingFiles := matchFilesToPlugin(sourceFiles, patterns)
		if len(matchedFiles) == 0 {
			continue
		}

		// Build input message
		input := &plugin.Input{
			Message: schema.InputMessageRenderV1{
				Release:      toSchemaReleaseInfo(renderCtx.Release),
				Values:       renderCtx.Values,
				Chart:        chartInfo,
				Subcharts:    subchartsInfo,
				Files:        files,
				Capabilities: capabilitiesInfo,
				SourceFiles:  matchedFiles,
			},
		}

		// Invoke the plugin
		output, err := p.Invoke(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke render plugin %q: %w", pluginDep.GetName(), err)
		}

		// Extract rendered files
		outputMsg, ok := output.Message.(schema.OutputMessageRenderV1)
		if !ok {
			return nil, fmt.Errorf("unexpected output type from render plugin %q", pluginDep.GetName())
		}

		// Check for errors
		if len(outputMsg.Errors) > 0 {
			return nil, fmt.Errorf("render plugin %q reported errors: %v", pluginDep.GetName(), outputMsg.Errors)
		}

		// Merge rendered files into result
		for name, content := range outputMsg.RenderedFiles {
			rendered[name] = content
		}

		// Update source files for next plugin
		// If the plugin returned ModifiedSourceFiles, use those plus remaining files
		// This allows plugins to add/modify/remove files for subsequent plugins
		if len(outputMsg.ModifiedSourceFiles) > 0 {
			// Plugin explicitly modified the source files
			// Merge modified files with remaining files (modified takes precedence)
			sourceFiles = mergeSourceFiles(remainingFiles, outputMsg.ModifiedSourceFiles)
		} else {
			// No modifications, just use remaining files
			sourceFiles = remainingFiles
		}
	}

	return rendered, nil
}

// loadPlugin loads a render plugin by its content hash (digest).
// Chart-defined plugins are identified by digest, not name/version.
// It tries the following locations in order:
// 1. Preloaded plugins (SDK: in-memory, for non-writable filesystems)
// 2. Content cache (archive-based) using digest from Chart.lock
func (r *PluginRenderer) loadPlugin(dep ci.PluginDependency) (plugin.Plugin, error) {
	var p plugin.Plugin
	var err error
	digest := dep.GetDigest()

	// Plugin must have a digest from Chart.lock for deterministic loading
	if digest == "" {
		return nil, fmt.Errorf("plugin %q has no digest: run 'helm dependency update' to resolve plugin versions", dep.GetName())
	}

	// 1. Check preloaded plugins first (SDK: non-writable filesystem support)
	if r.PreloadedPlugins != nil {
		if rawData, ok := r.PreloadedPlugins[digest]; ok {
			p, err = r.loadPluginFromBytes(rawData)
			if err == nil {
				if p.Metadata().Type != "render/v1" {
					return nil, fmt.Errorf("plugin %q is type %q, expected render/v1", dep.GetName(), p.Metadata().Type)
				}
				return p, nil
			}
			// If preloaded plugin failed, fall through to cache
		}
	}

	// 2. Load from content cache using digest
	if r.ContentCachePath != "" {
		p, err = r.loadPluginFromCache(digest)
		if err == nil {
			// Verify plugin type matches
			if p.Metadata().Type != "render/v1" {
				return nil, fmt.Errorf("plugin %q is type %q, expected render/v1", dep.GetName(), p.Metadata().Type)
			}
			return p, nil
		}
	}

	// Plugin not found in cache - user needs to download it
	return nil, fmt.Errorf("plugin %q (digest: %s) not found in cache: run 'helm dependency update' to download plugins", dep.GetName(), digest[:12])
}

// loadPluginFromCache loads a plugin from the content cache using its digest.
func (r *PluginRenderer) loadPluginFromCache(digest string) (plugin.Plugin, error) {
	// Build the cache file path: {ContentCachePath}/{first2chars}/{digest}.plugin
	// This matches the directory structure used by pkg/downloader/cache.go
	subdir := digest[:2]
	cacheFile := filepath.Join(r.ContentCachePath, subdir, digest+".plugin")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached plugin: %w", err)
	}

	return r.loadPluginFromBytes(data)
}

// loadPluginFromBytes parses plugin archive bytes and creates a plugin instance.
// Uses the configured compilation cache if set.
func (r *PluginRenderer) loadPluginFromBytes(data []byte) (plugin.Plugin, error) {
	archiveData, err := plugin.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse plugin archive: %w", err)
	}
	return plugin.CreatePluginFromArchiveWithCache(archiveData, r.CompilationCache)
}

// getPluginPatterns extracts the glob patterns from a render plugin's config.
func (r *PluginRenderer) getPluginPatterns(p plugin.Plugin) ([]glob.Glob, error) {
	config, ok := p.Metadata().Config.(*schema.ConfigRenderV1)
	if !ok {
		return nil, fmt.Errorf("plugin has invalid config type")
	}

	patterns := make([]glob.Glob, 0, len(config.Patterns))
	for _, pattern := range config.Patterns {
		g, err := glob.Compile(pattern, '/')
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		patterns = append(patterns, g)
	}
	return patterns, nil
}

// matchFilesToPlugin separates source files into those matching the patterns
// and those that don't match.
func matchFilesToPlugin(files []schema.SourceFile, patterns []glob.Glob) (matched, remaining []schema.SourceFile) {
	for _, f := range files {
		if matchesAnyPattern(f.Name, patterns) {
			matched = append(matched, f)
		} else {
			remaining = append(remaining, f)
		}
	}
	return matched, remaining
}

// matchesAnyPattern checks if a filename matches any of the given glob patterns.
func matchesAnyPattern(name string, patterns []glob.Glob) bool {
	for _, p := range patterns {
		if p.Match(name) {
			return true
		}
	}
	return false
}

// mergeSourceFiles merges two sets of source files, with modified files taking precedence.
// This allows plugins to add new files, modify existing files, or effectively remove files
// by not including them in the modified set.
func mergeSourceFiles(remaining, modified []schema.SourceFile) []schema.SourceFile {
	// Create a map from remaining files
	fileMap := make(map[string]schema.SourceFile)
	for _, f := range remaining {
		fileMap[f.Name] = f
	}

	// Modified files override remaining files with the same name
	// and add new files
	for _, f := range modified {
		fileMap[f.Name] = f
	}

	// Convert back to slice
	result := make([]schema.SourceFile, 0, len(fileMap))
	for _, f := range fileMap {
		result = append(result, f)
	}
	return result
}

// buildSourceFiles creates SourceFile objects from the chart's templates and other files.
func buildSourceFiles(accessor ci.Accessor) []schema.SourceFile {
	var files []schema.SourceFile

	// Add templates
	for _, t := range accessor.Templates() {
		if t == nil {
			continue
		}
		files = append(files, schema.SourceFile{
			Name: t.Name,
			Data: t.Data,
		})
	}

	return files
}

// buildChartInfoFromAccessor creates ChartInfo from an Accessor.
func buildChartInfoFromAccessor(accessor ci.Accessor) schema.ChartInfo {
	mdMap := accessor.MetadataAsMap()

	// Extract values from the map with safe defaults
	name, _ := mdMap["Name"].(string)
	version, _ := mdMap["Version"].(string)
	appVersion, _ := mdMap["AppVersion"].(string)
	description, _ := mdMap["Description"].(string)
	chartType, _ := mdMap["Type"].(string)

	return schema.ChartInfo{
		Name:        name,
		Version:     version,
		AppVersion:  appVersion,
		Description: description,
		Type:        chartType,
		IsRoot:      accessor.IsRoot(),
	}
}

// buildSubchartsInfo creates SubchartInfo map from chart dependencies.
func buildSubchartsInfo(accessor ci.Accessor) map[string]schema.SubchartInfo {
	subcharts := make(map[string]schema.SubchartInfo)
	for _, dep := range accessor.Dependencies() {
		subAccessor, err := ci.NewAccessor(dep)
		if err != nil {
			continue
		}
		mdMap := subAccessor.MetadataAsMap()
		name, _ := mdMap["Name"].(string)
		version, _ := mdMap["Version"].(string)

		subcharts[name] = schema.SubchartInfo{
			Name:    name,
			Version: version,
			Enabled: true, // If it's in Dependencies, it's enabled
		}
	}
	return subcharts
}

// buildCapabilitiesInfo converts Capabilities to CapabilitiesInfo.
func buildCapabilitiesInfo(caps *common.Capabilities) schema.CapabilitiesInfo {
	if caps == nil {
		return schema.CapabilitiesInfo{}
	}
	return schema.CapabilitiesInfo{
		KubeVersion: schema.KubeVersionInfo{
			Version: caps.KubeVersion.Version,
			Major:   caps.KubeVersion.Major,
			Minor:   caps.KubeVersion.Minor,
		},
		APIVersions: caps.APIVersions,
		HelmVersion: caps.HelmVersion.Version,
	}
}

// buildFiles creates SourceFile objects from the chart's non-template files.
func buildFiles(files []*common.File) []schema.SourceFile {
	result := make([]schema.SourceFile, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		result = append(result, schema.SourceFile{
			Name: f.Name,
			Data: f.Data,
		})
	}
	return result
}

// toSchemaReleaseInfo converts public ReleaseInfo to internal schema type.
func toSchemaReleaseInfo(r ReleaseInfo) schema.ReleaseInfo {
	return schema.ReleaseInfo{
		Name:      r.Name,
		Namespace: r.Namespace,
		Revision:  r.Revision,
		IsInstall: r.IsInstall,
		IsUpgrade: r.IsUpgrade,
		Service:   r.Service,
	}
}
