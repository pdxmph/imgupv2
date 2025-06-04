package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/templates"
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
	
	// Note: Force flag removed - now using reactive 404 handling in CLI
	
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
			Path      string   `json:"path"`
			URL       string   `json:"url"`
			ImageURL  string   `json:"imageUrl"`
			PhotoID   string   `json:"photoId"`
			Duplicate bool     `json:"duplicate"`
			Error     *string  `json:"error"`
			Warnings  []string `json:"warnings"`
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
		// Try to find JSON in output by looking for complete JSON objects
		jsonStart := strings.Index(outputStr, "{")
		jsonEnd := strings.LastIndex(outputStr, "}")
		
		if jsonStart >= 0 && jsonEnd >= jsonStart {
			// Extract the JSON portion
			jsonStr := outputStr[jsonStart : jsonEnd+1]
			
			// Try to parse the extracted JSON
			if err := json.Unmarshal([]byte(jsonStr), &jsonResponse); err != nil {
				// If that fails, try to at least extract successful uploads
				// This handles cases where JSON is partially corrupted
				result.Success = false
				result.Error = fmt.Sprintf("Failed to parse JSON response: %v", err)
				
				// Try to extract URLs from the output as a fallback
				urlPattern := `"url":\s*"([^"]+)"`
				if matches := regexp.MustCompile(urlPattern).FindAllStringSubmatch(outputStr, -1); matches != nil {
					for i, match := range matches {
						if i < len(request.Images) && len(match) > 1 {
							result.Outputs = append(result.Outputs, MultiPhotoOutputResult{
								Path: request.Images[i].Path,
								URL:  match[1],
								Alt:  request.Images[i].Alt,
							})
						}
					}
				}
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
				Path:      request.Images[i].Path,
				URL:       upload.URL,
				Alt:       request.Images[i].Alt,
				Duplicate: upload.Duplicate, // Pass duplicate status to frontend
				Warnings:  upload.Warnings,  // Pass warnings to frontend
			}
			
			if upload.Error != nil {
				output.Error = *upload.Error
			}
			
			// Generate format-specific output using templates
			if upload.URL != "" && request.Format != "" {
				// Debug: Check what URLs we have
				fmt.Printf("DEBUG: Format=%s, URL=%s, ImageURL=%s\n", request.Format, upload.URL, upload.ImageURL)
				
				// Load config to get templates
				cfg, err := config.Load()
				if err != nil {
					// If config fails to load, continue without templates
					fmt.Printf("ERROR: Failed to load config for templates: %v\n", err)
				} else {
					fmt.Printf("DEBUG: Config loaded, Templates=%v\n", cfg.Templates)
					if cfg.Templates != nil {
						// Create template variables
						vars := templates.Variables{
							PhotoID:     upload.PhotoID,
							URL:         upload.URL,      // Photo page URL
							ImageURL:    upload.ImageURL, // Direct image URL (this is what we want!)
							Filename:    filepath.Base(request.Images[i].Path),
							Title:       request.Images[i].Title,
							Description: request.Images[i].Description,
							Alt:         request.Images[i].Alt,
						}
						
						fmt.Printf("DEBUG: Template vars - ImageURL=%s, Alt=%s\n", vars.ImageURL, vars.Alt)
						
						// Debug: Show what template we're using
						if tmpl, ok := cfg.Templates[request.Format]; ok {
							fmt.Printf("DEBUG: Using template for %s: %s\n", request.Format, tmpl)
							
							// Process the template for the requested format
							switch request.Format {
							case "markdown":
								output.Markdown = templates.Process(tmpl, vars)
								fmt.Printf("DEBUG: Processed markdown: %s\n", output.Markdown)
							case "html":
								output.HTML = templates.Process(tmpl, vars)
								fmt.Printf("DEBUG: Processed HTML: %s\n", output.HTML)
							}
						} else {
							fmt.Printf("ERROR: No template found for format %s\n", request.Format)
						}
					} else {
						fmt.Printf("ERROR: Templates is nil in config\n")
					}
				}
			}
			
			result.Outputs = append(result.Outputs, output)
			
			// Debug: log what we're sending to frontend
			if request.Format == "markdown" && output.Markdown != "" {
				fmt.Printf("DEBUG: Sending to frontend - Markdown: %s\n", output.Markdown)
			}
			
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
