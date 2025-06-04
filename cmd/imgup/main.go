package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/pdxmph/imgupv2/pkg/types"
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
	
	// JSON input flags
	jsonInput        bool
	jsonFile         string
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
		Args:  cobra.RangeArgs(0, 1), // 0 for JSON stdin, 1 for single image
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
	
	// Add JSON input flags
	uploadCmd.Flags().BoolVar(&jsonInput, "json", false, "Read JSON upload specification from stdin")
	uploadCmd.Flags().StringVar(&jsonFile, "json-file", "", "Read JSON upload specification from file")

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
	// Check if JSON mode is requested
	if jsonInput || jsonFile != "" {
		if err := handleJSONUpload(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	
	// Single image mode - require exactly one argument
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error: Single image upload requires exactly one image path\n")
		cmd.Usage()
		os.Exit(1)
	}
	
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
			
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Duplicate detected!\n")
				fmt.Fprintf(os.Stderr, "  Service: %s\n", existingUpload.Service)
				fmt.Fprintf(os.Stderr, "  RemoteID: %s\n", photoID)
				fmt.Fprintf(os.Stderr, "  RemoteURL: %s\n", photoURL)
				fmt.Fprintf(os.Stderr, "  ImageURL: %s\n", imageURL)
			}
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
			
			// Print warnings to stderr unless in JSON mode
			if len(result.Warnings) > 0 && outputFormat != "json" {
				for _, warning := range result.Warnings {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
				}
			}
			
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
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Starting Bluesky post with photoID=%s, service=%s\n", photoID, service)
		}
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

func handleJSONUpload(cmd *cobra.Command) error {
	var input []byte
	var err error
	
	// Read JSON input
	if jsonInput {
		// Read from stdin
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else if jsonFile != "" {
		// Read from file
		input, err = os.ReadFile(jsonFile)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", jsonFile, err)
		}
	} else {
		return fmt.Errorf("no JSON input specified")
	}
	
	// Parse JSON
	var request types.BatchUploadRequest
	if err := json.Unmarshal(input, &request); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Validate request
	if len(request.Images) == 0 {
		return fmt.Errorf("no images specified in JSON")
	}
	
	// Validate all image paths exist
	for _, img := range request.Images {
		if _, err := os.Stat(img.Path); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", img.Path)
		}
	}
	
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Apply options from JSON
	if request.Options != nil {
		if request.Options.Force {
			force = true
		}
		if request.Options.DryRun {
			dryRun = true
		}
	}
	
	// Determine service
	service := determineService(cfg, request.Common)
	if service == "" {
		return fmt.Errorf("no upload service configured. Run 'imgup auth flickr' or 'imgup auth smugmug' first")
	}
	
	// Process uploads
	ctx := context.Background()
	response := &types.BatchUploadResponse{
		Success: true,
		Uploads: make([]types.UploadResult, len(request.Images)),
	}
	
	// Upload images (could be parallelized in future)
	var uploadedImages []uploadedImage
	for i, img := range request.Images {
		result := uploadSingleImage(ctx, cfg, service, img, request.Common)
		response.Uploads[i] = result
		
		if result.Error == nil {
			uploadedImages = append(uploadedImages, uploadedImage{
				URL:      result.URL,
				ImageURL: result.ImageURL,
				PhotoID:  result.PhotoID,
				Alt:      img.Alt,
			})
		} else {
			response.Success = false
		}
	}
	
	// Handle social media posting if at least one image uploaded successfully
	if len(uploadedImages) > 0 && request.Social != nil && !dryRun {
		response.Social = &types.SocialPostResults{}
		
		// Post to Mastodon
		if request.Social.Mastodon != nil && request.Social.Mastodon.Enabled {
			mastodonResult := postToMastodonBatch(cfg, uploadedImages, request.Social.Mastodon)
			response.Social.Mastodon = &mastodonResult
		}
		
		// Post to Bluesky
		if request.Social.Bluesky != nil && request.Social.Bluesky.Enabled {
			blueskyResult := postToBlueskyBatch(cfg, uploadedImages, request.Social.Bluesky)
			response.Social.Bluesky = &blueskyResult
		}
	}
	
	// Output JSON response
	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	fmt.Println(string(output))
	
	return nil
}

// Helper struct for passing uploaded image data
type uploadedImage struct {
	URL      string
	ImageURL string
	PhotoID  string
	Alt      string
}

