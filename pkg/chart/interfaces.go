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

package chart

import (
	common "helm.sh/helm/v4/pkg/chart/common"
)

type Charter any

type Dependency any

// PluginDependency represents a plugin dependency for the Accessor interface.
// This is a common interface that both v2 and v3 plugin dependencies implement.
type PluginDependency interface {
	GetName() string
	GetType() string
	GetRepository() string
	GetVersion() string
}

type Accessor interface {
	Name() string
	IsRoot() bool
	MetadataAsMap() map[string]any
	Files() []*common.File
	Templates() []*common.File
	ChartFullPath() string
	IsLibraryChart() bool
	Dependencies() []Charter
	MetaDependencies() []Dependency
	// Plugins returns the list of plugin dependencies for this chart.
	// These are processed sequentially in list order.
	Plugins() []PluginDependency
	Values() map[string]any
	Schema() []byte
	Deprecated() bool
}

type DependencyAccessor interface {
	Name() string
	Alias() string
}
