# imgupv2 Homebrew Installation

This directory contains the Homebrew Cask formula for imgupv2.

## For Users

Once the Cask is published, you'll be able to install imgupv2 with:

```bash
brew install --cask imgupv2
```

This will install:
- The imgupv2 GUI app
- The imgupv2 hotkey daemon
- The imgup CLI tool

### Post-Installation Setup

1. **Enable Global Hotkey (Option+Shift+I)**:
   - Open System Preferences > Security & Privacy > Privacy > Accessibility
   - Add `imgupv2-hotkey.app` to the list
   - Make sure it's checked

2. **Start the Hotkey Daemon**:
   ```bash
   open -a 'imgupv2-hotkey'
   ```

3. **Optional: Add to Login Items**:
   - System Preferences > Users & Groups > Login Items
   - Add `imgupv2-hotkey.app`

## For Development

### Building a Distribution Package

1. Make sure all components are built:
   ```bash
   # Build CLI
   go build -o imgup ./cmd/imgup
   
   # Build GUI (from gui/ directory)
   wails build
   
   # Build hotkey daemon (from gui/hotkey/ directory)
   go build -o imgupv2-hotkey .
   # Then create .app bundle...
   ```

2. Run the distribution build script:
   ```bash
   ./homebrew/build-dist.sh 0.1.0
   ```

3. This creates `dist/imgupv2-0.1.0-macOS.tar.gz` and shows the SHA256

4. Upload the tarball to GitHub releases

5. Update the Cask formula with:
   - The correct GitHub URL
   - The SHA256 hash

### Testing the Cask Locally

1. Create a test tap:
   ```bash
   mkdir -p ~/homebrew-test
   cd ~/homebrew-test
   cp /path/to/imgupv2.rb .
   ```

2. Install from local file:
   ```bash
   brew install --cask ./imgupv2.rb
   ```

### Submitting to Homebrew

Once tested and working:

1. Fork homebrew/homebrew-cask
2. Add the formula to Casks/i/imgupv2.rb
3. Submit a pull request

## Cask Structure

The Cask installs:
- `/Applications/imgupv2-gui.app` - Main GUI application
- `/Applications/imgupv2-hotkey.app` - Global hotkey daemon
- `/usr/local/bin/imgup` - CLI tool (symlinked)

Uninstalling removes all components and can optionally clean up preferences/logs.
