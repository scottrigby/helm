# Helm v4.0 Major Changes Summary

## Overview
Helm v4.0 represents a significant evolution from v3, introducing breaking changes, new architectural patterns, and enhanced functionality while maintaining backwards compatibility for charts.

**Analysis Scope**: 219 PRs from main branch compared to dev-v3 (v3.19.0-rc.1)  
**v4-only Changes**: 196 PRs (23 backported to v3 excluded)

*This summary consolidates the key changes from the comprehensive analysis in `helm-v4-changelog-full.md`*

## 🚨 Breaking Changes

### Core Architecture Changes
- **HIP-0026 Plugin System Overhaul**: Complete redesign with new runtime interface using WebAssembly (Extism)
- **HIP-0023 Server-side Apply**: Native Kubernetes server-side apply support for better resource management
- **Chart v3 Support**: New chart version with backwards compatibility to v2 charts
- **Package Restructuring**: Move to versioned packages (`pkg/chart/v2`, `pkg/release/v1`) for future compatibility

### CLI Breaking Changes  
- **Flag Renaming**: `--atomic` → `--rollback-on-failure`, `--force` → `--force-replace`
- **Deprecated Removals**: `--no-update` flag, `--create-pods` flag, legacy APIs removed
- **Library Changes**: Helm can now be used as a Go library with refactored cmd/helm structure

## 🚀 Major New Features

### Resource Management
- **kstatus watcher**: Advanced Kubernetes resource status monitoring (#13604)
- **OCI install by digest**: Enhanced OCI registry support (#12690) 
- **Server-side Apply**: Better conflict resolution and field ownership (#30812, #31030)

### Templating & Configuration
- **Multi-document values files**: Support for complex value configurations (#13655)
- **Template functions**: `mustToYaml`, `mustToJson` functions (#30553)
- **JSON arguments support**: Enhanced CLI flexibility (#30294)
- **Custom template functions**: Plugin-based template function extension (#30734)
- **Post-renderer for hooks**: Enhanced templating capabilities (#13154)

### Developer Experience  
- **CPU/Memory profiling**: Built-in performance monitoring (#13481)
- **Enhanced error formatting**: Better multi-line stacktrace display (#13586)
- **Color output**: Improved visual feedback for release statuses (#31034)
- **Timeout flags**: Configurable timeouts for repo operations (#30900)
- **Registry TLS config**: In-memory TLS config for registry login (#31076)

## 🏗️ Technical Improvements

### Performance & Reliability
- **Content-based caching** (#31165, #31178): Improved chart caching mechanism
- **Dependency optimization** (#11112): Avoid duplicate repo updates during dependency resolution
- **Concurrent safety** (#30939, #30958, #30972): Multiple mutex fixes for thread safety
- **YAML library upgrade** (#31101): Migration to yaml/v3 for better performance

### Security & Registry
- **Enhanced OCI authentication** (#30979, #30992): Improved OAuth and bearer token support
- **TLS improvements** (#30590): Better mTLS proxy support
- **Chart signing** (#30718): Support multiple chart signing with single passphrase
- **Registry URI handling** (#30862): Better absolute URI concatenation

### Code Quality
- **slog migration** (#30708): Modern structured logging throughout codebase  
- **Go 1.24 support** (#30677): Latest Go version compatibility
- **Linting improvements** (#30752, #30872): Updated golangci-lint with new rules
- **Dependency cleanup**: Removed deprecated libraries, updated security dependencies

## 🔧 Fixes & Stability

### Kubernetes Integration
- **Resource matching** (#12581): Full GroupVersionKind consideration for better resource identification
- **Hook management** (#30766, #30673): Improved hook execution and cleanup on failure
- **Custom resource support** (#30697): Better `--take-ownership` functionality for CRDs

### Testing & Development
- **Flaky test fixes**: Multiple PRs addressing non-deterministic test failures
- **XDG compliance** (#30955): Proper test isolation using temporary directories
- **Race condition fixes**: DNS, registry, and goroutine-related race conditions resolved

## 📦 Plugin System Revolution (HIP-0026)

The plugin system received a complete overhaul across 10+ PRs:

- **New Runtime Interface**: WebAssembly-based plugin execution with Extism
- **Plugin Types**: Structured plugin API with versioned interfaces  
- **OCI Distribution**: Plugins can be distributed via OCI registries
- **Enhanced Security**: Sandboxed execution environment
- **Post-renderer Integration**: Post-renderers are now plugin types
- **Simplified Management**: Streamlined installation and lifecycle management

## 🎯 Developer Impact

### For Helm Users
- **Better Resource Monitoring**: kstatus integration provides detailed resource status
- **Improved OCI Workflows**: Digest-based installations for supply chain security
- **Enhanced Configuration**: Multi-document values and JSON support
- **Better Debugging**: Profiling support and improved error messages

### For Plugin Developers  
- **Breaking**: Existing plugins need migration to new WebAssembly-based runtime
- **Enhanced**: More powerful plugin API with better security and distribution
- **Simplified**: Cleaner plugin development experience with structured types

### For Chart Developers
- **Chart v3 API**: Optional upgrade path with new capabilities
- **Better Linting**: Enhanced validation including CRDs directory support
- **Template Functions**: New helper functions for YAML/JSON generation

## 📋 Migration Considerations

### Required Actions
1. **Plugin Migration**: All plugins must be updated for new runtime
2. **CLI Flag Updates**: Update scripts using renamed flags
3. **Library Integration**: API changes for applications using Helm as library

### Compatibility
- **Chart Compatibility**: v2 charts continue to work unchanged
- **Registry Compatibility**: Enhanced OCI support is backwards compatible  
- **Configuration**: Most existing configurations work without changes

---

**Bottom Line**: Helm v4.0 is a major architectural upgrade focused on better Kubernetes integration, enhanced plugin capabilities, and improved developer experience while maintaining chart compatibility.