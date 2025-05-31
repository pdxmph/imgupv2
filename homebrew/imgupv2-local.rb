cask "imgupv2-local" do
  version "0.1.0"
  sha256 :no_check # For local testing

  # Use local file for testing
  url "file://#{Dir.pwd}/dist/imgupv2-#{version}-macOS.tar.gz"
  name "imgupv2"
  desc "Fast image uploader for Flickr with metadata review"
  homepage "https://github.com/yourusername/imgupv2"

  depends_on macos: ">= :monterey"

  # The tarball extracts to imgupv2-VERSION-macOS/
  # Install the apps from within that directory
  app "imgupv2-#{version}-macOS/imgupv2-gui.app"
  app "imgupv2-#{version}-macOS/imgupv2-hotkey.app"

  # Install the CLI binary
  binary "imgupv2-#{version}-macOS/imgup"

  postflight do
    # Instructions for accessibility permissions
    ohai "imgupv2 installed successfully!"
    ohai ""
    ohai "IMPORTANT: To enable the global hotkey (Option+Shift+I):"
    ohai "1. Open System Settings > Privacy & Security > Accessibility"
    ohai "2. Click the + button to add an app"
    ohai "3. Navigate to /Applications and select imgupv2-hotkey.app"
    ohai "4. Make sure the toggle is enabled"
    ohai ""
    ohai "To start the hotkey daemon:"
    ohai "  open -a 'imgupv2-hotkey'"
    ohai ""
    ohai "To add it to Login Items:"
    ohai "  System Settings > General > Login Items"
  end

  uninstall quit: [
              "com.imgupv2.gui",
              "com.imgupv2.hotkey"
            ]

  zap trash: [
    "~/Library/Application Support/imgupv2",
    "~/Library/Preferences/com.imgupv2.plist",
    "~/Library/Logs/imgupv2"
  ]
end
