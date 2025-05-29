# imgupv2

Fast command-line image uploader for photographers. A complete Go rewrite of imgup-cli.

## Status

🚧 Early development - MVP in progress

## Goals

- Upload images to Flickr/SmugMug in < 2 seconds
- Single static binary with no runtime dependencies
- OAuth that actually works (no URL copy/paste)
- Optional GUI support via JSON protocol

## Development

Requires Go 1.21+

```bash
go build -o imgup cmd/imgup/main.go
./imgup help
```

## Architecture

- `cmd/imgup/` - CLI entry point
- `pkg/backends/` - Service implementations (Flickr, SmugMug)
- `pkg/oauth/` - OAuth 1.0a implementation
- `pkg/metadata/` - EXIF/IPTC extraction
