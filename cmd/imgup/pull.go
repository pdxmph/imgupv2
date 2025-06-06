package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pdxmph/imgupv2/pkg/backends"
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/kitty"
	"github.com/pdxmph/imgupv2/pkg/services/mastodon"
	"github.com/pdxmph/imgupv2/pkg/services/bluesky"
	"github.com/pdxmph/imgupv2/pkg/types"
)

var (
	// Pull command flags
	pullService string
	pullAlbum   string
	pullFormat  string
	pullSize    string
	pullJSON    bool
	pullGUI     bool
	pullDryRun  bool
	pullMastodon bool
	pullBluesky  bool
	pullVisibility string
	pullPost    string
	pullTags    string
)

// createPullCommand creates the pull command
func createPullCommand() *cobra.Command {
	pullCmd := &cobra.Command{
		Use:   "pull [count]",
		Short: "Pull images from photo services for social posting",
		Long: `Pull already-uploaded images from SmugMug or Flickr for social media 
distribution and content generation. Fetches recent images from albums 
and presents them for selection.`,
		Args: cobra.MaximumNArgs(1),
		Run:  pullCommand,
	}

	// Add pull flags
	pullCmd.Flags().StringVar(&pullService, "service", "", "Source service: smugmug, flickr (uses default if not set)")
	pullCmd.Flags().StringVar(&pullAlbum, "album", "", "Album name (SmugMug default: 'Sharing', Flickr default: photostream)")
	pullCmd.Flags().StringVar(&pullFormat, "format", "social", "Output format: social, markdown, html, json")
	pullCmd.Flags().StringVar(&pullSize, "size", "", "Image size: large, medium, small (default: auto based on format)")
	pullCmd.Flags().BoolVar(&pullJSON, "json", false, "Output JSON without interactive selection")
	pullCmd.Flags().BoolVar(&pullGUI, "gui", false, "Open GUI instead of $EDITOR")
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Show what would be posted without posting")
	pullCmd.Flags().BoolVar(&pullMastodon, "mastodon", false, "Post to Mastodon")
	pullCmd.Flags().BoolVar(&pullBluesky, "bluesky", false, "Post to Bluesky")
	pullCmd.Flags().StringVar(&pullVisibility, "visibility", "public", "Mastodon visibility: public, unlisted, private (followers), direct")
	pullCmd.Flags().StringVar(&pullPost, "post", "", "Social media post text (skips editor if provided)")
	pullCmd.Flags().StringVar(&pullTags, "tags", "", "Filter by tags (comma-separated)")

	return pullCmd
}

func pullCommand(cmd *cobra.Command, args []string) {
	// Parse count argument
	count := 10 // default
	if len(args) > 0 {
		var err error
		count, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid count: %v\n", err)
			os.Exit(1)
		}
	}

	// Load config to get defaults
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Determine service (use flag, config default, or "smugmug")
	service := pullService
	if service == "" {
		if cfg.Default.PullService != "" {
			service = cfg.Default.PullService
		} else if cfg.Default.Service != "" {
			service = cfg.Default.Service
		} else {
			service = "smugmug"
		}
	}

	// Determine album
	album := pullAlbum
	if album == "" {
		switch service {
		case "smugmug":
			if cfg.SmugMug.PullAlbum != "" {
				album = cfg.SmugMug.PullAlbum
			} else {
				album = "Sharing" // SmugMug default
			}
		case "flickr":
			if cfg.Flickr.PullAlbum != "" {
				album = cfg.Flickr.PullAlbum
			}
			// For Flickr, empty album means photostream (not "Sharing")
		}
	}

	// Determine image size based on format if not specified
	size := pullSize
	if size == "" {
		switch pullFormat {
		case "social":
			size = "large" // 2048px max
		case "markdown", "html":
			size = "medium" // 800px for embedding
		default:
			size = "large"
		}
	}

	if !pullJSON {
		if service == "flickr" && album == "" {
			fmt.Printf("Fetching from %s photostream...\n\n", strings.Title(service))
		} else {
			fmt.Printf("Fetching from %s (album: %s)...\n\n", strings.Title(service), album)
		}
	}

	// Fetch images from service
	images, err := fetchImages(service, album, count, pullTags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch images: %v\n", err)
		os.Exit(1)
	}

	if len(images) == 0 {
		fmt.Println("No images found in the specified album.")
		return
	}

	if pullJSON {
		// Output JSON directly without selection
		outputJSON(images, service, album)
		return
	}

	// Present numbered list for selection
	displayImageList(images)

	// Get user selection
	selected := getUserSelection(images)
	if len(selected) == 0 {
		fmt.Println("No images selected.")
		return
	}

	// Create JSON for selected images
	pullReq := createPullRequest(selected, service, album)

	if pullGUI {
		// Launch GUI with pull data
		if err := launchGUIWithPullData(pullReq); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch GUI: %v\n", err)
			os.Exit(1)
		}
	} else {
		// If post text provided via flag, skip editor
		if pullPost != "" {
			processPullRequest(pullReq)
		} else {
			// Open in editor
			editPullRequest(pullReq)
		}
	}
}

