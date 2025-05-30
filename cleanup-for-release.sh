#!/bin/bash
# Cleanup script for imgupv2 release
# Run this before committing for a clean repository

echo "Cleaning up test files for release..."

# Remove all test shell scripts
rm -f test-*.sh
rm -f debug-upload.sh

# Remove test Go files from root
rm -f test_extract.go
rm -f test_oauth_sig.go
rm -f test-oauth-callback.go

# Remove test Python files
rm -f test-gui-python.py
rm -rf tools/

# Remove empty test-config directory
rmdir test-config/ 2>/dev/null

# Remove compiled binary (should not be in git)
rm -f imgup

# Clean up dist directory (already in .gitignore)
rm -rf dist/

echo "Cleanup complete!"
echo ""
echo "Files remaining:"
ls -la

echo ""
echo "Don't forget to:"
echo "1. git add -A"
echo "2. git commit -m 'Clean up test scripts for release'"
echo "3. git push"
