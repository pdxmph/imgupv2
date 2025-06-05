package kitty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// IsKittyTerminal detects if we're running in a Kitty terminal
func IsKittyTerminal() bool {
	// Check TERM environment variable
	term := os.Getenv("TERM")
	if strings.Contains(term, "kitty") {
		return true
	}
	
	// Check KITTY_WINDOW_ID
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	
	// Check KITTY_PID
	if os.Getenv("KITTY_PID") != "" {
		return true
	}
	
	return false
}

// ImageDisplay handles displaying images in Kitty terminal
type ImageDisplay struct {
	// tempFiles tracks temporary files for cleanup
	tempFiles []string
}

// NewImageDisplay creates a new Kitty image display handler
func NewImageDisplay() *ImageDisplay {
	return &ImageDisplay{
		tempFiles: make([]string, 0),
	}
}

// DisplayImage displays an image in the Kitty terminal using kitten icat
func (d *ImageDisplay) DisplayImage(reader io.Reader, width, height int) error {
	// Read the image data
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}
	
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "imgup-thumb-*.jpg")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()
	
	// Track for cleanup
	d.tempFiles = append(d.tempFiles, tmpFile.Name())
	
	// Write image data
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	
	// Use kitten icat to display the image inline
	// --align left ensures left alignment
	args := []string{"icat", "--align", "left"}
	args = append(args, tmpFile.Name())
	
	cmd := exec.Command("kitten", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitten icat failed: %w", err)
	}
	
	return nil
}

// ClearImages clears temporary files
func (d *ImageDisplay) ClearImages() {
	// Clean up temp files
	for _, f := range d.tempFiles {
		os.Remove(f)
	}
	d.tempFiles = d.tempFiles[:0]
}

// Cleanup should be called when done with the display
func (d *ImageDisplay) Cleanup() {
	for _, f := range d.tempFiles {
		os.Remove(f)
	}
}