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
)

// SmugMugAPI handles SmugMug API calls
type SmugMugAPI struct {
	*SmugMugUploader
}

// Album represents a SmugMug album
type Album struct {
	AlbumKey    string `json:"AlbumKey"`
	Name        string `json:"Name"`
	Description string `json:"Description,omitempty"`
	URLPath     string `json:"UrlPath"`
	URI         string `json:"Uri"`
	WebURI      string `json:"WebUri"`
	NodeID      string `json:"NodeId"`
	ImageCount  int    `json:"ImageCount"`
}

// Image represents a SmugMug image
type Image struct {
	ImageKey string `json:"ImageKey"`
	URI      string `json:"Uri"`
	WebURI   string `json:"WebUri"`
	Title    string `json:"Title,omitempty"`
	Caption  string `json:"Caption,omitempty"`
}

// AlbumImage represents an image within an album context
type AlbumImage struct {
	ImageKey string `json:"ImageKey"`
	URI      string `json:"Uri"`
	WebURI   string `json:"WebUri"`
	Title    string `json:"Title,omitempty"`
	Caption  string `json:"Caption,omitempty"`
	Image    struct {
		WebURI string `json:"WebUri"`
	} `json:"Uris,omitempty"`
}

// AlbumsResponse represents the response from the albums endpoint
type AlbumsResponse struct {
	Response struct {
		Album      []Album `json:"Album"`
		AlbumCount int     `json:"AlbumCount"`
		Pages      struct {
			Total        int    `json:"Total"`
			Start        int    `json:"Start"`
			Count        int    `json:"Count"`
			RequestedCount int  `json:"RequestedCount"`
			NextPage     string `json:"NextPage,omitempty"`
		} `json:"Pages"`
	} `json:"Response"`
}

// UserResponse represents the response from the authuser endpoint
type UserResponse struct {
	Response struct {
		User struct {
			NickName string `json:"NickName"`
			Name     string `json:"Name"`
			URI      string `json:"Uri"`
			WebURI   string `json:"WebUri"`
			Uris     struct {
				UserAlbums struct {
					URI string `json:"Uri"`
				} `json:"UserAlbums"`
			} `json:"Uris"`
		} `json:"User"`
	} `json:"Response"`
}

// NewSmugMugAPI creates a new SmugMug API client
func NewSmugMugAPI(cfg *config.SmugMugConfig) *SmugMugAPI {
	return &SmugMugAPI{
		SmugMugUploader: NewSmugMugUploader(
			cfg.ConsumerKey,
			cfg.ConsumerSecret,
			cfg.AccessToken,
			cfg.AccessSecret,
			cfg.AlbumID,
		),
	}
}

// GetAuthenticatedUser gets information about the authenticated user
func (api *SmugMugAPI) GetAuthenticatedUser(ctx context.Context) (*UserResponse, error) {
	endpoint := smugmugAPIURL + "/api/v2!authuser"
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    api.ConsumerKey,
		ConsumerSecret: api.ConsumerSecret,
	}
	
	token := oauth1.NewToken(api.AccessToken, api.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &result, nil
}

// ListAlbums gets all albums for the authenticated user with pagination
func (api *SmugMugAPI) ListAlbums(ctx context.Context) ([]Album, error) {
	// First get the authenticated user to get the albums URI
	userInfo, err := api.GetAuthenticatedUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	
	albumsURI := userInfo.Response.User.Uris.UserAlbums.URI
	if albumsURI == "" {
		// Fallback to constructing it
		albumsURI = fmt.Sprintf("/api/v2/user/%s!albums", userInfo.Response.User.NickName)
	}
	
	var allAlbums []Album
	nextPage := smugmugAPIURL + albumsURI + "?count=100" // Get 100 at a time
	
	for nextPage != "" {
		albums, next, err := api.fetchAlbumsPage(ctx, nextPage)
		if err != nil {
			return nil, err
		}
		
		allAlbums = append(allAlbums, albums...)
		
		// Check if there's a next page
		if next != "" && !strings.HasPrefix(next, "http") {
			// If it's a relative URL, make it absolute
			nextPage = smugmugAPIURL + next
		} else {
			nextPage = next
		}
	}
	
	return allAlbums, nil
}

