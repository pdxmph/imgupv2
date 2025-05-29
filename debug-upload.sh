#!/bin/bash

echo "Testing upload with title and description..."
echo

# Enable debug mode to see what's happening
export IMGUP_DEBUG=1

./imgup upload --title "Test Photo" --description "Testing imgupv2" tests/fixtures/test_metadata.jpeg

echo
echo "If this fails, try without description:"
./imgup upload --title "Test Photo" tests/fixtures/test_metadata.jpeg

echo
echo "Or just description:"
./imgup upload --description "Testing imgupv2" tests/fixtures/test_metadata.jpeg
