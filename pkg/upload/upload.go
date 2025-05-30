package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mph/imgupv2/pkg/backends"
	"github.com/mph/imgupv2/pkg/config"
	"github.com/mph/imgupv2/pkg/metadata"
	"github.com/mph/imgupv2/pkg/templates"
)

// Options for upload
type Options struct {
	Backend      string
	Format       string
	Title        string
	Description  string
	Tags         []string
	Alt          string
	Private      bool
}

// Result of an upload
type Result struct {
	PhotoID         string
	PhotoURL        string
	DirectURL       string
	EditURL         string
	FormattedOutput string
}

// Uploader interface for GUI server
type Uploader interface {
	Upload(ctx context.Context, imagePath string, opts Options) (*Result, error)
}

// Service implements the Uploader interface
type Service struct {
	config *config.Config
}

// New creates a new upload service
func New(cfg *config.Config) *Service {
	return &Service{config: cfg}
}

// Upload performs the upload to the specified backend
func (s *Service) Upload(ctx context.Context, imagePath string, opts Options) (*Result, error) {
	// Validate file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", imagePath)
	}

	// Check authentication
	if opts.Backend == "flickr" {
		if s.config.Flickr.AccessToken == "" || s.config.Flickr.AccessSecret == "" {
			return nil, fmt.Errorf("not authenticated with Flickr")
		}
	} else {
		return nil, fmt.Errorf("unsupported backend: %s", opts.Backend)
	}

	// Create uploader for backend
	var flickrUploader *backends.FlickrUploader
	switch opts.Backend {
	case "flickr":
		flickrUploader = backends.NewFlickrUploader(
			s.config.Flickr.ConsumerKey,
			s.config.Flickr.ConsumerSecret,
			s.config.Flickr.AccessToken,
			s.config.Flickr.AccessSecret,
		)
	default:
		return nil, fmt.Errorf("backend not implemented: %s", opts.Backend)
	}

	// Handle metadata embedding
	uploadPath := imagePath
	var tempFile string

	if (opts.Title != "" || opts.Description != "" || len(opts.Tags) > 0) && metadata.HasExiftool() {
		writer, err := metadata.NewWriter()
		if err == nil {
			tempPath, err := writer.CopyWithMetadata(imagePath, opts.Title, opts.Description, opts.Tags)
			if err == nil {
				uploadPath = tempPath
				tempFile = tempPath
			}
		}
	}

	// Clean up temp file when done
	if tempFile != "" {
		defer os.Remove(tempFile)
	}

	// Perform upload
	resp, err := flickrUploader.Upload(ctx, uploadPath, "", "", opts.Private)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	// Build result
	editURL := "https://www.flickr.com/photos/upload/edit/?ids=" + resp.PhotoID
	result := &Result{
		PhotoID:   resp.PhotoID,
		PhotoURL:  resp.URL,
		DirectURL: resp.ImageURL,
		EditURL:   editURL,
	}

	// Format output
	tmpl, exists := s.config.Templates[opts.Format]
	if !exists {
		tmpl = s.config.Templates["url"] // Default to URL format
	}
	
	vars := templates.Variables{
		PhotoID:     resp.PhotoID,
		URL:         resp.URL,
		ImageURL:    resp.ImageURL,
		EditURL:     editURL,
		Filename:    filepath.Base(imagePath),
		Title:       opts.Title,
		Description: opts.Description,
		Alt:         opts.Alt,
		Tags:        opts.Tags,
	}
	result.FormattedOutput = templates.Process(tmpl, vars)

	return result, nil
}