func fetchImages(service, album string, count int, tags string) ([]types.PullImage, error) {
	ctx := context.Background()
	
	// Load config to get credentials
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	switch service {
	case "smugmug":
		// Check if SmugMug is configured
		if cfg.SmugMug.AccessToken == "" {
			return nil, fmt.Errorf("SmugMug not authenticated. Run: imgup auth smugmug")
		}

		client := backends.NewSmugMugPullClient(&cfg.SmugMug)
		return client.PullImages(ctx, album, count, tags)

	case "flickr":
		// Check if Flickr is configured
		if cfg.Flickr.AccessToken == "" {
			return nil, fmt.Errorf("Flickr not authenticated. Run: imgup auth flickr")
		}
		
		client := backends.NewFlickrPullClient(&cfg.Flickr)
		return client.PullImages(ctx, album, count, tags)

	default:
		return nil, fmt.Errorf("unsupported service: %s", service)
	}
}

func displayImageList(images []types.PullImage) {
	// Load config to check if Kitty thumbnails are enabled
	cfg, err := config.Load()
	if err == nil && cfg.Default.KittyThumbnails && kitty.IsKittyTerminal() {
		// Try to display thumbnails in Kitty
		if err := displayKittyThumbnails(images); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to display Kitty thumbnails: %v\n", err)
			fmt.Fprintln(os.Stderr, "Falling back to text display...")
			displayTextList(images)
		}
	} else {
		// Fall back to text display
		displayTextList(images)
	}
}

func displayTextList(images []types.PullImage) {
	for i, img := range images {
		fmt.Printf("%d) %s", i+1, img.Title)
		if img.Description != "" {
			fmt.Printf(" -- %s", img.Description)
		}
		fmt.Println()
	}
	fmt.Println()
}

func displayKittyThumbnails(images []types.PullImage) error {
	display := kitty.NewImageDisplay()
	
	// Clear any existing images first
	display.ClearImages()
	
	// Download and display thumbnails
	fmt.Println("\nLoading thumbnails...\n")
	
	// Display each image with its info
	for i, img := range images {
		// Download thumbnail - prefer Small size for better visibility
		thumbURL := img.Sizes.Small
		if thumbURL == "" {
			thumbURL = img.Sizes.Thumb // fallback to thumb if no small
		}
		if thumbURL == "" {
			fmt.Printf("%d) %s [No thumbnail available]\n\n", i+1, img.Title)
			continue
		}
		
		resp, err := http.Get(thumbURL)
		if err != nil {
			fmt.Printf("%d) %s [Failed to download thumbnail]\n\n", i+1, img.Title)
			continue
		}
		
		// Check response status
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Printf("%d) %s [HTTP error: %d]\n\n", i+1, img.Title, resp.StatusCode)
			continue
		}
		
		// Read the image data
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("%d) %s [Failed to read thumbnail]\n\n", i+1, img.Title)
			continue
		}
		
		// Check data size
		if len(data) == 0 {
			fmt.Printf("%d) %s [Empty thumbnail data]\n\n", i+1, img.Title)
			continue
		}
		
		// Display the thumbnail flush left
		reader := bytes.NewReader(data)
		if err := display.DisplayImage(reader, 0, 0); err != nil {
			fmt.Printf("%d) %s [Failed to display thumbnail]\n\n", i+1, img.Title)
			continue
		}
		
		// Display metadata directly below the image
		fmt.Printf("%d) %s", i+1, img.Title)
		if img.Description != "" {
			fmt.Printf(" -- %s", img.Description)
		}
		fmt.Println("\n") // Extra line for spacing between items
	}
	
	// Clean up temp files when done
	display.Cleanup()
	
	return nil
}

