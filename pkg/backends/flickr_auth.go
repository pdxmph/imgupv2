package backends

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	
	"github.com/dghubble/oauth1"
)

// FlickrAuth handles Flickr OAuth authentication
type FlickrAuth struct {
	ConsumerKey    string
	ConsumerSecret string
}

// NewFlickrAuth creates a new Flickr authenticator
func NewFlickrAuth(consumerKey, consumerSecret string) *FlickrAuth {
	return &FlickrAuth{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
	}
}

// Authenticate performs the OAuth flow and returns the access token
func (a *FlickrAuth) Authenticate(ctx context.Context) (*oauth1.Token, error) {
	// Create OAuth1 config
	config := oauth1.Config{
		ConsumerKey:    a.ConsumerKey,
		ConsumerSecret: a.ConsumerSecret,
		CallbackURL:    "http://localhost:8749/callback",
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: "https://www.flickr.com/services/oauth/request_token",
			AuthorizeURL:    "https://www.flickr.com/services/oauth/authorize",
			AccessTokenURL:  "https://www.flickr.com/services/oauth/access_token",
		},
	}
	
	// Get request token
	fmt.Println("Getting request token from Flickr...")
	requestToken, requestSecret, err := config.RequestToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get request token: %w", err)
	}
	
	// Get authorization URL
	authorizationURL, err := config.AuthorizationURL(requestToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization URL: %w", err)
	}
	
	// Add perms parameter for write access
	authorizationURL.RawQuery = authorizationURL.RawQuery + "&perms=write"
	
	fmt.Printf("\nPlease visit this URL to authorize the app:\n%s\n\n", authorizationURL.String())
	
	// Start callback server
	verifier := make(chan string)
	server := &http.Server{Addr: ":8749"}
	
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body><h1>Authorization successful!</h1><p>You can close this window.</p></body></html>")
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
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}
	
	fmt.Println("Authentication successful!")
	return oauth1.NewToken(accessToken, accessSecret), nil
}

// makeOAuth1Request makes an OAuth1-signed request (for testing)
func (a *FlickrAuth) makeOAuth1Request(ctx context.Context, method, urlStr string, params url.Values, token *oauth1.Token) (*http.Response, error) {
	config := oauth1.Config{
		ConsumerKey:    a.ConsumerKey,
		ConsumerSecret: a.ConsumerSecret,
	}
	
	// Create OAuth1 HTTP client
	var httpClient *http.Client
	if token != nil {
		httpClient = config.Client(ctx, token)
	} else {
		// For requests without a token (like request token)
		httpClient = &http.Client{}
	}
	
	// Create request
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	
	// Add query params if GET
	if method == "GET" && params != nil {
		req.URL.RawQuery = params.Encode()
	}
	
	return httpClient.Do(req)
}