// fetchAlbumsPage fetches a single page of albums
func (api *SmugMugAPI) fetchAlbumsPage(ctx context.Context, pageURL string) ([]Album, string, error) {
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    api.ConsumerKey,
		ConsumerSecret: api.ConsumerSecret,
	}
	
	token := oauth1.NewToken(api.AccessToken, api.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list albums: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result AlbumsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}
	
	return result.Response.Album, result.Response.Pages.NextPage, nil
}

// GetAlbum gets details for a specific album
func (api *SmugMugAPI) GetAlbum(ctx context.Context, albumKey string) (*Album, error) {
	endpoint := fmt.Sprintf("%s/api/v2/album/%s", smugmugAPIURL, albumKey)
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    api.ConsumerKey,
		ConsumerSecret: api.ConsumerSecret,
	}
	
	token := oauth1.NewToken(api.AccessToken, api.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get album: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Response struct {
			Album Album `json:"Album"`
		} `json:"Response"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &result.Response.Album, nil
}

// GetImageSizes gets the available sizes for an uploaded image
func (api *SmugMugAPI) GetImageSizes(ctx context.Context, imageURI string) (map[string]interface{}, error) {
	// For AlbumImage URIs, we need to expand the Image to get sizes
	// Try with Image expansion first
	endpoint := smugmugAPIURL + imageURI + "?_expand=Image.ImageSizes,ImageSizes,ArchivedUri,ImageDownloadUrl"
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    api.ConsumerKey,
		ConsumerSecret: api.ConsumerSecret,
	}
	
	token := oauth1.NewToken(api.AccessToken, api.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get image sizes: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		// Try the !sizedetails endpoint
		endpoint = smugmugAPIURL + imageURI + "!sizedetails"
		req, err = http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		req.Header.Set("Accept", "application/json")
		
		resp, err = httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get image with sizedetails: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: GetImageSizes response has keys: %v\n", getMapKeys(result))
		if respData, ok := result["Response"].(map[string]interface{}); ok {
			fmt.Fprintf(os.Stderr, "DEBUG: Response object has keys: %v\n", getMapKeys(respData))
			
			// If there's an AlbumImage, show its structure
			if albumImage, ok := respData["AlbumImage"].(map[string]interface{}); ok {
				fmt.Fprintf(os.Stderr, "DEBUG: AlbumImage has keys: %v\n", getMapKeys(albumImage))
				
				// Check for nested Image object
				if img, ok := albumImage["Image"].(map[string]interface{}); ok {
					fmt.Fprintf(os.Stderr, "DEBUG: AlbumImage.Image has keys: %v\n", getMapKeys(img))
					
					// Check for ImageSizes in the Image
					if sizes, ok := img["ImageSizes"].(map[string]interface{}); ok {
						fmt.Fprintf(os.Stderr, "DEBUG: AlbumImage.Image.ImageSizes has keys: %v\n", getMapKeys(sizes))
					}
				}
				
				// Check for Uris
				if uris, ok := albumImage["Uris"].(map[string]interface{}); ok {
					fmt.Fprintf(os.Stderr, "DEBUG: AlbumImage.Uris has keys: %v\n", getMapKeys(uris))
				}
			}
			
			// Check for Image object
			if img, ok := respData["Image"].(map[string]interface{}); ok {
				fmt.Fprintf(os.Stderr, "DEBUG: Image has keys: %v\n", getMapKeys(img))
			}
		}
	}
	
	return result, nil
}

// GetImage gets details for a specific image
func (api *SmugMugAPI) GetImage(ctx context.Context, imageURI string) (*Image, error) {
	endpoint := smugmugAPIURL + imageURI
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    api.ConsumerKey,
		ConsumerSecret: api.ConsumerSecret,
	}
	
	token := oauth1.NewToken(api.AccessToken, api.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Response struct {
			Image Image `json:"Image"`
		} `json:"Response"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &result.Response.Image, nil
}

// GetAlbumImage gets details for an image in album context
func (api *SmugMugAPI) GetAlbumImage(ctx context.Context, albumImageURI string) (map[string]interface{}, error) {
	endpoint := smugmugAPIURL + albumImageURI
	
	// Create OAuth1 config and client
	config := oauth1.Config{
		ConsumerKey:    api.ConsumerKey,
		ConsumerSecret: api.ConsumerSecret,
	}
	
	token := oauth1.NewToken(api.AccessToken, api.AccessSecret)
	httpClient := config.Client(ctx, token)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get album image: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return result, nil
}


// getMapKeys helper function
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
