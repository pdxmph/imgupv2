package duplicate

import (
	"fmt"
	
	"github.com/pdxmph/imgupv2/pkg/config"
)

// SetupFlickrDuplicateChecker creates a duplicate checker for Flickr (local cache only)
func SetupFlickrDuplicateChecker(cfg *config.FlickrConfig) (*RemoteChecker, error) {
	// Create cache
	cache, err := NewSQLiteCache(DefaultCachePath())
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	// Create checker (no remote searchers)
	checker := NewRemoteChecker(cache, "flickr")
	return checker, nil
}

// SetupSmugMugDuplicateChecker creates a duplicate checker for SmugMug (local cache only)
func SetupSmugMugDuplicateChecker(cfg *config.SmugMugConfig) (*RemoteChecker, error) {
	// Create cache
	cache, err := NewSQLiteCache(DefaultCachePath())
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	// Create checker (no remote searchers)
	checker := NewRemoteChecker(cache, "smugmug")
	return checker, nil
}
