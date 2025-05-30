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

# Install to /usr/local/bin (macOS) or appropriate location
install: build
	@if [ -f build/bin/imgupv2-gui.app/Contents/MacOS/imgupv2-gui ]; then \
		echo "Installing macOS app..."; \
		cp -r build/bin/imgupv2-gui.app /Applications/; \
		echo "Creating command line launcher..."; \
		echo '#!/bin/bash\n/Applications/imgupv2-gui.app/Contents/MacOS/imgupv2-gui' > /usr/local/bin/imgupv2-gui; \
		chmod +x /usr/local/bin/imgupv2-gui; \
	elif [ -f build/bin/imgupv2-gui ]; then \
		echo "Installing Linux binary..."; \
		sudo cp build/bin/imgupv2-gui /usr/local/bin/; \
	fi

# Create a macOS service
service-mac:
	@echo "Creating Quick Action for macOS..."
	@echo "Please use Automator to create a Quick Action that runs:"
	@echo "/Applications/imgupv2-gui.app/Contents/MacOS/imgupv2-gui"
