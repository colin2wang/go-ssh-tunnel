package ui

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/getlantern/systray"

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
	tray       *systray.MenuItem

	// UI elements
	startBtn    *widget.Button
	stopBtn     *widget.Button
	reloadBtn   *widget.Button
	statusLabel *widget.Label
	tunnelList  *TunnelList
	logPanel    *LogPanel
}

func NewApp(cfg *config.Config, cfgPath string, version string) *App {
	a := app.New()
	w := a.NewWindow(fmt.Sprintf("SSH Tunnel Manager v%s", version))
	w.Resize(fyne.NewSize(1000, 650))

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

	// Close button quits the application
	w.SetCloseIntercept(func() {
		mgr.Stop()
		systray.Quit()
		fyne.CurrentApp().Quit()
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

	// Set up system tray in background
	go systray.Run(
		func() {
			// On ready
			systray.SetIcon(icon.IconBytes)
			systray.SetTitle("SSH Tunnel Manager")
			systray.SetTooltip("SSH Tunnel Manager")

			// Add menu items
			showItem := systray.AddMenuItem("Show Window", "Show the main window")
			quitItem := systray.AddMenuItem("Quit", "Quit the application")

			// Handle show click
			go func() {
				for range showItem.ClickedCh {
					fyne.Do(func() {
						a.mainWindow.Show()
						a.mainWindow.RequestFocus()
					})
				}
			}()

			// Handle quit click
			go func() {
				for range quitItem.ClickedCh {
					a.manager.Stop()
					systray.Quit()
					fyne.CurrentApp().Quit()
				}
			}()
		},
		func() {
			// On exit
		},
	)

	a.mainWindow.ShowAndRun()
}

func (a *App) startTrafficRefresh() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if a.manager.IsRunning() {
				fyne.Do(func() {
					a.tunnelList.Refresh()
				})
			}
		}
	}()
}

func (a *App) buildUI() fyne.CanvasObject {
	// Toolbar
	a.startBtn = widget.NewButton("Start", a.onStart)
	a.startBtn.Importance = widget.HighImportance
	a.stopBtn = widget.NewButton("Stop", a.onStop)
	a.stopBtn.Disable()
	a.reloadBtn = widget.NewButton("Reload Config", a.onReload)
	minimizeBtn := widget.NewButton("Minimize to Tray", func() {
		a.logPanel.Append(LogLevelInfo, "UI | Window minimized to tray")
		a.mainWindow.Hide()
	})

	a.statusLabel = widget.NewLabel("Disconnected")
	a.statusLabel.Importance = widget.MediumImportance

	toolbar := container.NewHBox(
		a.startBtn,
		a.stopBtn,
		a.reloadBtn,
		minimizeBtn,
		layout.NewSpacer(),
		widget.NewLabel("Status:"),
		a.statusLabel,
	)

	// Left panel: unified tunnel list
	a.tunnelList = NewTunnelList(a.cfg, a.manager)

	leftPanel := container.NewBorder(
		nil, nil, nil, nil,
		a.tunnelList.List(),
	)

	// Right panel: logs
	a.logPanel = NewLogPanel()

	logToolbar := container.NewHBox(
		widget.NewLabel("Logs"),
		a.logPanel.LevelSelect(),
		layout.NewSpacer(),
		a.logPanel.CleanButton(),
	)

	rightPanel := container.NewBorder(
		logToolbar, nil, nil, nil,
		a.logPanel.Entry(),
	)

	// Split
	split := container.NewHSplit(leftPanel, rightPanel)
	split.Offset = 0.35

	return container.NewBorder(toolbar, nil, nil, nil, split)
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

	// Stop old tunnels if running (blocking call)
	if a.manager != nil && a.manager.IsRunning() {
		a.manager.Stop()
		a.logPanel.Append(LogLevelInfo, "Reload | Stopped old tunnels")
	}

	a.cfg = cfg
	a.manager = tunnel.NewManager(cfg)
	a.tunnelList.SetManager(a.manager)

	// Reset button states and status label
	a.startBtn.Enable()
	a.stopBtn.Disable()
	a.statusLabel.SetText("Disconnected")

	a.logPanel.AppendInfo("Config reloaded")
	a.tunnelList.Refresh()
}

