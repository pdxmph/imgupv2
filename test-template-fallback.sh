#!/bin/bash

# Test the template engine more directly

echo "Test 1: Just alt text"
./imgup upload --format html --alt "My custom alt text" tests/fixtures/test_metadata.jpeg

echo -e "\nTest 2: Just description"  
./imgup upload --format html --description "My description" tests/fixtures/test_metadata.jpeg

echo -e "\nTest 3: Just title"
./imgup upload --format html --title "My title" tests/fixtures/test_metadata.jpeg

echo -e "\nTest 4: Empty (should fall back to filename)"
./imgup upload --format html tests/fixtures/test_metadata.jpeg
