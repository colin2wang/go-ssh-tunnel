package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"go-ssh-tunnel/internal/config"
)

// onAddTunnel opens the form to create a new tunnel.
func (a *App) onAddTunnel() {
	a.showTunnelForm("Add Tunnel", config.TunnelConfig{}, nil)
}

// onEditTunnel opens the form to edit an existing tunnel.
func (a *App) onEditTunnel(tc config.TunnelConfig) {
	name := tc.Name
	a.showTunnelForm("Edit Tunnel", tc, &name)
}

// onDeleteTunnel prompts for confirmation then deletes the tunnel.
func (a *App) onDeleteTunnel(tc config.TunnelConfig) {
	dialog.ShowConfirm("Delete Tunnel",
		fmt.Sprintf("Delete tunnel %q? This also stops it if running.", tc.Name),
		func(ok bool) {
			if ok {
				a.deleteTunnel(tc.Name)
			}
		}, a.mainWindow)
}

// showTunnelForm builds a reusable add/edit dialog backed by a YAML config.
// When originalName is nil the form performs an add; otherwise it updates the
// tunnel identified by originalName.
func (a *App) showTunnelForm(title string, tc config.TunnelConfig, originalName *string) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(tc.Name)
	localEntry := widget.NewEntry()
	localEntry.SetText(strconv.Itoa(tc.LocalPort))
	remoteHostEntry := widget.NewEntry()
	remoteHostEntry.SetText(tc.RemoteHost)
	remotePortEntry := widget.NewEntry()
	remotePortEntry.SetText(strconv.Itoa(tc.RemotePort))

	form := widget.NewForm(
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Local Port", localEntry),
		widget.NewFormItem("Remote Host", remoteHostEntry),
		widget.NewFormItem("Remote Port", remotePortEntry),
	)

	dlg := dialog.NewCustomConfirm(title, "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		localPort, err1 := strconv.Atoi(localEntry.Text)
		remotePort, err2 := strconv.Atoi(remotePortEntry.Text)
		if err1 != nil || err2 != nil || localPort <= 0 || remotePort <= 0 {
			a.logPanel.Append(LogLevelError, "Tunnel | Invalid port number")
			return
		}
		newTC := config.TunnelConfig{
			Name:       nameEntry.Text,
			LocalPort:  localPort,
			RemoteHost: remoteHostEntry.Text,
			RemotePort: remotePort,
		}
		if originalName != nil {
			a.updateTunnel(*originalName, newTC)
		} else {
			a.addTunnel(newTC)
		}
	}, a.mainWindow)
	dlg.Resize(fyne.NewSize(420, 280))
	dlg.Show()
}

// addTunnel appends a tunnel to the config, persists it, and (if connected)
// starts forwarding immediately.
func (a *App) addTunnel(tc config.TunnelConfig) {
	a.cfg.Tunnels = append(a.cfg.Tunnels, tc)
	a.saveConfig()
	if a.manager.IsRunning() {
		if err := a.manager.StartTunnel(tc); err != nil {
			a.logPanel.Append(LogLevelError, fmt.Sprintf("Tunnel | Failed to start %s: %v", tc.Name, err))
		}
	}
	a.tunnelList.Refresh()
	a.statsBar.update(a.manager, a.cfg)
	a.logPanel.AppendInfo(fmt.Sprintf("Tunnel | Added %q", tc.Name))
}

// updateTunnel replaces an existing tunnel, persists the change, and restarts
// its forward (if running) without disturbing other tunnels.
func (a *App) updateTunnel(oldName string, tc config.TunnelConfig) {
	for i := range a.cfg.Tunnels {
		if a.cfg.Tunnels[i].Name == oldName {
			a.cfg.Tunnels[i] = tc
			break
		}
	}
	a.saveConfig()
	if a.manager.IsRunning() {
		a.manager.StopTunnel(oldName)
		if err := a.manager.StartTunnel(tc); err != nil {
			a.logPanel.Append(LogLevelError, fmt.Sprintf("Tunnel | Failed to restart %s: %v", tc.Name, err))
		}
	}
	a.tunnelList.Refresh()
	a.statsBar.update(a.manager, a.cfg)
	a.logPanel.AppendInfo(fmt.Sprintf("Tunnel | Updated %q", tc.Name))
}

// deleteTunnel removes a tunnel from the config, persists the change, and
// stops its forward if running.
func (a *App) deleteTunnel(name string) {
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
	a.tunnelList.Refresh()
	a.statsBar.update(a.manager, a.cfg)
	a.logPanel.AppendInfo(fmt.Sprintf("Tunnel | Deleted %q", name))
}

// saveConfig writes the current config back to disk.
func (a *App) saveConfig() {
	if err := a.cfg.Save(a.cfgPath); err != nil {
		a.logPanel.Append(LogLevelError, fmt.Sprintf("Config | Save failed: %v", err))
	}
}
