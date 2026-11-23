package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:                    "Indie Studio · 像素工坊",
		Width:                    1280,
		Height:                   820,
		MinWidth:                 1040,
		MinHeight:                680,
		EnableDefaultContextMenu: true,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.AssetHandler(),
		},
		BackgroundColour: &options.RGBA{R: 11, G: 15, B: 20, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
