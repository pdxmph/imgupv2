package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
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
	// Debug: Log raw arguments
	fmt.Printf("DEBUG: main() started with %d args\n", len(os.Args))
	for i, arg := range os.Args {
		fmt.Printf("DEBUG: arg[%d] = %s\n", i, arg)
	}
	
	// Parse command line arguments
	var pullDataPath string
	flag.StringVar(&pullDataPath, "pull-data", "", "Path to pull request JSON file (use '-' for stdin)")
	flag.Parse()
	
	fmt.Printf("DEBUG: main() - args: %v\n", os.Args)
	fmt.Printf("DEBUG: main() - pullDataPath after parsing: %s\n", pullDataPath)
	
	// Create an instance of the app structure
	app := NewApp()
	
	// If pull data is provided, handle it
	if pullDataPath != "" {
		if pullDataPath == "-" {
			// Read from stdin
			fmt.Println("DEBUG: Reading pull data from stdin")
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Printf("ERROR: Failed to read from stdin: %v\n", err)
				os.Exit(1)
			}
			app.pullDataJSON = string(data)
			fmt.Printf("DEBUG: Read %d bytes from stdin\n", len(data))
		} else {
			// File path provided - keep for backward compatibility
			fmt.Printf("DEBUG: main() - Setting app.pullDataPath to: %s\n", pullDataPath)
			app.pullDataPath = pullDataPath
		}
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
