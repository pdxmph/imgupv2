package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/pdxmph/imgupv2/pkg/duplicate"
	"github.com/pdxmph/imgupv2/pkg/thumbnail"
	"github.com/pdxmph/imgupv2/pkg/types"
)

// App struct
type App struct {
	ctx       context.Context
	thumbGen  *thumbnail.Generator
	cachedPhotoIDs map[string]bool // Track which Photos IDs already have cached thumbnails
	currentPullRequest *types.PullRequest // Store current pull request
	pullDataPath string // Path to pull data file if launched from CLI
}

// PhotoMetadata represents the metadata for a photo
type PhotoMetadata struct {
	Path               string   `json:"path"`
	Title              string   `json:"title"`
	Alt                string   `json:"alt"`         // Alt text, not caption
	Description        string   `json:"description"` // Photo description
	Tags               []string `json:"tags"`
	Format             string   `json:"format"` // "url", "markdown", "html", "json"
	Private            bool     `json:"private"`
	MastodonEnabled    bool     `json:"mastodonEnabled"`
	MastodonText       string   `json:"mastodonText"`
	MastodonVisibility string   `json:"mastodonVisibility"`
	BlueskyEnabled     bool     `json:"blueskyEnabled"`
	BlueskyText        string   `json:"blueskyText"`
	// New fields for thumbnail display
	Thumbnail    string `json:"thumbnail"`    // base64 encoded thumbnail
	ImageWidth   int    `json:"imageWidth"`   // Original image width
	ImageHeight  int    `json:"imageHeight"`  // Original image height
	FileSize     int64  `json:"fileSize"`     // File size in bytes
	IsTemporary  bool   `json:"isTemporary"`  // True if from Photos app
	IsFromPhotos bool   `json:"isFromPhotos"` // True if selected from Photos
	PhotosIndex  int    `json:"photosIndex"`  // Index of photo in Photos selection
	PhotosID     string `json:"photosId"`     // Unique ID from Photos.app
	PhotosFilename string `json:"photosFilename"` // Original filename in Photos
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	Success    bool   `json:"success"`
	Snippet    string `json:"snippet"`
	Error      string `json:"error,omitempty"`
	Duplicate  bool   `json:"duplicate"`
	ForceAvailable bool `json:"forceAvailable"` // Indicates --force can be used
	SocialPostStatus string `json:"socialPostStatus,omitempty"` // Status of social media posting
}

// MultiPhotoUploadRequest represents the JSON structure for multi-photo uploads
type MultiPhotoUploadRequest struct {
	Post       string                   `json:"post"`
	Images     []MultiPhotoImageData   `json:"images"`
	Tags       []string                `json:"tags"`
	Mastodon   bool                   `json:"mastodon"`
	Bluesky    bool                   `json:"bluesky"`
	Visibility string                 `json:"visibility"`
	Format     string                 `json:"format"`
}

