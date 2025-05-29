package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// OAuth1Config holds the configuration for OAuth 1.0a
type OAuth1Config struct {
	ConsumerKey    string
	ConsumerSecret string
	RequestURL     string
	AuthorizeURL   string
	AccessURL      string
	CallbackURL    string
}

// OAuth1Token represents an OAuth 1.0a token
type OAuth1Token struct {
	Token       string
	Secret      string
	Verifier    string // Only used during auth flow
}

// OAuth1Client handles OAuth 1.0a authentication
type OAuth1Client struct {
	Config OAuth1Config
}

// NewOAuth1Client creates a new OAuth 1.0a client
func NewOAuth1Client(config OAuth1Config) *OAuth1Client {
	return &OAuth1Client{Config: config}
}

// GetRequestToken initiates the OAuth flow
func (c *OAuth1Client) GetRequestToken(ctx context.Context) (*OAuth1Token, error) {
	params := c.baseParams()
	params["oauth_callback"] = c.Config.CallbackURL
	
	signature := c.signature("GET", c.Config.RequestURL, params, "")
	params["oauth_signature"] = signature
	
	// Make HTTP request
	resp, err := c.makeRequest(ctx, "GET", c.Config.RequestURL, params)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	values, err := url.ParseQuery(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	token := values.Get("oauth_token")
	secret := values.Get("oauth_token_secret")
	
	if token == "" || secret == "" {
		return nil, fmt.Errorf("missing token in response: %s", resp)
	}
	
	return &OAuth1Token{
		Token:  token,
		Secret: secret,
	}, nil
}

// GetAccessToken exchanges the request token for an access token
func (c *OAuth1Client) GetAccessToken(ctx context.Context, requestToken *OAuth1Token) (*OAuth1Token, error) {
	params := c.baseParams()
	params["oauth_token"] = requestToken.Token
	params["oauth_verifier"] = requestToken.Verifier
	
	signature := c.signature("GET", c.Config.AccessURL, params, requestToken.Secret)
	params["oauth_signature"] = signature
	
	// Make HTTP request
	resp, err := c.makeRequest(ctx, "GET", c.Config.AccessURL, params)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	values, err := url.ParseQuery(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	token := values.Get("oauth_token")
	secret := values.Get("oauth_token_secret")
	
	if token == "" || secret == "" {
		return nil, fmt.Errorf("missing token in response: %s", resp)
	}
	
	return &OAuth1Token{
		Token:  token,
		Secret: secret,
	}, nil
}

// AuthorizeURL returns the URL where the user should authorize the app
func (c *OAuth1Client) AuthorizeURL(requestToken *OAuth1Token) string {
	return fmt.Sprintf("%s?oauth_token=%s", c.Config.AuthorizeURL, requestToken.Token)
}

// baseParams returns the base OAuth parameters
func (c *OAuth1Client) baseParams() map[string]string {
	return map[string]string{
		"oauth_consumer_key":     c.Config.ConsumerKey,
		"oauth_nonce":           c.nonce(),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":       fmt.Sprintf("%d", time.Now().Unix()),
		"oauth_version":         "1.0",
	}
}

// nonce generates a random nonce
func (c *OAuth1Client) nonce() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// signature calculates the OAuth signature
func (c *OAuth1Client) signature(method, baseURL string, params map[string]string, tokenSecret string) string {
	// Sort parameters
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Build parameter string
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", 
			url.QueryEscape(k), 
			url.QueryEscape(params[k])))
	}
	paramString := strings.Join(parts, "&")
	
	// Build signature base string
	signatureBase := fmt.Sprintf("%s&%s&%s",
		method,
		url.QueryEscape(baseURL),
		url.QueryEscape(paramString))
	
	// Calculate HMAC-SHA1
	key := fmt.Sprintf("%s&%s", 
		url.QueryEscape(c.Config.ConsumerSecret),
		url.QueryEscape(tokenSecret))
	
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(signatureBase))
	
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// makeRequest makes an OAuth-signed HTTP request
func (c *OAuth1Client) makeRequest(ctx context.Context, method, url string, params map[string]string) (string, error) {
	// Build authorization header
	var authParts []string
	for k, v := range params {
		if strings.HasPrefix(k, "oauth_") {
			authParts = append(authParts, fmt.Sprintf(`%s="%s"`, k, v))
		}
	}
	authHeader := "OAuth " + strings.Join(authParts, ", ")
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", authHeader)
	
	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}
	
	return string(body), nil
}
