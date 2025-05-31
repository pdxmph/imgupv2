package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/pdxmph/imgupv2/pkg/backends"
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/metadata"
	"github.com/pdxmph/imgupv2/pkg/services/mastodon"
	"github.com/pdxmph/imgupv2/pkg/templates"
)

var (
	// Version information (set by ldflags during build)
	version = "dev"
	commit  = "unknown"
	date    = "unknown"

	// Upload flags
	title        string
	description  string
	altText      string
	outputFormat string
	isPrivate    bool
	tags         []string
	
	// Mastodon flags
	postToMastodon   bool
	post             string
	visibility       string
)

func main() {
	var showVersion bool
	
	rootCmd := &cobra.Command{
		Use:     "imgup",
		Short:   "Fast image upload tool",
		Long: `imgupv2 - A fast command-line tool for uploading images to Flickr
with support for metadata embedding and multiple output formats.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Printf("imgupv2 version %s\n", version)
				if version != "dev" {
					fmt.Printf("  commit: %s\n", commit)
					fmt.Printf("  built:  %s\n", date)
				}
				return nil
			}
			// Show help if no subcommand is provided
			return cmd.Help()
		},
	}
	
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "version for imgup")

	// Auth command
	authCmd := &cobra.Command{
		Use:   "auth [service]",
		Short: "Authenticate with a photo service",
		Args:  cobra.ExactArgs(1),
		Run:   authCommand,
	}

	// Upload command
	uploadCmd := &cobra.Command{
		Use:   "upload [image]",
		Short: "Upload an image",
		Args:  cobra.ExactArgs(1),
		Run:   uploadCommand,
	}

	// Add upload flags
	uploadCmd.Flags().StringVar(&title, "title", "", "Photo title")
	uploadCmd.Flags().StringVar(&description, "description", "", "Photo description")
	uploadCmd.Flags().StringVar(&altText, "alt", "", "Alt text for accessibility")
	uploadCmd.Flags().StringVar(&outputFormat, "format", "url", "Output format: url, markdown, html, json")
	uploadCmd.Flags().BoolVar(&isPrivate, "private", false, "Make the photo private")
	uploadCmd.Flags().StringSliceVar(&tags, "tags", nil, "Comma-separated tags")
	
	// Add Mastodon flags
	uploadCmd.Flags().BoolVar(&postToMastodon, "mastodon", false, "Post to Mastodon after upload")
	uploadCmd.Flags().StringVar(&post, "post", "", "Text for Mastodon post")
	uploadCmd.Flags().StringVar(&visibility, "visibility", "public", "Mastodon post visibility: public, unlisted, followers, direct")

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show configuration",
		Run:   configShowCommand,
	}

	configSetCmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		Run:   configSetCommand,
	}

	configCmd.AddCommand(configShowCmd, configSetCmd)

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("imgupv2 version %s\n", version)
			if version != "dev" {
				fmt.Printf("  commit: %s\n", commit)
				fmt.Printf("  built:  %s\n", date)
			}
		},
	}

	// Add commands to root
	rootCmd.AddCommand(authCmd, uploadCmd, configCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func authCommand(cmd *cobra.Command, args []string) {
	service := args[0]
	switch service {
	case "flickr":
		if err := authFlickr(); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
	case "mastodon":
		if err := authMastodon(); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown service: %s\n", service)
		os.Exit(1)
	}
}

func authFlickr() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we have API credentials
	if cfg.Flickr.ConsumerKey == "" || cfg.Flickr.ConsumerSecret == "" {
		fmt.Println("Flickr API credentials not found.")
		fmt.Println("\nTo get your API credentials:")
		fmt.Println("1. Go to https://www.flickr.com/services/apps/create/")
		fmt.Println("2. Create a new app (non-commercial)")
		fmt.Println("3. Note your Key and Secret")
		fmt.Println("\nThen run:")
		fmt.Println("  imgup config set flickr.key YOUR_KEY")
		fmt.Println("  imgup config set flickr.secret YOUR_SECRET")
		return fmt.Errorf("missing API credentials")
	}

	// Create authenticator
	auth := backends.NewFlickrAuth(cfg.Flickr.ConsumerKey, cfg.Flickr.ConsumerSecret)

	// Perform OAuth flow
	ctx := context.Background()
	token, err := auth.Authenticate(ctx)
	if err != nil {
		return err
	}

	// Save tokens
	cfg.Flickr.AccessToken = token.Token
	cfg.Flickr.AccessSecret = token.TokenSecret

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Tokens saved to config!")
	return nil
}

func authMastodon() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we have instance URL
	if cfg.Mastodon.InstanceURL == "" {
		fmt.Println("Mastodon instance URL not found.")
		fmt.Println("\nFirst, set your Mastodon instance:")
		fmt.Println("  imgup config set mastodon.instance https://mastodon.social")
		fmt.Println("\nThen run 'imgup auth mastodon' again.")
		return fmt.Errorf("missing instance URL")
	}

	// Step 1: Register the app if we don't have client credentials
	if cfg.Mastodon.ClientID == "" || cfg.Mastodon.ClientSecret == "" {
		fmt.Println("Registering app with Mastodon instance...")
		
		// Register app
		appData := url.Values{}
		appData.Set("client_name", "imgupv2")
		appData.Set("redirect_uris", "http://localhost:8080/callback")
		appData.Set("scopes", "read write:media write:statuses")
		appData.Set("website", "https://github.com/pdxmph/imgupv2")
		
		resp, err := http.PostForm(cfg.Mastodon.InstanceURL+"/api/v1/apps", appData)
		if err != nil {
			return fmt.Errorf("failed to register app: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to register app: status %d", resp.StatusCode)
		}
		
		var appResp struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&appResp); err != nil {
			return fmt.Errorf("failed to decode app response: %w", err)
		}
		
		cfg.Mastodon.ClientID = appResp.ClientID
		cfg.Mastodon.ClientSecret = appResp.ClientSecret
		
		// Save the client credentials
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save client credentials: %w", err)
		}
		
		fmt.Println("App registered successfully!")
	}
	
	// Step 2: OAuth 2.0 authorization flow
	authURL := fmt.Sprintf("%s/oauth/authorize?client_id=%s&scope=read%%20write:media%%20write:statuses&redirect_uri=%s&response_type=code",
		cfg.Mastodon.InstanceURL,
		cfg.Mastodon.ClientID,
		url.QueryEscape("http://localhost:8080/callback"))
	
	fmt.Printf("\nPlease visit this URL to authorize imgupv2:\n%s\n\n", authURL)
	
	// Start local server to receive callback
	authCode := make(chan string, 1)
	errChan := make(chan error, 1)
	
	srv := &http.Server{Addr: ":8080"}
	
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			fmt.Fprintf(w, "Error: No authorization code received")
			return
		}
		
		authCode <- code
		fmt.Fprintf(w, "Authorization successful! You can close this window and return to the terminal.")
	})
	
	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	
	// Wait for auth code or error
	var code string
	select {
	case code = <-authCode:
		fmt.Println("Authorization code received!")
	case err := <-errChan:
		return fmt.Errorf("authorization failed: %w", err)
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authorization timeout")
	}
	
	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	
	// Step 3: Exchange code for access token
	tokenData := url.Values{}
	tokenData.Set("client_id", cfg.Mastodon.ClientID)
	tokenData.Set("client_secret", cfg.Mastodon.ClientSecret)
	tokenData.Set("code", code)
	tokenData.Set("grant_type", "authorization_code")
	tokenData.Set("redirect_uri", "http://localhost:8080/callback")
	tokenData.Set("scope", "read write:media write:statuses")
	
	resp, err := http.PostForm(cfg.Mastodon.InstanceURL+"/oauth/token", tokenData)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get access token: status %d", resp.StatusCode)
	}
	
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		CreatedAt   int64  `json:"created_at"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}
	
	// Save the access token
	cfg.Mastodon.AccessToken = tokenResp.AccessToken
	
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save access token: %w", err)
	}
	
	fmt.Println("\nAuthentication successful! Access token saved.")
	
	// Verify the token by getting account info
	verifyReq, err := http.NewRequest("GET", cfg.Mastodon.InstanceURL+"/api/v1/accounts/verify_credentials", nil)
	if err != nil {
		return fmt.Errorf("failed to create verify request: %w", err)
	}
	
	verifyReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	
	client := &http.Client{}
	verifyResp, err := client.Do(verifyReq)
	if err != nil {
		return fmt.Errorf("failed to verify credentials: %w", err)
	}
	defer verifyResp.Body.Close()
	
	if verifyResp.StatusCode == http.StatusOK {
		var account struct {
			Username string `json:"username"`
			Acct     string `json:"acct"`
		}
		if err := json.NewDecoder(verifyResp.Body).Decode(&account); err == nil {
			fmt.Printf("Authenticated as @%s\n", account.Acct)
		}
	}
	
	return nil
}

