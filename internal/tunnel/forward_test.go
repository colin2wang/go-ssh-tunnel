package tunnel

import (
	"log"
	"net"
	"os"
	"testing"
)

func TestForwardStatusString(t *testing.T) {
	cases := map[ForwardStatus]string{
		StatusDisconnected: "Disconnected",
		StatusConnecting:   "Connecting",
		StatusConnected:    "Connected",
		StatusError:        "Error",
		ForwardStatus(99):  "Unknown",
	}
	for st, want := range cases {
		if got := st.String(); got != want {
			t.Errorf("ForwardStatus(%d).String() = %q, want %q", st, got, want)
		}
	}
}

func TestNewForwardInitialState(t *testing.T) {
	f := NewForward("Test", 8080, "127.0.0.1", 9090, log.New(os.Stderr, "", 0))
	if f.Name != "Test" {
		t.Errorf("expected name Test, got %q", f.Name)
	}
	if f.LocalPort != 8080 {
		t.Errorf("expected local port 8080, got %d", f.LocalPort)
	}
	if f.RemoteHost != "127.0.0.1" {
		t.Errorf("expected remote host 127.0.0.1, got %q", f.RemoteHost)
	}
	if f.RemotePort != 9090 {
		t.Errorf("expected remote port 9090, got %d", f.RemotePort)
	}
	if f.Status != StatusDisconnected {
		t.Errorf("expected initial status Disconnected, got %v", f.Status)
	}
	if f.Stats == nil {
		t.Error("expected non-nil Stats")
	}
}

func TestForwardStopIdempotent(t *testing.T) {
	f := NewForward("Test", 8080, "127.0.0.1", 9090, log.New(os.Stderr, "", 0))
	// Stop without a listener must not panic and should keep status Disconnected.
	f.Stop()
	if f.Status != StatusDisconnected {
		t.Errorf("expected Disconnected after Stop, got %v", f.Status)
	}
	// Stop again must be safe.
	f.Stop()
}

func TestForwardStartPortInUse(t *testing.T) {
	// Occupy a port so Start on the same port fails deterministically
	// (avoids relying on privileged-port behaviour which varies by OS).
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	f := NewForward("Dup", port, "127.0.0.1", port+1, log.New(os.Stderr, "", 0))
	if err := f.Start(nil); err == nil {
		t.Fatal("expected error starting on occupied port")
	}
	if f.Status != StatusError {
		t.Errorf("expected StatusError, got %v", f.Status)
	}
	if f.Error == "" {
		t.Error("expected Error message to be set")
	}
}
