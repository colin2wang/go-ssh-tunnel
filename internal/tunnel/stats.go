package tunnel

import (
	"fmt"
	"sync"
	"time"
)

type statsEntry struct {
	bytes    int64
	finished time.Time
}

type TrafficStats struct {
	mu          sync.Mutex
	totalUp     int64
	totalDown   int64
	historyUp   []statsEntry
	historyDown []statsEntry
	window      time.Duration
}

func NewTrafficStats() *TrafficStats {
	return &TrafficStats{
		window: 60 * time.Second,
	}
}

func (s *TrafficStats) AddUpload(n int64) {
	s.mu.Lock()
	s.totalUp += n
	s.historyUp = append(s.historyUp, statsEntry{bytes: n, finished: time.Now()})
	s.mu.Unlock()
}

func (s *TrafficStats) AddDownload(n int64) {
	s.mu.Lock()
	s.totalDown += n
	s.historyDown = append(s.historyDown, statsEntry{bytes: n, finished: time.Now()})
	s.mu.Unlock()
}

func (s *TrafficStats) GetRate() (up, down float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-s.window)

	// Calculate upload rate
	var windowUp int64
	filteredUp := s.historyUp[:0]
	for _, e := range s.historyUp {
		if e.finished.After(cutoff) {
			filteredUp = append(filteredUp, e)
			windowUp += e.bytes
		}
	}
	s.historyUp = filteredUp

	// Calculate download rate
	var windowDown int64
	filteredDown := s.historyDown[:0]
	for _, e := range s.historyDown {
		if e.finished.After(cutoff) {
			filteredDown = append(filteredDown, e)
			windowDown += e.bytes
		}
	}
	s.historyDown = filteredDown

	elapsed := s.window.Seconds()
	if elapsed > 0 {
		up = float64(windowUp) / elapsed
		down = float64(windowDown) / elapsed
	}
	return
}

func (s *TrafficStats) GetTotal() (up, down int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalUp, s.totalDown
}

func FormatBytes(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func FormatRate(bps float64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bps >= GB:
		return fmt.Sprintf("%.1f GB/s", bps/float64(GB))
	case bps >= MB:
		return fmt.Sprintf("%.1f MB/s", bps/float64(MB))
	case bps >= KB:
		return fmt.Sprintf("%.1f KB/s", bps/float64(KB))
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}
