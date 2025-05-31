#!/bin/bash
set -e

VERSION="v0.3.2"
RELEASE_NAME="imgupv2-${VERSION}-macOS"
RELEASE_DIR="dist/${RELEASE_NAME}"

# Signing identity
SIGNING_IDENTITY="Developer ID Application: Michael Hall (WS4GXJ44LJ)"
ENTITLEMENTS="build-support/entitlements.plist"

echo "Building macOS release ${VERSION}..."

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
cd gui && ~/go/bin/wails build -clean
cp -r build/bin/imgupv2-gui.app "../${RELEASE_DIR}/"
cd ..

# Sign the GUI app
echo "Signing GUI app..."
codesign --deep --force --verify --verbose \
  --sign "${SIGNING_IDENTITY}" \
  --options runtime \
  --entitlements build-support/entitlements.plist \
  "${RELEASE_DIR}/imgupv2-gui.app"

# Build the hotkey app
echo "Building hotkey daemon..."
cd gui/hotkey
go build -o imgupv2-hotkey main.go
# Ensure the app bundle exists
if [ ! -d "imgupv2-hotkey.app" ]; then
    echo "Error: imgupv2-hotkey.app bundle not found"
    exit 1
fi
# Copy the binary into the app bundle
cp imgupv2-hotkey imgupv2-hotkey.app/Contents/MacOS/
# Copy the complete app bundle to release
cp -r imgupv2-hotkey.app "../../${RELEASE_DIR}/"
cd ../..

# Sign the hotkey app
echo "Signing hotkey app..."
codesign --deep --force --verify --verbose \
  --sign "${SIGNING_IDENTITY}" \
  --options runtime \
  --entitlements build-support/entitlements.plist \
  "${RELEASE_DIR}/imgupv2-hotkey.app"

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
xcrun stapler staple "${RELEASE_DIR}/imgupv2-hotkey.app"
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