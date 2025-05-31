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

  # Install the CLI binary
  binary "imgupv2-#{version}-macOS/imgup"

  postflight do
    # Instructions for launching
    ohai "imgupv2 installed successfully!"
    ohai ""
    ohai "To use imgupv2:"
    ohai "• GUI: Open imgupv2-gui from Applications or Spotlight"
    ohai "• CLI: Use 'imgup' command in Terminal"
    ohai ""
    ohai "For quick access, create a keyboard shortcut:"
    ohai "1. Open Shortcuts.app"
    ohai "2. Create new shortcut with 'Run Shell Script' action"
    ohai "3. Enter: open -a imgupv2-gui"
    ohai "4. Add keyboard shortcut (e.g., ⌘⇧U)"
  end

  uninstall quit: "com.imgupv2.gui"

  zap trash: [
    "~/Library/Application Support/imgupv2",
    "~/Library/Preferences/com.imgupv2.plist",
    "~/Library/Logs/imgupv2"
  ]
end
