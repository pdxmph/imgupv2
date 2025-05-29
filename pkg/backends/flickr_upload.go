package backends

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
	
	"github.com/mph/imgupv2/pkg/oauth"
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

// Upload uploads an image to Flickr
func (u *FlickrUploader) Upload(ctx context.Context, imagePath string, title, description string) (*UploadResult, error) {
	// Open the image file
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()
	
	// Create OAuth parameters
	client := oauth.NewOAuth1Client(oauth.OAuth1Config{
		ConsumerKey:    u.ConsumerKey,
		ConsumerSecret: u.ConsumerSecret,
	})
	
	oauthParams := map[string]string{
		"oauth_consumer_key":     u.ConsumerKey,
		"oauth_token":           u.AccessToken,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_version":         "1.0",
		"oauth_timestamp":       fmt.Sprintf("%d", time.Now().Unix()),
		"oauth_nonce":           client.Nonce(),
	}
	
	// Calculate signature - only OAuth params, not form data
	signature := client.Signature("POST", flickrUploadURL, oauthParams, u.AccessSecret)
	oauthParams["oauth_signature"] = signature
	
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Add OAuth parameters as form fields
	for k, v := range oauthParams {
		if err := writer.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("failed to write oauth field %s: %w", k, err)
		}
	}
	
	// Add title if provided
	if title != "" {
		if err := writer.WriteField("title", title); err != nil {
			return nil, fmt.Errorf("failed to write title: %w", err)
		}
	}
	
	// Add description if provided
	if description != "" {
		if err := writer.WriteField("description", description); err != nil {
			return nil, fmt.Errorf("failed to write description: %w", err)
		}
	}
	
	// Add image file last
	part, err := writer.CreateFormFile("photo", filepath.Base(imagePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}
	
	// Close the writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}
	
	// Create request without Authorization header
	req, err := http.NewRequestWithContext(ctx, "POST", flickrUploadURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(buf.Len())
	
	// Make request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, body)
	}
	
	// Parse response to get photo ID
	photoID := u.parsePhotoID(string(body))
	if photoID == "" {
		return nil, fmt.Errorf("failed to parse photo ID from response: %s", body)
	}
	
	// Get the actual photo URL and sizes
	api := &FlickrAPI{FlickrUploader: u}
	photoInfo, err := api.GetPhotoInfo(ctx, photoID)
	if err != nil {
		// Fall back to edit URL if we can't get photo info
		return &UploadResult{
			PhotoID: photoID,
			URL:     fmt.Sprintf("https://www.flickr.com/photos/upload/edit/?ids=%s", photoID),
		}, nil
	}
	
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
	
	return &UploadResult{
		PhotoID:  photoID,
		URL:      photoInfo.URL,
		ImageURL: imageURL,
	}, nil
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
