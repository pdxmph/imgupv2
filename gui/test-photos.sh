#!/bin/bash

# Make sure a photo is selected in Photos first
osascript -e 'tell application "Photos" to activate'

# Give Photos a moment to become frontmost
sleep 0.5

# Now launch the GUI
cd /Users/mph/code/imgupv2/gui && /Users/mph/go/bin/wails dev
