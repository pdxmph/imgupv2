package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
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

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	// Show the window initially if a photo is selected
	go func() {
		// Try a few times with delays to handle launch timing issues
		for i := 0; i < 3; i++ {
			time.Sleep(time.Duration(500+i*250) * time.Millisecond)
			metadata, err := a.GetSelectedPhoto()
			if err == nil && metadata.Path != "" {
				// Photo is selected, show the window
				wailsRuntime.WindowShow(a.ctx)
				return
			}
		}
		// No photo selected after retries, show window anyway with a message
		wailsRuntime.WindowShow(a.ctx)
	}()
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// Cleanup if needed
}

// ResizeWindow adjusts the window height based on whether Mastodon options are shown
func (a *App) ResizeWindow(showMastodon bool) {
	if showMastodon {
		wailsRuntime.WindowSetSize(a.ctx, 580, 660)
	} else {
		wailsRuntime.WindowSetSize(a.ctx, 580, 510)
	}
}

// GetSelectedPhoto gets the currently selected photo from Finder/Photos
func (a *App) GetSelectedPhoto() (*PhotoMetadata, error) {
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
			return a.getPhotoFromPhotosApp()
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

	// Run exiftool to get title/caption/keywords
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
			
			if title, ok := data["Title"].(string); ok {
				metadata.Title = title
			}
			
			if caption, ok := data["Caption-Abstract"].(string); ok {
				metadata.Alt = caption // Use caption as alt text
			}
			
			// Subject can be string or []interface{}
			switch v := data["Subject"].(type) {
			case string:
				metadata.Tags = strings.Split(v, ",")
			case []interface{}:
				for _, tag := range v {
					if s, ok := tag.(string); ok {
						metadata.Tags = append(metadata.Tags, strings.TrimSpace(s))
					}
				}
			}
		}
	}

	return metadata, nil
}

// getPhotoFromPhotosApp exports the selected photo from Photos.app with metadata
func (a *App) getPhotoFromPhotosApp() (*PhotoMetadata, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "imgupv2-photos-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	// AppleScript to export and get metadata from Photos
	exportScript := fmt.Sprintf(`
	set tempFolder to "%s"
	set exportResult to ""
	
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
		
		-- Get keywords (they're already strings)
		set pKeywords to {}
		try
			set photoKeywords to keywords of photo
			repeat with kw in photoKeywords
				copy (kw as string) to end of pKeywords
			end repeat
		end try
		
		-- Export with most recent edits as JPEG
		export {photo} to (POSIX file tempFolder)
		
		-- Build result string with delimiters
		set exportResult to "TITLE:" & pTitle & "|DESC:" & pDesc & "|KEYWORDS:"
		if (count of pKeywords) > 0 then
			set AppleScript's text item delimiters to ","
			set exportResult to exportResult & (pKeywords as string)
			set AppleScript's text item delimiters to ""
		end if
		
		return exportResult
	end tell`, tempDir)
	
	cmd := exec.Command("osascript", "-e", exportScript)
	out, err := cmd.CombinedOutput() // This captures both stdout and stderr
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to export from Photos: %w\nOutput: %s", err, string(out))
	}
	
	result := strings.TrimSpace(string(out))
	if strings.HasPrefix(result, "ERROR:") {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf(strings.TrimPrefix(result, "ERROR:"))
	}
	
	// Parse the metadata
	var title, desc string
	var keywords []string
	
	parts := strings.Split(result, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "TITLE:") {
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
	
	// Wait for export to complete
	time.Sleep(1 * time.Second)
	
	// Find the exported file
	files, err := os.ReadDir(tempDir)
	if err != nil || len(files) == 0 {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("no file exported from Photos")
	}
	
	// Get the most recent file
	exportedPath := filepath.Join(tempDir, files[0].Name())
	
	// Re-embed metadata using exiftool if we have any
	if title != "" || desc != "" || len(keywords) > 0 {
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
		
		if title != "" {
			exifArgs = append(exifArgs, "-XMP-dc:Title="+title)
		}
		
		if desc != "" {
			exifArgs = append(exifArgs, "-ImageDescription="+desc)
			exifArgs = append(exifArgs, "-XMP-dc:Description="+desc)
		}
		
		for _, kw := range keywords {
			exifArgs = append(exifArgs, "-XMP-dc:Subject+="+strings.TrimSpace(kw))
		}
		
		exifArgs = append(exifArgs, exportedPath)
		
		cmd := exec.Command(exiftoolPath, exifArgs...)
		if err := cmd.Run(); err != nil {
			// Non-fatal: continue even if metadata embedding fails
			fmt.Fprintf(os.Stderr, "Warning: failed to embed metadata: %v\n", err)
		}
		
		// Give exiftool time to write
		time.Sleep(500 * time.Millisecond)
	}
	
	// Create metadata object
	metadata := &PhotoMetadata{
		Path:        exportedPath,
		Title:       title,
		Alt:         desc, // Use description as alt text
		Description: desc,
		Tags:        keywords,
		Format:      "markdown", // default
		Private:     false,      // default to public
		// Mark this as a temp file that needs cleanup
	}
	
	// Schedule cleanup after 60 seconds (giving time for upload)
	go func() {
		time.Sleep(60 * time.Second)
		os.RemoveAll(tempDir)
	}()
	
	return metadata, nil
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
					// Basic markdown format
					title := metadata.Title
					if title == "" {
						title = "Image"
					}
					snippet = fmt.Sprintf("![%s](%s)", title, jsonResponse.URL)
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
					// Basic markdown format
					title := metadata.Title
					if title == "" {
						title = "Image"
					}
					snippet = fmt.Sprintf("![%s](%s)", title, jsonResponse.URL)
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