// MultiPhotoImageData represents a single image in the multi-photo upload
type MultiPhotoImageData struct {
	Path         string `json:"path"`
	Alt          string `json:"alt"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	IsFromPhotos bool   `json:"isFromPhotos"`
	PhotosIndex  int    `json:"photosIndex"`
	PhotosID     string `json:"photosId"`
}

// MultiPhotoUploadResult represents the result of a multi-photo upload
type MultiPhotoUploadResult struct {
	Success      bool                      `json:"success"`
	Outputs      []MultiPhotoOutputResult  `json:"outputs"`
	Error        string                   `json:"error,omitempty"`
	SocialStatus string                   `json:"socialStatus,omitempty"`
}

// MultiPhotoOutputResult represents the result for a single photo
type MultiPhotoOutputResult struct {
	Path      string   `json:"path"`
	URL       string   `json:"url"`
	Alt       string   `json:"alt"`
	Markdown  string   `json:"markdown,omitempty"`
	HTML      string   `json:"html,omitempty"`
	Error     string   `json:"error,omitempty"`
	Duplicate bool     `json:"duplicate"` // Track if this was a duplicate
	Warnings  []string `json:"warnings,omitempty"` // Non-fatal warnings
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		cachedPhotoIDs: make(map[string]bool),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	fmt.Println("DEBUG: startup called")
	a.ctx = ctx
	
	// Initialize thumbnail generator with cache
	fmt.Println("DEBUG: initializing cache")
	cache, err := duplicate.NewSQLiteCache(duplicate.DefaultCachePath())
	if err == nil {
		fmt.Println("DEBUG: cache initialized successfully")
		a.thumbGen = thumbnail.NewGenerator(cache)
	} else {
		fmt.Printf("DEBUG: cache init failed: %v\n", err)
		// Fall back to no-cache generator
		a.thumbGen = thumbnail.NewGenerator(nil)
	}
	
	// Check if we have pull data to load
	if a.pullDataPath != "" {
		fmt.Printf("DEBUG: Loading pull data from: %s\n", a.pullDataPath)
		go func() {
			// Give frontend minimal time to initialize event listeners
			time.Sleep(50 * time.Millisecond)
			
			// Read the pull data file
			data, err := os.ReadFile(a.pullDataPath)
			if err != nil {
				fmt.Printf("ERROR: Failed to read pull data: %v\n", err)
				wailsRuntime.WindowShow(a.ctx) // Show window anyway
				return
			}
			
			// Handle the pull request
			if err := a.HandlePullRequest(string(data)); err != nil {
				fmt.Printf("ERROR: Failed to handle pull request: %v\n", err)
				wailsRuntime.WindowShow(a.ctx) // Show window anyway
				return
			}
			
			// Clean up the temp file
			os.Remove(a.pullDataPath)
			
			// Show the window
			wailsRuntime.WindowShow(a.ctx)
		}()
		return
	}
	
	fmt.Println("DEBUG: starting window show goroutine")
	// Show the window initially if a photo is selected
	go func() {
		// Single quick check after brief delay for app initialization
		time.Sleep(100 * time.Millisecond)
		fmt.Println("DEBUG: checking for selected photo")
		metadata, err := a.GetSelectedPhoto()
		fmt.Printf("DEBUG: GetSelectedPhoto returned: %v, err: %v\n", metadata != nil, err)
		if err == nil && (metadata.Path != "" || metadata.IsFromPhotos) {
			// Photo is selected, show the window
			wailsRuntime.WindowShow(a.ctx)
		} else {
			// No photo selected, show window anyway
			wailsRuntime.WindowShow(a.ctx)
		}
		fmt.Println("DEBUG: window show complete")
	}()
	fmt.Println("DEBUG: startup complete")
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// Cleanup if needed
}

// ResizeWindow adjusts the window height based on whether Mastodon options are shown
func (a *App) ResizeWindow(showMastodon bool) {
	// Maintain horizontal layout, adjust height for social media fields
	if showMastodon {
		wailsRuntime.WindowSetSize(a.ctx, 900, 600)  // Extra height for social fields
	} else {
		wailsRuntime.WindowSetSize(a.ctx, 900, 500)  // Standard height
	}
}

// GetSelectedPhoto gets the currently selected photo from Finder/Photos
func (a *App) GetSelectedPhoto() (*PhotoMetadata, error) {
	// If we're in pull mode, don't try to get selected photos
	if a.pullDataPath != "" {
		return nil, fmt.Errorf("pull mode active")
	}
	
	var path string

	if runtime.GOOS == "darwin" {
		// First check if Photos has a selection
		photosCheckScript := `
		tell application "Photos"
			if running then
				if (count of selection) > 0 then
					return "has_selection"
				end if
			end if
		end tell
		return ""`
		
		cmd := exec.Command("osascript", "-e", photosCheckScript)
		if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "has_selection" {
			// Photos has a selection, use that
			return a.getPhotoMetadataFromPhotosApp()
		}
		
		// Otherwise try Finder
		script := `
		tell application "Finder"
			set theSelection to selection
			if length of theSelection is greater than 0 then
				set theFile to item 1 of theSelection as alias
				set thePath to POSIX path of theFile
				-- Remove trailing slash if it's there
				if thePath ends with "/" then
					set thePath to text 1 thru -2 of thePath
				end if
				return thePath
			end if
		end tell`

		cmd = exec.Command("osascript", "-e", script)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get Finder selection: %w", err)
		}
		path = strings.TrimSpace(string(out))
		
		if path == "" {
			return nil, fmt.Errorf("no file selected in Finder")
		}
		
		// Check if this is actually a file (not a directory)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("cannot access selected item: %w", err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("selected item is a directory, not a file")
		}
	} else {
		// Linux: Could check for nautilus/dolphin selection via DBus
		// For now, return empty - GUI can show file picker
		return &PhotoMetadata{}, nil
	}

	// Extract EXIF metadata using exiftool
	metadata := &PhotoMetadata{
		Path:    path,
		Format:  "markdown", // default
		Private: false,      // default to public
	}

	// Defer exiftool metadata extraction to background
	go func() {
		// Use full path to exiftool to avoid PATH issues
		exiftoolPath := "/usr/local/bin/exiftool"
		if _, err := os.Stat(exiftoolPath); err != nil {
			// Try homebrew location
			exiftoolPath = "/opt/homebrew/bin/exiftool"
			if _, err := os.Stat(exiftoolPath); err != nil {
				// Fall back to PATH
				exiftoolPath = "exiftool"
			}
		}
		cmd := exec.Command(exiftoolPath, "-json", "-Title", "-Caption-Abstract", "-Subject", path)
		if out, err := cmd.Output(); err == nil {
			var exifData []map[string]interface{}
			if err := json.Unmarshal(out, &exifData); err == nil && len(exifData) > 0 {
				data := exifData[0]
				
				metadataUpdate := make(map[string]interface{})
				
				if title, ok := data["Title"].(string); ok {
					metadataUpdate["title"] = title
				}
				
				if caption, ok := data["Caption-Abstract"].(string); ok {
					metadataUpdate["alt"] = caption
				}
				
				// Subject can be string or []interface{}
				var tags []string
				switch v := data["Subject"].(type) {
				case string:
					tags = strings.Split(v, ",")
				case []interface{}:
					for _, tag := range v {
						if s, ok := tag.(string); ok {
							tags = append(tags, strings.TrimSpace(s))
						}
					}
				}
				if len(tags) > 0 {
					metadataUpdate["tags"] = tags
				}
				
				// Send metadata update to frontend
				if len(metadataUpdate) > 0 {
					wailsRuntime.EventsEmit(a.ctx, "metadata-ready", metadataUpdate)
				}
			}
		}
	}()

	// Return metadata immediately, generate thumbnail async
	// Schedule thumbnail generation in background
	if a.thumbGen != nil && metadata.Path != "" {
		go func() {
			if result, err := a.thumbGen.Generate(context.Background(), metadata.Path, 100); err == nil {
				// Send thumbnail update to frontend
				wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
					"path":      metadata.Path,
					"thumbnail": "data:image/jpeg;base64," + result.ThumbnailData,
					"width":     result.Info.Width,
					"height":    result.Info.Height,
					"fileSize":  result.Info.FileSize,
				})
			}
		}()
	}

	return metadata, nil
}

// getPhotoMetadataFromPhotosApp gets metadata from the selected photo in Photos.app without exporting
func (a *App) getPhotoMetadataFromPhotosApp() (*PhotoMetadata, error) {
	// AppleScript to get metadata from Photos WITHOUT exporting
	metadataScript := `
	tell application "Photos"
		set sel to selection
		if sel is {} then
			return "ERROR:No photo selected"
		end if
		set photo to item 1 of sel
		
		-- Get metadata from Photos
		try
			set pTitle to name of photo
		on error
			set pTitle to ""
		end try
		
		try
			set pDesc to description of photo
		on error
			set pDesc to ""
		end try
		
		-- Get keywords
		set pKeywords to {}
		try
			set photoKeywords to keywords of photo
			repeat with kw in photoKeywords
				copy (kw as string) to end of pKeywords
			end repeat
		end try
		
		-- Get unique identifier
		try
			set pId to id of photo
		on error
			set pId to ""
		end try
		
		-- Get filename
		try
			set pFilename to filename of photo
		on error
			set pFilename to ""
		end try
		
		-- Build result string with delimiters
		set exportResult to "ID:" & pId & "|FILENAME:" & pFilename & "|TITLE:" & pTitle & "|DESC:" & pDesc & "|KEYWORDS:"
		if (count of pKeywords) > 0 then
			set AppleScript's text item delimiters to ","
			set exportResult to exportResult & (pKeywords as string)
			set AppleScript's text item delimiters to ""
		end if
		
		return exportResult
	end tell`
	
	cmd := exec.Command("osascript", "-e", metadataScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata from Photos: %w", err)
	}
	
	result := strings.TrimSpace(string(out))
	if strings.HasPrefix(result, "ERROR:") {
		return nil, fmt.Errorf(strings.TrimPrefix(result, "ERROR:"))
	}
	
	// Parse the metadata
	var title, desc, photoID, filename string
	var keywords []string
	
	parts := strings.Split(result, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "ID:") {
			photoID = strings.TrimPrefix(part, "ID:")
		} else if strings.HasPrefix(part, "FILENAME:") {
			filename = strings.TrimPrefix(part, "FILENAME:")
		} else if strings.HasPrefix(part, "TITLE:") {
			title = strings.TrimPrefix(part, "TITLE:")
		} else if strings.HasPrefix(part, "DESC:") {
			desc = strings.TrimPrefix(part, "DESC:")
		} else if strings.HasPrefix(part, "KEYWORDS:") {
			kwStr := strings.TrimPrefix(part, "KEYWORDS:")
			if kwStr != "" {
				keywords = strings.Split(kwStr, ",")
			}
		}
	}
	
	// Create metadata object
	metadata := &PhotoMetadata{
		Path:           "", // Will be set when exported
		Title:          title,
		Alt:            desc, // Use description as alt text
		Description:    desc,
		Tags:           keywords,
		Format:         "markdown", // default
		Private:        false,      // default to public
		IsFromPhotos:   true,
		PhotosIndex:    1, // Always first in selection for now
		PhotosID:       photoID,
		PhotosFilename: filename,
	}
	
	// Check if we have a cached thumbnail for this Photos ID
	if a.thumbGen != nil && photoID != "" {
		// Use Photos ID as cache key for consistent lookups
		if thumb, err := a.thumbGen.GetCachedThumbnail(context.Background(), photoID); err == nil && thumb != nil {
			fmt.Printf("DEBUG: Found cached thumbnail for Photos ID: %s\n", photoID)
			// Mark this Photos ID as having a cached thumbnail
			a.cachedPhotoIDs[photoID] = true
			
			// Store the thumbnail data in the metadata to be sent to frontend
			metadata.Thumbnail = "data:image/jpeg;base64," + thumb.ThumbnailData
			metadata.ImageWidth = thumb.Width
			metadata.ImageHeight = thumb.Height
			metadata.FileSize = int64(thumb.FileSize)
			
			// Still start the export for upload purposes (but no new thumbnail generation)
			go func(photosID string) {
				exportPath, err := a.exportPhotoFromPhotosApp()
				if err != nil {
					fmt.Printf("Failed to export photo: %v\n", err)
					return
				}
				// Update the path in metadata
				wailsRuntime.EventsEmit(a.ctx, "photos-path-ready", map[string]interface{}{
					"path": exportPath,
				})
			}(metadata.PhotosID)
			
			return metadata, nil
		} else {
			fmt.Printf("DEBUG: No cached thumbnail found for Photos ID: %s (err: %v)\n", photoID, err)
		}
	} else {
		fmt.Printf("DEBUG: Cache check skipped - thumbGen: %v, photoID: '%s'\n", a.thumbGen != nil, photoID)
	}
	
	// No cached thumbnail, proceed with normal async export
	
	// Start async export for preview
	go func(photosID string) {
		exportPath, err := a.exportPhotoFromPhotosApp()
		if err != nil {
			fmt.Printf("Failed to export photo for preview: %v\n", err)
			return
		}
		
		// Generate thumbnail
		if a.thumbGen != nil {
			if result, err := a.thumbGen.Generate(context.Background(), exportPath, 100); err == nil {
				// Save to cache with Photos ID if available
				if photosID != "" {
					thumb := &duplicate.Thumbnail{
						FileMD5:       photosID, // Use Photos ID as key
						ThumbnailData: result.ThumbnailData,
						Width:         result.Info.Width,
						Height:        result.Info.Height,
						FileSize:      result.Info.FileSize,
						CreatedAt:     time.Now(),
					}
					if err := a.thumbGen.SaveThumbnail(thumb); err != nil {
						fmt.Printf("DEBUG: Failed to save thumbnail to cache: %v\n", err)
					} else {
						fmt.Printf("DEBUG: Saved thumbnail to cache with Photos ID: %s\n", photosID)
					}
				}
				
				// Send both the path and thumbnail to frontend
				wailsRuntime.EventsEmit(a.ctx, "photos-export-ready", map[string]interface{}{
					"path":      exportPath,
					"thumbnail": "data:image/jpeg;base64," + result.ThumbnailData,
					"width":     result.Info.Width,
					"height":    result.Info.Height,
					"fileSize":  result.Info.FileSize,
				})
			}
		}
	}(metadata.PhotosID)
	
	return metadata, nil
}

// exportPhotoFromPhotosApp exports the first selected photo from Photos.app
func (a *App) exportPhotoFromPhotosApp() (string, error) {
	return a.exportPhotoFromPhotosAppByIndex(1)
}

// exportPhotoFromPhotosAppByIndex exports a specific photo from Photos.app by index (1-based)
func (a *App) exportPhotoFromPhotosAppByIndex(photoIndex int) (string, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "imgupv2-photos-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	// AppleScript to export from Photos
	exportScript := fmt.Sprintf(`
	set tempFolder to "%s"
	
	tell application "Photos"
		set sel to selection
		if sel is {} then
			return "ERROR:No photo selected"
		end if
		if (count of sel) < %d then
			return "ERROR:Photo index %d is out of range"
		end if
		set photo to item %d of sel
		
		-- Export with most recent edits
		export {photo} to (POSIX file tempFolder)
		
		return "OK"
	end tell`, tempDir, photoIndex, photoIndex, photoIndex)
	
	cmd := exec.Command("osascript", "-e", exportScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to export from Photos: %w\nOutput: %s", err, string(out))
	}
	
	result := strings.TrimSpace(string(out))
	if strings.HasPrefix(result, "ERROR:") {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf(strings.TrimPrefix(result, "ERROR:"))
	}
	
	// Wait for export to complete
	time.Sleep(1 * time.Second)
	
	// Find the exported file
	files, err := os.ReadDir(tempDir)
	if err != nil || len(files) == 0 {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("no file exported from Photos")
	}
	
	// Get the most recent file
	exportedPath := filepath.Join(tempDir, files[0].Name())
	fmt.Printf("DEBUG: Photos exported file: %s\n", exportedPath)
	
	// Check if it's a HEIC file and convert to JPEG if needed
	ext := strings.ToLower(filepath.Ext(exportedPath))
	if ext == ".heic" || ext == ".heif" {
		fmt.Printf("DEBUG: Converting HEIC to JPEG: %s\n", exportedPath)
		// Convert HEIC to JPEG using sips (built into macOS)
		jpegPath := strings.TrimSuffix(exportedPath, ext) + ".jpg"
		cmd := exec.Command("sips", "-s", "format", "jpeg", "-s", "formatOptions", "high", exportedPath, "--out", jpegPath)
		if err := cmd.Run(); err != nil {
			// Try to continue with HEIC file anyway
			fmt.Printf("Warning: failed to convert HEIC to JPEG: %v\n", err)
		} else {
			// Remove original HEIC and use JPEG
			os.Remove(exportedPath)
			exportedPath = jpegPath
			fmt.Printf("DEBUG: Converted to: %s\n", jpegPath)
		}
	}
	
	// Schedule cleanup after 5 minutes (giving plenty of time for upload)
	go func(dir string) {
		time.Sleep(5 * time.Minute)
		os.RemoveAll(dir)
		fmt.Printf("DEBUG: Cleaned up temp directory: %s\n", dir)
	}(tempDir)
	
	return exportedPath, nil
}

// GetRecentTags returns recently used tags for autocomplete
func (a *App) GetRecentTags() []string {
	// TODO: Read from ~/.config/imgupv2/tags.json or similar
	// For now, return some common photography tags
	return []string{
		"photography", "landscape", "portrait", "street",
		"nature", "architecture", "blackandwhite", "travel",
		"sunset", "sunrise", "night", "urban",
	}
}

// Upload handles the actual upload via imgup CLI
func (a *App) Upload(metadata PhotoMetadata) (*UploadResult, error) {
	// If this is from Photos.app and path is still empty, wait a bit or export now
	if metadata.IsFromPhotos && metadata.Path == "" {
		// Check if an export is already in progress by waiting briefly
		time.Sleep(100 * time.Millisecond)
		
		// If still no path, do a synchronous export
		if metadata.Path == "" {
			exportPath, err := a.exportPhotoFromPhotosApp()
			if err != nil {
				return &UploadResult{
					Success: false,
					Error:   fmt.Sprintf("Failed to export photo: %s", err.Error()),
				}, nil
			}
			metadata.Path = exportPath
			metadata.IsTemporary = true
		}
		
		// Re-embed metadata using exiftool if we have any
		if metadata.Title != "" || metadata.Description != "" || len(metadata.Tags) > 0 {
			// Find exiftool with full path
			exiftoolPath := "/usr/local/bin/exiftool"
			if _, err := os.Stat(exiftoolPath); err != nil {
				// Try homebrew location
				exiftoolPath = "/opt/homebrew/bin/exiftool"
				if _, err := os.Stat(exiftoolPath); err != nil {
					// Fall back to PATH
					exiftoolPath = "exiftool"
				}
			}
			
			exifArgs := []string{"-overwrite_original"}
			
			if metadata.Title != "" {
				exifArgs = append(exifArgs, "-XMP-dc:Title="+metadata.Title)
			}
			
			if metadata.Description != "" {
				exifArgs = append(exifArgs, "-ImageDescription="+metadata.Description)
				exifArgs = append(exifArgs, "-XMP-dc:Description="+metadata.Description)
			}
			
			for _, kw := range metadata.Tags {
				exifArgs = append(exifArgs, "-XMP-dc:Subject+="+strings.TrimSpace(kw))
			}
			
			exifArgs = append(exifArgs, metadata.Path)
			
			cmd := exec.Command(exiftoolPath, exifArgs...)
			if err := cmd.Run(); err != nil {
				// Non-fatal: continue even if metadata embedding fails
				fmt.Fprintf(os.Stderr, "Warning: failed to embed metadata: %v\n", err)
			}
		}
	}
	
	// Build imgup command
	args := []string{"upload"}
	
	// Always use JSON format with duplicate-info to get structured response
	args = append(args, "--format", "json")
	args = append(args, "--duplicate-info")
	
	// Only add title if not empty
	if metadata.Title != "" {
		args = append(args, "--title", metadata.Title)
	}
	
	// Only add alt text if not empty  
	if metadata.Alt != "" {
		args = append(args, "--alt", metadata.Alt)
	}
	
	// Only add description if not empty
	if metadata.Description != "" {
		args = append(args, "--description", metadata.Description)
	}

	// Add tags if present
	if len(metadata.Tags) > 0 {
		// Join tags with commas as per the help text
		args = append(args, "--tags", strings.Join(metadata.Tags, ","))
	}
	
	// Add private flag if set
	if metadata.Private {
		args = append(args, "--private")
	}
	
	// Add social media flags if enabled
	postText := ""
	
	// Determine post text (prefer whichever has content, or use the first one)
	if metadata.MastodonEnabled && metadata.MastodonText != "" {
		postText = metadata.MastodonText
	} else if metadata.BlueskyEnabled && metadata.BlueskyText != "" {
		postText = metadata.BlueskyText
	}
	
	// Add Mastodon flags if enabled
	if metadata.MastodonEnabled {
		args = append(args, "--mastodon")
		
		if metadata.MastodonVisibility != "" {
			args = append(args, "--visibility", metadata.MastodonVisibility)
		}
	}
	
	// Add Bluesky flags if enabled
	if metadata.BlueskyEnabled {
		args = append(args, "--bluesky")
	}
	
	// Add shared post text if either service is enabled and has text
	if postText != "" && (metadata.MastodonEnabled || metadata.BlueskyEnabled) {
		args = append(args, "--post", postText)
	}

	// Add the file path at the end
	args = append(args, metadata.Path)

	// Find imgup binary - check multiple locations
	imgupPath := "imgup"
	
	// Check common locations in order of preference
	searchPaths := []string{
		filepath.Join(os.Getenv("HOME"), "go", "bin", "imgup"),  // ~/go/bin/imgup
		filepath.Join("..", "imgup"),                             // parent directory (for development)
		"/opt/homebrew/bin/imgup",                               // Apple Silicon homebrew
		"/usr/local/bin/imgup",                                   // Intel homebrew
		"imgup",                                                  // rely on PATH
	}
	
	for _, path := range searchPaths {
		if fileExists(path) {
			imgupPath = path
			break
		}
	}

	// Run imgup CLI
	cmd := exec.Command(imgupPath, args...)
	
	// Use Output() which waits for the command to complete
	output, err := cmd.Output()
	if err != nil {
		// Get stderr if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &UploadResult{
				Success: false,
				Error:   fmt.Sprintf("Upload failed: %s\nStderr: %s", err.Error(), string(exitErr.Stderr)),
			}, nil
		}
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Upload failed: %s", err.Error()),
		}, nil
	}
	
	// Extract the JSON response from stdout
	outputStr := strings.TrimSpace(string(output))
	snippet := ""
	isDuplicate := false
	socialPostStatus := ""
	
	// Find the JSON response - it should be a complete JSON object
	// Look for a line that starts with { and parse from there
	jsonStart := strings.LastIndex(outputStr, "{")
	if jsonStart >= 0 {
		jsonStr := outputStr[jsonStart:]
		// Find the matching closing brace
		braceCount := 0
		jsonEnd := -1
		for i, ch := range jsonStr {
			if ch == '{' {
				braceCount++
			} else if ch == '}' {
				braceCount--
				if braceCount == 0 {
					jsonEnd = i + 1
					break
				}
			}
		}
		
		if jsonEnd > 0 {
			jsonLine := jsonStr[:jsonEnd]
			var jsonResponse struct {
				URL       string `json:"url"`
				Duplicate bool   `json:"duplicate"`
				PhotoID   string `json:"photoId"`
				ImageURL  string `json:"imageUrl,omitempty"`
			}
			if err := json.Unmarshal([]byte(jsonLine), &jsonResponse); err == nil {
				isDuplicate = jsonResponse.Duplicate
				
				// Check for social media posting output after JSON
				remainingOutput := outputStr[jsonStart+jsonEnd:]
				if remainingOutput != "" {
					// Check for successful posts
					if strings.Contains(remainingOutput, "Posted to Mastodon successfully!") {
						socialPostStatus = "mastodon_success"
					} else if strings.Contains(remainingOutput, "Mastodon post failed:") {
						socialPostStatus = "mastodon_failed"
					}
					
					// Check for Bluesky (can be both)
					if strings.Contains(remainingOutput, "Posted to Bluesky successfully!") {
						if socialPostStatus == "mastodon_success" {
							socialPostStatus = "both_success"
						} else {
							socialPostStatus = "bluesky_success"
						}
					} else if strings.Contains(remainingOutput, "Bluesky post failed:") {
						if socialPostStatus == "mastodon_failed" {
							socialPostStatus = "both_failed"
						} else if socialPostStatus == "mastodon_success" {
							socialPostStatus = "mastodon_success_bluesky_failed"
						} else {
							socialPostStatus = "bluesky_failed"
						}
					}
				}
				
				// Convert to requested format
				switch metadata.Format {
				case "url":
					snippet = jsonResponse.URL
				case "markdown":
					// Use imageURL if available, fall back to URL
					imageURL := jsonResponse.ImageURL
					if imageURL == "" {
						imageURL = jsonResponse.URL
					}
					// Basic markdown format
					title := metadata.Title
					if title == "" {
						title = "Image"
					}
					snippet = fmt.Sprintf("![%s](%s)", title, imageURL)
				case "html":
					// Basic HTML format
					altText := metadata.Alt
					if altText == "" {
						altText = metadata.Title
						if altText == "" {
							altText = "Image"
						}
					}
					snippet = fmt.Sprintf(`<img src="%s" alt="%s">`, jsonResponse.URL, altText)
				case "json":
					// Keep the original JSON
					snippet = jsonLine
				default:
					// Default to URL
					snippet = jsonResponse.URL
				}
			} else {
				// If JSON parsing fails, log the error and return the raw output
				fmt.Fprintf(os.Stderr, "Failed to parse JSON response: %v\nJSON: %s\n", err, jsonLine)
				snippet = outputStr
			}
		} else {
			// Could not find complete JSON, use raw output
			snippet = outputStr
		}
	} else {
		// No JSON found, use raw output
		snippet = outputStr
	}

	return &UploadResult{
		Success: true,
		Snippet: snippet,
		Duplicate: isDuplicate,
		ForceAvailable: isDuplicate, // Can use --force if it's a duplicate
		SocialPostStatus: socialPostStatus,
	}, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ForceUpload handles upload with --force flag for duplicates
func (a *App) ForceUpload(metadata PhotoMetadata) (*UploadResult, error) {
	// If this is from Photos.app and hasn't been exported yet, export it now
	if metadata.IsFromPhotos && metadata.Path == "" {
		exportPath, err := a.exportPhotoFromPhotosApp()
		if err != nil {
			return &UploadResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to export photo: %s", err.Error()),
			}, nil
		}
		metadata.Path = exportPath
		metadata.IsTemporary = true
		
		// Re-embed metadata using exiftool if we have any
		if metadata.Title != "" || metadata.Description != "" || len(metadata.Tags) > 0 {
			// Find exiftool with full path
			exiftoolPath := "/usr/local/bin/exiftool"
			if _, err := os.Stat(exiftoolPath); err != nil {
				// Try homebrew location
				exiftoolPath = "/opt/homebrew/bin/exiftool"
				if _, err := os.Stat(exiftoolPath); err != nil {
					// Fall back to PATH
					exiftoolPath = "exiftool"
				}
			}
			
			exifArgs := []string{"-overwrite_original"}
			
			if metadata.Title != "" {
				exifArgs = append(exifArgs, "-XMP-dc:Title="+metadata.Title)
			}
			
			if metadata.Description != "" {
				exifArgs = append(exifArgs, "-ImageDescription="+metadata.Description)
				exifArgs = append(exifArgs, "-XMP-dc:Description="+metadata.Description)
			}
			
			for _, kw := range metadata.Tags {
				exifArgs = append(exifArgs, "-XMP-dc:Subject+="+strings.TrimSpace(kw))
			}
			
			exifArgs = append(exifArgs, metadata.Path)
			
			cmd := exec.Command(exiftoolPath, exifArgs...)
			if err := cmd.Run(); err != nil {
				// Non-fatal: continue even if metadata embedding fails
				fmt.Fprintf(os.Stderr, "Warning: failed to embed metadata: %v\n", err)
			}
		}
	}
	
	// Build imgup command with --force flag
	args := []string{"upload", "--force"}
	
	// Always use JSON format with duplicate-info to get structured response
	args = append(args, "--format", "json")
	args = append(args, "--duplicate-info")
	
	// Only add title if not empty
	if metadata.Title != "" {
		args = append(args, "--title", metadata.Title)
	}
	
	// Only add alt text if not empty  
	if metadata.Alt != "" {
		args = append(args, "--alt", metadata.Alt)
	}
	
	// Only add description if not empty
	if metadata.Description != "" {
		args = append(args, "--description", metadata.Description)
	}

	// Add tags if present
	if len(metadata.Tags) > 0 {
		// Join tags with commas as per the help text
		args = append(args, "--tags", strings.Join(metadata.Tags, ","))
	}
	
	// Add private flag if set
	if metadata.Private {
		args = append(args, "--private")
	}
	
	// Add social media flags if enabled
	postText := ""
	
	// Determine post text (prefer whichever has content, or use the first one)
	if metadata.MastodonEnabled && metadata.MastodonText != "" {
		postText = metadata.MastodonText
	} else if metadata.BlueskyEnabled && metadata.BlueskyText != "" {
		postText = metadata.BlueskyText
	}
	
	// Add Mastodon flags if enabled
	if metadata.MastodonEnabled {
		args = append(args, "--mastodon")
		
		if metadata.MastodonVisibility != "" {
			args = append(args, "--visibility", metadata.MastodonVisibility)
		}
	}
	
	// Add Bluesky flags if enabled
	if metadata.BlueskyEnabled {
		args = append(args, "--bluesky")
	}
	
	// Add shared post text if either service is enabled and has text
	if postText != "" && (metadata.MastodonEnabled || metadata.BlueskyEnabled) {
		args = append(args, "--post", postText)
	}

	// Add the file path at the end
	args = append(args, metadata.Path)

	// Find imgup binary - check multiple locations
	imgupPath := "imgup"
	
	// Check common locations in order of preference
	searchPaths := []string{
		filepath.Join(os.Getenv("HOME"), "go", "bin", "imgup"),  // ~/go/bin/imgup
		filepath.Join("..", "imgup"),                             // parent directory (for development)
		"/opt/homebrew/bin/imgup",                               // Apple Silicon homebrew
		"/usr/local/bin/imgup",                                   // Intel homebrew
		"imgup",                                                  // rely on PATH
	}
	
	for _, path := range searchPaths {
		if fileExists(path) {
			imgupPath = path
			break
		}
	}

	// Run imgup CLI
	cmd := exec.Command(imgupPath, args...)
	
	// Use Output() which waits for the command to complete
	output, err := cmd.Output()
	if err != nil {
		// Get stderr if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &UploadResult{
				Success: false,
				Error:   fmt.Sprintf("Upload failed: %s\nStderr: %s", err.Error(), string(exitErr.Stderr)),
			}, nil
		}
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Upload failed: %s", err.Error()),
		}, nil
	}
	
	// Extract the JSON response from stdout
	outputStr := strings.TrimSpace(string(output))
	snippet := ""
	
	// Find the JSON response - it should be a complete JSON object
	jsonStart := strings.LastIndex(outputStr, "{")
	if jsonStart >= 0 {
		jsonStr := outputStr[jsonStart:]
		// Find the matching closing brace
		braceCount := 0
		jsonEnd := -1
		for i, ch := range jsonStr {
			if ch == '{' {
				braceCount++
			} else if ch == '}' {
				braceCount--
				if braceCount == 0 {
					jsonEnd = i + 1
					break
				}
			}
		}
		
		if jsonEnd > 0 {
			jsonLine := jsonStr[:jsonEnd]
			var jsonResponse struct {
				URL       string `json:"url"`
				Duplicate bool   `json:"duplicate"`
				PhotoID   string `json:"photoId"`
				ImageURL  string `json:"imageUrl,omitempty"`
			}
			if err := json.Unmarshal([]byte(jsonLine), &jsonResponse); err == nil {
				// Convert to requested format
				switch metadata.Format {
				case "url":
					snippet = jsonResponse.URL
				case "markdown":
					// Use imageURL if available, fall back to URL
					imageURL := jsonResponse.ImageURL
					if imageURL == "" {
						imageURL = jsonResponse.URL
					}
					// Basic markdown format
					title := metadata.Title
					if title == "" {
						title = "Image"
					}
					snippet = fmt.Sprintf("![%s](%s)", title, imageURL)
				case "html":
					// Basic HTML format
					altText := metadata.Alt
					if altText == "" {
						altText = metadata.Title
						if altText == "" {
							altText = "Image"
						}
					}
					snippet = fmt.Sprintf(`<img src="%s" alt="%s">`, jsonResponse.URL, altText)
				case "json":
					// Keep the original JSON
					snippet = jsonLine
				default:
					// Default to URL
					snippet = jsonResponse.URL
				}
			} else {
				// If JSON parsing fails, log the error and return the raw output
				fmt.Fprintf(os.Stderr, "Failed to parse JSON response: %v\nJSON: %s\n", err, jsonLine)
				snippet = outputStr
			}
		} else {
			// Could not find complete JSON, use raw output
			snippet = outputStr
		}
	} else {
		// No JSON found, use raw output
		snippet = outputStr
	}

	return &UploadResult{
		Success: true,
		Snippet: snippet,
		Duplicate: false, // Force upload always creates new upload
		ForceAvailable: false,
	}, nil
}



// findImgupBinary locates the imgup binary
func (a *App) findImgupBinary() string {
	// Check common locations in order of preference
	searchPaths := []string{
		filepath.Join(os.Getenv("HOME"), "go", "bin", "imgup"),
		filepath.Join("..", "imgup"),
		"/opt/homebrew/bin/imgup",
		"/usr/local/bin/imgup",
		"imgup",
	}
	
	for _, path := range searchPaths {
		if fileExists(path) {
			return path
		}
	}
	
	return "imgup" // Fall back to PATH
}

// findExiftoolBinary locates the exiftool binary
func (a *App) findExiftoolBinary() string {
	// Check common locations in order of preference
	searchPaths := []string{
		"/opt/homebrew/bin/exiftool",  // Apple Silicon homebrew
		"/usr/local/bin/exiftool",      // Intel homebrew
		"exiftool",                     // Fall back to PATH
	}
	
	for _, path := range searchPaths {
		if fileExists(path) {
			return path
		}
	}
	
	return "exiftool" // Fall back to PATH
}

// generateThumbnail generates a base64-encoded thumbnail for an image
func (a *App) generateThumbnail(imagePath string) (string, error) {
	fmt.Printf("DEBUG: generateThumbnail called for: %s\n", imagePath)
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(imagePath))
	isRaw := ext == ".dng" || ext == ".raw" || ext == ".cr2" || ext == ".nef" || ext == ".arw"
	
	// Use sips on macOS to generate thumbnail
	tempFile, err := os.CreateTemp("", fmt.Sprintf("thumb-%d-*.jpg", time.Now().UnixNano()))
	if err != nil {
		return "", err
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()
	
	fmt.Printf("DEBUG: Temp thumbnail file: %s\n", tempFile.Name())
	
	// For raw files, try to extract embedded JPEG first
	if isRaw {
		// Try using exiftool to extract embedded preview
		exifPaths := []string{
			"/opt/homebrew/bin/exiftool",
			"/usr/local/bin/exiftool",
		}
		
		var exifPath string
		for _, path := range exifPaths {
			if fileExists(path) {
				exifPath = path
				break
			}
		}
		
		if exifPath != "" {
			// Extract preview image
			cmd := exec.Command(exifPath, "-b", "-PreviewImage", imagePath)
			previewData, err := cmd.Output()
			if err == nil && len(previewData) > 0 {
				// Write preview to temp file
				if err := os.WriteFile(tempFile.Name(), previewData, 0644); err == nil {
					// Try to resize the preview
					resizeCmd := exec.Command("sips", "-Z", "64", tempFile.Name())
					if resizeCmd.Run() == nil {
						// Read the resized thumbnail
						thumbData, err := os.ReadFile(tempFile.Name())
						if err == nil {
							return fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(thumbData)), nil
						}
					}
					// If resize failed, just use the preview as-is
					return fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(previewData)), nil
				}
			}
		}
		
		// If we can't extract preview, try using qlmanage to generate thumbnail
		// Create a unique output directory for this thumbnail
		outputDir := filepath.Join(os.TempDir(), fmt.Sprintf("ql-%d", time.Now().UnixNano()))
		os.MkdirAll(outputDir, 0755)
		defer os.RemoveAll(outputDir)
		
		qlCmd := exec.Command("qlmanage", "-t", "-s", "64", "-o", outputDir, imagePath)
		qlCmd.Run() // Ignore errors, it creates files with specific names
		
		// qlmanage creates files with .png extension
		qlThumbPath := filepath.Join(outputDir, filepath.Base(imagePath)+".png")
		fmt.Printf("DEBUG: Looking for qlmanage thumbnail at: %s\n", qlThumbPath)
		if thumbData, err := os.ReadFile(qlThumbPath); err == nil {
			result := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(thumbData))
			fmt.Printf("DEBUG: Generated thumbnail via qlmanage for raw, size: %d bytes\n", len(thumbData))
			return result, nil
		}
		
		// If all else fails for raw files, return empty
		return "", fmt.Errorf("unable to generate thumbnail for raw file %s", ext)
	}
	
	// For non-raw files, use sips directly
	cmd := exec.Command("sips", "-Z", "64", imagePath, "--out", tempFile.Name())
	if err := cmd.Run(); err != nil {
		// Try qlmanage as fallback
		outputDir := filepath.Join(os.TempDir(), fmt.Sprintf("ql-%d", time.Now().UnixNano()))
		os.MkdirAll(outputDir, 0755)
		defer os.RemoveAll(outputDir)
		
		qlCmd := exec.Command("qlmanage", "-t", "-s", "64", "-o", outputDir, imagePath)
		if err := qlCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to generate thumbnail: %w", err)
		}
		
		// qlmanage creates files with .png extension
		qlThumbPath := filepath.Join(outputDir, filepath.Base(imagePath)+".png")
		if thumbData, err := os.ReadFile(qlThumbPath); err == nil {
			result := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(thumbData))
			fmt.Printf("DEBUG: Generated thumbnail via qlmanage fallback, size: %d bytes\n", len(thumbData))
			return result, nil
		}
		
		return "", fmt.Errorf("failed to generate thumbnail: %w", err)
	}
	
	// Read the thumbnail file
	thumbData, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return "", err
	}
	
	// Convert to base64
	result := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(thumbData))
	fmt.Printf("DEBUG: Generated thumbnail via sips, size: %d bytes, first 20 chars of base64: %s...\n", 
		len(thumbData), result[22:42])
	return result, nil
}

// extractMetadata extracts metadata from an image file
func (a *App) extractMetadata(imagePath string) PhotoMetadata {
	metadata := PhotoMetadata{
		Path: imagePath,
		Format: "markdown",
	}
	
	// Try to extract metadata using exiftool
	exifPaths := []string{
		"/opt/homebrew/bin/exiftool",
		"/usr/local/bin/exiftool", 
	}
	
	var exifPath string
	for _, path := range exifPaths {
		if fileExists(path) {
			exifPath = path
			break
		}
	}
	
	if exifPath != "" {
		// Extract title and keywords using exiftool
		cmd := exec.Command(exifPath, "-Title", "-Subject", "-Keywords", "-Description", "-j", imagePath)
		out, err := cmd.Output()
		if err == nil {
			var exifData []map[string]interface{}
			if err := json.Unmarshal(out, &exifData); err == nil && len(exifData) > 0 {
				data := exifData[0]
				
				if title, ok := data["Title"].(string); ok {
					metadata.Title = title
				}
				
				if desc, ok := data["Description"].(string); ok {
					metadata.Description = desc
					metadata.Alt = desc // Use description as alt text
				}
				
				// Extract keywords/tags
				var tags []string
				if keywords, ok := data["Keywords"]; ok {
					switch v := keywords.(type) {
					case string:
						tags = append(tags, v)
					case []interface{}:
						for _, tag := range v {
							if s, ok := tag.(string); ok {
								tags = append(tags, s)
							}
						}
					}
				}
				
				if subject, ok := data["Subject"]; ok {
					switch v := subject.(type) {
					case string:
						tags = append(tags, v)
					case []interface{}:
						for _, tag := range v {
							if s, ok := tag.(string); ok {
								tags = append(tags, s)
							}
						}
					}
				}
				
				metadata.Tags = tags
			}
		}
	}
	
	return metadata
}

// PullPhotoData represents photo data from pull command
type PullPhotoData struct {
	PhotoMetadata
	RemoteURL    string `json:"remoteUrl"`    // URL to the original photo page
	ThumbnailURL string `json:"thumbnailUrl"` // URL for thumbnail
	ImageURLs    struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
	} `json:"imageUrls"`
	SourceService string `json:"sourceService"` // "smugmug" or "flickr"
	SourceAlbum   string `json:"sourceAlbum"`
}

// HandlePullRequest receives pull data from CLI and prepares GUI for selection
func (a *App) HandlePullRequest(pullJSON string) error {
	fmt.Printf("DEBUG: HandlePullRequest called with %d bytes of JSON\n", len(pullJSON))
	
	// Parse the pull request
	var pullReq types.PullRequest
	if err := json.Unmarshal([]byte(pullJSON), &pullReq); err != nil {
		return fmt.Errorf("failed to parse pull request: %w", err)
	}
	
	fmt.Printf("DEBUG: Parsed pull request with %d images from %s\n", len(pullReq.Images), pullReq.Source.Service)
	fmt.Printf("DEBUG: Pull targets: %v, Post text: %q\n", pullReq.Targets, pullReq.Post)
	
	// Convert to GUI photo data format
	var photos []PullPhotoData
	for _, img := range pullReq.Images {
		photo := PullPhotoData{
			PhotoMetadata: PhotoMetadata{
				Title:       img.Title,
				Description: img.Description,
				Alt:         img.Alt,
				Tags:        img.Tags,
				Format:      "url", // Default for pull operations
			},
			RemoteURL:     img.SourceURL,
			ThumbnailURL:  img.Sizes.Thumb,
			SourceService: pullReq.Source.Service,
			SourceAlbum:   pullReq.Source.Album,
		}
		
		// Store image URLs
		photo.ImageURLs.Large = img.Sizes.Large
		photo.ImageURLs.Medium = img.Sizes.Medium
		photo.ImageURLs.Small = img.Sizes.Small
		
		photos = append(photos, photo)
	}
	
	// Store pull context for later use
	a.currentPullRequest = &pullReq
	
	// Download thumbnails in parallel
	go a.downloadPullThumbnails(photos)
	
	// Emit event to frontend with pull data
	wailsRuntime.EventsEmit(a.ctx, "pull-mode-init", map[string]interface{}{
		"photos":     photos,
		"service":    pullReq.Source.Service,
		"album":      pullReq.Source.Album,
		"postText":   pullReq.Post,
		"targets":    pullReq.Targets,
		"visibility": pullReq.Visibility,
		"format":     pullReq.Format,
	})
	
	return nil
}

// downloadPullThumbnails downloads thumbnails for pull photos in parallel
func (a *App) downloadPullThumbnails(photos []PullPhotoData) {
	for i, photo := range photos {
		go func(index int, p PullPhotoData) {
			if p.ThumbnailURL == "" {
				return
			}
			
			// Download thumbnail
			resp, err := http.Get(p.ThumbnailURL)
			if err != nil {
				fmt.Printf("Failed to download thumbnail for %s: %v\n", p.Title, err)
				return
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Failed to download thumbnail for %s: status %d\n", p.Title, resp.StatusCode)
				return
			}
			
			// Read thumbnail data
			thumbData, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Failed to read thumbnail for %s: %v\n", p.Title, err)
				return
			}
			
			// Convert to base64
			base64Thumb := base64.StdEncoding.EncodeToString(thumbData)
			
			// Emit thumbnail ready event
			wailsRuntime.EventsEmit(a.ctx, "pull-thumbnail-ready", map[string]interface{}{
				"index":     index,
				"thumbnail": "data:image/jpeg;base64," + base64Thumb,
			})
		}(i, photo)
	}
}

// PostPullSelection handles the social media posting for selected pull images
func (a *App) PostPullSelection(request types.PullRequest) (*MultiPhotoUploadResult, error) {
	// This will be implemented to handle the actual posting
	// For now, return a placeholder
	return &MultiPhotoUploadResult{
		Success: false,
		Error:   "Pull posting not yet implemented",
	}, nil
}
