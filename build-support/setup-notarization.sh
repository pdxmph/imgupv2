#!/bin/bash
# setup-notarization.sh - One-time setup for notarization credentials

echo "Setting up notarization credentials..."
echo ""
echo "You'll need an app-specific password from Apple ID:"
echo "1. Go to https://appleid.apple.com/account/manage"
echo "2. Sign in and go to 'Sign-In and Security'"
echo "3. Select 'App-Specific Passwords'"
echo "4. Click '+' to generate a new password"
echo "5. Name it 'imgupv2-notarization'"
echo "6. Copy the generated password"
echo ""
read -p "Press Enter when you have your app-specific password ready..."

# Store the password in keychain
xcrun notarytool store-credentials "imgupv2-notarization" \
  --apple-id "mike@puddingtime.org" \
  --team-id "WS4GXJ44LJ"

echo ""
echo "✅ Credentials stored! You can now use notarization."