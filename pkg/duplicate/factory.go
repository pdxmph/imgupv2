package duplicate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/pdxmph/imgupv2/pkg/backends"
	"github.com/pdxmph/imgupv2/pkg/config"
)

// SetupFlickrDuplicateChecker creates a duplicate checker for Flickr
func SetupFlickrDuplicateChecker(cfg *config.FlickrConfig) (*RemoteChecker, error) {
	// Create cache
	cache, err := NewSQLiteCache(DefaultCachePath())
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	// Create checker
	checker := NewRemoteChecker(cache, "flickr")

	// Warn if UserID is not set (old config)
	if cfg.UserID == "" {
		fmt.Fprintf(os.Stderr, "Warning: Flickr user ID not set. Duplicate detection may search all of Flickr.\n")
		fmt.Fprintf(os.Stderr, "Please re-authenticate with: imgup auth flickr\n")
	}

	// Create Flickr-specific searcher
	flickrSearcher := &flickrSearcherImpl{
		api:    backends.NewFlickrAPI(cfg),
		userID: cfg.UserID, // Use user ID from config
	}

	checker.RegisterSearcher("flickr", flickrSearcher)
	return checker, nil
}

// flickrSearcherImpl implements ServiceSearcher for Flickr
type flickrSearcherImpl struct {
	api    *backends.FlickrAPI
	userID string
}

func (f *flickrSearcherImpl) SearchByHash(ctx context.Context, md5Hash string) (*Upload, error) {
	// Search using machine tag imgupv2:checksum=MD5
	machineTag := fmt.Sprintf("imgupv2:checksum=%s", md5Hash)
	
	params := backends.PhotoSearchParams{
		UserID:      f.userID, // Empty means search all users (slower)
		MachineTags: []string{machineTag},
		PerPage:     10, // Should only be one match
	}
	
	result, err := f.api.PhotosSearch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("flickr search failed: %w", err)
	}
	
	if len(result.Photos) == 0 {
		return nil, nil // No duplicate found
	}
	
	// Take the first result
	photo := result.Photos[0]
	
	// Build URLs
	photoURL := f.api.BuildPhotoURL(photo)
	imageURL := f.api.BuildImageURL(photo, "b") // Large size
	
	return &Upload{
		FileMD5:    md5Hash,
		Service:    "flickr",
		RemoteID:   photo.ID,
		RemoteURL:  photoURL,
		ImageURL:   imageURL,
	}, nil
}

func (f *flickrSearcherImpl) SearchByMetadata(ctx context.Context, info *FileInfo) (*Upload, error) {
	// Try searching by filename first
	filename := filepath.Base(info.Path)
	
	params := backends.PhotoSearchParams{
		UserID:  f.userID,
		Text:    filename,
		PerPage: 20, // More results since filename match is less precise
	}
	
	result, err := f.api.PhotosSearch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("flickr metadata search failed: %w", err)
	}
	
	// Look through results for a likely match
	// This is less precise than hash matching
	for _, photo := range result.Photos {
		// Could do additional validation here if needed
		// For now, assume first result with matching filename is it
		if strings.Contains(photo.Title, strings.TrimSuffix(filename, filepath.Ext(filename))) {
			photoURL := f.api.BuildPhotoURL(photo)
			imageURL := f.api.BuildImageURL(photo, "b")
			
			return &Upload{
				FileMD5:    info.MD5,
				Service:    "flickr",
				RemoteID:   photo.ID,
				RemoteURL:  photoURL,
				ImageURL:   imageURL,
			}, nil
		}
	}
	
	return nil, nil // No match found
}

// SetupSmugMugDuplicateChecker creates a duplicate checker for SmugMug
func SetupSmugMugDuplicateChecker(cfg *config.SmugMugConfig) (*RemoteChecker, error) {
	// Create cache
	cache, err := NewSQLiteCache(DefaultCachePath())
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	// Create checker
	checker := NewRemoteChecker(cache, "smugmug")

	// Create SmugMug-specific searcher
	smugmugSearcher := &smugmugSearcherImpl{
		api:      backends.NewSmugMugAPI(cfg),
		albumKey: cfg.AlbumID,
	}

	checker.RegisterSearcher("smugmug", smugmugSearcher)
	return checker, nil
}

// smugmugSearcherImpl implements ServiceSearcher for SmugMug
type smugmugSearcherImpl struct {
	api      *backends.SmugMugAPI
	albumKey string
}

func (s *smugmugSearcherImpl) SearchByHash(ctx context.Context, md5Hash string) (*Upload, error) {
	// SmugMug doesn't support hash search directly, so we need to fetch all images
	// and compare MD5 hashes
	images, err := s.api.GetAlbumImages(ctx, s.albumKey)
	if err != nil {
		return nil, fmt.Errorf("smugmug get album images failed: %w", err)
	}
	
	// Look for matching MD5
	for _, img := range images {
		if strings.EqualFold(img.ArchivedMD5, md5Hash) {
			return &Upload{
				FileMD5:    md5Hash,
				Service:    "smugmug",
				RemoteID:   img.ImageKey,
				RemoteURL:  img.WebURI,
				ImageURL:   img.WebURI, // SmugMug WebURI can be used directly
				Filename:   img.FileName,
			}, nil
		}
	}
	
	return nil, nil // No match found
}

func (s *smugmugSearcherImpl) SearchByMetadata(ctx context.Context, info *FileInfo) (*Upload, error) {
	// Search by filename
	filename := filepath.Base(info.Path)
	
	images, err := s.api.SearchAlbumImages(ctx, s.albumKey, filename)
	if err != nil {
		return nil, fmt.Errorf("smugmug search failed: %w", err)
	}
	
	// Look for exact filename match
	for _, img := range images {
		if strings.EqualFold(img.FileName, filename) {
			// If we have MD5, verify it matches
			if info.MD5 != "" && img.ArchivedMD5 != "" && 
				!strings.EqualFold(img.ArchivedMD5, info.MD5) {
				continue // Not the same file
			}
			
			return &Upload{
				FileMD5:    img.ArchivedMD5,
				Service:    "smugmug", 
				RemoteID:   img.ImageKey,
				RemoteURL:  img.WebURI,
				ImageURL:   img.WebURI,
				Filename:   img.FileName,
			}, nil
		}
	}
	
	return nil, nil // No match found
}
