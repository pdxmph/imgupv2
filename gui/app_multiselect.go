package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetSelectedPhotos gets all currently selected photos from Finder/Photos
func (a *App) GetSelectedPhotos() ([]PhotoMetadata, error) {
	fmt.Println("DEBUG: GetSelectedPhotos called")
	
	// If we're in pull mode, don't try to get selected photos
	if a.pullDataPath != "" || a.pullDataJSON != "" {
		fmt.Println("DEBUG: In pull mode, returning empty")
		return nil, fmt.Errorf("pull mode active")
	}
	
	if runtime.GOOS != "darwin" {
		// Linux: Could check for nautilus/dolphin selection via DBus
		// For now, return empty
		return nil, fmt.Errorf("multi-selection not implemented for Linux")
	}

	// First check if Photos has a selection
	photosCheckScript := `
	tell application "Photos"
		if running then
			if (count of selection) > 0 then
				return "has_selection:" & (count of selection)
			end if
		end if
	end tell
	return ""`
	
	cmd := exec.Command("osascript", "-e", photosCheckScript)
	if out, err := cmd.Output(); err == nil {
		result := strings.TrimSpace(string(out))
		if strings.HasPrefix(result, "has_selection:") {
			// Photos has selections, get them all
			countStr := strings.TrimPrefix(result, "has_selection:")
			fmt.Printf("DEBUG: Photos has %s selections\n", countStr)
			return a.getMultiplePhotosMetadata()
		}
	}
	
	// Otherwise try Finder
	return a.getMultipleFinderSelections()
}

