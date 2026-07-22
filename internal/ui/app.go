package ui

import (
	"fmt"
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"go-ssh-tunnel/internal/config"
	"go-ssh-tunnel/internal/icon"
	"go-ssh-tunnel/internal/tunnel"
)

type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window
	cfg        *config.Config
	manager    *tunnel.Manager
	cfgPath    string
	version    string

	// UI elements
	startBtn    *widget.Button
	stopBtn     *widget.Button
	reloadBtn   *widget.Button
	addBtn      *widget.Button
	statusLabel *widget.Label
	tunnelList  *TunnelList
	logPanel    *LogPanel
	statsBar    *statsBar
}

func NewApp(cfg *config.Config, cfgPath string, version string) *App {
	a := app.New()
	w := a.NewWindow(fmt.Sprintf("SSH Tunnel Manager v%s", version))
	w.Resize(fyne.NewSize(1100, 720))
	// Allow the window to shrink but keep a usable minimum via the content's
	// natural minimum size (toolbar, stats bar, and list all contribute).
	w.SetFixedSize(false)

	// Set window icon
	w.SetIcon(icon.AppIcon)

	mgr := tunnel.NewManager(cfg)

	app := &App{
		fyneApp:    a,
		mainWindow: w,
		cfg:        cfg,
		manager:    mgr,
		cfgPath:    cfgPath,
		version:    version,
	}

	// Close button hides to tray instead of quitting
	w.SetCloseIntercept(func() {
		app.hideToTray()
	})

	mgr.SetCallbacks(
		func(msg string) { fyne.Do(func() { app.logPanel.AppendInfo(msg) }) },
		func(msg string) { fyne.Do(func() { app.statusLabel.SetText(msg) }) },
	)

	// Redirect Go standard log output to log panel
	log.SetOutput(app.logPanel)

	return app
}

func (a *App) Run() {
	content := a.buildUI()
	a.mainWindow.SetContent(content)
	a.startTrafficRefresh()
	a.startTray()

	a.mainWindow.ShowAndRun()
}

func (a *App) startTrafficRefresh() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(func() {
				if a.manager.IsRunning() {
					a.tunnelList.Refresh()
				}
				a.statsBar.update(a.manager, a.cfg)
			})
		}
	}()
}

func (a *App) buildUI() fyne.CanvasObject {
	a.statusLabel = widget.NewLabel("Disconnected")
	a.statusLabel.Importance = widget.MediumImportance

	// Left panel: unified tunnel list
	a.tunnelList = NewTunnelList(a.cfg, a.manager)
	a.tunnelList.onEdit = a.onEditTunnel
	a.tunnelList.onDelete = a.onDeleteTunnel

	leftHeader := container.NewHBox(
		sectionLabel("Tunnels"),
		layout.NewSpacer(),
	)
	leftPanel := container.NewBorder(
		leftHeader, nil, nil, nil,
		a.tunnelList.List(),
	)

	// Right panel: logs
	a.logPanel = NewLogPanel()

	rightHeader := container.NewHBox(
		sectionLabel("Logs"),
		layout.NewSpacer(),
		a.logPanel.LevelSelect(),
		a.logPanel.CleanButton(),
	)
	rightPanel := container.NewPadded(container.NewBorder(
		rightHeader, nil, nil, nil,
		a.logPanel.Content(),
	))

	// Split (draggable divider, scales with the window)
	split := container.NewHSplit(leftPanel, rightPanel)
	split.Offset = 0.38

	return container.NewBorder(a.buildToolbar(), nil, nil, nil, split)
}

func (a *App) onStart() {
	a.logPanel.Append(LogLevelInfo, "UI | Start button clicked")

	if a.cfg.SSH.Auth.Method == "password" {
		pass, _ := a.cfg.ReadPassword()
		if pass == "" {
			a.promptPassword(func(p string) {
				a.cfg.SSH.Auth.Method = "password"
				// Store password temporarily
				a.cfg.SSH.Auth.PasswordFile = ""
				go a.startTunnel(p)
			})
			return
		}
	}

	if a.cfg.SSH.Auth.Method == "key" {
		passphrase, _ := a.cfg.ReadKeyPassphrase()
		if passphrase == "" {
			a.promptPassword(func(p string) {
				a.cfg.SSH.Auth.KeyPassphraseFile = ""
				go a.startTunnelWithKeyPass(p)
			})
			return
		}
	}

	go a.startTunnel("")
}