// determineService figures out which service to use based on config and request
func determineService(cfg *config.Config, common *types.CommonSettings) string {
	// Check if service specified in request
	if common != nil && common.Service != "" {
		return common.Service
	}
	
	// Use default from config
	if cfg.Default.Service != "" {
		return cfg.Default.Service
	}
	
	// Auto-detect based on what's configured
	hasFlickr := cfg.Flickr.AccessToken != "" && cfg.Flickr.AccessSecret != ""
	hasSmugMug := cfg.SmugMug.AccessToken != "" && cfg.SmugMug.AccessSecret != ""
	
	if hasFlickr && !hasSmugMug {
		return "flickr"
	} else if hasSmugMug && !hasFlickr {
		return "smugmug"
	}
	
	return "" // Both or neither configured
}

// uploadSingleImage handles uploading a single image and returns the result
func uploadSingleImage(ctx context.Context, cfg *config.Config, service string, img types.ImageUpload, common *types.CommonSettings) types.UploadResult {
	result := types.UploadResult{
		Path: img.Path,
	}
	
	// Merge tags from image and common settings
	var tags []string
	if len(img.Tags) > 0 {
		tags = append(tags, img.Tags...)
	}
	if common != nil && len(common.Tags) > 0 {
		tags = append(tags, common.Tags...)
	}
	
	// Check private setting
	isPrivate := false
	if common != nil {
		isPrivate = common.Private
	}
	
	// Check for duplicates first
	if !force && cfg.IsDuplicateCheckEnabled() {
		isDuplicate, existingUpload := checkForDuplicate(ctx, cfg, service, img.Path)
		if isDuplicate && existingUpload != nil {
			result.Duplicate = true
			result.URL = existingUpload.RemoteURL
			result.ImageURL = existingUpload.ImageURL
			result.PhotoID = existingUpload.RemoteID
			return result
		}
	}
	
	// Get file info for machine tags
	fileInfo, _ := duplicate.GetFileInfo(img.Path)
	
	// Perform upload based on service
	switch service {
	case "flickr":
		// Add machine tag for duplicate detection
		if fileInfo != nil && fileInfo.MD5 != "" {
			machineTag := fmt.Sprintf("imgupv2:checksum=%s", fileInfo.MD5)
			tags = append(tags, machineTag)
		}
		
		uploader := backends.NewFlickrUploader(
			cfg.Flickr.ConsumerKey,
			cfg.Flickr.ConsumerSecret,
			cfg.Flickr.AccessToken,
			cfg.Flickr.AccessSecret,
		)
		
		uploadResult, err := uploader.Upload(ctx, img.Path, img.Title, img.Description, tags, isPrivate)
		if err != nil {
			errStr := err.Error()
			result.Error = &errStr
			return result
		}
		
		result.URL = uploadResult.URL
		result.ImageURL = uploadResult.ImageURL
		result.PhotoID = uploadResult.PhotoID
		result.Warnings = uploadResult.Warnings
		
	case "smugmug":
		uploader := backends.NewSmugMugUploader(
			cfg.SmugMug.ConsumerKey,
			cfg.SmugMug.ConsumerSecret,
			cfg.SmugMug.AccessToken,
			cfg.SmugMug.AccessSecret,
			cfg.SmugMug.AlbumID,
		)
		
		uploadResult, err := uploader.Upload(ctx, img.Path, img.Title, img.Description, tags, isPrivate)
		if err != nil {
			errStr := err.Error()
			result.Error = &errStr
			return result
		}
		
		result.URL = uploadResult.URL
		result.ImageURL = uploadResult.ImageURL
		result.PhotoID = uploadResult.ImageKey
		
	default:
		errStr := fmt.Sprintf("unsupported service: %s", service)
		result.Error = &errStr
		return result
	}
	
	// Record successful upload in cache
	if fileInfo != nil && result.Error == nil {
		recordUploadInCache(service, img.Path, result.PhotoID, result.URL, result.ImageURL, fileInfo)
	}
	
	return result
}

// checkForDuplicate checks if an image has already been uploaded
func checkForDuplicate(ctx context.Context, cfg *config.Config, service string, imagePath string) (bool, *duplicate.Upload) {
	var checker *duplicate.RemoteChecker
	var err error
	
	switch service {
	case "flickr":
		checker, err = duplicate.SetupFlickrDuplicateChecker(&cfg.Flickr)
	case "smugmug":
		checker, err = duplicate.SetupSmugMugDuplicateChecker(&cfg.SmugMug)
	default:
		return false, nil
	}
	
	if err != nil {
		return false, nil
	}
	defer checker.Close()
	
	existingUpload, err := checker.Check(ctx, imagePath)
	if err != nil || existingUpload == nil {
		return false, nil
	}
	
	return true, existingUpload
}

// recordUploadInCache records a successful upload for future duplicate detection
func recordUploadInCache(service, imagePath, photoID, photoURL, imageURL string, fileInfo *duplicate.FileInfo) {
	cache, err := duplicate.NewSQLiteCache(duplicate.DefaultCachePath())
	if err != nil {
		return
	}
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
	
	cache.Record(upload)
}

