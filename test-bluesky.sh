#!/bin/bash
# Test script for imgupv2 Bluesky integration

echo "Testing imgupv2 Bluesky integration..."
echo

# Create a test image if it doesn't exist
if [ ! -f test-bluesky.jpg ]; then
    echo "Creating test image..."
    # Create a small test image using ImageMagick (if available) or touch
    if command -v convert &> /dev/null; then
        convert -size 100x100 xc:blue test-bluesky.jpg
    else
        touch test-bluesky.jpg
    fi
fi

echo "1. Testing dry-run with Bluesky only:"
echo "======================================"
./imgup upload test-bluesky.jpg --bluesky --post "Testing imgupv2 with Bluesky!" --tags "test,imgupv2" --dry-run
echo

echo "2. Testing dry-run with crossposting:"
echo "====================================="
./imgup upload test-bluesky.jpg --mastodon --bluesky --post "Crossposting test from imgupv2!" --visibility public --dry-run
echo

echo "3. Testing warning for direct visibility with Bluesky:"
echo "======================================================"
./imgup upload test-bluesky.jpg --mastodon --bluesky --post "Private test" --visibility direct --dry-run
echo

echo "4. Testing character limit warning:"
echo "==================================="
LONG_TEXT="This is a very long post that will definitely exceed Bluesky's 300 character limit. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. This should trigger the warning!"
./imgup upload test-bluesky.jpg --bluesky --post "$LONG_TEXT" --dry-run
