#!/bin/bash

echo "imgupv2 OAuth Test"
echo "=================="
echo

# Check current config
echo "Current configuration:"
./imgup config show
echo

# Check if credentials are set
if ./imgup config show | grep -q "(not set)"; then
    echo "⚠️  You need to set up your Flickr API credentials first!"
    echo
    echo "Follow the instructions in docs/flickr-setup.md:"
    echo "1. Get your API key from https://www.flickr.com/services/apps/create/"
    echo "2. Run: ./imgup config set flickr.key YOUR_KEY"
    echo "3. Run: ./imgup config set flickr.secret YOUR_SECRET"
    echo
    exit 1
fi

echo "Ready to authenticate with Flickr!"
echo "Press Enter to continue..."
read

./imgup auth flickr
