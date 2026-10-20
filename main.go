package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"go-ssh-tunnel/internal/app"
	"go-ssh-tunnel/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ssh-tunnel %s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Resolve config path relative to executable directory
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	if !filepath.IsAbs(*cfgPath) {
		*cfgPath = filepath.Join(exeDir, *cfgPath)
	}

	// A missing/invalid config must not abort startup: `wails build` executes
	// the binary in a scratch directory to generate bindings, and the UI can
	// surface the error while a Reload picks up a fixed config.
	cfg, cfgErr := config.Load(*cfgPath)
	if cfgErr != nil {
		cfg = config.Default()
	}

	boundApp := app.New(cfg, *cfgPath, version, cfgErr)

	err := wails.Run(&options.App{
		Title:     fmt.Sprintf("SSH Tunnel Manager v%s", version),
		Width:     1100,
		Height:    720,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  boundApp.Startup,
		OnShutdown: boundApp.Shutdown,
		Bind:       []interface{}{boundApp},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start application: %v\n", err)
		os.Exit(1)
	}
}
