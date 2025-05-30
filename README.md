# imgupv2

A fast, Unix-friendly command-line tool for uploading images to Flickr with metadata preservation.

## What is imgupv2?

imgupv2 is a complete Go rewrite of the original Ruby-based imgup-cli. It's designed for photographers who want to quickly upload images to Flickr while preserving EXIF metadata and getting shareable links back.

**Key features:**
- 🚀 Fast uploads - typically under 2 seconds from selection to completion
- 📷 Preserves EXIF metadata (when exiftool is available)
- 🔗 Multiple output formats: URLs, Markdown, HTML, JSON
- 🔒 Secure OAuth authentication (no more copy-pasting callback URLs!)
- 💻 Single static binary - no runtime dependencies

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
go install github.com/pdxmph/imgupv2@latest
```

## Quick Start

### 1. Get Flickr API Keys

1. Visit [Flickr App Garden](https://www.flickr.com/services/apps/create/)
2. Apply for a non-commercial key
3. Note your Key and Secret

### 2. Configure imgupv2

```bash
# Add your API credentials
imgup config set flickr.consumer_key YOUR_KEY
imgup config set flickr.consumer_secret YOUR_SECRET

# Authenticate with Flickr
imgup auth flickr
```

### 3. Upload an Image

```bash
# Basic upload
imgup upload photo.jpg

# Upload with title
imgup upload -t "Sunset at Baker Beach" photo.jpg

# Get Markdown-formatted link
imgup upload -f markdown photo.jpg
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

# Friends and family only
imgup upload --friends --family photo.jpg
```

### Output formats
```bash
# Plain URL (default)
imgup upload photo.jpg
# https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg

# Markdown
imgup upload -f markdown photo.jpg
# ![](https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg)

# HTML
imgup upload -f html photo.jpg
# <img src="https://live.staticflickr.com/65535/12345678901_abc123def4_b.jpg" />

# JSON (for scripting)
imgup upload -f json photo.jpg | jq .url
```

### View configuration
```bash
imgup config list
```

## Requirements

- **Required**: macOS or Linux
- **Optional**: [exiftool](https://exiftool.org/) for metadata extraction

Install exiftool for richer metadata:
```bash
# macOS
brew install exiftool

# Ubuntu/Debian
sudo apt-get install libimage-exiftool-perl
```

## Differences from imgup-cli (Ruby version)

- **Faster**: Go binary vs Ruby interpreter startup
- **Simpler install**: Single binary vs Ruby gems
- **Better OAuth**: Built-in callback server (no URL copy-paste)
- **Focused**: Flickr-only for now (SmugMug coming later)

## Configuration

Configuration is stored in `~/.config/imgupv2/config.json`.

### Available settings

```bash
# Flickr API credentials
imgup config set flickr.consumer_key YOUR_KEY
imgup config set flickr.consumer_secret YOUR_SECRET

# Default privacy settings
imgup config set flickr.default_privacy.public false
imgup config set flickr.default_privacy.friend true
imgup config set flickr.default_privacy.family true

# List all settings
imgup config list
```

## Troubleshooting

### "401 Unauthorized" errors
Your authentication token may have expired. Re-authenticate:
```bash
imgup auth flickr
```

### No metadata in uploads
Install exiftool:
```bash
brew install exiftool  # macOS
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
