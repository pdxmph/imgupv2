package backends

import (
	"context"
	"fmt"
	"net/http"
	
	"github.com/dghubble/oauth1"
)

// SmugMugAuth handles SmugMug OAuth authentication
type SmugMugAuth struct {
	ConsumerKey    string
	ConsumerSecret string
}

// NewSmugMugAuth creates a new SmugMug authenticator
func NewSmugMugAuth(consumerKey, consumerSecret string) *SmugMugAuth {
	return &SmugMugAuth{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
	}
}

// Authenticate performs the OAuth flow and returns the access token
func (a *SmugMugAuth) Authenticate(ctx context.Context) (*oauth1.Token, string, error) {
	// Create OAuth1 config
	config := oauth1.Config{
		ConsumerKey:    a.ConsumerKey,
		ConsumerSecret: a.ConsumerSecret,
		CallbackURL:    "http://localhost:8749/callback",
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: "https://api.smugmug.com/services/oauth/1.0a/getRequestToken",
			AuthorizeURL:    "https://api.smugmug.com/services/oauth/1.0a/authorize",
			AccessTokenURL:  "https://api.smugmug.com/services/oauth/1.0a/getAccessToken",
		},
	}
	
	// Get request token
	fmt.Println("Getting request token from SmugMug...")
	requestToken, requestSecret, err := config.RequestToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get request token: %w", err)
	}
	
	// Get authorization URL
	authorizationURL, err := config.AuthorizationURL(requestToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get authorization URL: %w", err)
	}
	
	// Add permissions for full access
	authorizationURL.RawQuery = authorizationURL.RawQuery + "&Access=Full&Permissions=Modify"
	
	fmt.Printf("\nPlease visit this URL to authorize the app:\n%s\n\n", authorizationURL.String())
	
	// Start callback server
	verifier := make(chan string)
	server := &http.Server{Addr: ":8749"}
	
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body><h1>SmugMug authorization successful!</h1><p>You can close this window.</p></body></html>")
		verifier <- r.URL.Query().Get("oauth_verifier")
	})
	
	go func() {
		server.ListenAndServe()
	}()
	
	// Wait for callback
	fmt.Println("Waiting for authorization...")
	oauthVerifier := <-verifier
	
	// Shutdown server
	server.Shutdown(ctx)
	
	// Exchange for access token
	fmt.Println("Getting access token...")
	accessToken, accessSecret, err := config.AccessToken(requestToken, requestSecret, oauthVerifier)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get access token: %w", err)
	}
	
	token := &oauth1.Token{
		Token:       accessToken,
		TokenSecret: accessSecret,
	}
	
	// After successful auth, let's get the list of albums for selection
	api := &SmugMugAPI{
		SmugMugUploader: NewSmugMugUploader(
			a.ConsumerKey,
			a.ConsumerSecret,
			accessToken,
			accessSecret,
			"", // No album ID yet
		),
	}
	
	fmt.Println("\nFetching your SmugMug albums...")
	albums, err := api.ListAlbums(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list albums: %w", err)
	}
	
	if len(albums) == 0 {
		return nil, "", fmt.Errorf("no albums found in your SmugMug account")
	}
	
	// Display albums for selection
	fmt.Println("\nAvailable albums:")
	for i, album := range albums {
		desc := ""
		if album.Description != "" {
			desc = fmt.Sprintf(" - %s", album.Description)
		}
		fmt.Printf("%d. %s%s (%d images)\n", i+1, album.Name, desc, album.ImageCount)
	}
	
	// Get user selection
	var selection int
	for {
		fmt.Print("\nSelect an album (enter number): ")
		_, err := fmt.Scanln(&selection)
		if err != nil || selection < 1 || selection > len(albums) {
			fmt.Println("Invalid selection. Please try again.")
			continue
		}
		break
	}
	
	selectedAlbum := albums[selection-1]
	fmt.Printf("\nSelected album: %s\n", selectedAlbum.Name)
	
	return token, selectedAlbum.AlbumKey, nil
}
