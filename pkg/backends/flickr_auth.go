package backends

import (
	"context"
	"fmt"
	"time"
	
	"github.com/mph/imgupv2/pkg/oauth"
)

const (
	flickrRequestURL   = "https://www.flickr.com/services/oauth/request_token"
	flickrAuthorizeURL = "https://www.flickr.com/services/oauth/authorize"
	flickrAccessURL    = "https://www.flickr.com/services/oauth/access_token"
)

// FlickrAuth handles Flickr OAuth authentication
type FlickrAuth struct {
	ConsumerKey    string
	ConsumerSecret string
}

// NewFlickrAuth creates a new Flickr authenticator
func NewFlickrAuth(key, secret string) *FlickrAuth {
	return &FlickrAuth{
		ConsumerKey:    key,
		ConsumerSecret: secret,
	}
}

// Authenticate performs the OAuth flow and returns access tokens
func (a *FlickrAuth) Authenticate(ctx context.Context) (*oauth.OAuth1Token, error) {
	// Start callback server
	callbackServer := oauth.NewCallbackServer(8749, "/callback")
	if err := callbackServer.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	
	// Create OAuth client
	client := oauth.NewOAuth1Client(oauth.OAuth1Config{
		ConsumerKey:    a.ConsumerKey,
		ConsumerSecret: a.ConsumerSecret,
		RequestURL:     flickrRequestURL,
		AuthorizeURL:   flickrAuthorizeURL,
		AccessURL:      flickrAccessURL,
		CallbackURL:    callbackServer.URL(),
	})
	
	// Get request token
	fmt.Println("Getting request token from Flickr...")
	requestToken, err := client.GetRequestToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get request token: %w", err)
	}
	
	// Direct user to authorize
	authURL := client.AuthorizeURL(requestToken)
	fmt.Printf("\nPlease open this URL in your browser:\n%s\n\n", authURL)
	fmt.Println("Waiting for authorization...")
	
	// Wait for callback
	result, err := callbackServer.Wait(5 * time.Minute)
	if err != nil {
		return nil, fmt.Errorf("callback failed: %w", err)
	}
	
	// Update request token with verifier
	requestToken.Verifier = result.Verifier
	
	// Exchange for access token
	fmt.Println("Getting access token...")
	accessToken, err := client.GetAccessToken(ctx, requestToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}
	
	fmt.Println("Authentication successful!")
	return accessToken, nil
}
