# imgupv2 Homebrew Installation

This directory contains the Homebrew Cask formula for imgupv2.

## For Users

Once the Cask is published, you'll be able to install imgupv2 with:

```bash
brew install --cask imgupv2
```

This will install:
- The imgupv2 GUI app
- The imgup CLI tool

### Post-Installation Setup

**For Quick Access**, create a keyboard shortcut:
1. Open Shortcuts.app
2. Create new shortcut with 'Run Shell Script' action
3. Enter: `open -a imgupv2-gui`
4. Add keyboard shortcut (e.g., ⌘⇧U)

## For Development

### Building a Distribution Package

1. Make sure all components are built:
   ```bash
   # Build CLI
   go build -o imgup ./cmd/imgup
   
   # Build GUI (from gui/ directory)
   wails build
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
- `/usr/local/bin/imgup` - CLI tool (symlinked)

Uninstalling removes all components and can optionally clean up preferences/logs.
