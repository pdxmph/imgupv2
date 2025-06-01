#!/bin/bash

# Test script for private Flickr uploads with metadata
# Tests the refactored upload-then-set approach

echo "imgupv2 Private Upload Test Suite"
echo "================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Use the test image that already exists
TEST_IMAGE="../test.jpg"

# Check if test image exists
if [ ! -f "$TEST_IMAGE" ]; then
    echo -e "${RED}Error: test.jpg not found${NC}"
    exit 1
fi

echo -e "\n${YELLOW}Test 1: Private upload with all metadata (title, description, tags)${NC}"
echo "Command: imgup upload $TEST_IMAGE --private --title \"Private Test Photo\" --description \"Testing private upload with metadata\" --tags \"test,private,metadata\" --format json"
../imgup upload "$TEST_IMAGE" --private --title "Private Test Photo" --description "Testing private upload with metadata" --tags "test,private,metadata" --format json

echo -e "\n${YELLOW}Test 2: Private upload with only tags${NC}"
echo "Command: imgup upload $TEST_IMAGE --private --tags \"private,test,refactor\" --format url"
../imgup upload "$TEST_IMAGE" --private --tags "private,test,refactor" --format url

echo -e "\n${YELLOW}Test 3: Private upload with Mastodon integration${NC}"
echo "Command: imgup upload $TEST_IMAGE --private --title \"Private Mastodon Test\" --mastodon --post \"Testing private Flickr upload #imgupv2\" --visibility private"
../imgup upload "$TEST_IMAGE" --private --title "Private Mastodon Test" --mastodon --post "Testing private Flickr upload #imgupv2" --visibility private

echo -e "\n${YELLOW}Test 4: Debug mode - check for metadata embedding${NC}"
echo "This should NOT show 'Embedding metadata...' message"
echo "Command: IMGUP_DEBUG=1 imgup upload $TEST_IMAGE --private --title \"Debug Test\" --tags \"debug,test\""
IMGUP_DEBUG=1 ../imgup upload "$TEST_IMAGE" --private --title "Debug Test" --tags "debug,test" 2>&1 | grep -E "(Embedding metadata|exiftool|tempfile)" || echo -e "${GREEN}✓ No metadata embedding detected${NC}"

echo -e "\n${YELLOW}Test 5: Check that exiftool is not required${NC}"
echo "Moving exiftool temporarily to test dependency-free operation..."
# Save current PATH
OLD_PATH=$PATH
# Remove common exiftool locations from PATH
export PATH=$(echo $PATH | sed 's|/opt/homebrew/bin:||g' | sed 's|/usr/local/bin:||g')
../imgup upload "$TEST_IMAGE" --private --title "No Exiftool Test" --tags "no-exiftool"
# Restore PATH
export PATH=$OLD_PATH

echo -e "\n${GREEN}All tests completed!${NC}"
echo -e "Please verify on Flickr that:"
echo -e "1. All uploaded photos are marked as private"
echo -e "2. Titles, descriptions, and tags were properly set"
echo -e "3. No temporary files were left behind"
