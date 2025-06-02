package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Client represents a Bluesky API client
type Client struct {
	PDS         string // Personal Data Server URL (e.g., https://bsky.social)
	Handle      string
	AppPassword string
	DID         string // Decentralized Identifier
	AccessJWT   string
	RefreshJWT  string
}

// Session represents the response from createSession
type Session struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	Handle     string `json:"handle"`
	DID        string `json:"did"`
}

// BlobResponse represents the response from uploadBlob
type BlobResponse struct {
	Blob struct {
		Type     string `json:"$type"`
		Ref      BlobRef `json:"ref"`
		MimeType string `json:"mimeType"`
		Size     int    `json:"size"`
	} `json:"blob"`
}

// BlobRef represents a blob reference
type BlobRef struct {
	Link string `json:"$link"`
}

// PostRecord represents a Bluesky post
type PostRecord struct {
	Type      string    `json:"$type"`
	Text      string    `json:"text"`
	CreatedAt string    `json:"createdAt"`
	Embed     *Embed    `json:"embed,omitempty"`
	Facets    []Facet   `json:"facets,omitempty"`
}

// Facet represents a rich text annotation (links, mentions, etc)
type Facet struct {
	Index    FacetIndex    `json:"index"`
	Features []FacetFeature `json:"features"`
}

// FacetIndex specifies the byte range in the text
type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

// FacetFeature represents a facet feature (link, mention, etc)
type FacetFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri,omitempty"`
	DID  string `json:"did,omitempty"`
}

// Embed represents an embedded object (images)
type Embed struct {
	Type   string  `json:"$type"`
	Images []Image `json:"images"`
}

// Image represents an image in a post
type Image struct {
	Alt   string    `json:"alt"`
	Image ImageBlob `json:"image"`
}

// ImageBlob represents the blob data for an image
type ImageBlob struct {
	Type     string  `json:"$type"`
	Ref      BlobRef `json:"ref"`
	MimeType string  `json:"mimeType"`
	Size     int     `json:"size"`
}

// NewClient creates a new Bluesky client
func NewClient(pds, handle, appPassword string) *Client {
	// Ensure PDS URL doesn't have trailing slash
	pds = strings.TrimRight(pds, "/")
	
	// Default to bsky.social if not specified
	if pds == "" {
		pds = "https://bsky.social"
	}
	
	return &Client{
		PDS:         pds,
		Handle:      handle,
		AppPassword: appPassword,
	}
}

// Authenticate creates a session with Bluesky
func (c *Client) Authenticate() error {
	authData := map[string]string{
		"identifier": c.Handle,
		"password":   c.AppPassword,
	}
	
	jsonData, err := json.Marshal(authData)
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %w", err)
	}
	
	resp, err := http.Post(
		c.PDS+"/xrpc/com.atproto.server.createSession",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("failed to decode session: %w", err)
	}
	
	c.AccessJWT = session.AccessJwt
	c.RefreshJWT = session.RefreshJwt
	c.DID = session.DID
	
	return nil
}

// detectURLs finds URLs in text and returns facets for them
func detectURLs(text string) []Facet {
	// URL regex - simplified version that catches most common URLs
	// Based on the Bluesky docs recommendation
	urlRegex := regexp.MustCompile(`https?://[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*(/[a-zA-Z0-9_.\-~:/?#\[\]@!$&'()*+,;=]*)?`)
	
	facets := []Facet{}
	
	// Convert text to bytes for accurate byte indexing
	textBytes := []byte(text)
	
	// Find all URL matches
	matches := urlRegex.FindAllIndex(textBytes, -1)
	
	for _, match := range matches {
		byteStart := match[0]
		byteEnd := match[1]
		url := string(textBytes[byteStart:byteEnd])
		
		facet := Facet{
			Index: FacetIndex{
				ByteStart: byteStart,
				ByteEnd:   byteEnd,
			},
			Features: []FacetFeature{
				{
					Type: "app.bsky.richtext.facet#link",
					URI:  url,
				},
			},
		}
		
		facets = append(facets, facet)
	}
	
	return facets
}

