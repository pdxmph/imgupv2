package mastodon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client represents a Mastodon API client
type Client struct {
	InstanceURL  string
	ClientID     string
	ClientSecret string
	AccessToken  string
}

// NewClient creates a new Mastodon client
func NewClient(instanceURL, clientID, clientSecret, accessToken string) *Client {
	// Ensure instance URL doesn't have trailing slash
	instanceURL = strings.TrimRight(instanceURL, "/")
	
	return &Client{
		InstanceURL:  instanceURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  accessToken,
	}
}

// PostStatus posts a new status to Mastodon
func (c *Client) PostStatus(text string, mediaIDs []string, visibility string, tags []string) error {
	// Convert tags to hashtags
	for _, tag := range tags {
		// Only add hashtag if not already in the text
		hashtag := "#" + strings.ReplaceAll(tag, " ", "")
		if !strings.Contains(text, hashtag) {
			text += " " + hashtag
		}
	}
	
	// Build form data
	data := url.Values{}
	data.Set("status", text)
	data.Set("visibility", visibility)
	
	// Add media IDs
	for _, mediaID := range mediaIDs {
		data.Add("media_ids[]", mediaID)
	}
	
	// Create request
	req, err := http.NewRequest("POST", c.InstanceURL+"/api/v1/statuses", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post status: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response to get the status URL
	var statusResp struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	return nil
}

// UploadMedia uploads an image to Mastodon and returns the media ID
func (c *Client) UploadMedia(imagePath string, altText string) (string, error) {
	// Open the file
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Read the file contents to detect MIME type
	fileData, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	
	// Detect MIME type from actual file contents
	mimeType := http.DetectContentType(fileData)
	
	// Validate that it's an image type Mastodon accepts
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	
	if !validTypes[mimeType] {
		// Fall back to extension-based detection if content detection fails
		ext := strings.ToLower(filepath.Ext(imagePath))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		default:
			return "", fmt.Errorf("unsupported image type: %s", mimeType)
		}
	}
	
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Add file field with explicit content type
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(imagePath)))
	h.Set("Content-Type", mimeType)
	
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("failed to create form part: %w", err)
	}
	
	// Write the file data we already read
	if _, err := part.Write(fileData); err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}
	
	// Add description (alt text) if provided
	if altText != "" {
		if err := writer.WriteField("description", altText); err != nil {
			return "", fmt.Errorf("failed to write description field: %w", err)
		}
	}
	
	writer.Close()
	
	// Create request
	req, err := http.NewRequest("POST", c.InstanceURL+"/api/v2/media", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Send request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var mediaResp struct {
		ID string `json:"id"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&mediaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	return mediaResp.ID, nil
}

// UploadMediaFromURL downloads an image from URL and uploads it to Mastodon
func (c *Client) UploadMediaFromURL(imageURL string, altText string) (string, error) {
	// Download image to temp file
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()
	
	// Read the response body
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}
	
	// Detect MIME type from actual content
	detectedType := http.DetectContentType(imageData)
	
	// Check if we got HTML instead of an image
	if strings.HasPrefix(detectedType, "text/") {
		preview := string(imageData)
		if len(preview) > 100 {
			preview = preview[:100]
		}
		return "", fmt.Errorf("received HTML/text response instead of image from URL: %s", imageURL)
	}
	
	// Determine file extension from URL or Content-Type
	ext := filepath.Ext(imageURL)
	if ext == "" {
		// Try to get from Content-Type header
		contentType := resp.Header.Get("Content-Type")
		switch contentType {
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".jpg" // default to jpg
		}
	}
	
	// Create temp file with proper extension
	tempFile, err := os.CreateTemp("", "mastodon-upload-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	
	// Write image data
	_, err = tempFile.Write(imageData)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}
	
	// Upload the temp file
	return c.UploadMedia(tempFile.Name(), altText)
}
