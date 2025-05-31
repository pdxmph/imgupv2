#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get version from argument or prompt
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo -n "Enter version (e.g., v0.4.1): "
    read VERSION
fi

# Validate version format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo -e "${RED}Error: Version must be in format vX.Y.Z${NC}"
    exit 1
fi

echo -e "${GREEN}🚀 Starting release process for $VERSION${NC}"

# Step 1: Update version in build script
echo -e "\n${YELLOW}Step 1: Updating version in build script...${NC}"
sed -i.bak "s/VERSION=\"v[0-9]*\.[0-9]*\.[0-9]*\"/VERSION=\"$VERSION\"/" build-macos-release.sh
rm build-macos-release.sh.bak

# Step 2: Commit version bump
echo -e "\n${YELLOW}Step 2: Committing version bump...${NC}"
git add build-macos-release.sh
git commit -m "Bump version to $VERSION" || echo "No changes to commit"

# Step 3: Create and push tag
echo -e "\n${YELLOW}Step 3: Creating tag...${NC}"
if git tag | grep -q "^${VERSION}$"; then
    echo "Tag $VERSION already exists"
else
    echo "Enter release notes (press Ctrl-D when done):"
    RELEASE_NOTES=$(cat)
    git tag -a "$VERSION" -m "$RELEASE_NOTES"
fi

# Step 4: Push commits and tags
echo -e "\n${YELLOW}Step 4: Pushing to GitHub...${NC}"
git push
git push origin "$VERSION"

# Step 5: Build Linux releases with goreleaser
echo -e "\n${YELLOW}Step 5: Building Linux releases...${NC}"
goreleaser release --snapshot --clean --skip=publish

# Step 6: Build macOS release (signed and notarized)
echo -e "\n${YELLOW}Step 6: Building macOS release...${NC}"
./build-macos-release.sh

# Step 7: Create checksums
echo -e "\n${YELLOW}Step 7: Creating checksums...${NC}"
cd dist
shasum -a 256 *.tar.gz > checksums-complete.txt
cd ..

# Step 8: Update Homebrew cask
echo -e "\n${YELLOW}Step 8: Updating Homebrew cask...${NC}"
SHA256=$(shasum -a 256 "dist/imgupv2-${VERSION}-macOS.tar.gz" | cut -d' ' -f1)
sed -i.bak "s/version \"[0-9]*\.[0-9]*\.[0-9]*\"/version \"${VERSION#v}\"/" homebrew/imgupv2.rb
sed -i.bak "s/sha256 \"[a-f0-9]*\"/sha256 \"$SHA256\"/" homebrew/imgupv2.rb
rm homebrew/imgupv2.rb.bak

# Step 9: Commit and push Homebrew updates
echo -e "\n${YELLOW}Step 9: Committing Homebrew updates...${NC}"
git add homebrew/imgupv2.rb
git commit -m "Update Homebrew cask for $VERSION"
git push

# Step 10: Copy to Homebrew tap
echo -e "\n${YELLOW}Step 10: Updating Homebrew tap...${NC}"
cp homebrew/imgupv2.rb /opt/homebrew/Library/Taps/pdxmph/homebrew-tap/Casks/
cd /opt/homebrew/Library/Taps/pdxmph/homebrew-tap
git add .
git commit -m "Update imgupv2 to $VERSION"
git push origin main
cd -

# Step 11: Create GitHub release
echo -e "\n${YELLOW}Step 11: Creating GitHub release...${NC}"
gh release create "$VERSION" \
  --title "$VERSION" \
  --notes "$(git tag -l --format='%(contents)' "$VERSION")" \
  dist/imgupv2-${VERSION}-macOS.tar.gz \
  dist/imgupv2_Darwin_x86_64.tar.gz \
  dist/imgupv2_Linux_x86_64.tar.gz \
  dist/imgupv2_Linux_arm64.tar.gz \
  dist/checksums-complete.txt

echo -e "\n${GREEN}✅ Release $VERSION completed successfully!${NC}"
echo -e "${GREEN}View at: https://github.com/pdxmph/imgupv2/releases/tag/$VERSION${NC}"

# Step 12: Test installation
echo -e "\n${YELLOW}Step 12: Testing installation...${NC}"
echo "Run these commands to test:"
echo "  brew untap pdxmph/tap"
echo "  brew tap pdxmph/tap"
echo "  brew install --cask pdxmph/tap/imgupv2"
