#!/bin/bash

echo "Testing tag/keyword support..."
echo

echo "1. Upload with tags only:"
./imgup upload --tags "nature,sunset,landscape,test" tests/fixtures/test_metadata.jpeg
echo

echo "2. Upload with title, description, and tags:"
./imgup upload --title "Beautiful Sunset" --description "A stunning sunset over the mountains" --tags "nature,sunset,landscape,mountains,golden hour" tests/fixtures/test_metadata.jpeg
echo

echo "3. Test the --help to see new option:"
./imgup upload --help 2>&1 | grep -A1 tags
echo

echo "Check the photos on Flickr - they should have the tags applied!"
