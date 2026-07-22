package ui

import (
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go-ssh-tunnel/internal/config"
	"go-ssh-tunnel/internal/tunnel"
)

// TunnelList displays unified port forwards with traffic stats.
type TunnelList struct {
	cfg     *config.Config
	manager *tunnel.Manager
	list    *widget.List
	data    []config.TunnelConfig
	enabled map[string]bool // track which tunnels are enabled, keyed by name
	mu      sync.RWMutex

	onEdit   func(config.TunnelConfig)
	onDelete func(config.TunnelConfig)
}

// TunnelRow is a custom list item that holds strongly-typed child widgets, so
// refreshes update text/colors in place instead of walking fragile type
// assertions on a generic container. It also renders as a card with colored
// status text and inline edit/delete controls.
type TunnelRow struct {
	widget.BaseWidget
	check        *widget.Check
	dot          *canvas.Circle
	bg           *canvas.Rectangle
	nameLabel    *widget.Label
	statusLabel  *canvas.Text
	trafficLabel *widget.Label
	editBtn      *widget.Button
	delBtn       *widget.Button
	info         *fyne.Container
	onToggle     func(config.TunnelConfig, bool)
	onEdit       func(config.TunnelConfig)
	onDelete     func(config.TunnelConfig)
	tc           config.TunnelConfig
}

func NewTunnelRow(
	onToggle func(config.TunnelConfig, bool),
	onEdit func(config.TunnelConfig),
	onDelete func(config.TunnelConfig),
) *TunnelRow {
	r := &TunnelRow{
		check:    widget.NewCheck("", nil),
		dot:      canvas.NewCircle(color.Gray{Y: 100}),
		bg:       cardBackground(),
		onToggle: onToggle,
		onEdit:   onEdit,
		onDelete: onDelete,
	}
	r.editBtn = widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		if r.onEdit != nil {
			r.onEdit(r.tc)
		}
	})
	r.delBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if r.onDelete != nil {
			r.onDelete(r.tc)
		}
	})
	r.ExtendBaseWidget(r)
	return r
}

func (r *TunnelRow) CreateRenderer() fyne.WidgetRenderer {
	r.nameLabel = widget.NewLabel("")
	r.nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	r.statusLabel = canvas.NewText("", color.White)
	r.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	r.trafficLabel = widget.NewLabel("")

	top := container.NewHBox(r.nameLabel, layout.NewSpacer(), r.statusLabel)
	r.info = container.NewVBox(top, r.trafficLabel)

	return &tunnelRowRenderer{
		row: r,
		objects: []fyne.CanvasObject{
			r.bg, r.check, r.dot, r.info, r.editBtn, r.delBtn,
		},
	}
}

// Update refreshes the row's content in place.
func (r *TunnelRow) Update(tc config.TunnelConfig, status tunnel.ForwardStatus, rateUp, rateDown float64, totalUp, totalDown int64, enabled bool) {
	r.tc = tc
	r.check.SetChecked(enabled)
	r.check.OnChanged = func(checked bool) {
		if r.onToggle != nil {
			r.onToggle(r.tc, checked)
		}
	}
	c := statusColor(status)
	// Show blue indicator when connected and actively transferring data.
	if status == tunnel.StatusConnected && (rateUp > 0 || rateDown > 0) {
		c = activeColor()
	}
	r.dot.FillColor = c
	r.dot.Refresh()
	r.nameLabel.SetText(fmt.Sprintf("%s (:%d)", tc.Name, tc.LocalPort))
	r.statusLabel.Text = status.String()
	r.statusLabel.Color = c
	r.statusLabel.Refresh()
	r.trafficLabel.SetText(fmt.Sprintf("↑ %s (%s)  ↓ %s (%s)",
		tunnel.FormatRate(rateUp),
		tunnel.FormatBytes(totalUp),
		tunnel.FormatRate(rateDown),
		tunnel.FormatBytes(totalDown),
	))
}

type tunnelRowRenderer struct {
	row     *TunnelRow
	objects []fyne.CanvasObject
}

