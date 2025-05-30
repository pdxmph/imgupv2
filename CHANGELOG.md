# Changelog

All notable changes to imgupv2 will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-05-30

### Added
- Initial release of imgupv2 (Go rewrite of imgup-cli)
- Flickr OAuth authentication with built-in callback server
- Image upload to Flickr with metadata preservation
- Multiple output formats: URL, Markdown, HTML, JSON
- Configuration management via `config` command
- Private photo handling
- EXIF metadata extraction (when exiftool available)
- Support for macOS and Linux

### Changed
- Complete rewrite from Ruby to Go
- Improved OAuth flow - no more manual URL copying
- Faster startup and execution times
- Single static binary distribution

### Removed
- SmugMug support (planned for future release)
- GUI components (planned for future release)

[unreleased]: https://github.com/pdxmph/imgupv2/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/pdxmph/imgupv2/releases/tag/v0.1.0
