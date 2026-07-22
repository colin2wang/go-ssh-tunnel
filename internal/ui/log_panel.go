package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// LogLevel represents the severity level of log messages.
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

// LogPanel displays connection logs with level filtering and clean support.
// It implements io.Writer so it can be used as log.SetOutput target.
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
	// Read-only: ignore user edits by resetting text on change.
	lp.entry.OnChanged = func(s string) {
		lp.mu.Lock()
		rendered := lp.renderEntriesUnlocked()
		lp.mu.Unlock()
		fyne.Do(func() {
			lp.entry.SetText(rendered)
		})
	}

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

	lp.cleanBtn = widget.NewButton("Clear", lp.Clean)
	lp.cleanBtn.Importance = widget.LowImportance

	os.MkdirAll("logs", 0755)
	logPath := fmt.Sprintf("logs/ssh-tunnel-%s.log", time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		lp.logFile = f
	}

	return lp
}

func (lp *LogPanel) Entry() *widget.Entry        { return lp.entry }
func (lp *LogPanel) LevelSelect() *widget.Select { return lp.levelSelect }
func (lp *LogPanel) CleanButton() *widget.Button { return lp.cleanBtn }

// Write implements io.Writer - allows using LogPanel as log.SetOutput target.
func (lp *LogPanel) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(strings.TrimRight(string(p), "\r\n"), "\n")
	if msg == "" {
		return len(p), nil
	}
	lp.Append(LogLevelInfo, msg)
	return len(p), nil
}

// Append adds a new log message with the specified level.
//
// IMPORTANT: fyne.Do must NOT be called while holding lp.mu. Doing so can
// deadlock the main (UI) thread: a goroutine that holds the lock and blocks in
// fyne.Do waits for the main thread, while the main thread runs another Append
// that is blocked waiting for the same lock. So we compute everything under the
// lock, release it, and only then touch the UI via fyne.Do.
func (lp *LogPanel) Append(level LogLevel, msg string) {
	lp.mu.Lock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := logLevelPrefixes[level]
	line := fmt.Sprintf("%s | %s | %s", timestamp, levelName, msg)

	lp.entries = append(lp.entries, logEntry{level: level, message: line})

	// Keep last 500 lines.
	if len(lp.entries) > 500 {
		lp.entries = lp.entries[len(lp.entries)-500:]
	}

	// Write to log file (all levels always written to file).
	if lp.logFile != nil {
		lp.logFile.WriteString(line + "\n")
		lp.logFile.Sync()
	}

	var rendered string
	updateUI := level >= lp.minLevel
	if updateUI {
		rendered = lp.renderEntriesUnlocked()
	}

	lp.mu.Unlock()

	if updateUI {
		fyne.Do(func() {
			lp.entry.SetText(rendered)
		})
		lp.scrollToBottom(rendered)
	}
}

// AppendInfo is convenience method for Info-level messages (backward compat).
func (lp *LogPanel) AppendInfo(msg string) {
	lp.Append(LogLevelInfo, msg)
}

// AppendError is convenience method for Error-level messages.
func (lp *LogPanel) AppendError(msg string) {
	lp.Append(LogLevelError, msg)
}

// SetMinLevel changes the minimum log level and refreshes display.
func (lp *LogPanel) SetMinLevel(level LogLevel) {
	lp.mu.Lock()
	lp.minLevel = level
	rendered := lp.renderEntriesUnlocked()
	lp.mu.Unlock()

	fyne.Do(func() {
		lp.entry.SetText(rendered)
	})
	lp.scrollToBottom(rendered)
}

// Clean removes all log entries.
func (lp *LogPanel) Clean() {
	lp.mu.Lock()
	lp.entries = nil
	lp.mu.Unlock()

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

// scrollToBottom keeps the view pinned to the latest line after a SetText.
func (lp *LogPanel) scrollToBottom(rendered string) {
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

func (lp *LogPanel) Close() {
	if lp.logFile != nil {
		lp.logFile.Close()
	}
}
