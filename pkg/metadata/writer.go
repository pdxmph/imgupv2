package metadata

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Writer handles writing metadata to images
type Writer struct {
	exiftoolPath string
}

// NewWriter creates a new metadata writer
func NewWriter() (*Writer, error) {
	// Check if exiftool is available
	path, err := exec.LookPath("exiftool")
	if err != nil {
		return nil, fmt.Errorf("exiftool not found in PATH: %w", err)
	}
	
	return &Writer{
		exiftoolPath: path,
	}, nil
}

// WriteMetadata writes title, description, and keywords to image metadata
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
	_, err := exec.LookPath("exiftool")
	return err == nil
}

// ReadMetadata reads metadata from an image (for testing)
func (w *Writer) ReadMetadata(imagePath string) (map[string]string, error) {
	cmd := exec.Command(w.exiftoolPath, "-Title", "-Description", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	
	metadata := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			metadata[key] = value
		}
	}
	
	return metadata, nil
}
