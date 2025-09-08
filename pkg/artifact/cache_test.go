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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure DiskCache implements the Cache interface
var _ Cache = (*DiskCache)(nil)

func TestDiskCache_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &DiskCache{Root: tmpDir}

	// Test data
	content := []byte("hello world")
	digest := sha256.Sum256(content)

	t.Run("PutAndGetArtifact", func(t *testing.T) {
		// Put the data into the cache
		path, err := cache.Put(digest, bytes.NewBuffer(content), CacheArtifact)
		require.NoError(t, err, "Put should not return an error")

		// Verify the file exists at the returned path
		_, err = os.Stat(path)
		require.NoError(t, err, "File should exist after Put")

		// Get the file from the cache
		retrievedPath, err := cache.Get(digest, CacheArtifact)
		require.NoError(t, err, "Get should not return an error for existing file")
		assert.Equal(t, path, retrievedPath, "Get should return the same path as Put")

		// Verify content
		data, err := os.ReadFile(retrievedPath)
		require.NoError(t, err)
		assert.Equal(t, content, data, "Content of retrieved file should match original content")
	})

	t.Run("PutAndGetProvenance", func(t *testing.T) {
		provContent := []byte("provenance data")
		provDigest := sha256.Sum256(provContent)

		path, err := cache.Put(provDigest, bytes.NewBuffer(provContent), CacheProv)
		require.NoError(t, err)

		retrievedPath, err := cache.Get(provDigest, CacheProv)
		require.NoError(t, err)
		assert.Equal(t, path, retrievedPath)

		data, err := os.ReadFile(retrievedPath)
		require.NoError(t, err)
		assert.Equal(t, provContent, data)
	})
}

func TestDiskCache_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &DiskCache{Root: tmpDir}

	nonExistentDigest := sha256.Sum256([]byte("does not exist"))
	_, err := cache.Get(nonExistentDigest, CacheArtifact)
	assert.ErrorIs(t, err, os.ErrNotExist, "Get for a non-existent key should return os.ErrNotExist")
}

func TestDiskCache_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &DiskCache{Root: tmpDir}

	emptyContent := []byte{}
	emptyDigest := sha256.Sum256(emptyContent)

	path, err := cache.Put(emptyDigest, bytes.NewBuffer(emptyContent), CacheArtifact)
	require.NoError(t, err)

	// Get should return ErrNotExist for empty files
	_, err = cache.Get(emptyDigest, CacheArtifact)
	assert.ErrorIs(t, err, os.ErrNotExist, "Get for an empty file should return os.ErrNotExist")

	// But the file should exist on disk
	_, err = os.Stat(path)
	require.NoError(t, err, "Empty file should still exist on disk")
}

func TestDiskCache_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &DiskCache{Root: tmpDir}

	dirDigest := sha256.Sum256([]byte("i am a directory"))

	// Create a directory at the expected cache path
	expectedPath := cache.FileName(dirDigest, CacheArtifact)
	err := os.MkdirAll(expectedPath, 0755)
	require.NoError(t, err)

	// Get should detect it's a directory and return an error
	_, err = cache.Get(dirDigest, CacheArtifact)
	assert.EqualError(t, err, "is a directory")
}

func TestDiskCache_FileName(t *testing.T) {
	cache := &DiskCache{Root: "/tmp/cache"}
	digest := [sha256.Size]byte{0x13, 0x07, 0x99, 0x0e, 0x6b, 0xa5, 0xca, 0x14, 0x5e, 0xb3, 0x5e, 0x99, 0x18, 0x2a, 0x9b, 0xec, 0x46, 0x53, 0x1b, 0xc5, 0x4d, 0xdf, 0x65, 0x6a, 0x60, 0x2c, 0x78, 0x0f, 0xa0, 0x24, 0x0d, 0xee}

	// Test artifact cache filename
	artifactPath := cache.FileName(digest, CacheArtifact)
	expectedArtifact := filepath.Join("/tmp/cache", "13", "1307990e6ba5ca145eb35e99182a9bec46531bc54ddf656a602c780fa0240dee.chart")
	assert.Equal(t, expectedArtifact, artifactPath)

	// Test provenance cache filename
	provPath := cache.FileName(digest, CacheProv)
	expectedProv := filepath.Join("/tmp/cache", "13", "1307990e6ba5ca145eb35e99182a9bec46531bc54ddf656a602c780fa0240dee.prov")
	assert.Equal(t, expectedProv, provPath)
}

func TestCacheType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       CacheType
		expected string
	}{
		{"artifact", CacheArtifact, ".chart"},
		{"provenance", CacheProv, ".prov"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ct.String())
		})
	}
}

func TestDiskCache_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &DiskCache{Root: tmpDir}

	content := []byte("concurrent test data")
	digest := sha256.Sum256(content)

	// Test concurrent Put operations
	t.Run("ConcurrentPut", func(t *testing.T) {
		done := make(chan bool, 2)

		// Two goroutines trying to Put the same content
		go func() {
			_, err := cache.Put(digest, bytes.NewBuffer(content), CacheArtifact)
			assert.NoError(t, err)
			done <- true
		}()

		go func() {
			_, err := cache.Put(digest, bytes.NewBuffer(content), CacheArtifact)
			assert.NoError(t, err)
			done <- true
		}()

		// Wait for both to complete
		<-done
		<-done

		// Verify the file exists and has correct content
		path, err := cache.Get(digest, CacheArtifact)
		require.NoError(t, err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, data)
	})
}
