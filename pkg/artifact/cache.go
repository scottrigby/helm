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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/internal/fileutil"
)

// Cache defines the interface for artifact caching.
// The cache key is the sha256 hash of the content, providing a common key for checking content.
type Cache interface {
	// Get returns the path to cached content for the given key and cache type.
	Get(digest [sha256.Size]byte, cacheType CacheType) (string, error)
	// Put stores the given data for the given key and cache type.
	Put(digest [sha256.Size]byte, data io.Reader, cacheType CacheType) (string, error)
}

// CacheType indicates what type of cached item is being stored.
type CacheType int

const (
	CacheArtifact CacheType = iota // Artifact content (charts, plugins, etc.)
	CacheProv                      // Provenance files
)

// String returns the file extension for the cache type.
func (c CacheType) String() string {
	switch c {
	case CacheArtifact:
		return ".chart" // Keep compatible with existing cache
	case CacheProv:
		return ".prov"
	default:
		return ".unknown"
	}
}

// DiskCache is a cache that stores data on disk.
type DiskCache struct {
	Root string
}

// Get returns the path to cached content for the given key and cache type.
func (c *DiskCache) Get(digest [sha256.Size]byte, cacheType CacheType) (string, error) {
	p := c.FileName(digest, cacheType)
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	// Empty files treated as not exist because there is no content.
	if fi.Size() == 0 {
		return p, os.ErrNotExist
	}
	// directories should never happen unless something outside helm is operating
	// on this content.
	if fi.IsDir() {
		return p, errors.New("is a directory")
	}
	return p, nil
}

// Put stores the given data for the given key and cache type.
// It returns the path to the stored file.
func (c *DiskCache) Put(digest [sha256.Size]byte, data io.Reader, cacheType CacheType) (string, error) {
	// TODO: verify the key and digest of the data are the same.
	p := c.FileName(digest, cacheType)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		slog.Error("failed to create cache directory")
		return p, err
	}
	return p, fileutil.AtomicWriteFile(p, data, 0644)
}

// FileName generates the filename in a structured manner where the first part is the
// directory and the full hash is the filename. This method is public to allow
// cache path inspection for testing and debugging.
func (c *DiskCache) FileName(digest [sha256.Size]byte, cacheType CacheType) string {
	return filepath.Join(c.Root, fmt.Sprintf("%02x", digest[0]), fmt.Sprintf("%x", digest)+cacheType.String())
}