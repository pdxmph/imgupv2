#!/bin/bash
# Build script for creating imgupv2 distribution package

set -euo pipefail

VERSION="${1:-0.2.0}"
DIST_DIR="dist/imgupv2-${VERSION}-macOS"

echo "Building imgupv2 distribution package v${VERSION}..."

# Clean up previous builds
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# Build the CLI if needed
echo "Building CLI..."
go build -o imgup ./cmd/imgup

# Sign the CLI binary
echo "Signing CLI binary..."
codesign --force --deep --sign - imgup

# Copy components
echo "Copying components..."
cp imgup "$DIST_DIR/"
cp -R gui/build/bin/imgupv2-gui.app "$DIST_DIR/"
cp -R gui/hotkey/imgupv2-hotkey.app "$DIST_DIR/"

# Sign the apps (they should already be signed by Wails, but let's make sure)
echo "Signing apps..."
codesign --force --deep --sign - "$DIST_DIR/imgupv2-gui.app"
codesign --force --deep --sign - "$DIST_DIR/imgupv2-hotkey.app"

# Create the tarball
echo "Creating tarball..."
cd dist
tar -czf "imgupv2-${VERSION}-macOS.tar.gz" "imgupv2-${VERSION}-macOS"
cd ..

# Calculate SHA256
echo "Calculating SHA256..."
SHA256=$(shasum -a 256 "dist/imgupv2-${VERSION}-macOS.tar.gz" | cut -d' ' -f1)

echo ""
echo "Distribution package created: dist/imgupv2-${VERSION}-macOS.tar.gz"
echo "SHA256: $SHA256"
echo ""
echo "Update the Cask with this SHA256 and upload the tarball to GitHub releases"
