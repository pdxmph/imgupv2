#!/bin/bash
set -e

VERSION="v0.3.0"
RELEASE_NAME="imgupv2-${VERSION}-macOS"
RELEASE_DIR="dist/${RELEASE_NAME}"

echo "Building macOS release ${VERSION}..."

# Clean up any existing release directory
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

# Build the CLI binary
echo "Building CLI..."
cd cmd/imgup && go build -ldflags "-X 'main.version=${VERSION}' -X 'main.commit=$(git rev-parse --short HEAD)' -X 'main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o "../../${RELEASE_DIR}/imgup" .
cd ../..

# Build the GUI app
echo "Building GUI..."
cd gui && ~/go/bin/wails build -clean
cp -r build/bin/imgupv2-gui.app "../${RELEASE_DIR}/"
cd ..

# Create the archive
echo "Creating archive..."
cd dist
tar -czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}"
cd ..

# Calculate SHA256
echo "Calculating SHA256..."
SHA256=$(shasum -a 256 "dist/${RELEASE_NAME}.tar.gz" | cut -d' ' -f1)
echo "SHA256: ${SHA256}"

echo "Release archive created: dist/${RELEASE_NAME}.tar.gz"
echo "SHA256: ${SHA256}"
