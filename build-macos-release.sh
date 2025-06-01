#!/bin/bash
set -e

VERSION="v0.6.0"
RELEASE_NAME="imgupv2-${VERSION}-macOS"
RELEASE_DIR="dist/${RELEASE_NAME}"

# Signing identity
SIGNING_IDENTITY="Developer ID Application: Michael Hall (WS4GXJ44LJ)"
ENTITLEMENTS="build-support/entitlements.plist"

echo "Building macOS release ${VERSION}..."

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
cd gui && "$WAILS_PATH" build -clean

# Apply custom GUI icon
if [ -f build/appicon.icns ]; then
    echo "Applying custom GUI icon..."
    cp build/appicon.icns build/bin/imgupv2-gui.app/Contents/Resources/iconfile.icns
fi

cp -r build/bin/imgupv2-gui.app "../${RELEASE_DIR}/"
cd ..

# Sign the GUI app
echo "Signing GUI app..."
codesign --deep --force --verify --verbose \
  --sign "${SIGNING_IDENTITY}" \
  --options runtime \
  --entitlements build-support/entitlements.plist \
  "${RELEASE_DIR}/imgupv2-gui.app"

# Create a temporary zip for notarization
echo "Creating temporary zip for notarization..."
cd dist
zip -r "${RELEASE_NAME}-notarize.zip" "${RELEASE_NAME}"
cd ..

# Notarize the zip
echo "Notarizing applications..."
echo "(Using stored credentials from keychain)"

xcrun notarytool submit "dist/${RELEASE_NAME}-notarize.zip" \
  --keychain-profile "imgupv2-notarization" \
  --wait

# Remove the temporary zip
rm "dist/${RELEASE_NAME}-notarize.zip"

# Staple the notarization to the apps
echo "Stapling notarization tickets..."
xcrun stapler staple "${RELEASE_DIR}/imgupv2-gui.app"
# Note: Can't staple CLI binaries, only app bundles

# Create the final archive for distribution
echo "Creating distribution archive..."
cd dist
tar -czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}"
cd ..

# Calculate SHA256
echo "Calculating SHA256..."
SHA256=$(shasum -a 256 "dist/${RELEASE_NAME}.tar.gz" | cut -d' ' -f1)

echo "Release archive created: dist/${RELEASE_NAME}.tar.gz"
echo "SHA256: ${SHA256}"
echo ""
echo "✅ Build complete and notarized!"
echo "Next steps:"
echo "1. Update homebrew/imgupv2.rb with SHA256: ${SHA256}"
echo "2. Create GitHub release and upload the archive"