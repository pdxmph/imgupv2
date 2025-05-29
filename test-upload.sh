#!/bin/bash

echo "imgupv2 Upload Test"
echo "==================="
echo

# Check if an image was provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <image-file>"
    echo
    echo "Example: $0 photo.jpg"
    exit 1
fi

IMAGE="$1"

# Check if file exists
if [ ! -f "$IMAGE" ]; then
    echo "Error: File not found: $IMAGE"
    exit 1
fi

echo "Testing upload of: $IMAGE"
echo

# Test basic upload
echo "1. Basic upload (URL output):"
./imgup upload "$IMAGE"
echo

# Test with title
echo "2. Upload with title (Markdown output):"
./imgup upload --title "Test Photo" --format markdown "$IMAGE"
echo

# Test with full metadata (JSON output)
echo "3. Upload with metadata (JSON output):"
./imgup upload --title "Test Photo" --description "Uploaded via imgupv2" --format json "$IMAGE"
echo

echo "Upload tests complete!"
