cask "imgupv2" do
  version "0.2.0"
  sha256 "5c750982144937b508c4c6f3b8e734a7e9702b240b81f3b718d2446a4c4e2416"

  url "https://github.com/pdxmph/imgupv2/releases/download/v#{version}/imgupv2-#{version}-macOS.tar.gz"
  name "imgupv2"
  desc "Fast image uploader for Flickr with metadata review"
  homepage "https://github.com/YOURUSERNAME/imgupv2"

  depends_on macos: ">= :monterey"

  # The tarball extracts to imgupv2-VERSION-macOS/
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

  caveats <<~EOS
    The imgupv2 CLI tool has been installed to:
      #{HOMEBREW_PREFIX}/bin/imgup

    The first time you run the apps or CLI, macOS may show a security warning.
    You can allow them in System Settings > Privacy & Security.
  EOS
end
