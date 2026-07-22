package tunnel

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{5 * 1024 * 1024 * 1024, "5.0 GB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatRate(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0 B/s"},
		{512.0, "512 B/s"},
		{1024.0, "1.0 KB/s"},
		{1024 * 1024, "1.0 MB/s"},
		{1024 * 1024 * 1024, "1.0 GB/s"},
	}
	for _, c := range cases {
		if got := FormatRate(c.in); got != c.want {
			t.Errorf("FormatRate(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTrafficStatsTotals(t *testing.T) {
	s := NewTrafficStats()
	s.AddUpload(100)
	s.AddUpload(50)
	s.AddDownload(200)

	up, down := s.GetTotal()
	if up != 150 {
		t.Errorf("expected total up 150, got %d", up)
	}
	if down != 200 {
		t.Errorf("expected total down 200, got %d", down)
	}
}

func TestTrafficStatsRateWithinWindow(t *testing.T) {
	s := NewTrafficStats()
	s.window = time.Hour // wide window so entries stay fresh
	s.AddUpload(1024)
	s.AddDownload(2048)

	up, down := s.GetRate()
	// rate = bytes / window.Seconds(); with 1h window these are tiny but > 0
	if up <= 0 {
		t.Errorf("expected up rate > 0, got %v", up)
	}
	if down <= 0 {
		t.Errorf("expected down rate > 0, got %v", down)
	}
	// Totals preserved.
	tu, td := s.GetTotal()
	if tu != 1024 || td != 2048 {
		t.Errorf("expected totals 1024/2048, got %d/%d", tu, td)
	}
}

func TestTrafficStatsRateExpired(t *testing.T) {
	s := NewTrafficStats()
	s.window = time.Nanosecond // entries expire immediately
	s.AddUpload(1024)
	time.Sleep(2 * time.Millisecond)

	up, down := s.GetRate()
	if up != 0 {
		t.Errorf("expected up rate 0 after expiry, got %v", up)
	}
	if down != 0 {
		t.Errorf("expected down rate 0 after expiry, got %v", down)
	}
	// Total is independent of the rate window.
	tu, _ := s.GetTotal()
	if tu != 1024 {
		t.Errorf("expected total up preserved 1024, got %d", tu)
	}
}

func TestTrafficStatsConcurrent(t *testing.T) {
	s := NewTrafficStats()
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				s.AddUpload(1)
				s.AddDownload(1)
				s.GetRate()
				s.GetTotal()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	up, down := s.GetTotal()
	if up != 4000 || down != 4000 {
		t.Errorf("expected totals 4000/4000, got %d/%d", up, down)
	}
}
