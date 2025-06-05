package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/types"
)

// FlickrPullClient handles pulling images from Flickr
type FlickrPullClient struct {
	api *FlickrAPI
	cfg *config.FlickrConfig
}

// NewFlickrPullClient creates a new Flickr pull client
func NewFlickrPullClient(cfg *config.FlickrConfig) *FlickrPullClient {
	return &FlickrPullClient{
		api: NewFlickrAPI(cfg),
		cfg: cfg,
	}
}

// PullImages fetches recent images from Flickr
func (c *FlickrPullClient) PullImages(ctx context.Context, albumName string, count int, tags string) ([]types.PullImage, error) {
	// Get user ID first
	userID, err := c.api.GetUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	var photos []photosetPhoto
	var isPhotostream bool
	
	// If tags are specified, use search instead of album/photostream
	if tags != "" {
		// Parse comma-separated tags
		tagList := strings.Split(tags, ",")
		for i := range tagList {
			tagList[i] = strings.TrimSpace(tagList[i])
		}

		// Use search API to find photos by tags
		searchParams := PhotoSearchParams{
			UserID:  userID,
			Tags:    tagList,
			PerPage: count,
			Page:    1,
		}

		searchResp, err := c.api.PhotosSearch(ctx, searchParams)
		if err != nil {
			return nil, fmt.Errorf("failed to search photos by tags: %w", err)
		}

		// Convert search results to photosetPhoto format for consistency
		photos = make([]photosetPhoto, len(searchResp.Photos))
		for i, photo := range searchResp.Photos {
			photos[i] = photosetPhoto{
				ID:     photo.ID,
				Title:  photo.Title,
				Secret: photo.Secret,
				Server: photo.Server,
				Farm:   photo.Farm,
			}
		}

		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Found %d photos with tags %v\n", len(photos), tagList)
		}
	} else if albumName != "" && albumName != "photostream" {
		// Find the photoset by name
		photosetID, err := c.findPhotosetByName(ctx, userID, albumName)
		if err != nil {
			return nil, fmt.Errorf("failed to find photoset '%s': %w", albumName, err)
		}
		
		// Get photos from the photoset
		photos, err = c.getPhotosetPhotos(ctx, photosetID, count)
		if err != nil {
			return nil, fmt.Errorf("failed to get photos from photoset: %w", err)
		}
	} else {
		// Get photos from user's photostream
		isPhotostream = true
		photos, err = c.getUserPhotos(ctx, userID, count)
		if err != nil {
			return nil, fmt.Errorf("failed to get photos from photostream: %w", err)
		}
	}

	if os.Getenv("IMGUP_DEBUG") != "" {
		if isPhotostream {
			fmt.Fprintf(os.Stderr, "DEBUG: Found %d photos in photostream\n", len(photos))
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: Found %d photos in photoset\n", len(photos))
		}
	}

	// Convert to PullImage format
	pullImages := make([]types.PullImage, 0, len(photos))
	for i, photo := range photos {
		// Get photo info for metadata
		info, err := c.getPhotoInfo(ctx, photo.ID)
		if err != nil {
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to get info for photo %s: %v\n", photo.ID, err)
			}
			continue
		}

		// Get available sizes
		sizes, err := c.getImageSizes(ctx, photo.ID)
		if err != nil {
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to get sizes for photo %s: %v\n", photo.ID, err)
			}
			continue
		}

		// Build photo page URL
		photoURL := fmt.Sprintf("https://www.flickr.com/photos/%s/%s", userID, photo.ID)

		pullImage := types.PullImage{
			ID:          fmt.Sprintf("%d", i+1),
			Title:       info.Title,
			Description: info.Description,
			SourceURL:   photoURL,
			Sizes:       sizes,
			Tags:        info.Tags,
		}

		// Set alt text from description or title
		if info.Description != "" {
			pullImage.Alt = info.Description
		} else if info.Title != "" {
			pullImage.Alt = info.Title
		}

		pullImages = append(pullImages, pullImage)
	}

	return pullImages, nil
}