// getMultiplePhotosMetadata gets metadata for all selected photos in Photos.app
func (a *App) getMultiplePhotosMetadata() ([]PhotoMetadata, error) {
	// AppleScript to get metadata for ALL selected photos
	metadataScript := `
	tell application "Photos"
		set sel to selection
		if sel is {} then
			return "ERROR:No photos selected"
		end if
		
		set allResults to {}
		set photoIndex to 0
		
		repeat with photo in sel
			set photoIndex to photoIndex + 1
			
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
			set exportResult to "PHOTO_START|INDEX:" & photoIndex & "|ID:" & pId & "|FILENAME:" & pFilename & "|TITLE:" & pTitle & "|DESC:" & pDesc & "|KEYWORDS:"
			if (count of pKeywords) > 0 then
				set AppleScript's text item delimiters to ","
				set exportResult to exportResult & (pKeywords as string)
				set AppleScript's text item delimiters to ""
			end if
			set exportResult to exportResult & "|PHOTO_END"
			
			copy exportResult to end of allResults
		end repeat
		
		-- Join all results with newline
		set AppleScript's text item delimiters to "\n"
		set finalResult to (allResults as string)
		set AppleScript's text item delimiters to ""
		
		return finalResult
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
	
	// Parse multiple photo results
	var photos []PhotoMetadata
	photoStrings := strings.Split(result, "\n")
	
	for _, photoStr := range photoStrings {
		if !strings.Contains(photoStr, "PHOTO_START") {
			continue
		}
		
		// Parse the metadata for this photo
		var title, desc, photoID, filename string
		var keywords []string
		var index int
		
		// Remove markers
		photoStr = strings.Replace(photoStr, "PHOTO_START|", "", 1)
		photoStr = strings.Replace(photoStr, "|PHOTO_END", "", 1)
		
		parts := strings.Split(photoStr, "|")
		for _, part := range parts {
			if strings.HasPrefix(part, "INDEX:") {
				fmt.Sscanf(strings.TrimPrefix(part, "INDEX:"), "%d", &index)
			} else if strings.HasPrefix(part, "ID:") {
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
		metadata := PhotoMetadata{
			Path:           "", // Will be set when exported
			Title:          title,
			Alt:            desc, // Use description as alt text
			Description:    desc,
			Tags:           keywords,
			Format:         "markdown", // default
			Private:        false,      // default to public
			IsFromPhotos:   true,
			PhotosIndex:    index,
			PhotosID:       photoID,
			PhotosFilename: filename,
		}
		
		photos = append(photos, metadata)
	}
	
	// Don't start async exports here - wait for frontend to request them
	
	return photos, nil
}

// getMultipleFinderSelections gets all selected files from Finder
func (a *App) getMultipleFinderSelections() ([]PhotoMetadata, error) {
	script := `
	tell application "Finder"
		set theSelection to selection
		if length of theSelection is 0 then
			return ""
		end if
		
		set allPaths to {}
		repeat with theFile in theSelection
			set thePath to POSIX path of (theFile as alias)
			-- Remove trailing slash if it's there
			if thePath ends with "/" then
				set thePath to text 1 thru -2 of thePath
			end if
			
			-- Check if it's a file (not directory) by looking for extension
			if thePath contains "." then
				copy thePath to end of allPaths
			end if
		end repeat
		
		-- Join paths with newline
		set AppleScript's text item delimiters to "\n"
		set pathList to (allPaths as string)
		set AppleScript's text item delimiters to ""
		
		return pathList
	end tell`

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get Finder selection: %w", err)
	}
	
	pathList := strings.TrimSpace(string(out))
	if pathList == "" {
		return nil, fmt.Errorf("no files selected in Finder")
	}
	
	paths := strings.Split(pathList, "\n")
	var photos []PhotoMetadata
	
	// Process each file
	for _, path := range paths {
		if path == "" {
			continue
		}
		
		// Check if this is actually a file
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue // Skip directories and inaccessible items
		}
		
		// Create basic metadata
		metadata := PhotoMetadata{
			Path:    path,
			Format:  "markdown", // default
			Private: false,      // default to public
		}
		
		photos = append(photos, metadata)
	}
	
	if len(photos) == 0 {
		return nil, fmt.Errorf("no valid image files selected")
	}
	
	// Don't start async processing here - wait for frontend to request it
	
	return photos, nil
}

// startMultiplePhotosExports starts parallel exports for Photos selections
func (a *App) startMultiplePhotosExports(photos []PhotoMetadata) {
	var wg sync.WaitGroup
	
	for i := range photos {
		wg.Add(1)
		go func(index int, photo PhotoMetadata) {
			defer wg.Done()
			
			// Skip if we already have a cached thumbnail for this Photos ID
			if a.cachedPhotoIDs[photo.PhotosID] {
				fmt.Printf("DEBUG: Using cached thumbnail for photo %d (ID: %s, File: %s)\n", 
					index, photo.PhotosID, photo.PhotosFilename)
				
				// Retrieve the cached thumbnail
				if a.thumbGen != nil {
					if thumb, err := a.thumbGen.GetCachedThumbnail(a.ctx, photo.PhotosID); err == nil && thumb != nil {
						// Emit the cached thumbnail
						wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
							"index":     index,
							"thumbnail": "data:image/jpeg;base64," + thumb.ThumbnailData,
							"path":      photo.Path,
						})
						
						// Also emit metadata if available from the photo
						if photo.Title != "" || photo.Alt != "" || len(photo.Tags) > 0 {
							wailsRuntime.EventsEmit(a.ctx, "metadata-ready", map[string]interface{}{
								"index":    index,
								"title":    photo.Title,
								"alt":      photo.Alt,
								"keywords": photo.Tags,
							})
						}
					}
				}
				return
			}
			
			fmt.Printf("DEBUG: Starting export for photo index=%d, photosIndex=%d, id=%s\n", 
				index, photo.PhotosIndex, photo.PhotosID)
			
			// Export the photo and generate thumbnail
			exportScript := fmt.Sprintf(`
			on run
				tell application "Photos"
					set sel to selection
					if (count of sel) >= %d then
						set targetPhoto to item %d of sel
						
						-- Create temp folder path with unique identifier
						set tempPath to (path to temporary items as text)
						set exportFolderName to "imgupv2-export-" & (do shell script "date +%%s") & "-%d"
						set exportFolder to tempPath & exportFolderName & ":"
						
						-- Create the folder
						do shell script "mkdir -p " & quoted form of POSIX path of exportFolder
						
						-- Export the photo
						try
							export {targetPhoto} to (exportFolder as alias) with using originals
						on error errMsg
							return "ERROR:" & errMsg
						end try
						
						-- Wait a moment for export to complete
						delay 0.5
						
						-- Get the exported file
						set exportedFiles to (do shell script "ls -1 " & quoted form of POSIX path of exportFolder & " 2>/dev/null || echo ''")
						if exportedFiles is not "" then
							set firstFile to paragraph 1 of exportedFiles
							return POSIX path of exportFolder & firstFile
						else
							return "ERROR:No files exported"
						end if
					else
						return "ERROR:Invalid photo index"
					end if
				end tell
			end run`, photo.PhotosIndex, photo.PhotosIndex, index)
			
			cmd := exec.Command("osascript", "-e", exportScript)
			out, err := cmd.Output()
			if err != nil {
				// Get stderr for better error info
				if exitErr, ok := err.(*exec.ExitError); ok {
					fmt.Printf("DEBUG: Export stderr for photo %d: %s\n", index, string(exitErr.Stderr))
				}
				fmt.Printf("DEBUG: Export error for photo %d: %v\n", index, err)
				// Still emit event with empty thumbnail
				wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
					"index":     index,
					"thumbnail": "",
					"error":     err.Error(),
				})
				return
			}
			
			exportPath := strings.TrimSpace(string(out))
			fmt.Printf("DEBUG: Export result for photo %d: %s\n", index, exportPath)
			
			// Check for error in output
			if strings.HasPrefix(exportPath, "ERROR:") {
				errorMsg := strings.TrimPrefix(exportPath, "ERROR:")
				fmt.Printf("DEBUG: Export failed for photo %d: %s\n", index, errorMsg)
				wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
					"index":     index,
					"thumbnail": "",
					"error":     errorMsg,
				})
				return
			}
			
			if exportPath != "" {
				// Generate thumbnail
				thumbnail, err := a.generateThumbnail(exportPath)
				if err != nil {
					fmt.Printf("DEBUG: Thumbnail generation error for photo %d: %v\n", index, err)
				} else {
					fmt.Printf("DEBUG: Thumbnail generated successfully for photo %d (from %s)\n", index, exportPath)
				}
				
				// Emit event with the thumbnail
				wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
					"index":     index,
					"thumbnail": thumbnail,
					"path":      exportPath,
					"photosId":  photo.PhotosID,
					"filename":  photo.PhotosFilename,
				})
			} else {
				fmt.Printf("DEBUG: Empty export path for photo %d\n", index)
				// Emit event with empty thumbnail
				wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
					"index":     index,
					"thumbnail": "",
					"error":     "empty export path",
				})
			}
		}(i, photos[i])
	}
}

// startMultipleMetadataExtraction starts parallel metadata extraction for Finder files
func (a *App) startMultipleMetadataExtraction(photos []PhotoMetadata) {
	var wg sync.WaitGroup
	
	for i := range photos {
		wg.Add(1)
		go func(index int, photo PhotoMetadata) {
			defer wg.Done()
			
			// Generate thumbnail for the file
			thumbnail, err := a.generateThumbnail(photo.Path)
			if err != nil {
				fmt.Printf("DEBUG: Thumbnail error for %s: %v\n", photo.Path, err)
			}
			
			// Extract metadata if exiftool is available
			metadata := a.extractMetadata(photo.Path)
			
			// Emit thumbnail event
			wailsRuntime.EventsEmit(a.ctx, "thumbnail-ready", map[string]interface{}{
				"index":     index,
				"thumbnail": thumbnail,
				"path":      photo.Path,
			})
			
			// Emit metadata event
			wailsRuntime.EventsEmit(a.ctx, "metadata-ready", map[string]interface{}{
				"index":       index,
				"path":        photo.Path,
				"title":       metadata.Title,
				"alt":         metadata.Alt,
				"description": metadata.Description,  // Add description field
				"keywords":    metadata.Tags,
			})
		}(i, photos[i])
	}
}
// TestMultiSelect is a temporary method to test multi-selection
func (a *App) TestMultiSelect() (string, error) {
	photos, err := a.GetSelectedPhotos()
	if err != nil {
		return fmt.Sprintf("Error: %v", err), err
	}
	
	result := fmt.Sprintf("Found %d photos:\n", len(photos))
	for i, photo := range photos {
		if photo.IsFromPhotos {
			result += fmt.Sprintf("%d. Photos: %s (index: %d, id: %s)\n", 
				i+1, photo.PhotosFilename, photo.PhotosIndex, photo.PhotosID)
		} else {
			result += fmt.Sprintf("%d. Finder: %s\n", i+1, photo.Path)
		}
	}
	
	return result, nil
}

// StartThumbnailGeneration starts async thumbnail generation for the given photos
func (a *App) StartThumbnailGeneration(photos []PhotoMetadata) {
	fmt.Printf("DEBUG: Starting thumbnail generation for %d photos\n", len(photos))
	
	// Process based on source
	var photosFromApp []PhotoMetadata
	var filesFromFinder []PhotoMetadata
	
	for _, photo := range photos {
		if photo.IsFromPhotos {
			photosFromApp = append(photosFromApp, photo)
		} else {
			filesFromFinder = append(filesFromFinder, photo)
		}
	}
	
	// Start appropriate async processing
	if len(photosFromApp) > 0 {
		// Count how many are already cached
		cachedCount := 0
		for _, photo := range photosFromApp {
			if a.cachedPhotoIDs[photo.PhotosID] {
				cachedCount++
			}
		}
		fmt.Printf("DEBUG: Processing %d photos from Photos.app (%d cached, %d new)\n", 
			len(photosFromApp), cachedCount, len(photosFromApp)-cachedCount)
		
		a.startMultiplePhotosExports(photosFromApp)
	}
	if len(filesFromFinder) > 0 {
		a.startMultipleMetadataExtraction(filesFromFinder)
	}
}
