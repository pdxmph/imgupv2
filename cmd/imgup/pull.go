package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pdxmph/imgupv2/pkg/backends"
	"github.com/pdxmph/imgupv2/pkg/config"
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
	pullCmd.Flags().StringVar(&pullAlbum, "album", "", "Album name (default: configured album or 'Sharing')")
	pullCmd.Flags().StringVar(&pullFormat, "format", "social", "Output format: social, markdown, html, json")
	pullCmd.Flags().StringVar(&pullSize, "size", "", "Image size: large, medium, small (default: auto based on format)")
	pullCmd.Flags().BoolVar(&pullJSON, "json", false, "Output JSON without interactive selection")
	pullCmd.Flags().BoolVar(&pullGUI, "gui", false, "Open GUI instead of $EDITOR")
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Show what would be posted without posting")

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
			}
		case "flickr":
			if cfg.Flickr.PullAlbum != "" {
				album = cfg.Flickr.PullAlbum
			}
		}
		if album == "" {
			album = "Sharing" // default
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

	fmt.Printf("Fetching from %s (album: %s)...\n\n", strings.Title(service), album)

	// Fetch images from service
	images, err := fetchImages(service, album, count)
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
		// TODO: Pipe to GUI
		fmt.Println("GUI integration not yet implemented")
	} else {
		// Open in editor
		editPullRequest(pullReq)
	}
}

func fetchImages(service, album string, count int) ([]types.PullImage, error) {
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
		return client.PullImages(ctx, album, count)

	case "flickr":
		// TODO: Implement Flickr pull client
		return nil, fmt.Errorf("Flickr pull not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported service: %s", service)
	}
}

func displayImageList(images []types.PullImage) {
	for i, img := range images {
		fmt.Printf("%d) %s", i+1, img.Title)
		if img.Description != "" {
			fmt.Printf(" -- %s", img.Description)
		}
		fmt.Println()
	}
	fmt.Println()
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
		images[i].Post = ""  // User will fill this
		images[i].Alt = ""   // User can add
	}

	return &types.PullRequest{
		Source: types.PullSource{
			Service: service,
			Album:   album,
		},
		Images:     images,
		Targets:    []string{"mastodon", "bluesky"},
		Visibility: "public",
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

	// TODO: Process the edited request (post to social media, generate output, etc.)
	fmt.Println("\nEdited JSON received. Processing not yet implemented.")
	
	// For now, just output what we would do
	if pullDryRun {
		fmt.Println("\n[DRY RUN] Would process:")
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.Encode(editedReq)
	}
}