// photosetPhoto represents a photo in a photoset
type photosetPhoto struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Secret string `json:"secret"`
	Server string `json:"server"`
	Farm   int    `json:"farm"`
}

// findPhotosetByName finds a photoset by name
func (c *FlickrPullClient) findPhotosetByName(ctx context.Context, userID, name string) (string, error) {
	params := url.Values{}
	params.Set("method", "flickr.photosets.getList")
	params.Set("user_id", userID)
	params.Set("format", "json")
	params.Set("nojsoncallback", "1")
	
	resp, err := c.api.makeAPICall(ctx, "GET", params)
	if err != nil {
		return "", fmt.Errorf("failed to get photosets: %w", err)
	}
	
	var result struct {
		Photosets struct {
			Photoset []struct {
				ID    string `json:"id"`
				Title struct {
					Content string `json:"_content"`
				} `json:"title"`
				Photos int `json:"photos"`
			} `json:"photoset"`
		} `json:"photosets"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse photosets response: %w", err)
	}
	
	if result.Stat != "ok" {
		return "", fmt.Errorf("API error: %s", result.Message)
	}

	// Debug: print available photosets
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Available photosets:\n")
		for _, ps := range result.Photosets.Photoset {
			fmt.Fprintf(os.Stderr, "  - %s (ID: %s, %d photos)\n", ps.Title.Content, ps.ID, ps.Photos)
		}
	}

	// Find photoset by name
	for _, ps := range result.Photosets.Photoset {
		if strings.EqualFold(ps.Title.Content, name) {
			return ps.ID, nil
		}
	}

	// If not found, suggest similar photosets
	var suggestions []string
	for _, ps := range result.Photosets.Photoset {
		if strings.Contains(strings.ToLower(ps.Title.Content), strings.ToLower(name)) ||
		   strings.Contains(strings.ToLower(name), strings.ToLower(ps.Title.Content)) {
			suggestions = append(suggestions, ps.Title.Content)
		}
	}

	if len(suggestions) > 0 {
		return "", fmt.Errorf("photoset '%s' not found. Did you mean one of: %s", name, strings.Join(suggestions, ", "))
	}

	return "", fmt.Errorf("photoset '%s' not found", name)
}

// getPhotosetPhotos gets photos from a photoset
func (c *FlickrPullClient) getPhotosetPhotos(ctx context.Context, photosetID string, count int) ([]photosetPhoto, error) {
	params := url.Values{}
	params.Set("method", "flickr.photosets.getPhotos")
	params.Set("photoset_id", photosetID)
	params.Set("per_page", fmt.Sprintf("%d", count))
	params.Set("format", "json")
	params.Set("nojsoncallback", "1")
	
	resp, err := c.api.makeAPICall(ctx, "GET", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get photoset photos: %w", err)
	}
	
	var result struct {
		Photoset struct {
			Photo []photosetPhoto `json:"photo"`
		} `json:"photoset"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse photoset photos response: %w", err)
	}
	
	if result.Stat != "ok" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}
	
	return result.Photoset.Photo, nil
}

