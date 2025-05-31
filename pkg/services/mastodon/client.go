package mastodon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
	
	fmt.Printf("Posted to Mastodon: %s\n", statusResp.URL)
	
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
	
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
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
	
	// Create temp file
	tempFile, err := os.CreateTemp("", "mastodon-upload-*.jpg")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	
	// Copy image data
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}
	
	// Upload the temp file
	return c.UploadMedia(tempFile.Name(), altText)
}