// postToMastodonBatch posts multiple images to Mastodon
func postToMastodonBatch(cfg *config.Config, images []uploadedImage, settings *types.MastodonSettings) types.SocialPostResult {
	result := types.SocialPostResult{}
	
	// Check if Mastodon is configured
	if cfg.Mastodon.AccessToken == "" {
		errStr := "not authenticated with Mastodon"
		result.Error = &errStr
		return result
	}
	
	// Create Mastodon client
	client := mastodon.NewClient(
		cfg.Mastodon.InstanceURL,
		cfg.Mastodon.ClientID,
		cfg.Mastodon.ClientSecret,
		cfg.Mastodon.AccessToken,
	)
	
	// Upload all images to Mastodon and collect media IDs
	var mediaIDs []string
	for _, img := range images {
		// Get image URL for social posting
		imageURL := img.ImageURL
		if imageURL == "" {
			// Fall back to fetching it based on service
			// This would need the service info, but for now use what we have
			continue
		}
		
		mediaID, err := client.UploadMediaFromURL(imageURL, img.Alt)
		if err != nil {
			errStr := fmt.Sprintf("failed to upload media: %v", err)
			result.Error = &errStr
			return result
		}
		mediaIDs = append(mediaIDs, mediaID)
	}
	
	// Build status text
	statusText := settings.Post
	if statusText == "" {
		statusText = "Photos uploaded with imgupv2"
	}
	
	// Add URLs of all photos
	statusText += "\n\n"
	for i, img := range images {
		if i > 0 {
			statusText += "\n"
		}
		statusText += img.URL
	}
	
	// Post the status with all media
	visibility := settings.Visibility
	if visibility == "" {
		visibility = "public"
	}
	
	if err := client.PostStatus(statusText, mediaIDs, visibility, nil); err != nil {
		errStr := fmt.Sprintf("failed to post status: %v", err)
		result.Error = &errStr
		return result
	}
	
	result.Success = true
	// TODO: Get the actual Mastodon post URL from the response
	result.URL = cfg.Mastodon.InstanceURL // Placeholder
	
	return result
}

