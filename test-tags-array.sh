#!/bin/bash

echo "Testing improved tag handling..."
echo

# Create a test image copy
cp tests/fixtures/test_metadata.jpeg /tmp/test-tags.jpeg

echo "1. Writing tags to test image..."
exiftool -overwrite_original \
    -Keywords+="nature" \
    -Keywords+="sunset" \
    -Keywords+="landscape" \
    -Keywords+="mountains" \
    /tmp/test-tags.jpeg

echo
echo "2. Verifying tags were written as array:"
exiftool -Keywords /tmp/test-tags.jpeg
echo

echo "3. Now uploading with imgupv2:"
./imgup upload --tags "nature,sunset,landscape,mountains,golden hour" tests/fixtures/test_metadata.jpeg
echo

echo "Check Flickr - the tags should now appear as separate clickable tags, not one long string!"

# Cleanup
rm -f /tmp/test-tags.jpeg
