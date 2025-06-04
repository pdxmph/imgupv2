package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// UploadMultiplePhotos handles uploading multiple photos with shared metadata
func (a *App) UploadMultiplePhotos(request MultiPhotoUploadRequest) (*MultiPhotoUploadResult, error) {
	// Debug logging to trace the issue
	fmt.Printf("DEBUG: UploadMultiplePhotos called\n")
	fmt.Printf("DEBUG: Request - Mastodon: %v, Bluesky: %v\n", request.Mastodon, request.Bluesky)
	fmt.Printf("DEBUG: Request - Post text: %s\n", request.Post)
	
	result := &MultiPhotoUploadResult{
		Success: true,
		Outputs: []MultiPhotoOutputResult{},
	}
	
	// Track temporary files for cleanup
	var tempFiles []string
	// Defer cleanup with a longer delay to ensure upload completes
	defer func() {
		go func() {
			// Wait 5 minutes before cleanup to ensure imgup has time to process
			time.Sleep(5 * time.Minute)
			// Clean up temporary files
			for _, tempFile := range tempFiles {
				if tempFile != "" {
					os.Remove(tempFile)
					fmt.Printf("DEBUG: Cleaned up temp file: %s\n", tempFile)
				}
			}
		}()
	}()
	
	// Build the JSON request structure for imgup CLI
	jsonRequest := map[string]interface{}{
		"images": []map[string]interface{}{},
	}
	
	// Add options to force upload (bypass duplicate detection)
	// This helps avoid issues with stale cached URLs
	jsonRequest["options"] = map[string]interface{}{
		"force": true,
	}
	
	// Common settings
	if len(request.Tags) > 0 {
		if jsonRequest["common"] == nil {
			jsonRequest["common"] = map[string]interface{}{}
		}
		jsonRequest["common"].(map[string]interface{})["tags"] = request.Tags
	}
	
	// Social media settings
	if request.Mastodon || request.Bluesky {
		social := map[string]interface{}{}
		
		fmt.Printf("DEBUG: Building social settings - Mastodon: %v, Bluesky: %v\n", request.Mastodon, request.Bluesky)
		
		if request.Mastodon {
			social["mastodon"] = map[string]interface{}{
				"enabled": true,
				"post": request.Post,
				"visibility": request.Visibility,
			}
		}
		
		if request.Bluesky {
			social["bluesky"] = map[string]interface{}{
				"enabled": true,
				"post": request.Post,
			}
		}
		
		jsonRequest["social"] = social
		fmt.Printf("DEBUG: Social settings in JSON: %+v\n", social)
	}
	
	// Process each image
	for i, img := range request.Images {
		// Handle Photos.app exports
		imagePath := img.Path
		
		if img.IsFromPhotos && imagePath == "" {
			// Export the photo from Photos.app using 1-based index
			exportPath, err := a.exportPhotoFromPhotosAppByIndex(img.PhotosIndex)
			if err != nil {
				// Add failed result
				result.Outputs = append(result.Outputs, MultiPhotoOutputResult{
					Path:  img.Path,
					Error: fmt.Sprintf("Failed to export from Photos: %s", err.Error()),
				})
				
				// Emit failure event
				wailsRuntime.EventsEmit(a.ctx, "upload-failed", map[string]interface{}{
					"index": i,
					"path": img.Path,
					"error": fmt.Sprintf("Failed to export from Photos: %s", err.Error()),
				})
				
				continue // Skip to next image
			}
			imagePath = exportPath
			tempFiles = append(tempFiles, exportPath) // Track for cleanup
			
			// Re-embed metadata using exiftool if we have any
			if img.Title != "" || img.Description != "" || len(request.Tags) > 0 {
				exiftoolPath := a.findExiftoolBinary()
				
				exifArgs := []string{"-overwrite_original"}
				
				if img.Title != "" {
					exifArgs = append(exifArgs, "-XMP-dc:Title="+img.Title)
				}
				
				if img.Description != "" {
					exifArgs = append(exifArgs, "-ImageDescription="+img.Description)
					exifArgs = append(exifArgs, "-XMP-dc:Description="+img.Description)
				}
				
				for _, tag := range request.Tags {
					exifArgs = append(exifArgs, "-XMP-dc:Subject+="+strings.TrimSpace(tag))
				}
				
				exifArgs = append(exifArgs, imagePath)
				
				cmd := exec.Command(exiftoolPath, exifArgs...)
				if err := cmd.Run(); err != nil {
					// Non-fatal: continue even if metadata embedding fails
					fmt.Fprintf(os.Stderr, "Warning: failed to embed metadata: %v\n", err)
				}
			}
		}
		
		// Add image to JSON request
		imageData := map[string]interface{}{
			"path": imagePath,
		}
		
		if img.Title != "" {
			imageData["title"] = img.Title
		}
		if img.Alt != "" {
			imageData["alt"] = img.Alt
		}
		if img.Description != "" {
			imageData["description"] = img.Description
		}
		
		jsonRequest["images"] = append(jsonRequest["images"].([]map[string]interface{}), imageData)
		
		// Emit progress event
		wailsRuntime.EventsEmit(a.ctx, "upload-started", map[string]interface{}{
			"index": i,
			"path": imagePath,
			"total": len(request.Images),
		})
	}
	
	// If no valid images, return early
	if len(jsonRequest["images"].([]map[string]interface{})) == 0 {
		result.Success = false
		result.Error = "No valid images to upload"
		return result, nil
	}
	
	// Write JSON to temporary file
	jsonFile, err := os.CreateTemp("", "imgup-batch-*.json")
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create temporary file: %v", err)
		return result, nil
	}
	tempFiles = append(tempFiles, jsonFile.Name())
	
	jsonData, err := json.MarshalIndent(jsonRequest, "", "  ")
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to marshal JSON: %v", err)
		return result, nil
	}
	
	if _, err := jsonFile.Write(jsonData); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to write JSON file: %v", err)
		return result, nil
	}
	jsonFile.Close()
	
	// Find imgup binary
	imgupPath := a.findImgupBinary()
	
	// Run imgup CLI with JSON file
	cmd := exec.Command(imgupPath, "upload", "--json-file", jsonFile.Name())
	fmt.Printf("DEBUG: Executing command: %s upload --json-file %s\n", imgupPath, jsonFile.Name())
	fmt.Printf("DEBUG: JSON content:\n%s\n", string(jsonData))
	
	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	fmt.Printf("DEBUG: Raw output:\n%s\n", outputStr)
	
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Upload failed: %v", err)
		if outputStr != "" {
			result.Error += "\n" + outputStr
		}
		return result, nil
	}
	
	// Parse JSON response
	var jsonResponse struct {
		Success bool `json:"success"`
		Uploads []struct {
			Path      string  `json:"path"`
			URL       string  `json:"url"`
			ImageURL  string  `json:"imageUrl"`
			PhotoID   string  `json:"photoId"`
			Duplicate bool    `json:"duplicate"`
			Error     *string `json:"error"`
		} `json:"uploads"`
		Social *struct {
			Mastodon *struct {
				Success bool    `json:"success"`
				URL     string  `json:"url"`
				Error   *string `json:"error"`
			} `json:"mastodon"`
			Bluesky *struct {
				Success bool    `json:"success"`
				URL     string  `json:"url"`
				Error   *string `json:"error"`
			} `json:"bluesky"`
		} `json:"social"`
	}
	
	if err := json.Unmarshal([]byte(outputStr), &jsonResponse); err != nil {
		// Try to find JSON in output
		jsonStart := strings.Index(outputStr, "{")
		jsonEnd := strings.LastIndex(outputStr, "}")
		
		if jsonStart >= 0 && jsonEnd >= jsonStart {
			jsonStr := outputStr[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), &jsonResponse); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to parse JSON response: %v", err)
				return result, nil
			}
		} else {
			result.Success = false
			result.Error = "No JSON response found in output"
			return result, nil
		}
	}
	
	// Process results
	result.Success = jsonResponse.Success
	
	// Map uploads back to original images
	for i, upload := range jsonResponse.Uploads {
		if i < len(request.Images) {
			output := MultiPhotoOutputResult{
				Path: request.Images[i].Path,
				URL:  upload.URL,
				Alt:  request.Images[i].Alt,
			}
			
			if upload.Error != nil {
				output.Error = *upload.Error
			}
			
			// Generate format-specific output
			if upload.URL != "" && request.Format != "" {
				switch request.Format {
				case "markdown":
					if output.Alt != "" {
						output.Markdown = fmt.Sprintf("![%s](%s)", output.Alt, upload.URL)
					} else {
						output.Markdown = fmt.Sprintf("![](%s)", upload.URL)
					}
				case "html":
					if output.Alt != "" {
						output.HTML = fmt.Sprintf(`<img src="%s" alt="%s">`, upload.URL, output.Alt)
					} else {
						output.HTML = fmt.Sprintf(`<img src="%s">`, upload.URL)
					}
				}
			}
			
			result.Outputs = append(result.Outputs, output)
			
			// Emit completion event
			if upload.Error == nil {
				wailsRuntime.EventsEmit(a.ctx, "upload-completed", map[string]interface{}{
					"index": i,
					"path": request.Images[i].Path,
					"url": upload.URL,
				})
			} else {
				wailsRuntime.EventsEmit(a.ctx, "upload-failed", map[string]interface{}{
					"index": i,
					"path": request.Images[i].Path,
					"error": *upload.Error,
				})
			}
		}
	}
	
	// Process social media results
	if jsonResponse.Social != nil {
		var socialStatuses []string
		
		if jsonResponse.Social.Mastodon != nil {
			if jsonResponse.Social.Mastodon.Success {
				socialStatuses = append(socialStatuses, "Posted to Mastodon")
			} else if jsonResponse.Social.Mastodon.Error != nil {
				socialStatuses = append(socialStatuses, fmt.Sprintf("Mastodon failed: %s", *jsonResponse.Social.Mastodon.Error))
			}
		}
		
		if jsonResponse.Social.Bluesky != nil {
			if jsonResponse.Social.Bluesky.Success {
				socialStatuses = append(socialStatuses, "Posted to Bluesky")
			} else if jsonResponse.Social.Bluesky.Error != nil {
				socialStatuses = append(socialStatuses, fmt.Sprintf("Bluesky failed: %s", *jsonResponse.Social.Bluesky.Error))
			}
		}
		
		if len(socialStatuses) > 0 {
			result.SocialStatus = strings.Join(socialStatuses, "; ")
		}
	}
	
	return result, nil
}