// postToBlueskyBatch posts multiple images to Bluesky
func postToBlueskyBatch(cfg *config.Config, images []uploadedImage, settings *types.BlueskySettings) types.SocialPostResult {
	result := types.SocialPostResult{}
	
	// Check if Bluesky is configured
	if cfg.Bluesky.Handle == "" || cfg.Bluesky.AppPassword == "" {
		errStr := "not authenticated with Bluesky"
		result.Error = &errStr
		return result
	}
	
	// Create Bluesky client
	client := bluesky.NewClient(cfg.Bluesky.PDS, cfg.Bluesky.Handle, cfg.Bluesky.AppPassword)
	
	// Upload all images to Bluesky and collect blobs
	var blobs []bluesky.BlobResponse
	var altTexts []string
	
	for _, img := range images {
		imageURL := img.ImageURL
		if imageURL == "" {
			continue
		}
		
		blob, _, err := client.UploadMediaFromURL(imageURL, img.Alt)
		if err != nil {
			errStr := fmt.Sprintf("failed to upload media: %v", err)
			result.Error = &errStr
			return result
		}
		
		if blob != nil {
			blobs = append(blobs, *blob)
			altTexts = append(altTexts, img.Alt)
		}
	}
	
	// Build status text
	statusText := settings.Post
	if statusText == "" {
		statusText = "Photos uploaded with imgupv2"
	}
	
	// Add URLs
	statusText += "\n\n"
	for i, img := range images {
		if i > 0 {
			statusText += "\n"
		}
		statusText += img.URL
	}
	
	// Check character limit
	if len(statusText) > 300 {
		statusText = statusText[:297] + "..."
	}
	
	// Post the status with all media
	if err := client.PostStatus(statusText, blobs, altTexts, nil); err != nil {
		errStr := fmt.Sprintf("failed to post status: %v", err)
		result.Error = &errStr
		return result
	}
	
	result.Success = true
	// TODO: Get actual Bluesky post URL
	result.URL = "https://bsky.app/" // Placeholder
	
	return result
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

// getMapKeys is a helper function to get the keys from a map (for debugging)
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func postToMastodonService(cfg *config.Config, service string, photoID string, photoURL string, photoTitle string, photoDescription string, altText string, photoTags []string) error {
	// Check if Mastodon is configured
	if cfg.Mastodon.AccessToken == "" {
		return fmt.Errorf("not authenticated with Mastodon. Run 'imgup auth mastodon' first")
	}
	
	// Validate we have required photo data
	if photoID == "" {
		return fmt.Errorf("cannot post to Mastodon: no photo ID available")
	}
	if photoURL == "" {
		return fmt.Errorf("cannot post to Mastodon: no photo URL available")
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
	imageURL, err := getImageURLForSocialPosting(cfg, service, photoID)
	if err != nil {
		return fmt.Errorf("failed to get image for social posting: %w", err)
	}
	
	// Determine alt text: use explicit alt text, fall back to description
	mastodonAltText := altText
	if mastodonAltText == "" && photoDescription != "" {
		mastodonAltText = photoDescription
	}
	
	// Upload the resized image from photo service to Mastodon
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

// getImageURLForSocialPosting fetches an appropriate image URL for social media posting
// from either Flickr or SmugMug using the photo ID
func getImageURLForSocialPosting(cfg *config.Config, service string, photoID string) (string, error) {
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: getImageURLForSocialPosting called with service=%s, photoID=%s\n", service, photoID)
	}
	
	switch service {
	case "flickr":
		// Get photo sizes from Flickr to find a good size for social media
		api := backends.NewFlickrAPI(&cfg.Flickr)
		sizes, err := api.GetPhotoSizes(context.Background(), photoID)
		if err != nil {
			return "", fmt.Errorf("failed to get photo sizes from Flickr: %w", err)
		}
		
		// Find a good size for social media (prefer Large or Medium)
		var imageURL string
		for _, size := range sizes {
			// Prioritize these sizes for social media
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
			return "", fmt.Errorf("no suitable image size found from Flickr")
		}
		
		return imageURL, nil
		
	case "smugmug":
		// For SmugMug, we need to construct the proper URI from the photo ID
		// The photo ID from SmugMug is typically the AlbumImage URI
		api := backends.NewSmugMugAPI(&cfg.SmugMug)
		
		// Get image sizes
		sizes, err := api.GetImageSizes(context.Background(), photoID)
		if err != nil {
			return "", fmt.Errorf("failed to get image sizes from SmugMug (photo ID: %s): %w", photoID, err)
		}
		
		// Extract the image URL from the response
		// SmugMug's response structure is complex, so we need to navigate it
		if respData, ok := sizes["Response"].(map[string]interface{}); ok {
			// Try to find the image URL in various possible locations
			var imageURL string
			
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: SmugMug response keys: %v\n", getMapKeys(respData))
			}
			
			// Check for AlbumImage.Image.ArchivedUri (for large size)
			if albumImage, ok := respData["AlbumImage"].(map[string]interface{}); ok {
				if img, ok := albumImage["Image"].(map[string]interface{}); ok {
					if archivedUri, ok := img["ArchivedUri"].(string); ok && archivedUri != "" {
						imageURL = archivedUri
						if os.Getenv("IMGUP_DEBUG") != "" {
							fmt.Fprintf(os.Stderr, "DEBUG: Found ArchivedUri: %s\n", imageURL)
						}
					}
					
					// If no ArchivedUri, try ImageDownloadUrl
					if imageURL == "" {
						if downloadUrl, ok := img["ImageDownloadUrl"].(string); ok && downloadUrl != "" {
							imageURL = downloadUrl
							if os.Getenv("IMGUP_DEBUG") != "" {
								fmt.Fprintf(os.Stderr, "DEBUG: Found ImageDownloadUrl: %s\n", imageURL)
							}
						}
					}
				}
			}
			
			// If still no URL, try the Image object directly
			if imageURL == "" {
				if img, ok := respData["Image"].(map[string]interface{}); ok {
					if archivedUri, ok := img["ArchivedUri"].(string); ok && archivedUri != "" {
						imageURL = archivedUri
						if os.Getenv("IMGUP_DEBUG") != "" {
							fmt.Fprintf(os.Stderr, "DEBUG: Found ArchivedUri in Image: %s\n", imageURL)
						}
					}
				}
			}
			
			if imageURL != "" {
				return imageURL, nil
			}
		}
		
		return "", fmt.Errorf("could not extract image URL from SmugMug response - photo ID may be invalid or API response structure changed")
		
	default:
		return "", fmt.Errorf("unsupported service: %s", service)
	}
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
	
	// Validate we have required photo data
	if photoID == "" {
		return fmt.Errorf("cannot post to Bluesky: no photo ID available")
	}
	if photoURL == "" {
		return fmt.Errorf("cannot post to Bluesky: no photo URL available")
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
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Getting image URL for Bluesky posting...\n")
	}
	imageURL, err := getImageURLForSocialPosting(cfg, service, photoID)
	if err != nil {
		return fmt.Errorf("failed to get image for social posting: %w", err)
	}
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Got image URL: %s\n", imageURL)
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
