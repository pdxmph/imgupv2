package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// App struct
type App struct {
	ctx context.Context
}

// PhotoMetadata represents the metadata for a photo
type PhotoMetadata struct {
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Alt         string   `json:"alt"`         // Alt text, not caption
	Description string   `json:"description"` // Photo description
	Tags        []string `json:"tags"`
	Format      string   `json:"format"` // "url", "markdown", "html", "json"
	Private     bool     `json:"private"`
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	Success bool   `json:"success"`
	Snippet string `json:"snippet"`
	Error   string `json:"error,omitempty"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetSelectedPhoto gets the currently selected photo from Finder/Photos
func (a *App) GetSelectedPhoto() (*PhotoMetadata, error) {
	var path string

	if runtime.GOOS == "darwin" {
		// AppleScript to get selected file in Finder
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

		cmd := exec.Command("osascript", "-e", script)
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
	cmd := exec.Command("exiftool", "-json", "-Title", "-Caption-Abstract", "-Subject", path)
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
	
	// Add format flag
	args = append(args, "--format", metadata.Format)
	
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

	// Add the file path at the end
	args = append(args, metadata.Path)

	// Find imgup binary - first check if it's in the parent directory
	imgupPath := "imgup"
	if parentImgup := filepath.Join("..", "imgup"); fileExists(parentImgup) {
		imgupPath = parentImgup
	}

	// Run imgup CLI
	cmd := exec.Command(imgupPath, args...)
	
	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Upload failed: %s\nStderr: %s\nStdout: %s", err.Error(), stderr.String(), stdout.String()),
		}, nil
	}
	
	// Extract just the final line (the snippet) from stdout
	output := strings.TrimSpace(stdout.String())
	lines := strings.Split(output, "\n")
	snippet := ""
	if len(lines) > 0 {
		// The snippet should be the last non-empty line
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				snippet = lines[i]
				break
			}
		}
	}

	return &UploadResult{
		Success: true,
		Snippet: snippet,
	}, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
