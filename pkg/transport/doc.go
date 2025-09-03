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

// Package transport provides protocol-specific handlers for fetching artifacts.
//
// This package defines the core Transport interface and optional capability
// interfaces that transports can implement to declare their configuration needs.
//
// Transports are protocol handlers (HTTP, OCI, Git, S3, etc.) that know how to
// fetch raw bytes from URLs. They are used by pkg/artifact for unified artifact
// downloading with verification and caching.
package transport
