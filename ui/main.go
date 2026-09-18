package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "SatisFacts",
		Width:     1400,
		Height:    900,
		MinWidth:  900,
		MinHeight: 600,
		MaxWidth:  32767,
		MaxHeight: 32767,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 13, B: 17, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			if w, h := wailsruntime.WindowGetSize(ctx); w > 0 && h > 0 {
				if s := LoadSettings(); s != nil {
					s.WindowWidth = w
					s.WindowHeight = h
					_ = SaveSettings(s)
				}
			}
			return false
		},
		// ZoomFactor is deliberately not set here. The frontend already applies
		// the saved zoom via document.body.style.zoom (see applyZoom in state.js),
		// which is what the header zoom dropdown drives on every platform.
		// Setting it here too made the two multiply on Windows, so a saved 1.25
		// rendered at ~1.56x.
		Windows: &windows.Options{
			IsZoomControlEnabled: true,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: false,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
