# imgupv2

A fast, Unix-friendly command-line tool for uploading images to Flickr and SmugMug.

## What is imgupv2?

imgupv2 is a complete Go rewrite of the original Ruby-based imgup-cli. It's designed for photographers who want to quickly upload images to their favorite photo sharing services and get shareable links back.

**Key features:**
- 🚀 Fast uploads - typically under 2 seconds from selection to completion
- 📸 Supports both Flickr and SmugMug photo services
- 🔗 Multiple output formats: URLs, Markdown, HTML, JSON, Org-mode
- 🔒 Secure OAuth authentication (no more copy-pasting callback URLs!)
- ⚙️ Configurable defaults for format and service
- 💻 Single static binary - truly no runtime dependencies

## Installation

### macOS/Linux - Download Binary

Download the latest release for your platform from the [releases page](https://github.com/pdxmph/imgupv2/releases).

```bash
# macOS example
curl -L https://github.com/pdxmph/imgupv2/releases/latest/download/imgupv2_darwin_amd64.tar.gz | tar xz
sudo mv imgup /usr/local/bin/
```

### Build from Source

Requires Go 1.20 or later:

```bash
go install github.com/pdxmph/imgupv2/cmd/imgup@latest
```

**Note:** After installing with `go install`, the binary will be in `~/go/bin/`. Make sure this is in your PATH:

```bash
# Add to your ~/.zshrc or ~/.bashrc
export PATH="$HOME/go/bin:$PATH"

# Reload your shell config
source ~/.zshrc  # or source ~/.bashrc

# Verify installation
which imgup
```

## Quick Start

### 1. Get API Keys

#### For Flickr:
1. Visit [Flickr App Garden](https://www.flickr.com/services/apps/create/)
2. Apply for a non-commercial key
3. Note your Key and Secret
4. Click "Edit auth flow for this app" and add the callback URL: `http://localhost:8749/callback`

#### For SmugMug:
1. Visit [SmugMug API](https://api.smugmug.com/api/developer/apply)
2. Apply for an API key
3. Note your Key and Secret

### 2. Configure imgupv2

```bash
# For Flickr
imgup config set flickr.key YOUR_KEY
imgup config set flickr.secret YOUR_SECRET
imgup auth flickr

# For SmugMug
imgup config set smugmug.key YOUR_KEY
imgup config set smugmug.secret YOUR_SECRET
imgup auth smugmug  # This will prompt you to select an album

# Set defaults (optional)
imgup config set default.service flickr    # or smugmug
imgup config set default.format markdown   # or url, html, json, org
```

### 3. Upload an Image

```bash
# Basic upload (uses default service or auto-detects)
imgup upload photo.jpg

# Upload to specific service
imgup upload --service flickr photo.jpg
imgup upload --service smugmug photo.jpg

# Upload with title and description
imgup upload --title "Sunset at Baker Beach" --description "Golden hour magic" photo.jpg

# Get Markdown-formatted link
imgup upload --format markdown photo.jpg
```

## Usage Examples

### Upload multiple images
```bash
imgup upload *.jpg
```

### Set privacy options
```bash
# Upload as private
imgup upload --private photo.jpg

# SmugMug: Photos are uploaded to your selected album
# Flickr: Additional privacy options
imgup upload --private --friends --family photo.jpg
```

### Output formats
```bash
# Plain URL (default)
imgup upload photo.jpg
# https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg

# Markdown
imgup upload --format markdown photo.jpg
# ![Sunset at Baker Beach](https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg)

# HTML
imgup upload --format html photo.jpg
# <img src="https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg" alt="Sunset at Baker Beach">

# JSON (for scripting)
imgup upload --format json photo.jpg | jq .url

# Org-mode
imgup upload --format org photo.jpg
# [[https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg][Sunset at Baker Beach]]
```

### View configuration
```bash
imgup config show
```

### Custom Output Templates

You can create custom output formats using template variables:

```bash
# Create a custom template
imgup config set template.custom "Photo: %title% - %url%"

# Use your custom template
imgup upload --format custom photo.jpg
```

**Available template variables:**
- `%url%` - Web page URL for the photo
- `%image_url%` - Direct image URL
- `%photo_id%` - Photo ID from the service
- `%title%` - Photo title
- `%description%` - Photo description
- `%alt%` - Alt text (for accessibility)
- `%filename%` - Original filename (without extension)
- `%tags%` - Comma-separated tags
- `%alt|description|title|filename%` - Falls through to first non-empty value

## Requirements

- macOS or Linux
- That's it! No external dependencies required.

## Differences from imgup-cli (Ruby version)

- **Faster**: Go binary vs Ruby interpreter startup
- **Simpler install**: Single binary vs Ruby gems
- **Better OAuth**: Built-in callback server (no URL copy-paste)
- **Multi-service**: Supports both Flickr and SmugMug
- **No dependencies**: Metadata sent via API, no exiftool needed
- **Configurable defaults**: Set preferred service and output format

## Configuration

Configuration is stored in `~/.config/imgupv2/config.json`.

### Available settings

```bash
# Default settings (avoid repetitive flags)
imgup config set default.service flickr    # or smugmug
imgup config set default.format markdown   # or url, html, json, org

# Flickr API credentials
imgup config set flickr.key YOUR_KEY
imgup config set flickr.secret YOUR_SECRET

# SmugMug API credentials
imgup config set smugmug.key YOUR_KEY
imgup config set smugmug.secret YOUR_SECRET

# Custom output templates
imgup config set template.custom "![%alt|description|title|filename%](%image_url%)"

# List all settings
imgup config show
```

## Troubleshooting

### "Both Flickr and SmugMug are configured" error
When you have both services configured, you need to either:
- Specify the service: `imgup upload --service flickr photo.jpg`
- Set a default: `imgup config set default.service flickr`

### SmugMug: "No album selected" error
You need to select an album during authentication:
```bash
imgup auth smugmug
# Follow prompts to select an album
```

### "Flickr doesn't recognise the permission set" error
Make sure you've configured the callback URL in your Flickr app:
1. Go to [Flickr App Garden](https://www.flickr.com/services/apps/)
2. Click on your app
3. Click "Edit auth flow for this app"  
4. Add callback URL: `http://localhost:8749/callback`
5. Save changes

### "401 Unauthorized" errors
Your authentication token may have expired. Re-authenticate:
```bash
imgup auth flickr   # or smugmug
```

### Can't find imgup command
Make sure `/usr/local/bin` is in your PATH:
```bash
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## Building from Source

```bash
git clone https://github.com/pdxmph/imgupv2.git
cd imgupv2
go build -o imgup cmd/imgup/main.go
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Original [imgup-cli](https://github.com/pdxmph/imgup-cli) Ruby implementation

### Open Source Libraries

This project uses the following excellent Go libraries:

- [dghubble/oauth1](https://github.com/dghubble/oauth1) - OAuth 1.0 implementation for Flickr authentication
- [spf13/cobra](https://github.com/spf13/cobra) - Modern CLI library for creating powerful commands
- [google/uuid](https://github.com/google/uuid) - UUID generation for request tracking
