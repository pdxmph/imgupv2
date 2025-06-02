package duplicate

import (
	"context"
)

// Checker provides duplicate detection functionality
type Checker interface {
	// Check looks for an existing upload by file hash
	Check(ctx context.Context, filePath string) (*Upload, error)
	
	// Record saves an upload to the cache
	Record(upload *Upload) error
}

// ServiceSearcher defines the interface for service-specific duplicate searches
type ServiceSearcher interface {
	// SearchByHash searches for images with matching hash/checksum
	SearchByHash(ctx context.Context, md5Hash string) (*Upload, error)
	
	// SearchByMetadata searches using filename, date, and other metadata
	SearchByMetadata(ctx context.Context, info *FileInfo) (*Upload, error)
}
