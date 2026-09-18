package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 1. Parse command-line flags and environment variables
	nameFlag := flag.String("name", "", "Custom instance display name (env: CASTPIPE_NAME)")
	flag.StringVar(nameFlag, "n", "", "Custom instance display name (shorthand)")

	portFlag := flag.Int("port", 0, "HTTP receiver port, 0 for ephemeral (env: CASTPIPE_PORT)")
	flag.IntVar(portFlag, "p", 0, "HTTP receiver port (shorthand)")

	dropDirFlag := flag.String("drop-dir", "", "Custom download directory (env: CASTPIPE_DROP_DIR)")
	flag.StringVar(dropDirFlag, "d", "", "Custom download directory (shorthand)")

	flag.Parse()

	name := strings.TrimSpace(*nameFlag)
	if name == "" {
		name = strings.TrimSpace(os.Getenv("CASTPIPE_NAME"))
	}

	port := *portFlag
	if port == 0 {
		if envPort := strings.TrimSpace(os.Getenv("CASTPIPE_PORT")); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
				port = p
			}
		}
	}

	dropDir := strings.TrimSpace(*dropDirFlag)
	if dropDir == "" {
		dropDir = strings.TrimSpace(os.Getenv("CASTPIPE_DROP_DIR"))
	}

	appConfig := AppConfig{
		Name:        name,
		Port:        port,
		DownloadDir: dropDir,
	}

	// Create an instance of the app structure
	app := NewAppWithConfig(appConfig)

	title := "Castpipe"
	if appConfig.Name != "" {
		title = fmt.Sprintf("Castpipe - %s", appConfig.Name)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     title,
		Width:     1120,
		Height:    780,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 15, B: 20, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

