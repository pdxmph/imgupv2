#!/bin/bash
# Launch imgupv2 GUI

# Find the app in common locations
if [ -d "$HOME/code/imgupv2/gui/build/bin/imgupv2-gui.app" ]; then
    open "$HOME/code/imgupv2/gui/build/bin/imgupv2-gui.app"
elif [ -d "/Applications/imgupv2-gui.app" ]; then
    open "/Applications/imgupv2-gui.app"
elif [ -d "$HOME/Applications/imgupv2-gui.app" ]; then
    open "$HOME/Applications/imgupv2-gui.app"
else
    echo "imgupv2-gui.app not found!"
    exit 1
fi