// getUserPhotos gets photos from user's photostream
func (c *FlickrPullClient) getUserPhotos(ctx context.Context, userID string, count int) ([]photosetPhoto, error) {
	params := url.Values{}
	params.Set("method", "flickr.people.getPhotos")
	params.Set("user_id", userID)
	params.Set("per_page", fmt.Sprintf("%d", count))
	params.Set("format", "json")
	params.Set("nojsoncallback", "1")
	
	resp, err := c.api.makeAPICall(ctx, "GET", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get user photos: %w", err)
	}
	
	var result struct {
		Photos struct {
			Photo []photosetPhoto `json:"photo"`
		} `json:"photos"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse user photos response: %w", err)
	}
	
	if result.Stat != "ok" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}
	
	return result.Photos.Photo, nil
}

// photoInfo contains detailed photo information
type photoInfo struct {
	Title       string
	Description string
	Tags        []string
}

// getPhotoInfo gets detailed information about a photo
func (c *FlickrPullClient) getPhotoInfo(ctx context.Context, photoID string) (*photoInfo, error) {
	params := url.Values{}
	params.Set("method", "flickr.photos.getInfo")
	params.Set("photo_id", photoID)
	params.Set("format", "json")
	params.Set("nojsoncallback", "1")
	
	resp, err := c.api.makeAPICall(ctx, "GET", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get photo info: %w", err)
	}
	
	var result struct {
		Photo struct {
			Title struct {
				Content string `json:"_content"`
			} `json:"title"`
			Description struct {
				Content string `json:"_content"`
			} `json:"description"`
			Tags struct {
				Tag []struct {
					Raw string `json:"raw"`
				} `json:"tag"`
			} `json:"tags"`
		} `json:"photo"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse photo info response: %w", err)
	}
	
	if result.Stat != "ok" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}
	
	info := &photoInfo{
		Title:       result.Photo.Title.Content,
		Description: result.Photo.Description.Content,
		Tags:        make([]string, 0, len(result.Photo.Tags.Tag)),
	}
	
	for _, tag := range result.Photo.Tags.Tag {
		info.Tags = append(info.Tags, tag.Raw)
	}
	
	return info, nil
}

// getImageSizes fetches all available sizes for an image
func (c *FlickrPullClient) getImageSizes(ctx context.Context, photoID string) (types.ImageSizes, error) {
	// Use the existing GetPhotoSizes method from FlickrAPI
	photoSizes, err := c.api.GetPhotoSizes(ctx, photoID)
	if err != nil {
		return types.ImageSizes{}, err
	}
	
	sizes := types.ImageSizes{}
	
	// Map Flickr sizes to our standard sizes
	// Priority order for each size category
	largeSizes := []string{"Large 2048", "Large 1600", "Large", "Medium 800"}
	mediumSizes := []string{"Medium 800", "Medium 640", "Medium"}
	smallSizes := []string{"Small 320", "Small", "Medium"}
	thumbSizes := []string{"Thumbnail", "Square", "Small"}
	
	// Helper to find first matching size
	findSize := func(labels []string) string {
		for _, label := range labels {
			for _, size := range photoSizes {
				if size.Label == label {
					return size.Source
				}
			}
		}
		return ""
	}
	
	// Assign sizes based on priority
	sizes.Large = findSize(largeSizes)
	sizes.Medium = findSize(mediumSizes)
	sizes.Small = findSize(smallSizes)
	sizes.Thumb = findSize(thumbSizes)
	
	// Fallback to any available size if specific sizes not found
	if sizes.Large == "" || sizes.Medium == "" {
		for _, size := range photoSizes {
			if sizes.Large == "" && (size.Width >= 1024 || strings.Contains(size.Label, "Large")) {
				sizes.Large = size.Source
			}
			if sizes.Medium == "" && (size.Width >= 640 || strings.Contains(size.Label, "Medium")) {
				sizes.Medium = size.Source
			}
			if sizes.Small == "" && (size.Width >= 320 || strings.Contains(size.Label, "Small")) {
				sizes.Small = size.Source
			}
			if sizes.Thumb == "" {
				sizes.Thumb = size.Source
			}
		}
	}
	
	// Final fallback - use original if available
	if sizes.Large == "" {
		for _, size := range photoSizes {
			if size.Label == "Original" {
				sizes.Large = size.Source
				if sizes.Medium == "" {
					sizes.Medium = size.Source
				}
				if sizes.Small == "" {
					sizes.Small = size.Source
				}
				if sizes.Thumb == "" {
					sizes.Thumb = size.Source
				}
				break
			}
		}
	}
	
	return sizes, nil
}
