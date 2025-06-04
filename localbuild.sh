#!/bin/bash
cd ~/code/imgupv2

# Build CLI
go build -o imgup cmd/imgup/main.go
sudo cp imgup /usr/local/bin/imgup

# Build GUI
cd gui
wails build -clean

echo "CLI installed to /usr/local/bin/imgup"
echo "GUI at ~/code/imgupv2/gui/build/bin/imgupv2-gui.app"
echo "Copy GUI to Applications if desired"
