package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go-ssh-tunnel/internal/config"
	"go-ssh-tunnel/internal/ui"
)

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

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	app := ui.NewApp(cfg, *cfgPath, version)
	app.Run()
}
