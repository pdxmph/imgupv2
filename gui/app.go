package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
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
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Caption  string   `json:"caption"`
	Tags     []string `json:"tags"`
	Backend  string   `json:"backend"` // "flickr" or "smugmug"
	Format   string   `json:"format"`  // "markdown", "html", "org"
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
				return POSIX path of theFile
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
	} else {
		// Linux: Could check for nautilus/dolphin selection via DBus
		// For now, return empty - GUI can show file picker
		return &PhotoMetadata{}, nil
	}

	// Extract EXIF metadata using exiftool
	metadata := &PhotoMetadata{
		Path:    path,
		Backend: "flickr", // default
		Format:  "markdown",
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
				metadata.Caption = caption
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
	args := []string{
		"upload",
		"--backend", metadata.Backend,
		"--format", metadata.Format,
		"--title", metadata.Title,
		"--caption", metadata.Caption,
	}

	for _, tag := range metadata.Tags {
		if tag != "" {
			args = append(args, "--tag", tag)
		}
	}

	args = append(args, metadata.Path)

	// Find imgup binary - first check if it's in the parent directory
	imgupPath := "imgup"
	if parentImgup := filepath.Join("..", "imgup"); fileExists(parentImgup) {
		imgupPath = parentImgup
	}

	// Run imgup CLI
	cmd := exec.Command(imgupPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Upload failed: %s\nOutput: %s", err.Error(), string(out)),
		}, nil
	}

	return &UploadResult{
		Success: true,
		Snippet: string(out),
	}, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := exec.LookPath(path)
	return err == nil
}