func getUserSelection(images []types.PullImage) []types.PullImage {
	fmt.Print("Select images (e.g., 1,3,5): ")
	
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		return nil
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var selected []types.PullImage
	parts := strings.Split(input, ",")
	
	for _, part := range parts {
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || num < 1 || num > len(images) {
			fmt.Fprintf(os.Stderr, "Invalid selection: %s\n", part)
			continue
		}
		selected = append(selected, images[num-1])
	}

	return selected
}

func createPullRequest(images []types.PullImage, service, album string) *types.PullRequest {
	// Reset IDs to sequential numbers for cleaner JSON
	for i := range images {
		images[i].ID = strconv.Itoa(i + 1)
		// Auto-populate alt text with description if available
		if images[i].Description != "" {
			images[i].Alt = images[i].Description
		} else {
			images[i].Alt = ""
		}
	}

	// Build targets based on flags
	var targets []string
	if pullMastodon {
		targets = append(targets, "mastodon")
	}
	if pullBluesky {
		targets = append(targets, "bluesky")
	}

	return &types.PullRequest{
		Source: types.PullSource{
			Service: service,
			Album:   album,
		},
		Post:       pullPost,  // Use the flag value if provided
		Images:     images,
		Targets:    targets,
		Visibility: pullVisibility,
		Format:     pullFormat,
	}
}

func outputJSON(images []types.PullImage, service, album string) {
	pullReq := createPullRequest(images, service, album)
	
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(pullReq); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

func editPullRequest(pullReq *types.PullRequest) {
	// Create temporary file
	tmpfile, err := os.CreateTemp("", "imgup-pull-*.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpfile.Name())

	// Write JSON to temp file
	encoder := json.NewEncoder(tmpfile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(pullReq); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write JSON: %v\n", err)
		os.Exit(1)
	}
	tmpfile.Close()

	// Give user instructions
	fmt.Println("\nOpening editor. Fill in the 'post' field at the top for your social media text.")
	fmt.Println("Example: \"post\": \"Check out these photos from the show!\"")
	fmt.Println("You can also edit 'alt' text for individual images.\n")

	// Get editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // fallback
	}

	// Open editor
	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open editor: %v\n", err)
		os.Exit(1)
	}

	// Read back the edited JSON
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read edited file: %v\n", err)
		os.Exit(1)
	}

	// Parse the edited JSON
	var editedReq types.PullRequest
	if err := json.Unmarshal(data, &editedReq); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid JSON: %v\n", err)
		os.Exit(1)
	}

	// Debug output in dry-run mode
	if pullDryRun {
		fmt.Println("\n[DRY RUN] Parsed JSON successfully")
		fmt.Printf("Post text: %q\n", editedReq.Post)
		fmt.Printf("Images selected: %d\n", len(editedReq.Images))
		for i, img := range editedReq.Images {
			fmt.Printf("  %d. %s\n", i+1, img.Title)
		}
		fmt.Println()
	}

	// Process the edited request
	processPullRequest(&editedReq)
}

