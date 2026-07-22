package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go-ssh-tunnel/internal/tunnel"
)

// statusColor maps a forward status to its indicator color.
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

// activeColor returns the indicator color when a connected tunnel has active traffic.
func activeColor() color.Color {
	return color.NRGBA{R: 0, G: 140, B: 255, A: 255} // Blue
}

// sectionLabel returns a label styled as a small uppercase section heading.
func sectionLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	l.Alignment = fyne.TextAlignLeading
	return l
}

// cardBackground returns a rounded rectangle that acts as a card backdrop,
// tinted with the theme's hover color so it adapts to light/dark themes.
func cardBackground() *canvas.Rectangle {
	r := canvas.NewRectangle(theme.Color(theme.ColorNameHover))
	r.CornerRadius = 6
	r.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	r.StrokeWidth = 1
	return r
}
