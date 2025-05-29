#!/bin/bash

echo "Testing uploads with different parameters..."
echo

echo "1. No metadata (this works):"
./imgup upload tests/fixtures/test_metadata.jpeg
echo

echo "2. Single word title (let's see if this works):"
./imgup upload --title "Test" tests/fixtures/test_metadata.jpeg
echo

echo "3. Title with space:"
./imgup upload --title "Test Photo" tests/fixtures/test_metadata.jpeg
echo

echo "4. Description only:"
./imgup upload --description "Testing" tests/fixtures/test_metadata.jpeg
echo

echo "5. With debug for title with space:"
IMGUP_DEBUG=1 ./imgup upload --title "Test Photo" tests/fixtures/test_metadata.jpeg 2>&1 | grep -A30 DEBUG || echo "Upload failed"
