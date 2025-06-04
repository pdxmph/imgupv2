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
