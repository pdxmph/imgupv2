package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/pdxmph/imgupv2/pkg/backends"
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/duplicate"
	"github.com/pdxmph/imgupv2/pkg/services/bluesky"
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
	service      string
	
	// Mastodon flags
	postToMastodon   bool
	post             string
	visibility       string
	
	// Bluesky flag (shares post with Mastodon)
	postToBluesky    bool
	
	// Testing flag
	dryRun           bool
	
	// Duplicate detection flags
	force            bool
	duplicateInfo    bool  // GUI flag to get duplicate status in JSON
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
	uploadCmd.Flags().StringVar(&service, "service", "", "Upload service: flickr or smugmug (auto-detected if not specified)")
	
	// Add social posting flags
	uploadCmd.Flags().BoolVar(&postToMastodon, "mastodon", false, "Post to Mastodon after upload")
	uploadCmd.Flags().BoolVar(&postToBluesky, "bluesky", false, "Post to Bluesky after upload")
	uploadCmd.Flags().StringVar(&post, "post", "", "Text for social media post (shared by Mastodon and Bluesky)")
	uploadCmd.Flags().StringVar(&visibility, "visibility", "public", "Mastodon post visibility: public, unlisted, followers, direct (Mastodon only)")
	uploadCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be posted without actually posting")
	
	// Add duplicate detection flags
	uploadCmd.Flags().BoolVar(&duplicateInfo, "duplicate-info", false, "Include duplicate status in JSON output (for GUI)")
	uploadCmd.Flags().BoolVar(&force, "force", false, "Force upload even if duplicate is found")

	// Check command
	checkCmd := &cobra.Command{
		Use:   "check [image]",
		Short: "Check if an image has already been uploaded",
		Args:  cobra.ExactArgs(1),
		Run:   checkCommand,
	}
	
	// Add check flags
	checkCmd.Flags().StringVar(&outputFormat, "format", "url", "Output format: url, markdown, html, json")
	checkCmd.Flags().StringVar(&service, "service", "", "Upload service: flickr or smugmug (auto-detected if not specified)")

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
	rootCmd.AddCommand(authCmd, uploadCmd, checkCmd, configCmd, versionCmd)

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
	case "bluesky":
		if err := authBluesky(); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
	case "smugmug":
		if err := authSmugMug(); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown service: %s\n", service)
		fmt.Fprintf(os.Stderr, "Available services: flickr, mastodon, bluesky, smugmug\n")
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

	// Get user ID using the new tokens
	api := backends.NewFlickrAPI(&cfg.Flickr)
	userID, err := api.GetUserID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}
	cfg.Flickr.UserID = userID

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Authentication successful! Tokens saved to config.\n")
	fmt.Printf("Authenticated as user: %s\n", userID)
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

