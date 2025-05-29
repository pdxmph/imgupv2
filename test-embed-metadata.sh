#!/bin/bash

echo "Testing metadata embedding approach..."
echo

echo "1. Upload with title and description (embedded in image):"
./imgup upload --title "Test Photo with Metadata" --description "This metadata was embedded in the image before upload" tests/fixtures/test_metadata.jpeg

echo
echo "2. Let's verify what metadata Flickr sees:"
echo "(Check the photo on Flickr - it should have the title and description)"
