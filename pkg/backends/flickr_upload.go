package backends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/dghubble/oauth1"
)

const (
	flickrUploadURL = "https://up.flickr.com/services/upload/"
	flickrAPIURL    = "https://api.flickr.com/services/rest/"
)

// FlickrUploader handles image uploads to Flickr
type FlickrUploader struct {
	ConsumerKey    string
	ConsumerSecret string
	AccessToken    string
	AccessSecret   string
}

// UploadResult contains the result of an upload
type UploadResult struct {
	PhotoID  string
	URL      string   // Photo page URL
	ImageURL string   // Direct image URL for embedding
	Warnings []string // Non-fatal warnings (e.g., failed to set tags)
}

// NewFlickrUploader creates a new Flickr uploader
func NewFlickrUploader(consumerKey, consumerSecret, accessToken, accessSecret string) *FlickrUploader {
	return &FlickrUploader{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		AccessToken:    accessToken,
		AccessSecret:   accessSecret,
	}
}

// Upload uploads an image to Flickr using upload-then-set pattern
func (u *FlickrUploader) Upload(ctx context.Context, imagePath string, title, description string, tags []string, isPrivate bool) (*UploadResult, error) {
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Upload called with isPrivate=%v\n", isPrivate)
	}
	
	result := &UploadResult{
		Warnings: []string{},
	}
	
	// Step 1: Upload the photo with NO metadata
	photoID, err := u.uploadPhoto(ctx, imagePath)
	if err != nil {
		return nil, err
	}
	result.PhotoID = photoID
	
	// Step 2: Set metadata if provided
	if title != "" || description != "" {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Setting photo metadata (title: %q, description: %q)\n", title, description)
		}
		if err := u.setPhotoMeta(ctx, photoID, title, description); err != nil {
			// Collect warning instead of printing
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to set photo metadata: %v", err))
		} else if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Successfully set photo metadata\n")
		}
	}
	
	// Step 3: Add tags if provided
	if len(tags) > 0 {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Adding tags: %v\n", tags)
		}
		if err := u.addTags(ctx, photoID, tags); err != nil {
			// Collect warning instead of printing
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to add tags: %v", err))
		} else if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Successfully added tags\n")
		}
	}
	
	// Step 4: Set privacy if needed
	if isPrivate {
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Setting photo as private (photo_id: %s)\n", photoID)
		}
		if err := u.setPhotoPerms(ctx, photoID, false, false, false); err != nil {
			// Collect warning instead of printing
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to set photo privacy: %v", err))
		} else if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Successfully set photo as private\n")
		}
	}
	
	// Get the photo info and URLs regardless of privacy setting
	api := &FlickrAPI{FlickrUploader: u}
	photoInfo, err := api.GetPhotoInfo(ctx, photoID)
	if err != nil {
		// Fall back to basic URL if we can't get photo info
		result.URL = fmt.Sprintf("https://www.flickr.com/photos/98806759@N00/%s", photoID)
		return result, nil
	}
	result.URL = photoInfo.URL
	
	// Get photo sizes to find a good image URL
	sizes, err := api.GetPhotoSizes(ctx, photoID)
	imageURL := ""
	if err == nil && len(sizes) > 0 {
		// Look for "Large" size, or fall back to the last (usually largest) size
		for _, size := range sizes {
			if size.Label == "Large" || size.Label == "Large 1024" {
				imageURL = size.Source
				break
			}
		}
		if imageURL == "" && len(sizes) > 0 {
			// Use the last size (usually the largest)
			imageURL = sizes[len(sizes)-1].Source
		}
	}
	result.ImageURL = imageURL
	
	return result, nil
}

// uploadPhoto uploads just the photo file without any metadata
func (u *FlickrUploader) uploadPhoto(ctx context.Context, imagePath string) (string, error) {
	// Open the image file
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()
	
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Add image file
	part, err := writer.CreateFormFile("photo", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}
	
	// Close the writer
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    u.ConsumerKey,
		ConsumerSecret: u.ConsumerSecret,
	}
	
	token := oauth1.NewToken(u.AccessToken, u.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", flickrUploadURL, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(buf.Len())
	
	// Make request
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, body)
	}
	
	// Parse response to get photo ID
	photoID := u.parsePhotoID(string(body))
	if photoID == "" {
		return "", fmt.Errorf("failed to parse photo ID from response: %s", body)
	}
	
	// Check if response indicates an error
	if strings.Contains(string(body), "stat=\"fail\"") || strings.Contains(string(body), "<err") {
		return "", fmt.Errorf("upload failed - Flickr returned error: %s", body)
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Photo uploaded successfully with ID: %s\n", photoID)
		fmt.Fprintf(os.Stderr, "DEBUG: Full upload response: %s\n", string(body))
	}
	
	return photoID, nil
}

