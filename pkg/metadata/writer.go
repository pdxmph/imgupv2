package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Writer handles writing metadata to images
// Deprecated: Metadata embedding is no longer used. Flickr and SmugMug APIs handle metadata directly.
type Writer struct {
	exiftoolPath string
}

// NewWriter creates a new metadata writer
// Deprecated: Use backend APIs directly instead of embedding metadata
func NewWriter() (*Writer, error) {
	// Check if exiftool is available in PATH
	path, err := exec.LookPath("exiftool")
	if err == nil {
		return &Writer{
			exiftoolPath: path,
		}, nil
	}
	
	// Check common locations
	possiblePaths := []string{
		"/opt/homebrew/bin/exiftool",
		"/usr/local/bin/exiftool",
		"/usr/bin/exiftool",
	}
	
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return &Writer{
				exiftoolPath: path,
			}, nil
		}
	}
	
	return nil, fmt.Errorf("exiftool not found in PATH or common locations")
}

// WriteMetadata writes title, description, and keywords to image metadata
// Deprecated: Use backend APIs directly instead of embedding metadata
func (w *Writer) WriteMetadata(imagePath, title, description string, keywords []string) error {
	// Build exiftool command arguments
	args := []string{
		"-overwrite_original", // Don't create backup files
	}
	
	if title != "" {
		// Write to multiple fields for better compatibility
		args = append(args, 
			fmt.Sprintf("-Title=%s", title),
			fmt.Sprintf("-XMP:Title=%s", title),
			fmt.Sprintf("-IPTC:ObjectName=%s", title),
		)
	}
	
	if description != "" {
		// Write to multiple fields for better compatibility
		args = append(args,
			fmt.Sprintf("-Description=%s", description),
			fmt.Sprintf("-XMP:Description=%s", description),
			fmt.Sprintf("-IPTC:Caption-Abstract=%s", description),
		)
	}
	
	if len(keywords) > 0 {
		// Write keywords/tags as separate values
		// Flickr needs them as an array, not a comma-separated string
		for _, keyword := range keywords {
			args = append(args,
				fmt.Sprintf("-Keywords+=%s", keyword),      // += adds to array
				fmt.Sprintf("-XMP:Subject+=%s", keyword),   // += adds to array
				fmt.Sprintf("-IPTC:Keywords+=%s", keyword), // += adds to array
			)
		}
	}
	
	// Add the file path
	args = append(args, imagePath)
	
	// Run exiftool
	cmd := exec.Command(w.exiftoolPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w\nOutput: %s", err, output)
	}
	
	return nil
}

// CopyWithMetadata creates a temporary copy of the image with metadata
// Deprecated: Use backend APIs directly instead of embedding metadata
func (w *Writer) CopyWithMetadata(imagePath, title, description string, keywords []string) (string, error) {
	// Create temp file with same extension
	ext := filepath.Ext(imagePath)
	tempFile, err := os.CreateTemp("", fmt.Sprintf("imgup-*%s", ext))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFile.Close()
	
	// Copy original file
	input, err := os.ReadFile(imagePath)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to read original: %w", err)
	}
	
	if err := os.WriteFile(tempFile.Name(), input, 0644); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	
	// Write metadata to the copy
	if err := w.WriteMetadata(tempFile.Name(), title, description, keywords); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}
	
	return tempFile.Name(), nil
}

// HasExiftool checks if exiftool is available
func HasExiftool() bool {
	// First check PATH
	if _, err := exec.LookPath("exiftool"); err == nil {
		return true
	}
	
	// Check common locations
	possiblePaths := []string{
		"/opt/homebrew/bin/exiftool",
		"/usr/local/bin/exiftool",
		"/usr/bin/exiftool",
	}
	
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	
	return false
}

// ExtractMetadata extracts title, description, and keywords from image
func ExtractMetadata(imagePath string) (title, description string, keywords []string, err error) {
	// Find exiftool
	var exiftoolPath string
	
	// Check PATH first
	if path, err := exec.LookPath("exiftool"); err == nil {
		exiftoolPath = path
	} else {
		// Check common locations
		possiblePaths := []string{
			"/opt/homebrew/bin/exiftool",
			"/usr/local/bin/exiftool",
			"/usr/bin/exiftool",
		}
		
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				exiftoolPath = path
				break
			}
		}
	}
	
	if exiftoolPath == "" {
		fmt.Fprintf(os.Stderr, "DEBUG ExtractMetadata: exiftool not found\n")
		return "", "", nil, nil
	}
	
	// Run exiftool to extract metadata
	cmd := exec.Command(exiftoolPath, "-json", "-Title", "-ObjectName", "-Description", "-Caption-Abstract", "-Keywords", "-Subject", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to extract metadata: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "DEBUG ExtractMetadata: Using %s, output length: %d\n", exiftoolPath, len(output))
	
	// Parse JSON output
	var results []map[string]interface{}
	if err := json.Unmarshal(output, &results); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse exiftool output: %w", err)
	}
	
	if len(results) == 0 {
		return "", "", nil, nil
	}
	
	result := results[0]
	
	return extractFromResult(result)
}

func extractFromResult(result map[string]interface{}) (title, description string, keywords []string, err error) {
	// Debug: print what we got
	fmt.Fprintf(os.Stderr, "DEBUG ExtractMetadata: Got %d fields\n", len(result))
	
	// Extract title (try multiple fields)
	if val, ok := result["Title"]; ok && val != nil {
		title = fmt.Sprintf("%v", val)
	} else if val, ok := result["ObjectName"]; ok && val != nil {
		title = fmt.Sprintf("%v", val)
	}
	
	// Extract description (try multiple fields)  
	if val, ok := result["Description"]; ok && val != nil {
		description = fmt.Sprintf("%v", val)
	} else if val, ok := result["Caption-Abstract"]; ok && val != nil {
		description = fmt.Sprintf("%v", val)
	} else if val, ok := result["ImageDescription"]; ok && val != nil {
		description = fmt.Sprintf("%v", val)
	}
	
	// Extract keywords (can be string or array)
	keywordSet := make(map[string]bool)
	for _, field := range []string{"Keywords", "Subject"} {
		if val, ok := result[field]; ok && val != nil {
			switch v := val.(type) {
			case string:
				// Single keyword or comma-separated
				for _, k := range strings.Split(v, ",") {
					if trimmed := strings.TrimSpace(k); trimmed != "" {
						keywordSet[trimmed] = true
					}
				}
			case []interface{}:
				// Array of keywords
				for _, k := range v {
					if str, ok := k.(string); ok && str != "" {
						keywordSet[str] = true
					}
				}
			}
		}
	}
	
	// Convert set to slice
	for k := range keywordSet {
		keywords = append(keywords, k)
	}
	
	fmt.Fprintf(os.Stderr, "DEBUG ExtractMetadata: Final - Title: %q, Desc: %q, Tags: %v\n", title, description, keywords)
	
	return title, description, keywords, nil
}