func (a *App) promptPassword(callback func(string)) {
	entry := widget.NewPasswordEntry()
	entry.SetPlaceHolder("Enter password...")

	dialog.ShowCustomConfirm("SSH Password", "Connect", "Cancel", entry, func(ok bool) {
		if ok {
			callback(entry.Text)
		}
	}, a.mainWindow)
}

func (a *App) startTunnel(password string) {
	fyne.Do(func() {
		a.startBtn.Disable()
		a.logPanel.Append(LogLevelInfo, "Tunnel | Starting with password auth...")
	})

	if password != "" {
		// Override password for this session
		tmpFile := "secrets/.tmp_pass"
		_ = writeFile(tmpFile, password)
		a.cfg.SSH.Auth.PasswordFile = tmpFile
		defer func() {
			_ = deleteFile(tmpFile)
			a.cfg.SSH.Auth.PasswordFile = "secrets/password.txt"
		}()
	}

	if err := a.manager.Start(); err != nil {
		fyne.Do(func() {
			a.logPanel.Append(LogLevelError, fmt.Sprintf("Tunnel | Start failed: %v", err))
			a.startBtn.Enable()
		})
		return
	}

	fyne.Do(func() {
		a.stopBtn.Enable()
		a.tunnelList.Refresh()
		a.logPanel.Append(LogLevelInfo, "Tunnel | Started successfully")
	})
}

func (a *App) startTunnelWithKeyPass(passphrase string) {
	fyne.Do(func() {
		a.startBtn.Disable()
		a.logPanel.Append(LogLevelInfo, "Tunnel | Starting with key auth...")
	})

	tmpFile := "secrets/.tmp_keypass"
	_ = writeFile(tmpFile, passphrase)
	a.cfg.SSH.Auth.KeyPassphraseFile = tmpFile
	defer func() {
		_ = deleteFile(tmpFile)
		a.cfg.SSH.Auth.KeyPassphraseFile = ""
	}()

	if err := a.manager.Start(); err != nil {
		fyne.Do(func() {
			a.logPanel.Append(LogLevelError, fmt.Sprintf("Tunnel | Start failed: %v", err))
			a.startBtn.Enable()
		})
		return
	}

	fyne.Do(func() {
		a.stopBtn.Enable()
		a.tunnelList.Refresh()
		a.logPanel.Append(LogLevelInfo, "Tunnel | Started successfully")
	})
}

func (a *App) onStop() {
	a.logPanel.Append(LogLevelInfo, "UI | Stop button clicked")
	a.stopBtn.Disable()
	a.startBtn.Disable()
	go func() {
		a.manager.Stop()
		fyne.Do(func() {
			a.startBtn.Enable()
			a.tunnelList.Refresh()
			a.logPanel.Append(LogLevelInfo, "Tunnel | Stopped")
		})
	}()
}

func (a *App) onReload() {
	cfg, err := config.Load(a.cfgPath)
	if err != nil {
		dialog.ShowError(err, a.mainWindow)
		return
	}

	// Stop old tunnels off the UI thread: Stop's callbacks use fyne.Do,
	// which deadlocks if invoked from the main thread.
	if a.manager != nil && a.manager.IsRunning() {
		a.startBtn.Disable()
		a.stopBtn.Disable()
		go func() {
			a.manager.Stop()
			fyne.Do(func() {
				a.applyReload(cfg)
				a.logPanel.Append(LogLevelInfo, "Reload | Stopped old tunnels")
			})
		}()
		return
	}

	a.applyReload(cfg)
}

// applyReload swaps in the freshly loaded config. Must be called on the UI
// thread (directly, or via fyne.Do).
func (a *App) applyReload(cfg *config.Config) {
	a.cfg = cfg
	a.manager = tunnel.NewManager(cfg)
	a.manager.SetCallbacks(
		func(msg string) { fyne.Do(func() { a.logPanel.AppendInfo(msg) }) },
		func(msg string) { fyne.Do(func() { a.statusLabel.SetText(msg) }) },
	)
	a.tunnelList.SetManager(a.manager)
	a.tunnelList.SetConfig(cfg)

	// Reset button states and status label
	a.startBtn.Enable()
	a.stopBtn.Disable()
	a.statusLabel.SetText("Disconnected")

	a.logPanel.AppendInfo("Config reloaded")
	a.tunnelList.Refresh()
	a.statsBar.update(a.manager, a.cfg)
}

// Helpers
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

func deleteFile(path string) error {
	return os.Remove(path)
}
