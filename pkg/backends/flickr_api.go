package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	
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