// PostStatus posts a new status to Bluesky
func (c *Client) PostStatus(text string, mediaBlobs []BlobResponse, altTexts []string, tags []string) error {
	// Ensure we're authenticated
	if c.AccessJWT == "" {
		if err := c.Authenticate(); err != nil {
			return fmt.Errorf("failed to authenticate: %w", err)
		}
	}
	
	// Convert tags to hashtags
	for _, tag := range tags {
		// Only add hashtag if not already in the text
		hashtag := "#" + strings.ReplaceAll(tag, " ", "")
		if !strings.Contains(text, hashtag) {
			text += " " + hashtag
		}
	}
	
	// Check character limit (300 for Bluesky)
	if len(text) > 300 {
		return fmt.Errorf("text exceeds Bluesky's 300 character limit (%d characters)", len(text))
	}
	
	// Create post record
	post := PostRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	
	// Detect URLs and add facets to make them clickable
	facets := detectURLs(text)
	if len(facets) > 0 {
		post.Facets = facets
	}
	
	// Add images if provided
	if len(mediaBlobs) > 0 {
		embed := &Embed{
			Type:   "app.bsky.embed.images",
			Images: make([]Image, len(mediaBlobs)),
		}
		
		for i, blob := range mediaBlobs {
			// Use provided alt text if available
			altText := ""
			if i < len(altTexts) && altTexts[i] != "" {
				altText = altTexts[i]
			}
			
			embed.Images[i] = Image{
				Alt: altText,
				Image: ImageBlob{
					Type:     blob.Blob.Type,
					Ref:      blob.Blob.Ref,
					MimeType: blob.Blob.MimeType,
					Size:     blob.Blob.Size,
				},
			}
		}
		
		post.Embed = embed
	}
	
	// Create the full request body
	reqBody := map[string]interface{}{
		"repo":       c.DID,
		"collection": "app.bsky.feed.post",
		"record":     post,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal post data: %w", err)
	}
	
	// Create request
	req, err := http.NewRequest("POST", c.PDS+"/xrpc/com.atproto.repo.createRecord", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.AccessJWT)
	req.Header.Set("Content-Type", "application/json")
	
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
	
	// Parse response to get the post URI
	var postResp struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&postResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Convert AT URI to web URL
	// at://did:plc:xxx/app.bsky.feed.post/yyy -> https://bsky.app/profile/handle/post/yyy
	parts := strings.Split(postResp.URI, "/")
	if len(parts) >= 5 {
		postID := parts[len(parts)-1]
		webURL := fmt.Sprintf("https://bsky.app/profile/%s/post/%s", c.Handle, postID)
		fmt.Printf("Posted to Bluesky: %s\n", webURL)
	}
	
	return nil
}

// UploadMedia uploads an image to Bluesky and returns the blob response
func (c *Client) UploadMedia(imagePath string, altText string) (*BlobResponse, string, error) {
	// Ensure we're authenticated
	if c.AccessJWT == "" {
		if err := c.Authenticate(); err != nil {
			return nil, "", fmt.Errorf("failed to authenticate: %w", err)
		}
	}
	
	// Open the file
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Get file info for size check
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Check file size (1MB limit)
	if fileInfo.Size() > 1000000 {
		return nil, "", fmt.Errorf("image file size too large. Maximum is 1MB, got %d bytes", fileInfo.Size())
	}
	
	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}
	
	// Determine MIME type
	mimeType := "image/jpeg" // default
	ext := strings.ToLower(filepath.Ext(imagePath))
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}
	
	// Create request
	req, err := http.NewRequest("POST", c.PDS+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewReader(fileBytes))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.AccessJWT)
	req.Header.Set("Content-Type", mimeType)
	
	// Send request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to upload media: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var blobResp BlobResponse
	if err := json.NewDecoder(resp.Body).Decode(&blobResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Store alt text separately - it will be added when creating the post
	// Bluesky includes alt text in the embed, not during upload
	
	return &blobResp, altText, nil
}

// UploadMediaFromURL downloads an image from URL and uploads it to Bluesky
func (c *Client) UploadMediaFromURL(imageURL string, altText string) (*BlobResponse, string, error) {
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Bluesky UploadMediaFromURL called with URL: %s\n", imageURL)
	}
	
	// Download image to temp file with timeout
	client := &http.Client{
		Timeout: 30 * time.Second, // 30 second timeout for download
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Downloading image from %s...\n", imageURL)
	}
	
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Image downloaded successfully, creating temp file...\n")
	}
	
	// Create temp file
	tempFile, err := os.CreateTemp("", "bluesky-upload-*.jpg")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	
	// Copy image data
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to save image: %w", err)
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Temp file created, uploading to Bluesky...\n")
	}
	
	// Upload the temp file
	return c.UploadMedia(tempFile.Name(), altText)
}
