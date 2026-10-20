// Package app implements the Wails-bound application: it exposes the tunnel
// manager and config operations to the Vue frontend and pushes logs/status
// updates as events.
package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-ssh-tunnel/internal/config"
	"go-ssh-tunnel/internal/tunnel"
)

// SSHInfo describes the SSH connection section of the config.
type SSHInfo struct {
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	User                 string `json:"user"`
	AuthMethod           string `json:"authMethod"`
	HasPasswordFile      bool   `json:"hasPasswordFile"`
	HasKeyFile           bool   `json:"hasKeyFile"`
	HasKeyPassphraseFile bool   `json:"hasKeyPassphraseFile"`
}

// AppState is the initial snapshot returned to the frontend.
type AppState struct {
	Version    string                `json:"version"`
	ConfigPath string                `json:"configPath"`
	SSH        SSHInfo               `json:"ssh"`
	Tunnels    []config.TunnelConfig `json:"tunnels"`
}

// TunnelStat is the live state of one tunnel (polled by the frontend).
type TunnelStat struct {
	Name      string  `json:"name"`
	LocalPort int     `json:"localPort"`
	Status    string  `json:"status"`
	Error     string  `json:"error"`
	RateUp    float64 `json:"rateUp"`
	RateDown  float64 `json:"rateDown"`
	TotalUp   int64   `json:"totalUp"`
	TotalDown int64   `json:"totalDown"`
}

// Stats is the polled aggregate state of the manager.
type Stats struct {
	Running   bool         `json:"running"`
	Status    string       `json:"status"`
	UpRate    float64      `json:"upRate"`
	DownRate  float64      `json:"downRate"`
	TotalUp   int64        `json:"totalUp"`
	TotalDown int64        `json:"totalDown"`
	Active    int          `json:"active"`
	Tunnels   []TunnelStat `json:"tunnels"`
}

// LogEntry is one log line pushed to the frontend.
type LogEntry struct {
	Level string `json:"level"` // debug|info|warn|error
	Time  string `json:"time"`
	Msg   string `json:"msg"`
}

// App is the object bound to the frontend.
type App struct {
	ctx     context.Context
	cfg     *config.Config
	cfgPath string
	version string
	cfgErr  error // config load error, surfaced to the UI at startup
	manager *tunnel.Manager

	mu      sync.Mutex
	status  string
	logFile *os.File
}

func New(cfg *config.Config, cfgPath string, version string, cfgErr error) *App {
	a := &App{
		cfg:     cfg,
		cfgPath: cfgPath,
		version: version,
		status:  "Disconnected",
		cfgErr:  cfgErr,
	}
	a.manager = tunnel.NewManager(cfg)
	a.manager.SetCallbacks(
		func(msg string) { a.emitLog("info", msg) },
		func(msg string) { a.setStatus(msg) },
	)
	return a
}

// Startup is called by Wails when the app starts.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	os.MkdirAll("logs", 0755)
	logPath := fmt.Sprintf("logs/ssh-tunnel-%s.log", time.Now().Format("2006-01-02"))
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		a.logFile = f
	}

	// Redirect Go standard log output into the UI log panel / log file.
	log.SetFlags(0)
	log.SetOutput(a)

	if a.cfgErr != nil {
		a.emitLog("error", fmt.Sprintf("Config | Load failed: %v", a.cfgErr))
	}
}

// Shutdown is called by Wails when the app closes.
func (a *App) Shutdown(ctx context.Context) {
	if a.manager != nil && a.manager.IsRunning() {
		a.manager.Stop()
	}
	if a.logFile != nil {
		a.logFile.Close()
	}
}

// Write implements io.Writer so App can be the target of log.SetOutput.
func (a *App) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\r\n")
	if msg != "" {
		a.emitLog("info", msg)
	}
	return len(p), nil
}