func processPullRequest(pullReq *types.PullRequest) {
	// Check if post text exists
	if pullReq.Post == "" {
		fmt.Println("No post text provided. Use the 'post' field at the top of the JSON or --post flag.")
		return
	}

	if len(pullReq.Images) == 0 {
		fmt.Println("No images selected.")
		return
	}

	fmt.Printf("Posting %d images with text: %q\n\n", len(pullReq.Images), pullReq.Post)
	// Load config for social media credentials
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize social media clients if needed
	var mastodonClient *mastodon.Client
	var blueskyClient *bluesky.Client

	if contains(pullReq.Targets, "mastodon") && cfg.Mastodon.AccessToken != "" {
		mastodonClient = mastodon.NewClient(
			cfg.Mastodon.InstanceURL,
			cfg.Mastodon.ClientID,
			cfg.Mastodon.ClientSecret,
			cfg.Mastodon.AccessToken,
		)
	}

	if contains(pullReq.Targets, "bluesky") && cfg.Bluesky.AppPassword != "" {
		blueskyClient = bluesky.NewClient(
			"", // Uses default bsky.social
			cfg.Bluesky.Handle,
			cfg.Bluesky.AppPassword,
		)
		if err := blueskyClient.Authenticate(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate with Bluesky: %v\n", err)
			if !pullDryRun {
				os.Exit(1)
			}
		}
	}

	// Collect all tags from selected images, filtering out imgupv2 machine tags
	allTags := make(map[string]bool)
	for _, img := range pullReq.Images {
		for _, tag := range img.Tags {
			// Skip imgupv2 machine tags
			if !strings.HasPrefix(tag, "imgupv2:") {
				allTags[tag] = true
			}
		}
	}
	var uniqueTags []string
	for tag := range allTags {
		uniqueTags = append(uniqueTags, tag)
	}

	// Build post text with tags
	postText := pullReq.Post

	if pullDryRun {
		fmt.Printf("[DRY RUN] Would post to: %v\n", pullReq.Targets)
		fmt.Printf("  Text: %s\n", postText)
		fmt.Printf("  Images: %d\n", len(pullReq.Images))
		for i, img := range pullReq.Images {
			imageURL := selectImageSize(img.Sizes, pullSize)
			fmt.Printf("    %d. %s (%s)\n", i+1, img.Title, imageURL)
			if img.Alt != "" {
				fmt.Printf("       Alt: %s\n", img.Alt)
			}
		}
		fmt.Printf("  Tags: %v\n", uniqueTags)
		fmt.Printf("  Visibility: %s\n", pullReq.Visibility)
		return
	}

	// Upload all images and collect media IDs/blobs
	var mastodonMediaIDs []string
	var blueskyBlobs []bluesky.BlobResponse
	var blueskyAltTexts []string

	if mastodonClient != nil && contains(pullReq.Targets, "mastodon") {
		fmt.Println("Uploading images to Mastodon...")
		for _, img := range pullReq.Images {
			imageURL := selectImageSize(img.Sizes, pullSize)
			fmt.Printf("  Uploading %s...", img.Title)
			mediaID, err := mastodonClient.UploadMediaFromURL(imageURL, img.Alt)
			if err != nil {
				fmt.Printf(" failed: %v\n", err)
				continue
			}
			mastodonMediaIDs = append(mastodonMediaIDs, mediaID)
			fmt.Printf(" done\n")
		}
	}

	if blueskyClient != nil && contains(pullReq.Targets, "bluesky") {
		fmt.Println("Uploading images to Bluesky...")
		for _, img := range pullReq.Images {
			imageURL := selectImageSize(img.Sizes, pullSize)
			fmt.Printf("  Uploading %s...", img.Title)
			blob, altText, err := blueskyClient.UploadMediaFromURL(imageURL, img.Alt)
			if err != nil {
				fmt.Printf(" failed: %v\n", err)
				continue
			}
			blueskyBlobs = append(blueskyBlobs, *blob)
			blueskyAltTexts = append(blueskyAltTexts, altText)
			fmt.Printf(" done\n")
		}
	}

	// Post to social media platforms
	posted := false

	if mastodonClient != nil && contains(pullReq.Targets, "mastodon") && len(mastodonMediaIDs) > 0 {
		fmt.Printf("\nPosting to Mastodon...")
		visibility := pullReq.Visibility
		if visibility == "" {
			visibility = "public"
		}
		err = mastodonClient.PostStatus(postText, mastodonMediaIDs, visibility, uniqueTags)
		if err != nil {
			fmt.Printf(" failed: %v\n", err)
		} else {
			fmt.Printf(" done\n")
			posted = true
		}
	}

	if blueskyClient != nil && contains(pullReq.Targets, "bluesky") && len(blueskyBlobs) > 0 {
		fmt.Printf("Posting to Bluesky...")
		err = blueskyClient.PostStatus(postText, blueskyBlobs, blueskyAltTexts, uniqueTags)
		if err != nil {
			fmt.Printf(" failed: %v\n", err)
		} else {
			fmt.Printf(" done\n")
			posted = true
		}
	}

	// Generate output based on format
	if posted && pullReq.Format != "social" {
		fmt.Println("\nOutput:")
		for _, img := range pullReq.Images {
			imageURL := selectImageSize(img.Sizes, pullSize)
			output := generateOutput(img, pullReq.Format, imageURL)
			if output != "" {
				fmt.Println(output)
			}
		}
	}

	if posted {
		fmt.Printf("\nSuccessfully posted %d images\n", len(pullReq.Images))
	} else {
		fmt.Println("\nNo posts were made")
	}
}