func authSmugMug() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we have API credentials
	if cfg.SmugMug.ConsumerKey == "" || cfg.SmugMug.ConsumerSecret == "" {
		fmt.Println("SmugMug API credentials not found.")
		fmt.Println("\nTo get your API credentials:")
		fmt.Println("1. Go to https://api.smugmug.com/api/developer/apply")
		fmt.Println("2. Apply for an API key")
		fmt.Println("3. Note your Key and Secret")
		fmt.Println("\nThen run:")
		fmt.Println("  imgup config set smugmug.key YOUR_KEY")
		fmt.Println("  imgup config set smugmug.secret YOUR_SECRET")
		return fmt.Errorf("missing API credentials")
	}

	// Create authenticator
	auth := backends.NewSmugMugAuth(cfg.SmugMug.ConsumerKey, cfg.SmugMug.ConsumerSecret)

	// Perform OAuth flow with album selection
	ctx := context.Background()
	token, albumID, err := auth.Authenticate(ctx)
	if err != nil {
		return err
	}

	// Save tokens and album ID
	cfg.SmugMug.AccessToken = token.Token
	cfg.SmugMug.AccessSecret = token.TokenSecret
	cfg.SmugMug.AlbumID = albumID

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\nTokens and album selection saved to config!")
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
	
	// Variables to track upload results
	var photoID, photoURL, imageURL string
	var isDuplicate bool

	// Apply defaults from config if flags weren't explicitly set
	if !cmd.Flags().Changed("format") && cfg.Default.Format != "" {
		outputFormat = cfg.Default.Format
	}
	if !cmd.Flags().Changed("service") && cfg.Default.Service != "" {
		service = cfg.Default.Service
	}

	// Determine which service to use
	if service == "" {
		// Auto-detect based on which service is configured
		hasFlickr := cfg.Flickr.AccessToken != "" && cfg.Flickr.AccessSecret != ""
		hasSmugMug := cfg.SmugMug.AccessToken != "" && cfg.SmugMug.AccessSecret != ""
		
		if hasFlickr && hasSmugMug {
			// If default service is set, use it
			if cfg.Default.Service != "" {
				service = cfg.Default.Service
			} else {
				fmt.Fprintf(os.Stderr, "Error: Both Flickr and SmugMug are configured. Please specify --service or set a default:\n")
				fmt.Fprintf(os.Stderr, "  imgup config set default.service flickr\n")
				fmt.Fprintf(os.Stderr, "  imgup config set default.service smugmug\n")
				os.Exit(1)
			}
		} else if hasFlickr {
			service = "flickr"
		} else if hasSmugMug {
			service = "smugmug"
		} else {
			fmt.Fprintf(os.Stderr, "Error: Not authenticated. Run 'imgup auth flickr' or 'imgup auth smugmug' first.\n")
			os.Exit(1)
		}
	}
	
	// Validate service
	if service != "flickr" && service != "smugmug" {
		fmt.Fprintf(os.Stderr, "Error: Invalid service '%s'. Must be 'flickr' or 'smugmug'\n", service)
		os.Exit(1)
	}
	
	// Check authentication for specified service
	switch service {
	case "flickr":
		if cfg.Flickr.AccessToken == "" || cfg.Flickr.AccessSecret == "" {
			fmt.Fprintf(os.Stderr, "Error: Not authenticated with Flickr. Run 'imgup auth flickr' first.\n")
			os.Exit(1)
		}
	case "smugmug":
		if cfg.SmugMug.AccessToken == "" || cfg.SmugMug.AccessSecret == "" {
			fmt.Fprintf(os.Stderr, "Error: Not authenticated with SmugMug. Run 'imgup auth smugmug' first.\n")
			os.Exit(1)
		}
		if cfg.SmugMug.AlbumID == "" {
			fmt.Fprintf(os.Stderr, "Error: No SmugMug album selected. Run 'imgup auth smugmug' again.\n")
			os.Exit(1)
		}
	}


	// Always check for duplicates unless --force is specified or disabled in config
	if !force && cfg.IsDuplicateCheckEnabled() {
		var checker *duplicate.RemoteChecker
		
		switch service {
		case "flickr":
			checker, err = duplicate.SetupFlickrDuplicateChecker(&cfg.Flickr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up duplicate checker: %v\n", err)
				os.Exit(1)
			}
			
		case "smugmug":
			checker, err = duplicate.SetupSmugMugDuplicateChecker(&cfg.SmugMug)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up duplicate checker: %v\n", err)
				os.Exit(1)
			}
		}
		defer checker.Close()

		// Silent duplicate checking - no verbose messages
		ctx := context.Background()
		
		existingUpload, err := checker.Check(ctx, imagePath)
		if err != nil {
			// Only show error if it's significant
			if duplicateInfo {
				// For GUI mode, we need to handle errors differently
				fmt.Fprintf(os.Stderr, "Error checking for duplicate: %v\n", err)
			}
			// Continue with upload if duplicate check fails
		} else if existingUpload != nil {
			// Found a duplicate! Set our variables instead of exiting
			isDuplicate = true
			photoID = existingUpload.RemoteID
			photoURL = existingUpload.RemoteURL
			imageURL = existingUpload.ImageURL
		}
	}

	// Perform the upload based on service
	ctx := context.Background()
	
	// Calculate MD5 for the file (used for machine tags and caching)
	fileInfo, err := duplicate.GetFileInfo(imagePath)
	if err != nil {
		// Log warning but continue - upload can still work without MD5
		fmt.Fprintf(os.Stderr, "Warning: Failed to calculate file hash: %v\n", err)
	}
	
	// Only perform actual upload if not a duplicate
	if !isDuplicate {
		// Silent operation - no verbose messages
		
		switch service {
		case "flickr":
			// Add machine tag for duplicate detection if we have MD5
			uploadTags := tags
			if fileInfo != nil && fileInfo.MD5 != "" {
				machineTag := fmt.Sprintf("imgupv2:checksum=%s", fileInfo.MD5)
				uploadTags = append(uploadTags, machineTag)
			}
			
			uploader := backends.NewFlickrUploader(
				cfg.Flickr.ConsumerKey,
				cfg.Flickr.ConsumerSecret,
				cfg.Flickr.AccessToken,
				cfg.Flickr.AccessSecret,
			)
			result, err := uploader.Upload(ctx, imagePath, title, description, uploadTags, isPrivate)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
				os.Exit(1)
			}
			photoID = result.PhotoID
			photoURL = result.URL
			imageURL = result.ImageURL
			
		case "smugmug":
			uploader := backends.NewSmugMugUploader(
				cfg.SmugMug.ConsumerKey,
				cfg.SmugMug.ConsumerSecret,
				cfg.SmugMug.AccessToken,
				cfg.SmugMug.AccessSecret,
				cfg.SmugMug.AlbumID,
			)
			result, err := uploader.Upload(ctx, imagePath, title, description, tags, isPrivate)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
				os.Exit(1)
			}
			photoID = result.ImageKey
			photoURL = result.URL
			imageURL = result.ImageURL
		}

		// Always record successful upload in cache for future duplicate detection
		// Reuse the fileInfo we calculated earlier
		if fileInfo != nil {
			// Create cache and record the upload
			cache, err := duplicate.NewSQLiteCache(duplicate.DefaultCachePath())
			if err == nil {
				defer cache.Close()
				
				upload := &duplicate.Upload{
					FileMD5:    fileInfo.MD5,
					Service:    service,
					RemoteID:   photoID,
					RemoteURL:  photoURL,
					ImageURL:   imageURL,
					UploadTime: time.Now(),
					Filename:   filepath.Base(imagePath),
					FileSize:   fileInfo.Size,
				}
				
				if err := cache.Record(upload); err != nil {
					// Log error but don't fail the upload
					fmt.Fprintf(os.Stderr, "Warning: Failed to cache upload: %v\n", err)
				}
			}
		}
	}

	// Output result using templates
	
	// For GUI mode with --duplicate-info and JSON format, output special format
	if duplicateInfo && outputFormat == "json" {
		jsonOutput := map[string]interface{}{
			"duplicate": isDuplicate,
			"url":       photoURL,
			"imageUrl":  imageURL,
			"photoId":   photoID,
		}
		jsonBytes, _ := json.MarshalIndent(jsonOutput, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		// Normal output using templates
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
		
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Using template for format '%s': %s\n", outputFormat, template)
		}

		// Build template variables
		filename := filepath.Base(imagePath)
		filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		
		// Build edit URL based on service
		editURL := ""
		if service == "flickr" {
			editURL = "https://www.flickr.com/photos/upload/edit/?ids=" + photoID
		}
		// SmugMug doesn't have a direct edit URL pattern we can construct
		
		// Debug output
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Building template variables:\n")
			fmt.Fprintf(os.Stderr, "  photoID: %s\n", photoID)
			fmt.Fprintf(os.Stderr, "  photoURL: %s\n", photoURL)
			fmt.Fprintf(os.Stderr, "  imageURL: %s\n", imageURL)
			fmt.Fprintf(os.Stderr, "  title: %s\n", title)
			fmt.Fprintf(os.Stderr, "  description: %s\n", description)
			fmt.Fprintf(os.Stderr, "  altText: %s\n", altText)
			fmt.Fprintf(os.Stderr, "  tags: %v\n", tags)
			fmt.Fprintf(os.Stderr, "  filenameNoExt: %s\n", filenameNoExt)
		}
		
		vars := templates.Variables{
			PhotoID:     photoID,
			URL:         photoURL,
			ImageURL:    imageURL,
			EditURL:     editURL,
			Filename:    filenameNoExt,
			Title:       title,
			Description: description,
			Alt:         altText,
			Tags:        tags,
		}

		// Process and output
		output := templates.Process(template, vars)
		fmt.Println(output)
	}

	// Warn if using direct visibility with Bluesky
	if postToBluesky && visibility == "direct" {
		fmt.Fprintf(os.Stderr, "\nWarning: Bluesky does not support private posts. Your post will be PUBLIC on Bluesky.\n")
		if !dryRun {
			fmt.Fprintf(os.Stderr, "Use --dry-run to test without posting, or create a test account for safe testing.\n\n")
		}
	}
	
	// Post to Mastodon if requested
	if postToMastodon && !dryRun {
		if err := postToMastodonService(cfg, service, photoID, photoURL, title, description, altText, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Mastodon post failed: %v\n", err)
			// Don't exit - the upload was successful
		} else {
			fmt.Println("Posted to Mastodon successfully!")
		}
	} else if postToMastodon && dryRun {
		fmt.Printf("\n[DRY RUN] Would post to Mastodon:\n")
		fmt.Printf("  Visibility: %s\n", visibility)
		statusText := post
		if statusText == "" && title != "" {
			statusText = title
		}
		statusText += "\n\n" + photoURL
		fmt.Printf("  Text: %s\n", statusText)
		if len(tags) > 0 {
			fmt.Printf("  Tags: %v\n", tags)
		}
	}
	
	// Post to Bluesky if requested
	if postToBluesky && !dryRun {
		if err := postToBlueskyService(cfg, service, photoID, photoURL, title, description, altText, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Bluesky post failed: %v\n", err)
			// Don't exit - the upload was successful
		} else {
			fmt.Println("Posted to Bluesky successfully!")
		}
	} else if postToBluesky && dryRun {
		fmt.Printf("\n[DRY RUN] Would post to Bluesky:\n")
		fmt.Printf("  Visibility: PUBLIC (all Bluesky posts are public)\n")
		statusText := post
		if statusText == "" && title != "" {
			statusText = title
		}
		statusText += "\n\n" + photoURL
		// Add hashtags
		for _, tag := range tags {
			hashtag := "#" + strings.ReplaceAll(tag, " ", "")
			if !strings.Contains(statusText, hashtag) {
				statusText += " " + hashtag
			}
		}
		fmt.Printf("  Text (%d chars): %s\n", len(statusText), statusText)
		if len(statusText) > 300 {
			fmt.Printf("  WARNING: Text exceeds Bluesky's 300 character limit!\n")
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
	
	// Show defaults if any are set
	if cfg.Default.Format != "" || cfg.Default.Service != "" || cfg.Default.DuplicateCheck != nil {
		fmt.Printf("  Default:\n")
		if cfg.Default.Format != "" {
			fmt.Printf("    Format: %s\n", cfg.Default.Format)
		}
		if cfg.Default.Service != "" {
			fmt.Printf("    Service: %s\n", cfg.Default.Service)
		}
		fmt.Printf("    Duplicate Check: %v\n", cfg.IsDuplicateCheckEnabled())
		fmt.Println()
	}
	
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
	
	fmt.Printf("\n  Bluesky:\n")
	fmt.Printf("    Handle: %s\n", cfg.Bluesky.Handle)
	fmt.Printf("    App Password: %s\n", maskString(cfg.Bluesky.AppPassword))
	pds := cfg.Bluesky.PDS
	if pds == "" {
		pds = "https://bsky.social (default)"
	}
	fmt.Printf("    PDS: %s\n", pds)

	fmt.Printf("\n  SmugMug:\n")
	fmt.Printf("    Consumer Key: %s\n", maskString(cfg.SmugMug.ConsumerKey))
	fmt.Printf("    Consumer Secret: %s\n", maskString(cfg.SmugMug.ConsumerSecret))
	fmt.Printf("    Access Token: %s\n", maskString(cfg.SmugMug.AccessToken))
	fmt.Printf("    Access Secret: %s\n", maskString(cfg.SmugMug.AccessSecret))
	fmt.Printf("    Album ID: %s\n", cfg.SmugMug.AlbumID)

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
	case key == "default.format":
		cfg.Default.Format = value
	case key == "default.service":
		cfg.Default.Service = value
	case key == "default.duplicate_check":
		// Parse boolean value
		boolValue := value == "true" || value == "yes" || value == "on" || value == "1"
		cfg.Default.DuplicateCheck = &boolValue
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
	case key == "bluesky.handle":
		cfg.Bluesky.Handle = value
	case key == "bluesky.app_password":
		cfg.Bluesky.AppPassword = value
	case key == "bluesky.pds":
		cfg.Bluesky.PDS = value
	case key == "smugmug.key":
		cfg.SmugMug.ConsumerKey = value
	case key == "smugmug.secret":
		cfg.SmugMug.ConsumerSecret = value
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

func postToMastodonService(cfg *config.Config, service string, photoID string, photoURL string, photoTitle string, photoDescription string, altText string, photoTags []string) error {
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
	statusText += "\n\n" + photoURL
	
	// Get a suitable image URL for Mastodon based on the service
	var imageURL string
	
	if service == "flickr" {
		// Get photo sizes from Flickr to find a good size for Mastodon
		api := backends.NewFlickrAPI(&cfg.Flickr)
		sizes, err := api.GetPhotoSizes(context.Background(), photoID)
		if err != nil {
			return fmt.Errorf("failed to get photo sizes from Flickr: %w", err)
		}
		
		// Find a good size for Mastodon (prefer Large or Medium)
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
	} else if service == "smugmug" {
		// For SmugMug, we already have the imageURL from the upload result
		// Just need to get it from where we stored it
		// SmugMug provides good-sized images already, so we can use them directly
		// The imageURL is already set from the upload response
		return fmt.Errorf("SmugMug to Mastodon posting not yet implemented")
	}
	
	if imageURL == "" {
		return fmt.Errorf("no suitable image size found from %s", service)
	}
	
	// Determine alt text: use explicit alt text, fall back to description
	mastodonAltText := altText
	if mastodonAltText == "" && photoDescription != "" {
		mastodonAltText = photoDescription
	}
	
	// Upload the resized image from Flickr to Mastodon
	mediaID, err := client.UploadMediaFromURL(imageURL, mastodonAltText)
	if err != nil {
		return fmt.Errorf("failed to upload media: %w", err)
	}
	
	// Post the status
	if err := client.PostStatus(statusText, []string{mediaID}, visibility, photoTags); err != nil {
		return fmt.Errorf("failed to post status: %w", err)
	}
	
	return nil
}


func authBluesky() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Check if we have handle
	if cfg.Bluesky.Handle == "" {
		fmt.Println("Bluesky handle not found.")
		fmt.Println("\nFirst, set your Bluesky handle:")
		fmt.Println("  imgup config set bluesky.handle yourhandle.bsky.social")
		fmt.Println("\nThen run 'imgup auth bluesky' again.")
		return fmt.Errorf("missing handle")
	}
	
	// Check if we have app password
	if cfg.Bluesky.AppPassword == "" {
		fmt.Println("Bluesky app password not found.")
		fmt.Println("\nTo create an app password:")
		fmt.Println("1. Go to https://bsky.app/settings/app-passwords")
		fmt.Println("2. Click 'Add App Password'")
		fmt.Println("3. Give it a name (e.g., 'imgupv2')")
		fmt.Println("4. Copy the generated password")
		fmt.Println("\nThen run:")
		fmt.Println("  imgup config set bluesky.app_password YOUR_APP_PASSWORD")
		fmt.Println("\nOptionally, if not using bsky.social:")
		fmt.Println("  imgup config set bluesky.pds https://your-pds-server.com")
		return fmt.Errorf("missing app password")
	}
	
	// Test authentication
	client := bluesky.NewClient(cfg.Bluesky.PDS, cfg.Bluesky.Handle, cfg.Bluesky.AppPassword)
	
	fmt.Println("Testing authentication...")
	if err := client.Authenticate(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	
	fmt.Printf("Successfully authenticated as @%s!\n", cfg.Bluesky.Handle)
	
	// Note: Unlike OAuth services, we don't save any tokens since Bluesky
	// uses the app password directly for each session
	
	return nil
}


func postToBlueskyService(cfg *config.Config, service string, photoID string, photoURL string, photoTitle string, photoDescription string, altText string, photoTags []string) error {
	// Check if Bluesky is configured
	if cfg.Bluesky.Handle == "" || cfg.Bluesky.AppPassword == "" {
		return fmt.Errorf("not authenticated with Bluesky. Run 'imgup auth bluesky' first")
	}
	
	// Create Bluesky client
	client := bluesky.NewClient(cfg.Bluesky.PDS, cfg.Bluesky.Handle, cfg.Bluesky.AppPassword)
	
	// Use post text if provided, otherwise use title
	statusText := post
	if statusText == "" && photoTitle != "" {
		statusText = photoTitle
	}
	
	// Add the photo URL to the post
	statusText += "\n\n" + photoURL
	
	// Check character limit (300 for Bluesky)
	if len(statusText) > 300 {
		// Warn but continue with truncated text
		fmt.Fprintf(os.Stderr, "Warning: Post text exceeds Bluesky's 300 character limit (%d chars). Truncating...\n", len(statusText))
		// Leave room for "..."
		statusText = statusText[:297] + "..."
	}
	
	// Get a suitable image URL based on the service
	var imageURL string
	
	if service == "flickr" {
		// Get photo sizes from Flickr to find a good size for Bluesky
		api := backends.NewFlickrAPI(&cfg.Flickr)
		sizes, err := api.GetPhotoSizes(context.Background(), photoID)
		if err != nil {
			return fmt.Errorf("failed to get photo sizes from Flickr: %w", err)
		}
		
		// Find a suitable size - prefer Large (1024px) or Original
		for _, size := range sizes {
			if size.Label == "Large" || size.Label == "Large 1600" || size.Label == "Large 2048" {
				imageURL = size.Source
				break
			}
		}
		
		// Fallback to original if no large size found
		if imageURL == "" {
			for _, size := range sizes {
				if size.Label == "Original" {
					imageURL = size.Source
					break
				}
			}
		}
	} else if service == "smugmug" {
		// SmugMug provides good-sized images already
		// The imageURL is already set from the upload response
		return fmt.Errorf("SmugMug to Bluesky posting not yet implemented")
	}
	
	if imageURL == "" {
		return fmt.Errorf("no suitable image size found from %s", service)
	}
	
	// Determine alt text: use explicit alt text, fall back to description
	blueskyAltText := altText
	if blueskyAltText == "" && photoDescription != "" {
		blueskyAltText = photoDescription
	}
	
	// Upload the image from the photo service to Bluesky
	blob, _, err := client.UploadMediaFromURL(imageURL, blueskyAltText)
	if err != nil {
		return fmt.Errorf("failed to upload media: %w", err)
	}
	
	// Post the status
	if err := client.PostStatus(statusText, []bluesky.BlobResponse{*blob}, []string{blueskyAltText}, photoTags); err != nil {
		return fmt.Errorf("failed to post status: %w", err)
	}
	
	return nil
}

func checkCommand(cmd *cobra.Command, args []string) {
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

	// Apply defaults from config if flags weren't explicitly set
	if !cmd.Flags().Changed("format") && cfg.Default.Format != "" {
		outputFormat = cfg.Default.Format
	}
	if !cmd.Flags().Changed("service") && cfg.Default.Service != "" {
		service = cfg.Default.Service
	}

	// Determine which service to use (same logic as upload command)
	if service == "" {
		hasFlickr := cfg.Flickr.AccessToken != "" && cfg.Flickr.AccessSecret != ""
		hasSmugMug := cfg.SmugMug.AccessToken != "" && cfg.SmugMug.AccessSecret != ""
		
		if hasFlickr && hasSmugMug {
			if cfg.Default.Service != "" {
				service = cfg.Default.Service
			} else {
				fmt.Fprintf(os.Stderr, "Error: Both Flickr and SmugMug are configured. Please specify --service or set a default:\n")
				fmt.Fprintf(os.Stderr, "  imgup config set default.service flickr\n")
				fmt.Fprintf(os.Stderr, "  imgup config set default.service smugmug\n")
				os.Exit(1)
			}
		} else if hasFlickr {
			service = "flickr"
		} else if hasSmugMug {
			service = "smugmug"
		} else {
			fmt.Fprintf(os.Stderr, "Error: Not authenticated. Run 'imgup auth flickr' or 'imgup auth smugmug' first.\n")
			os.Exit(1)
		}
	}

	// Create duplicate checker based on service
	ctx := context.Background()
	var checker *duplicate.RemoteChecker
	
	switch service {
	case "flickr":
		checker, err = duplicate.SetupFlickrDuplicateChecker(&cfg.Flickr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up duplicate checker: %v\n", err)
			os.Exit(1)
		}
		
	case "smugmug":
		checker, err = duplicate.SetupSmugMugDuplicateChecker(&cfg.SmugMug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up duplicate checker: %v\n", err)
			os.Exit(1)
		}
		
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown service: %s\n", service)
		os.Exit(1)
	}
	defer checker.Close()

	// Check for duplicate
	
	upload, err := checker.Check(ctx, imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for duplicate: %v\n", err)
		os.Exit(1)
	}

	if upload == nil {
		// Not found - no output for silent operation
		os.Exit(1)  // Exit with error code to indicate not found
	}

	// Image found! Output using the same template system as upload
	
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
	filename := filepath.Base(imagePath)
	filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// Build edit URL based on service
	editURL := ""
	if service == "flickr" {
		editURL = "https://www.flickr.com/photos/upload/edit/?ids=" + upload.RemoteID
	}
	
	vars := templates.Variables{
		PhotoID:     upload.RemoteID,
		URL:         upload.RemoteURL,
		ImageURL:    upload.ImageURL,
		EditURL:     editURL,
		Filename:    filenameNoExt,
		Title:       "", // We don't have title in cache
		Description: "", // We don't have description in cache
		Alt:         "", // We don't have alt text in cache
		Tags:        []string{}, // We don't have tags in cache
	}

	result := templates.Process(template, vars)
	fmt.Println(result)
}