// TunnelList displays unified port forwards with traffic stats
type TunnelList struct {
	cfg     *config.Config
	manager *tunnel.Manager
	list    *widget.List
	data    []config.TunnelConfig
	enabled map[int]bool // track which tunnels are enabled by index
	mu      sync.Mutex
}

func statusColor(status tunnel.ForwardStatus) color.Color {
	switch status {
	case tunnel.StatusConnected:
		return color.NRGBA{G: 180, A: 255} // Green
	case tunnel.StatusError:
		return color.NRGBA{R: 200, A: 255} // Red
	case tunnel.StatusConnecting:
		return color.NRGBA{R: 200, G: 160, A: 255} // Yellow
	default:
		return color.NRGBA{R: 100, G: 100, B: 100, A: 255} // Gray
	}
}

func NewTunnelList(cfg *config.Config, mgr *tunnel.Manager) *TunnelList {
	tl := &TunnelList{cfg: cfg, manager: mgr, enabled: make(map[int]bool)}
	tl.data = cfg.Tunnels
	// Initialize all tunnels as enabled
	for i := range tl.data {
		tl.enabled[i] = true
	}
	tl.list = widget.NewList(
		func() int { return len(tl.data) },
		func() fyne.CanvasObject {
			// Layout: checkbox | dot | name (port) ... status | ↑ rate (total) | ↓ rate (total)
			check := widget.NewCheck("", nil)
			dot := canvas.NewCircle(color.Gray{Y: 100})
			nameLabel := widget.NewLabel("Name (12345)")
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}
			statusLabel := widget.NewLabel("Disconnected")
			trafficLabel := widget.NewLabel("↑ 0 B/s (0 B) | ↓ 0 B/s (0 B)")

			topRow := container.NewHBox(
				nameLabel,
				layout.NewSpacer(),
				statusLabel,
			)

			infoCol := container.NewVBox(
				topRow,
				trafficLabel,
			)

			return container.NewHBox(
				check,
				dot,
				infoCol,
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(tl.data) {
				return
			}
			tc := tl.data[id]
			box := obj.(*fyne.Container)

			// Get checkbox
			check := box.Objects[0].(*widget.Check)
			check.SetChecked(tl.enabled[id])
			check.OnChanged = func(checked bool) {
				tl.mu.Lock()
				tl.enabled[id] = checked
				tl.mu.Unlock()
				go tl.toggleTunnel(id, checked)
			}

			// Get status dot
			dot := box.Objects[1].(*canvas.Circle)

			// Find matching forward and get status/traffic
			status := tunnel.StatusDisconnected
			var rateUp, rateDown float64
			var totalUp, totalDown int64

			if tl.manager != nil && tl.manager.IsRunning() {
				for _, fwd := range tl.manager.GetForwards() {
					if fwd.LocalPort == tc.LocalPort {
						status = fwd.Status
						rateUp, rateDown = fwd.Stats.GetRate()
						totalUp, totalDown = fwd.Stats.GetTotal()
						break
					}
				}
			}

			// Update dot color based on status
			dot.FillColor = statusColor(status)

			// Update info column
			infoCol := box.Objects[2].(*fyne.Container)
			topRow := infoCol.Objects[0].(*fyne.Container)

			// Update name with port
			nameLabel := topRow.Objects[0].(*widget.Label)
			nameLabel.SetText(fmt.Sprintf("%s (%d)", tc.Name, tc.LocalPort))

			// Update status
			statusLabel := topRow.Objects[2].(*widget.Label)
			statusLabel.SetText(status.String())

			// Update traffic
			trafficLabel := infoCol.Objects[1].(*widget.Label)
			trafficLabel.SetText(
				fmt.Sprintf("↑ %s (%s) | ↓ %s (%s)",
					tunnel.FormatRate(rateUp),
					tunnel.FormatBytes(totalUp),
					tunnel.FormatRate(rateDown),
					tunnel.FormatBytes(totalDown),
				),
			)
		},
	)
	return tl
}