// setPhotoMeta sets the title and description of a photo
func (u *FlickrUploader) setPhotoMeta(ctx context.Context, photoID, title, description string) error {
	// Build parameters
	params := url.Values{
		"method":         {"flickr.photos.setMeta"},
		"photo_id":       {photoID},
		"title":          {title},
		"description":    {description},
		"format":         {"json"},
		"nojsoncallback": {"1"},
	}
	
	// Make API call
	resp, err := u.makeAPICall(ctx, "POST", params)
	if err != nil {
		return err
	}
	
	// Parse response
	var result struct {
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	if result.Stat != "ok" {
		return fmt.Errorf("API error: %s", result.Message)
	}
	
	return nil
}

// addTags adds tags to a photo
func (u *FlickrUploader) addTags(ctx context.Context, photoID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	
	// Build parameters
	params := url.Values{
		"method":         {"flickr.photos.addTags"},
		"photo_id":       {photoID},
		"tags":           {strings.Join(tags, " ")},
		"format":         {"json"},
		"nojsoncallback": {"1"},
	}
	
	// Make API call
	resp, err := u.makeAPICall(ctx, "POST", params)
	if err != nil {
		return err
	}
	
	// Parse response
	var result struct {
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	if result.Stat != "ok" {
		return fmt.Errorf("API error: %s", result.Message)
	}
	
	return nil
}

// setPhotoPerms sets the privacy settings of a photo
func (u *FlickrUploader) setPhotoPerms(ctx context.Context, photoID string, isPublic, isFriend, isFamily bool) error {
	// Build parameters
	params := url.Values{
		"method":         {"flickr.photos.setPerms"},
		"photo_id":       {photoID},
		"is_public":      {boolToString(isPublic)},
		"is_friend":      {boolToString(isFriend)},
		"is_family":      {boolToString(isFamily)},
		"format":         {"json"},
		"nojsoncallback": {"1"},
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Calling flickr.photos.setPerms with params: %v\n", params)
	}
	
	// Make API call
	resp, err := u.makeAPICall(ctx, "POST", params)
	if err != nil {
		return err
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: flickr.photos.setPerms response: %s\n", string(resp))
	}
	
	// Parse response
	var result struct {
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	if result.Stat != "ok" {
		return fmt.Errorf("API error: %s", result.Message)
	}
	
	return nil
}

// makeAPICall makes an OAuth-signed API call
func (u *FlickrUploader) makeAPICall(ctx context.Context, method string, params url.Values) ([]byte, error) {
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    u.ConsumerKey,
		ConsumerSecret: u.ConsumerSecret,
	}
	
	token := oauth1.NewToken(u.AccessToken, u.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	// Create request
	var req *http.Request
	var err error
	
	if method == "POST" {
		req, err = http.NewRequestWithContext(ctx, method, flickrAPIURL, strings.NewReader(params.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, flickrAPIURL+"?"+params.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
	}
	
	// Make request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		// Check if response is HTML (common for 504 Gateway Timeout errors)
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/html") || strings.HasPrefix(string(body), "<") {
			// Sanitize HTML content in error message
			return nil, fmt.Errorf("request failed with status %d (HTML response)", resp.StatusCode)
		}
		// For non-HTML errors, truncate body if too long
		errorBody := string(body)
		if len(errorBody) > 200 {
			errorBody = errorBody[:200] + "..."
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, errorBody)
	}
	
	return body, nil
}

// parsePhotoID extracts the photo ID from the upload response
func (u *FlickrUploader) parsePhotoID(response string) string {
	// Flickr returns XML like: <photoid>1234567890</photoid>
	start := "<photoid>"
	end := "</photoid>"
	
	startIdx := bytes.Index([]byte(response), []byte(start))
	if startIdx == -1 {
		return ""
	}
	
	startIdx += len(start)
	endIdx := bytes.Index([]byte(response[startIdx:]), []byte(end))
	if endIdx == -1 {
		return ""
	}
	
	return response[startIdx : startIdx+endIdx]
}

// boolToString converts a bool to "1" or "0"
func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
