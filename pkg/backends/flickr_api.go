package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	
	"github.com/pdxmph/imgupv2/pkg/config"
)

// FlickrAPI handles Flickr API calls
type FlickrAPI struct {
	*FlickrUploader
}

// PhotoInfo contains basic photo information
type PhotoInfo struct {
	ID       string
	Owner    string
	OwnerNSID string
	URL      string
	Sizes    []PhotoSize
}

// PhotoSize represents a photo size variant
type PhotoSize struct {
	Label  string
	Width  int
	Height int
	Source string
}

// NewFlickrAPI creates a new Flickr API client
func NewFlickrAPI(cfg *config.FlickrConfig) *FlickrAPI {
	return &FlickrAPI{
		FlickrUploader: NewFlickrUploader(
			cfg.ConsumerKey,
			cfg.ConsumerSecret,
			cfg.AccessToken,
			cfg.AccessSecret,
		),
	}
}

// GetPhotoInfo gets information about a photo
func (api *FlickrAPI) GetPhotoInfo(ctx context.Context, photoID string) (*PhotoInfo, error) {
	// Call flickr.photos.getInfo
	params := url.Values{
		"method":         {"flickr.photos.getInfo"},
		"api_key":        {api.ConsumerKey},
		"photo_id":       {photoID},
		"format":         {"json"},
		"nojsoncallback": {"1"},
	}
	
	resp, err := http.Get(flickrAPIURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to get photo info: %w", err)
	}
	defer resp.Body.Close()
	
	var result struct {
		Photo struct {
			ID    string `json:"id"`
			Owner struct {
				NSID     string `json:"nsid"`
				Username string `json:"username"`
			} `json:"owner"`
		} `json:"photo"`
		Stat string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if result.Stat != "ok" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}
	
	// Build the photo URL
	photoURL := fmt.Sprintf("https://www.flickr.com/photos/%s/%s", 
		result.Photo.Owner.NSID, 
		result.Photo.ID)
	
	return &PhotoInfo{
		ID:        result.Photo.ID,
		OwnerNSID: result.Photo.Owner.NSID,
		URL:       photoURL,
	}, nil
}

