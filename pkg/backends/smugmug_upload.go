package backends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/dghubble/oauth1"
)

const (
	smugmugUploadURL = "https://upload.smugmug.com/"
	smugmugAPIURL    = "https://api.smugmug.com"
)

// SmugMugUploader handles image uploads to SmugMug
type SmugMugUploader struct {
	ConsumerKey    string
	ConsumerSecret string
	AccessToken    string
	AccessSecret   string
	AlbumID        string
}

// SmugMugUploadResult contains the result of an upload
type SmugMugUploadResult struct {
	ImageURI string
	ImageKey string
	URL      string   // Web URL
	ImageURL string   // Direct image URL for embedding
}

// NewSmugMugUploader creates a new SmugMug uploader
func NewSmugMugUploader(consumerKey, consumerSecret, accessToken, accessSecret, albumID string) *SmugMugUploader {
	return &SmugMugUploader{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		AccessToken:    accessToken,
		AccessSecret:   accessSecret,
		AlbumID:        albumID,
	}
}

// Upload uploads an image to SmugMug
func (u *SmugMugUploader) Upload(ctx context.Context, imagePath string, title, description string, tags []string, isPrivate bool) (*SmugMugUploadResult, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Add the file
	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    u.ConsumerKey,
		ConsumerSecret: u.ConsumerSecret,
	}
	
	token := oauth1.NewToken(u.AccessToken, u.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", smugmugUploadURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set content type
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Set SmugMug-specific headers
	req.Header.Set("X-Smug-AlbumUri", fmt.Sprintf("/api/v2/album/%s", u.AlbumID))
	req.Header.Set("X-Smug-ResponseType", "JSON")
	req.Header.Set("X-Smug-Version", "v2")
	req.Header.Set("X-Smug-Filename", filepath.Base(imagePath))
	
	if title != "" {
		req.Header.Set("X-Smug-Title", title)
	}
	if description != "" {
		req.Header.Set("X-Smug-Caption", description)
	}
	if len(tags) > 0 {
		req.Header.Set("X-Smug-Keywords", strings.Join(tags, ";"))
	}
	if isPrivate {
		req.Header.Set("X-Smug-Hidden", "true")
	}
	
	// Perform the upload using the OAuth client
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse the response
	var uploadResp struct {
		Image struct {
			ImageUri  string `json:"ImageUri"`
			Uri       string `json:"Uri"`
			ImageKey  string `json:"ImageKey"`
			AlbumImageUri string `json:"AlbumImageUri,omitempty"`
			UploadKey string `json:"UploadKey,omitempty"`
			StatusImageReplaceUri string `json:"StatusImageReplaceUri,omitempty"`
		} `json:"Image"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to parse upload response: %w", err)
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Upload response: %+v\n", uploadResp)
		fmt.Fprintf(os.Stderr, "DEBUG: ImageUri: %s\n", uploadResp.Image.ImageUri)
		fmt.Fprintf(os.Stderr, "DEBUG: AlbumImageUri: %s\n", uploadResp.Image.AlbumImageUri)
		
		// Extract the image key from the URI
		if uploadResp.Image.ImageKey == "" && uploadResp.Image.ImageUri != "" {
			// Extract from URI like /api/v2/image/bRX7kBM-0
			parts := strings.Split(uploadResp.Image.ImageUri, "/")
			if len(parts) > 0 {
				uploadResp.Image.ImageKey = parts[len(parts)-1]
				fmt.Fprintf(os.Stderr, "DEBUG: Extracted ImageKey: %s\n", uploadResp.Image.ImageKey)
			}
		}
	}
	
	if uploadResp.Stat != "ok" && uploadResp.Stat != "" {
		return nil, fmt.Errorf("upload failed: %s", uploadResp.Message)
	}
	
	// Try AlbumImageUri first, fall back to ImageUri
	imageURI := uploadResp.Image.AlbumImageUri
	if imageURI == "" {
		imageURI = uploadResp.Image.ImageUri
		if imageURI == "" {
			imageURI = uploadResp.Image.Uri
		}
	}
	
	if imageURI == "" {
		return nil, fmt.Errorf("no image URI in upload response")
	}
	
	// Get the image details to find the web URL
	api := &SmugMugAPI{SmugMugUploader: u}
	
	// For now, let's skip trying to get image details and go straight to sizes
	// The upload response doesn't seem to populate all fields immediately
	
	// Get the full image URL by fetching image sizes
	sizesResp, err := api.GetImageSizes(ctx, imageURI)
	if err != nil {
		// If we can't get sizes, try with just the ImageUri instead of AlbumImageUri
		if uploadResp.Image.ImageUri != "" && uploadResp.Image.ImageUri != imageURI {
			imageURI = uploadResp.Image.ImageUri
			sizesResp, err = api.GetImageSizes(ctx, imageURI)
			if err != nil {
				return nil, fmt.Errorf("failed to get image sizes: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get image sizes: %w", err)
		}
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: GetImageSizes returned with top-level keys: %v\n", getMapKeys(sizesResp))
	}
	
	// Extract the best URL from sizes response
	imageURL := u.extractBestImageURL(sizesResp)
	
	// For SmugMug, we need to get the web URL from the AlbumImage
	// Let's try to get it using the AlbumImageUri
	webURL := ""
	
	if uploadResp.Image.AlbumImageUri != "" {
		// Try to get the AlbumImage details which should have WebUri
		albumImageResp, err := api.GetAlbumImage(ctx, uploadResp.Image.AlbumImageUri)
		if err == nil && albumImageResp != nil {
			// Extract WebUri from the response
			if respData, ok := albumImageResp["Response"].(map[string]interface{}); ok {
				if albumImage, ok := respData["AlbumImage"].(map[string]interface{}); ok {
					if uri, ok := albumImage["WebUri"].(string); ok {
						webURL = uri
					}
				}
			}
		}
	}
	
	// If we still don't have a web URL, we'll use the image URL as fallback
	if webURL == "" {
		// Also check the sizes response for AlbumImage
		if sizesResp != nil && imageURL == "" {
			if respData, ok := sizesResp["Response"].(map[string]interface{}); ok {
				if albumImage, ok := respData["AlbumImage"].(map[string]interface{}); ok {
					// Get the image URLs from various possible fields
					if archivedUri, ok := albumImage["ArchivedUri"].(string); ok && archivedUri != "" {
						imageURL = archivedUri
					} else if imgUrl, ok := albumImage["ImageUrl"].(string); ok && imgUrl != "" {
						imageURL = imgUrl
					}
					
					// Also check for ThumbnailUrl, LargeImageUrl, etc.
					urlFields := []string{"LargeImageUrl", "X2LargeImageUrl", "X3LargeImageUrl", "OriginalImageUrl", "ImageDownloadUrl"}
					for _, field := range urlFields {
						if url, ok := albumImage[field].(string); ok && url != "" && imageURL == "" {
							imageURL = url
							break
						}
					}
					
					if os.Getenv("IMGUP_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "DEBUG: AlbumImage keys: %v\n", getMapKeys(albumImage))
					}
				}
			}
		}
		
		if webURL == "" && imageURL != "" {
			webURL = imageURL
		}
		
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: After AlbumImage check - webURL: %s, imageURL: %s\n", webURL, imageURL)
		}
	}
	
	// Extract image key if not present
	imageKey := uploadResp.Image.ImageKey
	if imageKey == "" && imageURI != "" {
		// Extract from URI like /api/v2/image/bRX7kBM-0
		parts := strings.Split(imageURI, "/")
		if len(parts) > 0 {
			imageKey = parts[len(parts)-1]
		}
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Final results:\n")
		fmt.Fprintf(os.Stderr, "  ImageURI: %s\n", imageURI)
		fmt.Fprintf(os.Stderr, "  ImageKey: %s\n", imageKey)
		fmt.Fprintf(os.Stderr, "  WebURL: %s\n", webURL)
		fmt.Fprintf(os.Stderr, "  ImageURL: %s\n", imageURL)
	}
	
	return &SmugMugUploadResult{
		ImageURI: imageURI,
		ImageKey: imageKey,
		URL:      webURL,
		ImageURL: imageURL,
	}, nil
}

// extractBestImageURL extracts the best available image URL from the sizes response
func (u *SmugMugUploader) extractBestImageURL(sizesResp map[string]interface{}) string {
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: extractBestImageURL called with keys: %v\n", getMapKeys(sizesResp))
	}
	
	// Navigate to the sizes data
	resp, ok := sizesResp["Response"].(map[string]interface{})
	if !ok {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: No Response key in sizesResp\n")
		}
		return ""
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Response keys: %v\n", getMapKeys(resp))
	}
	
	// Check if we have an AlbumImage response (from AlbumImage endpoint)
	if albumImage, ok := resp["AlbumImage"].(map[string]interface{}); ok {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Found AlbumImage with keys: %v\n", getMapKeys(albumImage))
		}
		
		// Check for direct URL fields in AlbumImage
		urlFields := []string{"ArchivedUri", "LargeImageUrl", "X2LargeImageUrl", "X3LargeImageUrl", "OriginalImageUrl", "ImageDownloadUrl"}
		for _, field := range urlFields {
			if url, ok := albumImage[field].(string); ok && url != "" {
				if os.Getenv("IMGUP_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "DEBUG: Found URL in AlbumImage.%s: %s\n", field, url)
				}
				return url
			}
		}
		
		// Check for nested Image object
		if image, ok := albumImage["Image"].(map[string]interface{}); ok {
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Found nested Image with keys: %v\n", getMapKeys(image))
			}
			
			// Check for ImageSizes in the nested Image
			if imageSizes, ok := image["ImageSizes"].(map[string]interface{}); ok {
				if os.Getenv("IMGUP_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "DEBUG: Found Image.ImageSizes with keys: %v\n", getMapKeys(imageSizes))
				}
				
				// Process the ImageSizes
				resp["ImageSizes"] = imageSizes
				// Continue to process below
			}
			
			// Also check for direct URLs in the Image object
			for _, field := range urlFields {
				if url, ok := image[field].(string); ok && url != "" {
					if os.Getenv("IMGUP_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "DEBUG: Found URL in Image.%s: %s\n", field, url)
					}
					return url
				}
			}
		}
		
		// Check for Uris object
		if uris, ok := albumImage["Uris"].(map[string]interface{}); ok {
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Found Uris with keys: %v\n", getMapKeys(uris))
			}
			
			// Check for ImageDownload or similar
			uriFields := []string{"ImageDownload", "LargestImage", "Image"}
			for _, field := range uriFields {
				if uriObj, ok := uris[field].(map[string]interface{}); ok {
					if url, ok := uriObj["Uri"].(string); ok && url != "" {
						// This is likely a relative URI, need to make it absolute
						if !strings.HasPrefix(url, "http") {
							// For download URIs, we might need a different base URL
							// Let's see what we get first
							if os.Getenv("IMGUP_DEBUG") != "" {
								fmt.Fprintf(os.Stderr, "DEBUG: Found relative URI in Uris.%s: %s\n", field, url)
							}
						}
						return url
					}
				}
			}
		}
	}
	
	// First check for ImageSizeDetails (from !sizedetails)
	if imageSizeDetails, ok := resp["ImageSizeDetails"].(map[string]interface{}); ok {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Found ImageSizeDetails with keys: %v\n", getMapKeys(imageSizeDetails))
		}
		
		// Look for specific sizes in order of preference
		sizePreference := []string{
			"ImageSizeX3Large",
			"ImageSizeX2Large", 
			"ImageSizeXLarge",
			"ImageSizeLarge",
			"ImageSizeMedium",
			"ImageSizeOriginal",
		}
		
		for _, sizeName := range sizePreference {
			if sizeData, ok := imageSizeDetails[sizeName].(map[string]interface{}); ok {
				if url, ok := sizeData["Url"].(string); ok && url != "" {
					if os.Getenv("IMGUP_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "DEBUG: Found URL in %s: %s\n", sizeName, url)
					}
					return url
				}
			}
		}
	}
	
	imageSizes, ok := resp["ImageSizes"].(map[string]interface{})
	if !ok {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: No ImageSizes key in Response\n")
		}
		return ""
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: ImageSizes keys: %v\n", getMapKeys(imageSizes))
	}
	
	// Try new v2 array format first
	if sizeArray, ok := imageSizes["Size"].([]interface{}); ok && len(sizeArray) > 0 {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Found Size array with %d items\n", len(sizeArray))
		}
		
		// Find the largest size
		var bestURL string
		var maxWidth int
		
		for _, sizeItem := range sizeArray {
			if size, ok := sizeItem.(map[string]interface{}); ok {
				width := 0
				if w, ok := size["Width"].(float64); ok {
					width = int(w)
				}
				
				if url, ok := size["Url"].(string); ok && width > maxWidth {
					maxWidth = width
					bestURL = url
				}
			}
		}
		
		if bestURL != "" {
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Best URL from Size array: %s (width: %d)\n", bestURL, maxWidth)
			}
			return bestURL
		}
	}
	
	// Fallback to old format with specific size URLs
	sizePreference := []string{
		"XLargeImageUrl",
		"X2LargeImageUrl", 
		"X3LargeImageUrl",
		"LargestImageUrl",
		"LargeImageUrl",
		"OriginalImageUrl",
	}
	
	for _, key := range sizePreference {
		if url, ok := imageSizes[key].(string); ok && url != "" {
			return url
		}
	}
	
	// Last resort: find any URL that ends with "ImageUrl"
	for key, value := range imageSizes {
		if strings.HasSuffix(key, "ImageUrl") {
			if url, ok := value.(string); ok && url != "" {
				return url
			}
		}
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: No URL found in extractBestImageURL\n")
	}
	
	return ""
}
