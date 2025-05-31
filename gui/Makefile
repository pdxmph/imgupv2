.PHONY: build dev clean install

# Build the Wails app
build:
	wails build -clean

# Run in development mode with hot reload
dev:
	wails dev

# Build for macOS specifically
build-mac:
	wails build -platform darwin/universal -clean

# Build for Linux
build-linux:
	wails build -platform linux/amd64 -clean

# Clean build artifacts
clean:
	rm -rf build/bin
	rm -f imgupv2-gui

# Install to user locations (no sudo required)
install: build install-icons
	@if [ -f build/bin/imgupv2-gui.app/Contents/MacOS/imgupv2-gui ]; then \
		echo "Installing macOS app to ~/Applications..."; \
		mkdir -p ~/Applications; \
		cp -r build/bin/imgupv2-gui.app ~/Applications/; \
		echo "Creating command line launcher..."; \
		mkdir -p ~/bin; \
		echo '#!/bin/bash' > ~/bin/imgupv2-gui; \
		echo 'exec ~/Applications/imgupv2-gui.app/Contents/MacOS/imgupv2-gui "$$@"' >> ~/bin/imgupv2-gui; \
		chmod +x ~/bin/imgupv2-gui; \
		echo "✓ GUI installed to ~/Applications/imgupv2-gui.app"; \
		echo "✓ CLI launcher at ~/bin/imgupv2-gui (add ~/bin to PATH if needed)"; \
	elif [ -f build/bin/imgupv2-gui ]; then \
		echo "Installing Linux binary..."; \
		mkdir -p ~/bin; \
		cp build/bin/imgupv2-gui ~/bin/; \
		chmod +x ~/bin/imgupv2-gui; \
	fi

# Apply custom icons after build
install-icons:
	@if [ -f build/appicon.icns ] && [ -f build/bin/imgupv2-gui.app/Contents/Resources/iconfile.icns ]; then \
		echo "Applying custom GUI icon..."; \
		cp build/appicon.icns build/bin/imgupv2-gui.app/Contents/Resources/iconfile.icns; \
	fi