// GetPhotoSizes gets available sizes for a photo
func (api *FlickrAPI) GetPhotoSizes(ctx context.Context, photoID string) ([]PhotoSize, error) {
	params := url.Values{
		"method":         {"flickr.photos.getSizes"},
		"api_key":        {api.ConsumerKey},
		"photo_id":       {photoID},
		"format":         {"json"},
		"nojsoncallback": {"1"},
	}
	
	resp, err := http.Get(flickrAPIURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to get photo sizes: %w", err)
	}
	defer resp.Body.Close()
	
	var result struct {
		Sizes struct {
			Size []struct {
				Label  string `json:"label"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
				Source string `json:"source"`
			} `json:"size"`
		} `json:"sizes"`
		Stat string `json:"stat"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if result.Stat != "ok" {
		return nil, fmt.Errorf("API returned error")
	}
	
	var sizes []PhotoSize
	for _, s := range result.Sizes.Size {
		sizes = append(sizes, PhotoSize{
			Label:  s.Label,
			Width:  s.Width,
			Height: s.Height,
			Source: s.Source,
		})
	}
	
	return sizes, nil
}

// PhotoSearchParams contains parameters for photo search
type PhotoSearchParams struct {
	UserID      string   // User NSID (optional, but recommended for performance)
	Tags        []string // Regular tags
	MachineTags []string // Machine tags (e.g., "imgupv2:checksum=abc123")
	Text        string   // Free text search
	MinTakenDate string  // Minimum taken date (MySQL datetime)
	MaxTakenDate string  // Maximum taken date (MySQL datetime)
	Page        int      // Page number (default 1)
	PerPage     int      // Results per page (default 100, max 500)
}

// PhotoSearchResult represents a photo in search results
type PhotoSearchResult struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Secret   string `json:"secret"`
	Server   string `json:"server"`
	Farm     int    `json:"farm"`
	Title    string `json:"title"`
	IsPublic int    `json:"ispublic"`
	IsFriend int    `json:"isfriend"`
	IsFamily int    `json:"isfamily"`
}

// PhotoSearchResponse contains the search response
type PhotoSearchResponse struct {
	Photos []PhotoSearchResult
	Page   int
	Pages  int
	Total  int
}

// PhotosSearch searches for photos using various criteria
func (api *FlickrAPI) PhotosSearch(ctx context.Context, params PhotoSearchParams) (*PhotoSearchResponse, error) {
	// Build query parameters
	qp := url.Values{
		"method":         {"flickr.photos.search"},
		"format":         {"json"},
		"nojsoncallback": {"1"},
	}
	
	// Add optional parameters
	if params.UserID != "" {
		qp.Set("user_id", params.UserID)
	}
	
	if len(params.Tags) > 0 {
		qp.Set("tags", strings.Join(params.Tags, ","))
		qp.Set("tag_mode", "all") // Require all tags
	}
	
	if len(params.MachineTags) > 0 {
		qp.Set("machine_tags", strings.Join(params.MachineTags, ","))
		qp.Set("machine_tag_mode", "all") // Require all machine tags
	}
	
	if params.Text != "" {
		qp.Set("text", params.Text)
	}
	
	if params.MinTakenDate != "" {
		qp.Set("min_taken_date", params.MinTakenDate)
	}
	
	if params.MaxTakenDate != "" {
		qp.Set("max_taken_date", params.MaxTakenDate)
	}
	
	// Pagination
	if params.Page > 0 {
		qp.Set("page", fmt.Sprintf("%d", params.Page))
	}
	
	perPage := params.PerPage
	if perPage == 0 {
		perPage = 100 // Default
	} else if perPage > 500 {
		perPage = 500 // Max allowed by Flickr
	}
	qp.Set("per_page", fmt.Sprintf("%d", perPage))
	
	// Make the API call
	resp, err := api.makeAPICall(ctx, "GET", qp)
	if err != nil {
		return nil, fmt.Errorf("failed to search photos: %w", err)
	}
	
	// Parse response
	var result struct {
		Photos struct {
			Page    int                 `json:"page"`
			Pages   int                 `json:"pages"`
			PerPage int                 `json:"perpage"`
			Total   json.RawMessage     `json:"total"`
			Photo   []PhotoSearchResult `json:"photo"`
		} `json:"photos"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}
	
	if result.Stat != "ok" {
		return nil, fmt.Errorf("search failed: %s", result.Message)
	}
	
	// Parse total - handle both string and number formats
	var total int
	if len(result.Photos.Total) > 0 {
		// Try parsing as number first
		if err := json.Unmarshal(result.Photos.Total, &total); err != nil {
			// If that fails, try as string
			var totalStr string
			if err := json.Unmarshal(result.Photos.Total, &totalStr); err == nil {
				fmt.Sscanf(totalStr, "%d", &total)
			}
		}
	}
	
	return &PhotoSearchResponse{
		Photos: result.Photos.Photo,
		Page:   result.Photos.Page,
		Pages:  result.Photos.Pages,
		Total:  total,
	}, nil
}

// BuildPhotoURL constructs the Flickr photo page URL from search result
func (api *FlickrAPI) BuildPhotoURL(photo PhotoSearchResult) string {
	return fmt.Sprintf("https://www.flickr.com/photos/%s/%s", photo.Owner, photo.ID)
}

// BuildImageURL constructs the direct image URL from search result
// Size can be: s (square 75), q (square 150), t (thumbnail), m (small), 
// n (small 320), z (medium), c (medium 800), b (large), h (large 1600), 
// k (large 2048), o (original)
func (api *FlickrAPI) BuildImageURL(photo PhotoSearchResult, size string) string {
	if size == "" {
		size = "b" // Default to large
	}
	return fmt.Sprintf("https://farm%d.staticflickr.com/%s/%s_%s_%s.jpg",
		photo.Farm, photo.Server, photo.ID, photo.Secret, size)
}

// GetUserID gets the authenticated user's NSID using flickr.test.login
func (api *FlickrAPI) GetUserID(ctx context.Context) (string, error) {
	params := url.Values{}
	params.Set("method", "flickr.test.login")
	params.Set("format", "json")
	params.Set("nojsoncallback", "1")
	
	resp, err := api.makeAPICall(ctx, "GET", params)
	if err != nil {
		return "", fmt.Errorf("failed to call test.login: %w", err)
	}
	
	var result struct {
		User struct {
			ID       string `json:"id"`
			Username struct {
				Content string `json:"_content"`
			} `json:"username"`
		} `json:"user"`
		Stat    string `json:"stat"`
		Message string `json:"message,omitempty"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse test.login response: %w", err)
	}
	
	if result.Stat != "ok" {
		return "", fmt.Errorf("test.login failed: %s", result.Message)
	}
	
	if result.User.ID == "" {
		return "", fmt.Errorf("test.login returned empty user ID")
	}
	
	return result.User.ID, nil
}