func (r *tunnelRowRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *tunnelRowRenderer) Destroy()                     {}
func (r *tunnelRowRenderer) Refresh() {
	r.row.check.Refresh()
	r.row.dot.Refresh()
	r.row.bg.Refresh()
	r.row.info.Refresh()
	r.row.editBtn.Refresh()
	r.row.delBtn.Refresh()
}
func (r *tunnelRowRenderer) Layout(size fyne.Size) {
	const padding float32 = 8

	r.row.bg.Resize(size)
	r.row.bg.Move(fyne.NewPos(0, 0))

	x := padding
	checkSize := r.row.check.MinSize()
	r.row.check.Resize(checkSize)
	r.row.check.Move(fyne.NewPos(x, (size.Height-checkSize.Height)/2))
	x += checkSize.Width + 6

	dotSize := fyne.NewSize(12, 12)
	r.row.dot.Resize(dotSize)
	r.row.dot.Move(fyne.NewPos(x, (size.Height-dotSize.Height)/2))
	x += dotSize.Width + 8

	editSize := r.row.editBtn.MinSize()
	delSize := r.row.delBtn.MinSize()
	r.row.delBtn.Resize(delSize)
	r.row.delBtn.Move(fyne.NewPos(size.Width-padding-delSize.Width, (size.Height-delSize.Height)/2))
	r.row.editBtn.Resize(editSize)
	r.row.editBtn.Move(fyne.NewPos(size.Width-padding-delSize.Width-6-editSize.Width, (size.Height-editSize.Height)/2))

	infoW := size.Width - padding - delSize.Width - 6 - editSize.Width - x
	infoSize := fyne.NewSize(infoW, size.Height-2*padding)
	r.row.info.Resize(infoSize)
	r.row.info.Move(fyne.NewPos(x, padding))
}
func (r *tunnelRowRenderer) MinSize() fyne.Size {
	checkMin := r.row.check.MinSize()
	nameMin := r.row.nameLabel.MinSize()
	statusMin := r.row.statusLabel.MinSize()
	trafficMin := r.row.trafficLabel.MinSize()
	editMin := r.row.editBtn.MinSize()
	delMin := r.row.delBtn.MinSize()
	w := float32(8) + checkMin.Width + 6 + 12 + 8 + nameMin.Width + statusMin.Width + 8 + editMin.Width + 6 + delMin.Width + 8
	h := float32(20) + nameMin.Height + trafficMin.Height + 4
	return fyne.NewSize(w, h)
}

func NewTunnelList(cfg *config.Config, mgr *tunnel.Manager) *TunnelList {
	tl := &TunnelList{cfg: cfg, manager: mgr, enabled: make(map[string]bool)}
	tl.data = cfg.Tunnels
	for i := range tl.data {
		tl.enabled[tl.data[i].Name] = true
	}
	tl.list = widget.NewList(
		func() int {
			tl.mu.RLock()
			defer tl.mu.RUnlock()
			return len(tl.data)
		},
		func() fyne.CanvasObject {
			return NewTunnelRow(
				func(tc config.TunnelConfig, checked bool) { tl.toggleTunnel(tc, checked) },
				func(tc config.TunnelConfig) { tl.editTunnel(tc) },
				func(tc config.TunnelConfig) { tl.deleteTunnel(tc) },
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			tl.mu.RLock()
			if id >= len(tl.data) {
				tl.mu.RUnlock()
				return
			}
			tc := tl.data[id]
			// Safe single assertion: the list item is always a *TunnelRow we
			// created ourselves.
			row := obj.(*TunnelRow)

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

			enabled := tl.enabled[tc.Name]
			tl.mu.RUnlock()
			row.Update(tc, status, rateUp, rateDown, totalUp, totalDown, enabled)
		},
	)
	return tl
}

// toggleTunnel starts or stops a tunnel based on checkbox state.
func (tl *TunnelList) toggleTunnel(tc config.TunnelConfig, enabled bool) {
	if tl.manager == nil || !tl.manager.IsRunning() {
		return
	}
	if enabled {
		if err := tl.manager.StartTunnel(tc); err != nil {
			tl.manager.Log(fmt.Sprintf("[Tunnel] Failed to start %s: %v", tc.Name, err))
		} else {
			tl.manager.Log(fmt.Sprintf("[Tunnel] %s enabled", tc.Name))
		}
	} else {
		tl.manager.StopTunnel(tc.Name)
		tl.manager.Log(fmt.Sprintf("[Tunnel] %s disabled", tc.Name))
	}
	tl.mu.Lock()
	tl.enabled[tc.Name] = enabled
	tl.mu.Unlock()
	fyne.Do(func() {
		tl.list.Refresh()
	})
}

func (tl *TunnelList) editTunnel(tc config.TunnelConfig) {
	if tl.onEdit != nil {
		tl.onEdit(tc)
	}
}

func (tl *TunnelList) deleteTunnel(tc config.TunnelConfig) {
	if tl.onDelete != nil {
		tl.onDelete(tc)
	}
}

func (tl *TunnelList) List() *widget.List { return tl.list }

func (tl *TunnelList) SetManager(mgr *tunnel.Manager) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.manager = mgr
}

func (tl *TunnelList) SetConfig(cfg *config.Config) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.cfg = cfg
}

func (tl *TunnelList) Refresh() {
	tl.mu.Lock()
	// Copy config tunnels and enabled map under write lock.
	newData := make([]config.TunnelConfig, len(tl.cfg.Tunnels))
	copy(newData, tl.cfg.Tunnels)
	newEnabled := make(map[string]bool, len(tl.enabled))
	for k, v := range tl.enabled {
		newEnabled[k] = v
	}
	// Ensure newly added tunnels default to enabled.
	for i := range newData {
		if _, ok := newEnabled[newData[i].Name]; !ok {
			newEnabled[newData[i].Name] = true
		}
	}
	tl.data = newData
	tl.enabled = newEnabled
	tl.mu.Unlock()
	tl.list.Refresh()
}