func uploadCommand(cmd *cobra.Command, args []string) {
	imagePath := args[0]

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File not found: %s\n", imagePath)
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if authenticated
	if cfg.Flickr.AccessToken == "" || cfg.Flickr.AccessSecret == "" {
		fmt.Fprintf(os.Stderr, "Error: Not authenticated. Run 'imgup auth flickr' first.\n")
		os.Exit(1)
	}

	// Create uploader
	uploader := backends.NewFlickrUploader(
		cfg.Flickr.ConsumerKey,
		cfg.Flickr.ConsumerSecret,
		cfg.Flickr.AccessToken,
		cfg.Flickr.AccessSecret,
	)

	// Check if we should embed metadata
	uploadPath := imagePath
	var tempFile string

	// Save original values for output formatting
	originalTitle := title
	originalDescription := description

	if (title != "" || description != "" || len(tags) > 0) && metadata.HasExiftool() {
		// Try to embed metadata into the image
		writer, err := metadata.NewWriter()
		if err == nil {
			fmt.Println("Embedding metadata into image...")
			tempPath, err := writer.CopyWithMetadata(imagePath, title, description, tags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not embed metadata: %v\n", err)
				fmt.Fprintf(os.Stderr, "Uploading without embedded metadata...\n")
			} else {
				uploadPath = tempPath
				tempFile = tempPath
				// Clear title/description so we don't try to send them as form fields
				title = ""
				description = ""
			}
		}
	}

	// Clean up temp file when done
	if tempFile != "" {
		defer os.Remove(tempFile)
	}

	// Upload
	fmt.Printf("Uploading %s to Flickr...\n", imagePath)
	ctx := context.Background()
	result, err := uploader.Upload(ctx, uploadPath, title, description, isPrivate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		os.Exit(1)
	}

	// Output result using templates
	template, exists := cfg.Templates[outputFormat]
	if !exists {
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", outputFormat)
		fmt.Fprintf(os.Stderr, "Available formats: ")
		var formats []string
		for k := range cfg.Templates {
			formats = append(formats, k)
		}
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(formats, ", "))
		os.Exit(1)
	}

	// Build template variables
	vars := templates.BuildVariables(result, imagePath, originalTitle, originalDescription, altText, tags)

	// Process and output
	output := templates.Process(template, vars)
	fmt.Println(output)

	// Post to Mastodon if requested
	if postToMastodon {
		if err := postToMastodonService(cfg, result, originalTitle, originalDescription, altText, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Mastodon post failed: %v\n", err)
			// Don't exit - the upload was successful
		}
	}

	// Show accessibility tip for markdown without explicit alt text
	if altText == "" && outputFormat == "markdown" {
		fmt.Fprintf(os.Stderr, "\nTip: Use --alt to provide descriptive alt text for better accessibility.\n")
		fmt.Fprintf(os.Stderr, "Example: --alt \"Person standing on mountain peak at sunset\"\n")
	}
}