// toggleTunnel starts or stops a tunnel based on checkbox state
func (tl *TunnelList) toggleTunnel(id int, enabled bool) {
	if tl.manager == nil || !tl.manager.IsRunning() {
		return
	}

	tc := tl.data[id]
	if enabled {
		// Start tunnel
		if err := tl.manager.StartTunnel(tc); err != nil {
			tl.manager.Log(fmt.Sprintf("[Tunnel] Failed to start %s: %v", tc.Name, err))
		} else {
			tl.manager.Log(fmt.Sprintf("[Tunnel] %s enabled", tc.Name))
		}
	} else {
		// Stop tunnel
		tl.manager.StopTunnel(tc.Name)
		tl.manager.Log(fmt.Sprintf("[Tunnel] %s disabled", tc.Name))
	}
	// Refresh UI
	fyne.Do(func() {
		tl.list.Refresh()
	})
}

func (tl *TunnelList) List() *widget.List { return tl.list }

func (tl *TunnelList) SetManager(mgr *tunnel.Manager) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.manager = mgr
}

func (tl *TunnelList) Refresh() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.data = tl.cfg.Tunnels
	tl.list.Refresh()
}

// LogLevel represents the severity level of log messages
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var logLevelNames = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
}

var logLevelPrefixes = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO ",
	LogLevelWarn:  "WARN ",
	LogLevelError: "ERROR",
}

type logEntry struct {
	level   LogLevel
	message string
}

// getGoroutineID returns the current goroutine ID
func getGoroutineID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Stack format: "goroutine 1 [running]:\n..."
	s := string(buf[:n])
	s = s[len("goroutine "):]
	s = s[:strings.IndexByte(s, ' ')]
	id := 0
	for _, c := range s {
		id = id*10 + int(c-'0')
	}
	return id
}

// LogPanel displays connection logs with level filtering and clean support
// It implements io.Writer so it can be used as log.SetOutput target
type LogPanel struct {
	entry       *widget.Entry
	levelSelect *widget.Select
	cleanBtn    *widget.Button
	mu          sync.Mutex
	entries     []logEntry // typed entries with level info
	logFile     *os.File
	minLevel    LogLevel
}

func NewLogPanel() *LogPanel {
	lp := &LogPanel{
		entry:    widget.NewMultiLineEntry(),
		minLevel: LogLevelDebug,
	}
	lp.entry.Wrapping = fyne.TextWrapWord
	lp.entry.TextStyle = fyne.TextStyle{Monospace: true}

	// Level selector
	lp.levelSelect = widget.NewSelect([]string{
		logLevelNames[LogLevelDebug],
		logLevelNames[LogLevelInfo],
		logLevelNames[LogLevelWarn],
		logLevelNames[LogLevelError],
	}, func(s string) {
		for lvl, name := range logLevelNames {
			if name == s {
				lp.Append(LogLevelInfo, fmt.Sprintf("UI | Log level changed to %s", s))
				lp.SetMinLevel(lvl)
				break
			}
		}
	})
	lp.levelSelect.SetSelected(logLevelNames[LogLevelDebug])

	// Clean button
	lp.cleanBtn = widget.NewButton("Clear", lp.Clean)

	lp.entry.OnChanged = func(s string) {
		// Prevent user editing - restore original text
		lp.mu.Lock()
		defer lp.mu.Unlock()
		rendered := lp.renderEntries()
		fyne.Do(func() {
			if lp.entry.Text != rendered {
				lp.entry.SetText(rendered)
			}
		})
		// Delay cursor update to ensure layout is calculated
		go func() {
			time.Sleep(10 * time.Millisecond)
			fyne.Do(func() {
				if lp.entry.Text == rendered {
					rowCount := strings.Count(rendered, "\n") + 1
					lp.entry.CursorRow = rowCount - 1
					lp.entry.CursorColumn = 0
					canvas.Refresh(lp.entry)
				}
			})
		}()
	}

	// Create logs directory and file
	os.MkdirAll("logs", 0755)
	logPath := fmt.Sprintf("logs/ssh-tunnel-%s.log", time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		lp.logFile = f
	}

	return lp
}

