package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pdxmph/imgupv2/pkg/backends"
	"github.com/pdxmph/imgupv2/pkg/config"
	"github.com/pdxmph/imgupv2/pkg/metadata"
	"github.com/pdxmph/imgupv2/pkg/templates"
)

var (
	// Upload flags
	title        string
	description  string
	altText      string
	outputFormat string
	isPrivate    bool
	tags         []string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "imgup",
		Short: "Fast image upload tool",
		Long: `imgupv2 - A fast command-line tool for uploading images to Flickr
with support for metadata embedding and multiple output formats.`,
	}

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

	// Add commands to root
	rootCmd.AddCommand(authCmd, uploadCmd, configCmd)

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