// GetState returns the initial app snapshot.
func (a *App) GetState() AppState {
	return AppState{
		Version:    a.version,
		ConfigPath: a.cfgPath,
		SSH: SSHInfo{
			Host:                 a.cfg.SSH.Host,
			Port:                 a.cfg.SSH.Port,
			User:                 a.cfg.SSH.User,
			AuthMethod:           a.cfg.SSH.Auth.Method,
			HasPasswordFile:      a.cfg.SSH.Auth.PasswordFile != "",
			HasKeyFile:           a.cfg.SSH.Auth.KeyFile != "",
			HasKeyPassphraseFile: a.cfg.SSH.Auth.KeyPassphraseFile != "",
		},
		Tunnels: append([]config.TunnelConfig(nil), a.cfg.Tunnels...),
	}
}

// Start connects to the SSH server and starts all tunnels. password carries an
// interactively entered password / key passphrase; it is empty when the config
// already provides one via a file.
func (a *App) Start(password string) error {
	a.mu.Lock()
	if a.manager.IsRunning() {
		a.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	a.mu.Unlock()

	restore := func() {}
	if password != "" {
		os.MkdirAll("secrets", 0700)
		if a.cfg.SSH.Auth.Method == "key" {
			tmp := filepath.Join("secrets", ".tmp_keypass")
			if err := writeFile(tmp, password); err == nil {
				old := a.cfg.SSH.Auth.KeyPassphraseFile
				a.cfg.SSH.Auth.KeyPassphraseFile = tmp
				restore = func() {
					os.Remove(tmp)
					a.cfg.SSH.Auth.KeyPassphraseFile = old
				}
			}
		} else {
			tmp := filepath.Join("secrets", ".tmp_pass")
			if err := writeFile(tmp, password); err == nil {
				old := a.cfg.SSH.Auth.PasswordFile
				a.cfg.SSH.Auth.PasswordFile = tmp
				restore = func() {
					os.Remove(tmp)
					a.cfg.SSH.Auth.PasswordFile = old
				}
			}
		}
	}
	defer restore()

	a.setStatus("Connecting...")
	a.emitLog("info", fmt.Sprintf("Connecting to %s:%d as %s...", a.cfg.SSH.Host, a.cfg.SSH.Port, a.cfg.SSH.User))

	if err := a.manager.Start(); err != nil {
		a.emitLog("error", fmt.Sprintf("Start failed: %v", err))
		return err
	}
	return nil
}

// Stop stops all tunnels.
func (a *App) Stop() {
	a.manager.Stop()
}

// Reload reloads the config from disk, stopping running tunnels first.
func (a *App) Reload() error {
	cfg, err := config.Load(a.cfgPath)
	if err != nil {
		a.emitLog("error", fmt.Sprintf("Config | Load failed: %v", err))
		return err
	}

	if a.manager.IsRunning() {
		a.manager.Stop()
	}

	a.cfg = cfg
	a.manager = tunnel.NewManager(cfg)
	a.manager.SetCallbacks(
		func(msg string) { a.emitLog("info", msg) },
		func(msg string) { a.setStatus(msg) },
	)

	a.setStatus("Disconnected")
	a.emitLog("info", "Config reloaded")
	return nil
}

// AddTunnel appends a tunnel to the config, persists it, and (if connected)
// starts forwarding immediately.
func (a *App) AddTunnel(tc config.TunnelConfig) error {
	if err := validateTunnel(tc); err != nil {
		return err
	}
	a.cfg.Tunnels = append(a.cfg.Tunnels, tc)
	a.saveConfig()
	if a.manager.IsRunning() {
		if err := a.manager.StartTunnel(tc); err != nil {
			a.emitLog("error", fmt.Sprintf("Tunnel | Failed to start %s: %v", tc.Name, err))
			return err
		}
	}
	a.emitLog("info", fmt.Sprintf("Tunnel | Added %q", tc.Name))
	return nil
}

// UpdateTunnel replaces an existing tunnel, persists the change, and restarts
// its forward (if running) without disturbing other tunnels.
func (a *App) UpdateTunnel(oldName string, tc config.TunnelConfig) error {
	if err := validateTunnel(tc); err != nil {
		return err
	}
	found := false
	for i := range a.cfg.Tunnels {
		if a.cfg.Tunnels[i].Name == oldName {
			a.cfg.Tunnels[i] = tc
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tunnel %q not found", oldName)
	}
	a.saveConfig()
	if a.manager.IsRunning() {
		a.manager.StopTunnel(oldName)
		if err := a.manager.StartTunnel(tc); err != nil {
			a.emitLog("error", fmt.Sprintf("Tunnel | Failed to restart %s: %v", tc.Name, err))
			return err
		}
	}
	a.emitLog("info", fmt.Sprintf("Tunnel | Updated %q", tc.Name))
	return nil
}

// DeleteTunnel removes a tunnel from the config, persists the change, and
// stops its forward if running.
func (a *App) DeleteTunnel(name string) error {
	filtered := a.cfg.Tunnels[:0]
	for _, t := range a.cfg.Tunnels {
		if t.Name != name {
			filtered = append(filtered, t)
		}
	}
	a.cfg.Tunnels = filtered
	a.saveConfig()
	if a.manager.IsRunning() {
		a.manager.StopTunnel(name)
	}
	a.emitLog("info", fmt.Sprintf("Tunnel | Deleted %q", name))
	return nil
}

// ToggleTunnel starts or stops a single tunnel while the manager is running.
func (a *App) ToggleTunnel(tc config.TunnelConfig, enabled bool) error {
	if !a.manager.IsRunning() {
		return fmt.Errorf("not connected")
	}
	if enabled {
		if err := a.manager.StartTunnel(tc); err != nil {
			a.emitLog("error", fmt.Sprintf("Tunnel | Failed to start %s: %v", tc.Name, err))
			return err
		}
		a.emitLog("info", fmt.Sprintf("Tunnel | %s enabled", tc.Name))
	} else {
		a.manager.StopTunnel(tc.Name)
		a.emitLog("info", fmt.Sprintf("Tunnel | %s disabled", tc.Name))
	}
	return nil
}

// GetStats returns the polled manager state for the UI.
func (a *App) GetStats() Stats {
	a.mu.Lock()
	status := a.status
	a.mu.Unlock()

	running := a.manager.IsRunning()
	s := Stats{
		Running: running,
		Status:  status,
		Tunnels: make([]TunnelStat, 0, len(a.cfg.Tunnels)),
	}

	for _, tc := range a.cfg.Tunnels {
		ts := TunnelStat{
			Name:      tc.Name,
			LocalPort: tc.LocalPort,
			Status:    tunnel.StatusDisconnected.String(),
		}
		if running {
			for _, f := range a.manager.GetForwards() {
				if f.LocalPort == tc.LocalPort {
					ts.Status = f.Status.String()
					ts.Error = f.Error
					ts.RateUp, ts.RateDown = f.Stats.GetRate()
					ts.TotalUp, ts.TotalDown = f.Stats.GetTotal()
					s.UpRate += ts.RateUp
					s.DownRate += ts.RateDown
					s.TotalUp += ts.TotalUp
					s.TotalDown += ts.TotalDown
					if f.Status == tunnel.StatusConnected {
						s.Active++
					}
					break
				}
			}
		}
		s.Tunnels = append(s.Tunnels, ts)
	}
	return s
}

func validateTunnel(tc config.TunnelConfig) error {
	if tc.LocalPort <= 0 || tc.LocalPort > 65535 {
		return fmt.Errorf("invalid local port")
	}
	if tc.RemotePort <= 0 || tc.RemotePort > 65535 {
		return fmt.Errorf("invalid remote port")
	}
	if tc.RemoteHost == "" {
		return fmt.Errorf("remote host is required")
	}
	return nil
}

func (a *App) saveConfig() {
	if err := a.cfg.Save(a.cfgPath); err != nil {
		a.emitLog("error", fmt.Sprintf("Config | Save failed: %v", err))
	}
}

func (a *App) setStatus(msg string) {
	a.mu.Lock()
	a.status = msg
	a.mu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "status", msg)
	}
}

func (a *App) emitLog(level, msg string) {
	entry := LogEntry{
		Level: level,
		Time:  time.Now().Format("2006-01-02 15:04:05"),
		Msg:   msg,
	}

	a.mu.Lock()
	if a.logFile != nil {
		a.logFile.WriteString(fmt.Sprintf("%s | %s | %s\n", entry.Time, strings.ToUpper(level), msg))
		a.logFile.Sync()
	}
	a.mu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log", entry)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

var _ io.Writer = (*App)(nil)
