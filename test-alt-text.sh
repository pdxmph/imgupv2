#!/bin/bash

echo "Testing accessibility-focused alt text..."
echo

echo "1. With explicit alt text (best practice):"
./imgup upload --alt "Silhouette of person photographing sunset over mountain range" --format markdown tests/fixtures/test_metadata.jpeg
echo

echo "2. Without alt text (shows tip):"
./imgup upload --title "Sunset Photo" --format markdown tests/fixtures/test_metadata.jpeg
echo

echo "3. With description but no alt (uses description as alt):"
./imgup upload --title "Sunset" --description "A photographer captures the golden hour light" --format markdown tests/fixtures/test_metadata.jpeg
echo

echo "4. Multiple options (alt text takes priority):"
./imgup upload --title "Sunset" --description "Beautiful sunset" --alt "Person with camera on tripod photographing orange and purple sunset" --format markdown tests/fixtures/test_metadata.jpeg
