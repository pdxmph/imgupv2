package types

// BatchUploadRequest represents the JSON input for batch upload operations
type BatchUploadRequest struct {
	Images  []ImageUpload      `json:"images"`
	Common  *CommonSettings    `json:"common,omitempty"`
	Social  *SocialSettings    `json:"social,omitempty"`
	Options *UploadOptions     `json:"options,omitempty"`
}

// ImageUpload represents a single image in the batch
type ImageUpload struct {
	Path        string   `json:"path"`
	Title       string   `json:"title,omitempty"`
	Alt         string   `json:"alt,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CommonSettings applies to all images in the batch
type CommonSettings struct {
	Tags    []string `json:"tags,omitempty"`
	Private bool     `json:"private,omitempty"`
	Service string   `json:"service,omitempty"` // "flickr" or "smugmug"
}

// SocialSettings configures social media posting
type SocialSettings struct {
	Mastodon *MastodonSettings `json:"mastodon,omitempty"`
	Bluesky  *BlueskySettings  `json:"bluesky,omitempty"`
}

// MastodonSettings for Mastodon posts
type MastodonSettings struct {
	Enabled    bool   `json:"enabled"`
	Post       string `json:"post,omitempty"`
	Visibility string `json:"visibility,omitempty"` // public, unlisted, followers, direct
}

// BlueskySettings for Bluesky posts
type BlueskySettings struct {
	Enabled bool   `json:"enabled"`
	Post    string `json:"post,omitempty"`
}

// UploadOptions controls upload behavior
type UploadOptions struct {
	Format string `json:"format,omitempty"` // Output format preference
	DryRun bool   `json:"dry_run,omitempty"`
	Force  bool   `json:"force,omitempty"` // Force upload even if duplicate
}

// BatchUploadResponse represents the JSON output from batch uploads
type BatchUploadResponse struct {
	Success bool                `json:"success"`
	Uploads []UploadResult      `json:"uploads"`
	Social  *SocialPostResults  `json:"social,omitempty"`
}

// UploadResult represents the result of a single image upload
type UploadResult struct {
	Path      string   `json:"path"`
	URL       string   `json:"url,omitempty"`
	ImageURL  string   `json:"imageUrl,omitempty"`
	PhotoID   string   `json:"photoId,omitempty"`
	Duplicate bool     `json:"duplicate"`
	Error     *string  `json:"error"`
	Warnings  []string `json:"warnings,omitempty"`
}

// SocialPostResults contains results from social media posting
type SocialPostResults struct {
	Mastodon *SocialPostResult `json:"mastodon,omitempty"`
	Bluesky  *SocialPostResult `json:"bluesky,omitempty"`
}

// SocialPostResult represents the result of a social media post
type SocialPostResult struct {
	Success bool    `json:"success"`
	URL     string  `json:"url,omitempty"`
	Error   *string `json:"error"`
}

// PullRequest represents the JSON format for pull operations
type PullRequest struct {
	Source  PullSource    `json:"source"`
	Post    string        `json:"post"`                    // Single post text for all images
	Images  []PullImage   `json:"images"`
	Targets []string      `json:"targets,omitempty"`       // ["mastodon", "bluesky"]
	Visibility string     `json:"visibility,omitempty"`    // for mastodon
	Format  string        `json:"format,omitempty"`        // output format: social, markdown, html
}

// PullSource identifies where images are pulled from
type PullSource struct {
	Service string `json:"service"`           // "smugmug" or "flickr"
	Album   string `json:"album,omitempty"`   // album name
}

// PullImage represents an image that can be selected for posting
type PullImage struct {
	ID          string      `json:"id"`                     // temporary ID for selection
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	SourceURL   string      `json:"source_url"`             // original photo page
	Sizes       ImageSizes  `json:"sizes"`
	Alt         string      `json:"alt"`                    // alt text
	Tags        []string    `json:"tags,omitempty"`         // from source service
}

// ImageSizes contains URLs for different image sizes
type ImageSizes struct {
	Large  string `json:"large"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Thumb  string `json:"thumb"`
}
