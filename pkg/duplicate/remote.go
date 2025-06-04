package duplicate

import (
	"context"
	"fmt"
)

// RemoteChecker implements duplicate checking with local cache only
type RemoteChecker struct {
	cache   *SQLiteCache
	service string // current service name for cache entries
}

// NewRemoteChecker creates a new checker with cache
func NewRemoteChecker(cache *SQLiteCache, service string) *RemoteChecker {
	return &RemoteChecker{
		cache:   cache,
		service: service,
	}
}

// Check looks for an existing upload in the local cache only
func (r *RemoteChecker) Check(ctx context.Context, filePath string) (*Upload, error) {
	// Get file info including MD5
	info, err := GetFileInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("get file info: %w", err)
	}

	// Check local cache only (fast path)
	upload, err := r.cache.Check(ctx, info.MD5)
	if err != nil {
		return nil, fmt.Errorf("cache check: %w", err)
	}
	
	// Return result - nil means not found
	return upload, nil
}

// Record saves an upload to the cache
func (r *RemoteChecker) Record(upload *Upload) error {
	return r.cache.Record(upload)
}

// SetService changes the active service
func (r *RemoteChecker) SetService(service string) {
	r.service = service
}

// Close closes the underlying cache connection
func (r *RemoteChecker) Close() error {
	if r.cache != nil {
		return r.cache.Close()
	}
	return nil
}
