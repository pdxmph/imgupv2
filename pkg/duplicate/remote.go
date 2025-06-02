package duplicate

import (
	"context"
	"fmt"
)

// RemoteChecker implements duplicate checking with local cache and remote fallback
type RemoteChecker struct {
	cache     *SQLiteCache
	searchers map[string]ServiceSearcher
	service   string // current default service
}

// NewRemoteChecker creates a new checker with cache and service searchers
func NewRemoteChecker(cache *SQLiteCache, service string) *RemoteChecker {
	return &RemoteChecker{
		cache:     cache,
		searchers: make(map[string]ServiceSearcher),
		service:   service,
	}
}

// RegisterSearcher adds a service-specific searcher
func (r *RemoteChecker) RegisterSearcher(service string, searcher ServiceSearcher) {
	r.searchers[service] = searcher
}

// Check looks for an existing upload, trying cache first then remote
func (r *RemoteChecker) Check(ctx context.Context, filePath string) (*Upload, error) {
	// Get file info including MD5
	info, err := GetFileInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("get file info: %w", err)
	}

	// 1. Check local cache first (fast path)
	upload, err := r.cache.Check(ctx, info.MD5)
	if err != nil {
		return nil, fmt.Errorf("cache check: %w", err)
	}
	if upload != nil {
		// Found in cache - return silently
		return upload, nil
	}

	// 2. Fall back to remote search if service searcher available
	searcher, ok := r.searchers[r.service]
	if !ok {
		// No remote searcher available, not in cache = not a duplicate
		return nil, nil
	}

	// Try hash-based search first (if supported)
	upload, err = searcher.SearchByHash(ctx, info.MD5)
	if err != nil {
		// Silently continue to metadata search
	}
	if upload != nil {
		// Found by hash - cache it
		if err := r.cache.Record(upload); err != nil {
			// Ignore cache errors silently
		}
		return upload, nil
	}

	// Try metadata-based search as fallback
	upload, err = searcher.SearchByMetadata(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("metadata search: %w", err)
	}
	if upload != nil {
		// Found by metadata - cache it
		if err := r.cache.Record(upload); err != nil {
			// Ignore cache errors silently
		}
		return upload, nil
	}

	// Not found anywhere
	return nil, nil
}

// Record saves an upload to the cache
func (r *RemoteChecker) Record(upload *Upload) error {
	return r.cache.Record(upload)
}

// SetService changes the active service for checking
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