func configShowCommand(cmd *cobra.Command, args []string) {
	if err := configShow(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func configSetCommand(cmd *cobra.Command, args []string) {
	if err := configSet(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func configShow() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Configuration:")
	fmt.Printf("  Flickr:\n")
	fmt.Printf("    Consumer Key: %s\n", maskString(cfg.Flickr.ConsumerKey))
	fmt.Printf("    Consumer Secret: %s\n", maskString(cfg.Flickr.ConsumerSecret))
	fmt.Printf("    Access Token: %s\n", maskString(cfg.Flickr.AccessToken))
	fmt.Printf("    Access Secret: %s\n", maskString(cfg.Flickr.AccessSecret))

	fmt.Printf("\n  Mastodon:\n")
	fmt.Printf("    Instance URL: %s\n", cfg.Mastodon.InstanceURL)
	fmt.Printf("    Client ID: %s\n", maskString(cfg.Mastodon.ClientID))
	fmt.Printf("    Client Secret: %s\n", maskString(cfg.Mastodon.ClientSecret))
	fmt.Printf("    Access Token: %s\n", maskString(cfg.Mastodon.AccessToken))

	fmt.Printf("\n  Templates:\n")
	for name, template := range cfg.Templates {
		// Truncate long templates for display
		display := template
		if len(display) > 60 {
			display = display[:57] + "..."
		}
		fmt.Printf("    %s: %s\n", name, display)
	}

	return nil
}

func configSet(key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch {
	case key == "flickr.key":
		cfg.Flickr.ConsumerKey = value
	case key == "flickr.secret":
		cfg.Flickr.ConsumerSecret = value
	case key == "mastodon.instance":
		cfg.Mastodon.InstanceURL = value
	case key == "mastodon.client_id":
		cfg.Mastodon.ClientID = value
	case key == "mastodon.client_secret":
		cfg.Mastodon.ClientSecret = value
	case strings.HasPrefix(key, "template."):
		// Handle template settings
		templateName := strings.TrimPrefix(key, "template.")
		if cfg.Templates == nil {
			cfg.Templates = make(map[string]string)
		}
		cfg.Templates[templateName] = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Set %s\n", key)
	return nil
}

func maskString(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

func postToMastodonService(cfg *config.Config, uploadResult *backends.UploadResult, photoTitle string, photoDescription string, altText string, photoTags []string) error {
	// Check if Mastodon is configured
	if cfg.Mastodon.AccessToken == "" {
		return fmt.Errorf("not authenticated with Mastodon. Run 'imgup auth mastodon' first")
	}
	
	// Create Mastodon client
	client := mastodon.NewClient(
		cfg.Mastodon.InstanceURL,
		cfg.Mastodon.ClientID,
		cfg.Mastodon.ClientSecret,
		cfg.Mastodon.AccessToken,
	)
	
	// Use post text if provided, otherwise use title
	statusText := post
	if statusText == "" && photoTitle != "" {
		statusText = photoTitle
	}
	
	// Add the photo URL to the post
	statusText += "\n\n" + uploadResult.URL
	
	// Get photo sizes from Flickr to find a good size for Mastodon
	api := backends.NewFlickrAPI(&cfg.Flickr)
	sizes, err := api.GetPhotoSizes(context.Background(), uploadResult.PhotoID)
	if err != nil {
		return fmt.Errorf("failed to get photo sizes from Flickr: %w", err)
	}
	
	// Find a good size for Mastodon (prefer Large or Medium)
	var imageURL string
	for _, size := range sizes {
		// Prioritize these sizes for Mastodon
		if size.Label == "Large" || size.Label == "Large 1024" {
			imageURL = size.Source
			break
		} else if size.Label == "Medium" || size.Label == "Medium 800" {
			imageURL = size.Source
			// Keep looking for Large
		}
	}
	
	// Fallback to whatever we have
	if imageURL == "" && len(sizes) > 0 {
		// Use a middle size if available
		if len(sizes) > 2 {
			imageURL = sizes[len(sizes)/2].Source
		} else {
			imageURL = sizes[0].Source
		}
	}
	
	if imageURL == "" {
		return fmt.Errorf("no suitable image size found from Flickr")
	}
	
	// Determine alt text: use explicit alt text, fall back to description
	mastodonAltText := altText
	if mastodonAltText == "" && photoDescription != "" {
		mastodonAltText = photoDescription
	}
	
	// Upload the resized image from Flickr to Mastodon
	fmt.Println("Downloading resized image from Flickr...")
	mediaID, err := client.UploadMediaFromURL(imageURL, mastodonAltText)
	if err != nil {
		return fmt.Errorf("failed to upload media: %w", err)
	}
	
	// Post the status
	fmt.Printf("Posting to Mastodon (visibility: %s)...\n", visibility)
	if err := client.PostStatus(statusText, []string{mediaID}, visibility, photoTags); err != nil {
		return fmt.Errorf("failed to post status: %w", err)
	}
	
	fmt.Println("Posted to Mastodon!")
	return nil
}
