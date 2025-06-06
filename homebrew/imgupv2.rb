cask "imgupv2" do
  version "0.12.0"
  sha256 "d8e538cac40bd52f537b7889e794144b9ed9fadcd5c614d583687ca9ba911464"

  url "https://github.com/pdxmph/imgupv2/releases/download/v#{version}/imgupv2-v#{version}-macOS.tar.gz"
  name "imgupv2"
  desc "Fast image uploader for Flickr with metadata review"
  homepage "https://github.com/pdxmph/imgupv2"

  depends_on macos: ">= :monterey"

  # The tarball extracts to imgupv2-vVERSION-macOS/
  app "imgupv2-v#{version}-macOS/imgupv2-gui.app"

  # Install the CLI binary
  binary "imgupv2-v#{version}-macOS/imgup"

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

  caveats <<~EOS
    The imgupv2 CLI tool has been installed to:
      #{HOMEBREW_PREFIX}/bin/imgup
  EOS
end
