#!/bin/bash
# Test script for imgupv2 GUI with Bluesky

echo "Testing imgupv2 GUI with Bluesky support"
echo "========================================"
echo
echo "To test the GUI with Bluesky:"
echo
echo "1. Launch the GUI app:"
echo "   open gui/build/bin/imgupv2-gui.app"
echo
echo "2. Or run in development mode with console:"
echo "   cd gui && wails dev"
echo
echo "3. Select an image in Finder first"
echo
echo "4. In the GUI:"
echo "   - Check 'Post to Bluesky'"
echo "   - Enter post text"
echo "   - Note: When both Mastodon and Bluesky are checked,"
echo "     the post text is shared between them"
echo
echo "5. Click Upload to test"
echo
echo "Remember:"
echo "- All Bluesky posts are public"
echo "- 300 character limit for Bluesky"
echo "- URLs are automatically made clickable"
