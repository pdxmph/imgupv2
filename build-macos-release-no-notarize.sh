#!/bin/bash
set -e

VERSION="${1:-v0.5.0}"
RELEASE_NAME="imgupv2-${VERSION}-macOS"
RELEASE_DIR="dist/${RELEASE_NAME}"

# Signing identity
SIGNING_IDENTITY="Developer ID Application: Michael Hall (WS4GXJ44LJ)"
ENTITLEMENTS="build-support/entitlements.plist"

echo "Building macOS release ${VERSION} (without notarization)..."

# Find wails binary - check multiple locations
WAILS_PATH=""
WAILS_SEARCH_PATHS=(
  "/opt/homebrew/bin/wails"     # Apple Silicon Homebrew
  "/usr/local/bin/wails"         # Intel Homebrew
  "$HOME/go/bin/wails"          # Go install fallback
  "wails"                        # PATH fallback
)

for path in "${WAILS_SEARCH_PATHS[@]}"; do
  if command -v "$path" &> /dev/null || [ -f "$path" ]; then
    WAILS_PATH="$path"
    echo "Found wails at: $WAILS_PATH"
    break
  fi
done

if [ -z "$WAILS_PATH" ]; then
  echo "Error: wails not found. Please install via 'brew install wails' or 'go install github.com/wailsapp/wails/v2/cmd/wails@latest'"
  exit 1
fi

# Clean up any existing release directory
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

# Build the CLI binary
echo "Building CLI..."
cd cmd/imgup && go build -ldflags "-X 'main.version=${VERSION}' -X 'main.commit=$(git rev-parse --short HEAD)' -X 'main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o "../../${RELEASE_DIR}/imgup" .
cd ../..

# Sign the CLI binary
echo "Signing CLI binary..."
codesign --deep --force --verify --verbose \
  --sign "${SIGNING_IDENTITY}" \
  --options runtime \
  --entitlements "${ENTITLEMENTS}" \
  "${RELEASE_DIR}/imgup"

# Build the GUI app
echo "Building GUI..."
cd gui
$WAILS_PATH build -clean -platform darwin/universal
cp -r build/bin/imgupv2-gui.app "../${RELEASE_DIR}/"
cd ..

# Sign the GUI app
echo "Signing GUI app..."
codesign --deep --force --verify --verbose \
  --sign "${SIGNING_IDENTITY}" \
  --options runtime \
  --entitlements build-support/entitlements.plist \
  "${RELEASE_DIR}/imgupv2-gui.app"

# Create the final archive for distribution
echo "Creating distribution archive..."
cd dist
tar -czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}"
cd ..

# Calculate SHA256
echo "Calculating SHA256..."
SHA256=$(shasum -a 256 "dist/${RELEASE_NAME}.tar.gz" | cut -d' ' -f1)

echo "Build complete!"
echo ""
echo "Archive: dist/${RELEASE_NAME}.tar.gz"
echo "SHA256: ${SHA256}"
echo ""
echo "Note: This build is signed but NOT notarized."
echo "Users may need to right-click and select 'Open' on first launch."
echo ""
echo "To enable notarization, set up your credentials with:"
echo "xcrun notarytool store-credentials 'imgupv2-notarization' --apple-id YOUR_APPLE_ID --team-id WS4GXJ44LJ"