package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go-ssh-tunnel/internal/config"
	"go-ssh-tunnel/internal/tunnel"
)

// statsBar shows aggregate traffic/connection metrics at the top of the window.
type statsBar struct {
	container *fyne.Container
	up        *widget.Label
	down      *widget.Label
	active    *widget.Label
	tunnels   *widget.Label
}

// buildToolbar constructs the top action bar (Start / Stop / Reload / Add /
// To Tray / Quit) with icons, plus the aggregate stats bar beneath it.
func (a *App) buildToolbar() fyne.CanvasObject {
	a.startBtn = widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), a.onStart)
	a.startBtn.Importance = widget.HighImportance
	a.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), a.onStop)
	a.stopBtn.Disable()
	a.reloadBtn = widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), a.onReload)
	a.addBtn = widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), a.onAddTunnel)
	trayBtn := widget.NewButtonWithIcon("To Tray", theme.VisibilityOffIcon(), func() {
		a.logPanel.Append(LogLevelInfo, "UI | Minimized to tray")
		a.mainWindow.Hide()
	})
	quitBtn := widget.NewButtonWithIcon("Quit", theme.CancelIcon(), func() {
		a.logPanel.Append(LogLevelInfo, "UI | Quitting")
		a.quit()
	})

	a.statsBar = a.buildStatsBar()

	toolbarRow := container.NewHBox(
		a.startBtn,
		a.stopBtn,
		a.reloadBtn,
		a.addBtn,
		trayBtn,
		quitBtn,
		layout.NewSpacer(),
		widget.NewLabel("Status:"),
		a.statusLabel,
	)

	return container.NewVBox(toolbarRow, a.statsBar.container)
}

func (a *App) buildStatsBar() *statsBar {
	s := &statsBar{
		up:      widget.NewLabel("↑ 0 B/s"),
		down:    widget.NewLabel("↓ 0 B/s"),
		active:  widget.NewLabel("0"),
		tunnels: widget.NewLabel("0"),
	}
	s.up.Alignment = fyne.TextAlignCenter
	s.down.Alignment = fyne.TextAlignCenter
	s.active.Alignment = fyne.TextAlignCenter
	s.tunnels.Alignment = fyne.TextAlignCenter

	block := func(title string, value *widget.Label) fyne.CanvasObject {
		t := widget.NewLabel(title)
		t.Alignment = fyne.TextAlignCenter
		t.TextStyle = fyne.TextStyle{Bold: true}
		return container.NewVBox(t, value)
	}

	s.container = container.NewHBox(
		layout.NewSpacer(),
		block("UP RATE", s.up),
		block("DOWN RATE", s.down),
		block("ACTIVE", s.active),
		block("TUNNELS", s.tunnels),
		layout.NewSpacer(),
	)
	return s
}

// update refreshes the aggregate metrics from the current manager state.
func (s *statsBar) update(mgr *tunnel.Manager, cfg *config.Config) {
	var upRate, downRate float64
	var active int
	if mgr != nil && mgr.IsRunning() {
		for _, f := range mgr.GetForwards() {
			u, d := f.Stats.GetRate()
			upRate += u
			downRate += d
			if f.Status == tunnel.StatusConnected {
				active++
			}
		}
	}
	count := 0
	if cfg != nil {
		count = len(cfg.Tunnels)
	}
	s.up.SetText("↑ " + tunnel.FormatRate(upRate))
	s.down.SetText("↓ " + tunnel.FormatRate(downRate))
	s.active.SetText(fmt.Sprintf("%d", active))
	s.tunnels.SetText(fmt.Sprintf("%d", count))
}
