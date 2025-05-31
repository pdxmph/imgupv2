#!/bin/bash
set -e

# Configuration
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo "Usage: ./release.sh v0.4.0"
    exit 1
fi

# Ensure we're in the right directory
cd "$(dirname "$0")"

# Check if tag exists
if ! git tag | grep -q "^${VERSION}$"; then
    echo "Error: Tag ${VERSION} does not exist"
    echo "Create it with: git tag -a ${VERSION} -m 'Release ${VERSION}'"
    exit 1
fi

# Check if release already exists
if gh release view "$VERSION" &>/dev/null; then
    echo "Release $VERSION already exists. Delete it first with:"
    echo "  gh release delete $VERSION"
    exit 1
fi

# Generate release notes from tag annotation
echo "Extracting release notes from tag..."
RELEASE_NOTES=$(git tag -l --format='%(contents)' "$VERSION")

# Create the release
echo "Creating GitHub release $VERSION..."
gh release create "$VERSION" \
  --title "$VERSION" \
  --notes "$RELEASE_NOTES" \
  dist/imgupv2-${VERSION}-macOS.tar.gz \
  dist/imgupv2_Linux_x86_64.tar.gz \
  dist/imgupv2_Linux_arm64.tar.gz \
  dist/checksums-complete.txt

echo "✅ Release $VERSION created successfully!"
echo ""
echo "View it at: https://github.com/pdxmph/imgupv2/releases/tag/$VERSION"
