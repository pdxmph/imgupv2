# imgupv2 GUI

A lightweight Wails-based GUI for imgupv2 that provides a quick popup interface for uploading images with metadata.

## Features

- Instant popup when invoked via hotkey or service
- Auto-detects selected image in Finder (macOS)
- Pre-fills metadata from EXIF if available
- Quick editing of title, caption, and tags
- Tag autocomplete based on recent usage
- Backend selection (Flickr/SmugMug)
- Output format selection (Markdown/HTML/Org-mode)
- Copies snippet to clipboard after upload
- Closes automatically after successful upload

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

### macOS

1. After building, the app can be found in `build/bin/imgupv2-gui.app`
2. Copy it to `/Applications/` for easy access
3. Create a Quick Action in Automator:
   - Open Automator
   - Create new Quick Action
   - Add "Run Shell Script" action
   - Set script to: `/Applications/imgupv2-gui.app/Contents/MacOS/imgupv2-gui`
   - Save with a memorable name

4. Assign a keyboard shortcut in System Preferences → Keyboard → Shortcuts

### Linux

The binary will be built to `build/bin/imgupv2-gui`. You can:
- Add it to your application menu
- Create a keyboard shortcut
- Use it with file manager extensions

## Workflow

1. Select an image in Finder/file manager
2. Invoke imgupv2-gui via hotkey or service
3. GUI pops up with image preview and pre-filled metadata
4. Edit title, caption, and tags as needed
5. Select backend and output format
6. Hit Upload (or Cmd+Enter)
7. Snippet is copied to clipboard
8. Window closes automatically

## Architecture

- **Frontend**: Plain HTML/CSS/JS (no build step required)
- **Backend**: Go with Wails bindings
- **Communication**: Direct function calls via Wails runtime
- **Size**: ~12MB standalone binary (compared to ~100MB for Electron)

## Integration with imgupv2 CLI

The GUI calls the imgupv2 CLI binary directly, so all the same features and authentication are available. The GUI is just a thin wrapper that makes the common use case (single image upload with quick metadata editing) more convenient.
