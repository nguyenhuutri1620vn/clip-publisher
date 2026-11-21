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
		Title:     "Clip Publisher",
		Width:     1320,
		Height:    860,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.mediaHandler(),
		},
		BackgroundColour: &options.RGBA{R: 22, G: 20, B: 18, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "clip-publisher-6fe7430",
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
