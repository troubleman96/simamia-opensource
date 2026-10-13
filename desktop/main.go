package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"simamia/api"
)

//go:embed all:frontend/out
var frontend embed.FS

func main() {
	assets, err := fs.Sub(frontend, "frontend/out")
	if err != nil {
		log.Fatalf("load embedded Simamia UI: %v", err)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("find user data directory: %v", err)
	}
	databasePath := filepath.Join(configDir, "Simamia", "simamia.db")
	apiHandler, closeStore, err := api.NewDesktopHTTPHandler(databasePath)
	if err != nil {
		log.Fatalf("open local Simamia database: %v", err)
	}
	defer func() {
		if err := closeStore(); err != nil {
			log.Printf("close local Simamia database: %v", err)
		}
	}()

	if err := wails.Run(&options.App{
		Title:       "Simamia",
		Width:       1280,
		Height:      820,
		MinWidth:    960,
		MinHeight:   640,
		AssetServer: &assetserver.Options{Assets: assets, Handler: apiHandler},
	}); err != nil {
		log.Fatal(err)
	}
}
