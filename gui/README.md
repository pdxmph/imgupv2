# imgupv2 GUI

A lightweight Wails-based GUI for imgupv2 that provides a quick popup interface for uploading images with metadata.

## Features

- **Instant popup interface** - Quick access for single image uploads
- **Auto-detects selected image in Finder** (macOS)
- **Pre-fills metadata from EXIF** if available
- **Quick editing** of title, alt text, description, and tags
- **Tag autocomplete** based on recent usage
- **Output format selection** (Markdown/HTML/URL/JSON)
- **Private upload option**
- **Copies snippet to clipboard** after upload
- **Closes automatically** after successful upload
- **Keyboard shortcuts**: Escape to cancel, Cmd+Enter to upload

## Building

### Prerequisites

1. Install Wails:
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

2. Make sure you have the imgupv2 CLI built in the parent directory

### Build Commands

```bash
# Development mode with hot reload
make dev

# Build for current platform
make build

# Build specifically for macOS
make build-mac

# Build for Linux
make build-linux

# Install to system
make install
```

## Usage

### macOS Workflow

1. Select an image in Finder
2. Launch imgupv2-gui (via keyboard shortcut, Dock, or command line)
3. GUI pops up with image preview and pre-filled metadata
4. Edit metadata as needed
5. Click Upload (or press Cmd+Enter)
6. Markdown/HTML snippet is copied to clipboard
7. Window closes automatically

### Creating a macOS Quick Action

1. Open Automator
2. Create new Quick Action
3. Add "Run Shell Script" action
4. Set script to: `/Applications/imgupv2-gui.app/Contents/MacOS/imgupv2-gui`
5. Save with a memorable name (e.g., "Upload to Flickr")
6. Assign keyboard shortcut in System Preferences → Keyboard → Shortcuts → Services

### Linux

The binary will be built to `build/bin/imgupv2-gui`. You can:
- Add it to your application menu
- Create a keyboard shortcut
- Use it with file manager extensions

## Architecture

- **Frontend**: Plain HTML/CSS/JS (no build step required)
- **Backend**: Go with Wails bindings
- **Communication**: Direct function calls via Wails runtime
- **Size**: ~12MB standalone binary (compared to ~100MB for Electron)

## Configuration

The GUI uses the same configuration as the imgupv2 CLI:
- Config file: `~/.config/imgupv2/config.json`
- OAuth tokens are shared with the CLI
- Upload templates are defined in the config

## Photos.app Support (Planned)

The groundwork has been laid for Photos.app integration:
- Detect when Photos is the active app
- Export selected photo with metadata
- Re-embed metadata using exiftool
- Upload with preserved metadata

This feature is currently disabled but can be re-enabled in future versions.

## Development

### Project Structure
```
gui/
├── app.go               # Main application logic
├── main.go             # Wails entry point
├── frontend/
│   ├── index.html      # UI layout
│   ├── style.css       # Styling
│   └── main.js         # Frontend logic
├── build/              # Build output
└── Makefile           # Build commands
```

### Key Functions

- `GetSelectedPhoto()` - Detects selected image from Finder
- `Upload()` - Handles the imgup CLI execution
- `GetRecentTags()` - Provides tag suggestions

## Troubleshooting

### Upload Hanging
- Fixed by using `cmd.Output()` instead of manual buffer management
- Ensure imgup CLI works from command line first

### No Image Selected
- Make sure an image is selected in Finder before launching
- The GUI will close after 2 seconds if no image is detected

### Missing Dependencies
- Ensure exiftool is installed for metadata extraction
- Verify imgup CLI is in the parent directory or PATH

## Integration with imgupv2 CLI

The GUI calls the imgupv2 CLI binary directly, so all the same features and authentication are available. The GUI is just a thin wrapper that makes the common use case (single image upload with quick metadata editing) more convenient.

## Known Issues

- Template fallback syntax (e.g., `%alt%|%title%|%filename%`) in config.json needs to be simplified
- Photos.app integration is disabled pending further testing
