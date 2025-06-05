#!/bin/bash
# Launch imgupv2-gui with pull data

PULL_DATA_PATH="$1"
APP_PATH="$HOME/code/imgupv2/gui/build/bin/imgupv2-gui.app"
BINARY_PATH="$APP_PATH/Contents/MacOS/imgupv2-gui"

echo "DEBUG: launch-pull.sh called with: $PULL_DATA_PATH"
echo "DEBUG: Binary path: $BINARY_PATH"

if [ -f "$BINARY_PATH" ]; then
    echo "DEBUG: Launching GUI with pull data..."
    # Make sure the app comes to foreground on macOS
    osascript -e 'tell application "System Events" to set frontmost of process "imgupv2-gui" to true' 2>/dev/null || true
    exec "$BINARY_PATH" --pull-data "$PULL_DATA_PATH"
else
    echo "ERROR: GUI binary not found at $BINARY_PATH"
    exit 1
fi
