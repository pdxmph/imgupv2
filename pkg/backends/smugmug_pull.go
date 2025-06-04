package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	
	"github.com/dghubble/oauth1"
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/types"
)

// SmugMugPullClient handles pulling images from SmugMug
type SmugMugPullClient struct {
	api *SmugMugAPI
	cfg *config.SmugMugConfig
}

// NewSmugMugPullClient creates a new SmugMug pull client
func NewSmugMugPullClient(cfg *config.SmugMugConfig) *SmugMugPullClient {
	return &SmugMugPullClient{
		api: NewSmugMugAPI(cfg),
		cfg: cfg,
	}
}

// PullImages fetches recent images from SmugMug
func (c *SmugMugPullClient) PullImages(ctx context.Context, albumName string, count int) ([]types.PullImage, error) {
	// If no album name is provided, use the configured album
	if albumName == "" {
		if c.cfg.PullAlbum != "" {
			albumName = c.cfg.PullAlbum
		} else {
			albumName = "Sharing"
		}
	}

	// Get user info
	userResp, err := c.api.GetAuthenticatedUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Find the album by name
	album, err := c.findAlbumByName(ctx, userResp.Response.User.NickName, albumName)
	if err != nil {
		return nil, fmt.Errorf("failed to find album '%s': %w", albumName, err)
	}

	// Get images from the album
	images, err := c.api.GetAlbumImages(ctx, album.AlbumKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get images from album: %w", err)
	}

	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Found %d images in album\n", len(images))
	}

	// Limit to requested count
	if len(images) > count {
		images = images[:count]
	}

	// Convert to PullImage format
	pullImages := make([]types.PullImage, 0, len(images))
	for i, img := range images {
		sizes, err := c.getImageSizes(ctx, img.ImageKey)
		if err != nil {
			// Log error but continue with other images
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to get sizes for image %s: %v\n", img.ImageKey, err)
			}
			continue
		}

		// Use filename as title if title is empty
		title := img.Title
		if title == "" {
			title = img.FileName
			// Remove extension from filename for cleaner title
			if idx := strings.LastIndex(title, "."); idx > 0 {
				title = title[:idx]
			}
		}

		pullImage := types.PullImage{
			ID:          fmt.Sprintf("%d", i+1),
			Title:       title,
			Description: img.Caption,
			SourceURL:   img.WebURI,
			Sizes:       sizes,
		}

		// Parse keywords into tags
		if img.Keywords != "" {
			pullImage.Tags = strings.Split(img.Keywords, ";")
			// Trim whitespace from tags
			for j := range pullImage.Tags {
				pullImage.Tags[j] = strings.TrimSpace(pullImage.Tags[j])
			}
		}

		pullImages = append(pullImages, pullImage)
	}

	return pullImages, nil
}

// findAlbumByName finds an album by name
func (c *SmugMugPullClient) findAlbumByName(ctx context.Context, nickname, albumName string) (*Album, error) {
	albums, err := c.api.ListAlbums(ctx)
	if err != nil {
		return nil, err
	}

	// Debug: print available albums if debug mode is on
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Available albums:\n")
		for _, album := range albums {
			fmt.Fprintf(os.Stderr, "  - %s (Key: %s, %d images)\n", album.Name, album.AlbumKey, album.ImageCount)
		}
	}

	for _, album := range albums {
		if strings.EqualFold(album.Name, albumName) {
			return &album, nil
		}
	}

	// If not found, suggest similar albums
	var suggestions []string
	for _, album := range albums {
		if strings.Contains(strings.ToLower(album.Name), strings.ToLower(albumName)) ||
		   strings.Contains(strings.ToLower(albumName), strings.ToLower(album.Name)) {
			suggestions = append(suggestions, album.Name)
		}
	}

	if len(suggestions) > 0 {
		return nil, fmt.Errorf("album '%s' not found. Did you mean one of: %s", albumName, strings.Join(suggestions, ", "))
	}

	return nil, fmt.Errorf("album '%s' not found", albumName)
}

// getImageSizes fetches all available sizes for an image
func (c *SmugMugPullClient) getImageSizes(ctx context.Context, imageKey string) (types.ImageSizes, error) {
	// Construct the URL for image size details
	url := fmt.Sprintf("%s/api/v2/image/%s!sizedetails", smugmugAPIURL, imageKey)

	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    c.cfg.ConsumerKey,
		ConsumerSecret: c.cfg.ConsumerSecret,
	}
	
	token := oauth1.NewToken(c.cfg.AccessToken, c.cfg.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.ImageSizes{}, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return types.ImageSizes{}, fmt.Errorf("failed to fetch size details: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return types.ImageSizes{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Decode the response as a generic map first
	var rawResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return types.ImageSizes{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Navigate to the ImageSizeDetails
	respObj, ok := rawResp["Response"].(map[string]interface{})
	if !ok {
		return types.ImageSizes{}, fmt.Errorf("missing Response in API response")
	}

	imageSizeDetails, ok := respObj["ImageSizeDetails"].(map[string]interface{})
	if !ok {
		return types.ImageSizes{}, fmt.Errorf("missing ImageSizeDetails in Response")
	}

	// Map SmugMug sizes to our standard sizes
	sizes := types.ImageSizes{}
	
	// Helper function to extract URL from size data
	extractURL := func(sizeName string) string {
		if sizeData, ok := imageSizeDetails[sizeName].(map[string]interface{}); ok {
			if urlStr, ok := sizeData["Url"].(string); ok {
				return urlStr
			}
		}
		return ""
	}
	
	// Large: X2Large (2048px) for social media
	sizes.Large = extractURL("ImageSizeX2Large")
	if sizes.Large == "" {
		sizes.Large = extractURL("ImageSizeXLarge")
	}
	
	// Medium: Large (800px) for embedding
	sizes.Medium = extractURL("ImageSizeLarge")
	if sizes.Medium == "" {
		sizes.Medium = extractURL("ImageSizeMedium")
	}
	
	// Small: Small or Medium as fallback
	sizes.Small = extractURL("ImageSizeSmall")
	if sizes.Small == "" {
		sizes.Small = extractURL("ImageSizeMedium")
	}
	
	// Thumb: Thumb or Small as fallback
	sizes.Thumb = extractURL("ImageSizeThumb")
	if sizes.Thumb == "" {
		sizes.Thumb = extractURL("ImageSizeSmall")
	}

	// Fallback to any available size if specific sizes not found
	if sizes.Large == "" || sizes.Medium == "" {
		sizePreference := []string{
			"ImageSizeX3Large", "ImageSizeX2Large", "ImageSizeXLarge", 
			"ImageSizeLarge", "ImageSizeMedium", "ImageSizeOriginal",
		}
		
		for _, sizeName := range sizePreference {
			url := extractURL(sizeName)
			if url != "" {
				if sizes.Large == "" {
					sizes.Large = url
				}
				if sizes.Medium == "" {
					sizes.Medium = url
				}
				if sizes.Small == "" {
					sizes.Small = url
				}
				if sizes.Thumb == "" {
					sizes.Thumb = url
				}
			}
		}
	}

	return sizes, nil
}
