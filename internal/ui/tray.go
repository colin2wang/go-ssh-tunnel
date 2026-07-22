package ui

import (
	"os"
	"time"

	"fyne.io/fyne/v2"
	"github.com/getlantern/systray"

	"go-ssh-tunnel/internal/icon"
)

// startTray launches the system tray icon. Must be called before ShowAndRun.
// systray.Run blocks until Quit, so it runs in its own goroutine.
func (a *App) startTray() {
	go systray.Run(
		func() {
			systray.SetIcon(icon.IconBytes)
			systray.SetTitle("SSH Tunnel Manager")
			systray.SetTooltip("SSH Tunnel Manager")

			showItem := systray.AddMenuItem("Show Window", "Show the main window")
			systray.AddSeparator()
			quitItem := systray.AddMenuItem("Quit", "Quit the application")

			// Show / hide window
			go func() {
				for range showItem.ClickedCh {
					fyne.Do(func() {
						a.mainWindow.Show()
						a.mainWindow.RequestFocus()
					})
				}
			}()

			// Quit
			go func() {
				for range quitItem.ClickedCh {
					a.quit()
				}
			}()
		},
		nil,
	)
}

func (a *App) hideToTray() {
	a.mainWindow.Hide()
}

// quit stops tunnels, tears down systray, and exits the process.
func (a *App) quit() {
	a.manager.Stop()
	systray.Quit()
	fyne.CurrentApp().Quit()
	// Ensure the process really exits — systray or Fyne may leave goroutines alive.
	go func() {
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}
