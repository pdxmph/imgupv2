#!/bin/bash

echo "Testing template system..."
echo

echo "1. Show default templates:"
./imgup config show
echo

echo "2. Test default formats:"
echo "  URL format:"
./imgup upload --format url tests/fixtures/test_metadata.jpeg
echo
echo "  Edit URL format:"
./imgup upload --format edit_url tests/fixtures/test_metadata.jpeg
echo
echo "  HTML format:"
./imgup upload --format html --alt "Test photo with metadata" tests/fixtures/test_metadata.jpeg
echo
echo "  Markdown format (with fallback):"
./imgup upload --format markdown --title "Template Test" tests/fixtures/test_metadata.jpeg
echo

echo "3. Add custom template:"
./imgup config set template.org "#+CAPTION: %title%|%filename%
[[%image_url%]]"
echo

echo "4. Test custom template:"
./imgup upload --format org --title "Org Mode Test" tests/fixtures/test_metadata.jpeg
echo

echo "5. Show updated config:"
./imgup config show | grep -A10 Templates
