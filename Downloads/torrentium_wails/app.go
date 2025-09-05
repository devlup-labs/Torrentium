package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var iconFS embed.FS

type App struct {
	ctx        context.Context
	shouldQuit bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx

	// Start background tasks
	go a.StartTorrentTasks()

	// Setup system tray
	go func() {
		systray.Run(a.onReady, a.onExit)
	}()
}

// OnBeforeClose is called when the application is about to quit
func (a *App) OnBeforeClose(ctx context.Context) (prevent bool) {
	if a.shouldQuit {
		return false // allow the app to close
	}
	runtime.WindowHide(ctx)
	log.Println("Window hidden. Torrentium still running in background.")
	return true // prevent the app from closing
}

func (a *App) onReady() {
	// Load icon from embedded assets
	iconData, err := iconFS.ReadFile("build/windows/icon.ico")
	if err != nil {
		log.Printf("Failed to load icon: %v", err)
		// Use a fallback simple icon if the file can't be loaded
		iconData = []byte{} // Empty fallback
	}
	
	// Set the systray icon
	systray.SetIcon(iconData)
	systray.SetTitle("Torrentium")
	systray.SetTooltip("Torrentium - BitTorrent Client")

	// Menu items
	mShow := systray.AddMenuItem("Show Torrentium", "Show the main window")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Torrentium")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				runtime.WindowShow(a.ctx)
				runtime.WindowUnminimise(a.ctx)
			case <-mQuit.ClickedCh:
				a.shouldQuit = true
				systray.Quit()
				runtime.Quit(a.ctx)
				return
			}
		}
	}()
}

func (a *App) onExit() {
	// Cleanup when systray exits
	log.Println("Systray exiting...")
}

func (a *App) StartTorrentTasks() {
	for {
		log.Println("Torrentium running in background...")
		time.Sleep(15 * time.Second)
	}
}