func selectImageSize(sizes types.ImageSizes, requestedSize string) string {
	switch requestedSize {
	case "small":
		if sizes.Small != "" {
			return sizes.Small
		}
		fallthrough
	case "medium":
		if sizes.Medium != "" {
			return sizes.Medium
		}
		fallthrough
	case "large":
		if sizes.Large != "" {
			return sizes.Large
		}
		fallthrough
	default:
		// Return first available size
		if sizes.Large != "" {
			return sizes.Large
		}
		if sizes.Medium != "" {
			return sizes.Medium
		}
		if sizes.Small != "" {
			return sizes.Small
		}
		return sizes.Thumb
	}
}

func generateOutput(img types.PullImage, format string, imageURL string) string {
	switch format {
	case "url":
		return img.SourceURL
	case "markdown":
		if img.Alt != "" {
			return fmt.Sprintf("![%s](%s)", img.Alt, imageURL)
		}
		return fmt.Sprintf("![%s](%s)", img.Title, imageURL)
	case "html":
		if img.Alt != "" {
			return fmt.Sprintf(`<img src="%s" alt="%s">`, imageURL, img.Alt)
		}
		return fmt.Sprintf(`<img src="%s" alt="%s">`, imageURL, img.Title)
	case "social":
		// For social format, we've already posted, so return empty
		return ""
	default:
		return img.SourceURL
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// launchGUIWithPullData launches the GUI app with pull request data
func launchGUIWithPullData(pullReq *types.PullRequest) error {
	// Serialize pull request to JSON
	jsonData, err := json.Marshal(pullReq)
	if err != nil {
		return fmt.Errorf("failed to serialize pull request: %w", err)
	}
	
	// Debug: Show what we're sending
	if os.Getenv("IMGUP_DEBUG") != "" {
		fmt.Printf("DEBUG: Sending pull request JSON (%d bytes) to GUI\n", len(jsonData))
		// Show a sample of the JSON to verify URLs are present
		var debugReq types.PullRequest
		if err := json.Unmarshal(jsonData, &debugReq); err == nil && len(debugReq.Images) > 0 {
			fmt.Printf("DEBUG: First image sizes - Large: %s\n", debugReq.Images[0].Sizes.Large)
		}
	}
	
	// Find the GUI app
	guiPath := findGUIApp()
	if guiPath == "" {
		return fmt.Errorf("imgupv2-gui.app not found. Please ensure the GUI is installed.")
	}
	
	// Set up the command
	var cmd *exec.Cmd
	
	if strings.HasSuffix(guiPath, ".app") {
		// It's an app bundle - run the binary inside it directly
		binaryPath := filepath.Join(guiPath, "Contents", "MacOS", "imgupv2-gui")
		if _, err := os.Stat(binaryPath); err == nil {
			// Run the binary directly with stdin
			cmd = exec.Command(binaryPath, "--pull-data", "-")
		} else {
			// Can't use open command with stdin
			return fmt.Errorf("cannot access GUI binary inside app bundle")
		}
	} else {
		// Direct binary path
		cmd = exec.Command(guiPath, "--pull-data", "-")
	}
	
	// Set up stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	
	// Capture stdout and stderr for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GUI: %w", err)
	}
	
	// Write JSON data to stdin
	if _, err := stdin.Write(jsonData); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to write to stdin: %w", err)
	}
	stdin.Close()
	
	// Wait for GUI to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("GUI exited with error: %w", err)
	}
	
	return nil
}

// findGUIApp locates the imgupv2-gui.app
func findGUIApp() string {
	// Check common locations - prioritize development build
	searchPaths := []string{
		// Development build location FIRST
		filepath.Join(os.Getenv("HOME"), "code", "imgupv2", "gui", "build", "bin", "imgupv2-gui.app"),
		// Then installed versions
		"/Applications/imgupv2-gui.app",
		filepath.Join(os.Getenv("HOME"), "Applications", "imgupv2-gui.app"),
	}
	
	for _, path := range searchPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	
	// Try to find using mdfind (Spotlight)
	cmd := exec.Command("mdfind", "kMDItemCFBundleIdentifier == 'com.wails.imgupv2-gui'")
	if output, err := cmd.Output(); err == nil {
		apps := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(apps) > 0 && apps[0] != "" {
			return apps[0]
		}
	}
	
	return ""
}
