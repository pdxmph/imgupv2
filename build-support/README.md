# macOS Code Signing & Notarization

This directory contains the infrastructure for properly signing and notarizing imgupv2 for macOS distribution.

## One-Time Setup

1. **Apple Developer Certificate**: Already installed ✓
   - Developer ID: Michael Hall (WS4GXJ44LJ)

2. **Set up notarization credentials**:
   ```bash
   ./build-support/setup-notarization.sh
   ```
   This stores your Apple ID app-specific password in the keychain.

## Building Signed Releases

The main build script now handles signing and notarization automatically:

```bash
./build-macos-release.sh
```

This will:
1. Build all three components (CLI, GUI, hotkey app)
2. Sign each component with your Developer ID
3. Create the release archive
4. Notarize the archive with Apple
5. Output the SHA256 for Homebrew

## Files in this directory

- `entitlements.plist` - Required entitlements for hardened runtime
- `setup-notarization.sh` - One-time setup for Apple notarization

## Testing

After building, you can verify signing:
```bash
codesign -dv --verbose=4 dist/imgupv2-vX.X.X-macOS/imgup
spctl -a -vvv -t install dist/imgupv2-vX.X.X-macOS/imgup
```

## Troubleshooting

If notarization fails:
- Check status: `xcrun notarytool history --keychain-profile "imgupv2-notarization"`
- Get details: `xcrun notarytool log <submission-id> --keychain-profile "imgupv2-notarization"`