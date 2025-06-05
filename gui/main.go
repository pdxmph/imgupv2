package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

// main is the application entry point
func main() {
	// Parse command line arguments
	var pullDataPath string
	flag.StringVar(&pullDataPath, "pull-data", "", "Path to pull request JSON file")
	flag.Parse()
	
	// Create an instance of the app structure
	app := NewApp()
	
	// If pull data is provided, set it up for loading after startup
	if pullDataPath != "" {
		fmt.Printf("DEBUG: Pull data path provided: %s\n", pullDataPath)
		app.pullDataPath = pullDataPath
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "imgupv2",
		Width:     900,
		Height:    500,
		MinWidth:  800,
		MinHeight: 450,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
			},
			About: &mac.AboutInfo{
				Title:   "imgupv2",
				Message: "Fast image upload tool for photographers",
			},
			// Allow the app to keep running when window is closed
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		StartHidden: false,  // Show window normally
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