func (lp *LogPanel) Entry() *widget.Entry { return lp.entry }

func (lp *LogPanel) LevelSelect() *widget.Select { return lp.levelSelect }

func (lp *LogPanel) CleanButton() *widget.Button { return lp.cleanBtn }

// Write implements io.Writer - allows using LogPanel as log.SetOutput target
func (lp *LogPanel) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(strings.TrimRight(string(p), "\r\n"), "\n")
	if msg == "" {
		return len(p), nil
	}
	lp.Append(LogLevelInfo, msg)
	return len(p), nil
}

// Append adds a new log message with the specified level
func (lp *LogPanel) Append(level LogLevel, msg string) {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := logLevelPrefixes[level]
	line := fmt.Sprintf("%s | %s | %s", timestamp, levelName, msg)

	lp.entries = append(lp.entries, logEntry{level: level, message: line})

	// Keep last 500 lines
	if len(lp.entries) > 500 {
		lp.entries = lp.entries[len(lp.entries)-500:]
	}

	// Write to log file (all levels always written to file)
	if lp.logFile != nil {
		lp.logFile.WriteString(line + "\n")
		lp.logFile.Sync()
	}

	// Only update UI if message passes current level filter
	if level >= lp.minLevel {
		rendered := lp.renderEntriesUnlocked()
		fyne.Do(func() {
			lp.entry.SetText(rendered)
		})
		// Delay cursor update to ensure layout is calculated
		go func() {
			time.Sleep(10 * time.Millisecond)
			fyne.Do(func() {
				rowCount := strings.Count(rendered, "\n") + 1
				lp.entry.CursorRow = rowCount - 1
				lp.entry.CursorColumn = 0
				canvas.Refresh(lp.entry)
			})
		}()
	}
}

// AppendInfo is convenience method for Info-level messages (backward compat)
func (lp *LogPanel) AppendInfo(msg string) {
	lp.Append(LogLevelInfo, msg)
}

// SetMinLevel changes the minimum log level and refreshes display
func (lp *LogPanel) SetMinLevel(level LogLevel) {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	lp.minLevel = level

	rendered := lp.renderEntriesUnlocked()
	fyne.Do(func() {
		lp.entry.SetText(rendered)
	})
	// Delay cursor update to ensure layout is calculated
	go func() {
		time.Sleep(10 * time.Millisecond)
		fyne.Do(func() {
			rowCount := strings.Count(rendered, "\n") + 1
			lp.entry.CursorRow = rowCount - 1
			lp.entry.CursorColumn = 0
			canvas.Refresh(lp.entry)
		})
	}()
}

// Clear removes all log entries
func (lp *LogPanel) Clean() {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	lp.entries = nil
	fyne.Do(func() {
		lp.entry.SetText("")
	})
}

func (lp *LogPanel) renderEntriesUnlocked() string {
	var visible []string
	for _, e := range lp.entries {
		if e.level >= lp.minLevel {
			visible = append(visible, e.message)
		}
	}
	return strings.Join(visible, "\n")
}

func (lp *LogPanel) renderEntries() string {
	return lp.renderEntriesUnlocked()
}

func (lp *LogPanel) Close() {
	if lp.logFile != nil {
		lp.logFile.Close()
	}
}

// Helpers
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

func deleteFile(path string) error {
	return os.Remove(path)
}
