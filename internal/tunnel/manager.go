package tunnel

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"go-ssh-tunnel/internal/config"

	"golang.org/x/crypto/ssh"
)

type Manager struct {
	cfg        *config.Config
	sshClient  *ssh.Client
	forwards   []*Forward
	logger     *log.Logger
	mu         sync.Mutex
	running    bool
	stopCh     chan struct{}
	StatusFunc func(string) // callback for status updates
	LogFunc    func(string) // callback for log messages
}

func NewManager(cfg *config.Config) *Manager {
	logger := log.New(os.Stdout, "[tunnel] ", log.LstdFlags)
	return &Manager{
		cfg:    cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (m *Manager) SetCallbacks(logFunc func(string), statusFunc func(string)) {
	m.LogFunc = logFunc
	m.StatusFunc = statusFunc
	m.logger = log.New(os.Stdout, "[tunnel] ", 0)
}

func (m *Manager) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	m.logger.Print(msg)
	if m.LogFunc != nil {
		m.LogFunc(msg)
	}
}

// Log is public version of log for external callers
func (m *Manager) Log(msg string) {
	m.logger.Print(msg)
	if m.LogFunc != nil {
		m.LogFunc(msg)
	}
}

func (m *Manager) updateStatus(msg string) {
	if m.StatusFunc != nil {
		m.StatusFunc(msg)
	}
}

func (m *Manager) GetForwards() []*Forward {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.forwards
}

// hasForward reports whether a forward with the given name is already tracked.
// Caller must hold m.mu.
func (m *Manager) hasForward(name string) bool {
	for _, f := range m.forwards {
		if f.Name == name {
			return true
		}
	}
	return false
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("already running")
	}
	m.running = true
	// Fresh stop channel for this session so Stop can reliably signal the
	// keepalive goroutine even across reconnects.
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	m.updateStatus("Connecting...")
	m.log("Connecting to %s:%d as %s...", m.cfg.SSH.Host, m.cfg.SSH.Port, m.cfg.SSH.User)

	sshConfig, err := m.buildSSHConfig()
	if err != nil {
		m.updateStatus("Auth failed")
		return fmt.Errorf("build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.SSH.Host, m.cfg.SSH.Port)
	client, err := dialWithTimeout(addr, sshConfig)
	if err != nil {
		m.updateStatus("Connection failed")
		return fmt.Errorf("SSH dial: %w", err)
	}

	// If Stop()/Reload was invoked while we were dialing, abandon the freshly
	// established client instead of wiring it into a manager that the UI may
	// have already replaced. Without this, a concurrent Reload leaves a zombie
	// connection and the app never returns to a consistent state.
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		client.Close()
		m.updateStatus("Disconnected")
		m.log("Connect aborted: stop requested during handshake")
		return fmt.Errorf("stopped during connect")
	}
	m.sshClient = client
	m.mu.Unlock()

	m.log("Connected to %s", addr)
	m.updateStatus("Connected")

	for _, tc := range m.cfg.Tunnels {
		// Skip duplicates by name to avoid double-appending on re-Start.
		if m.hasForward(tc.Name) {
			continue
		}
		fwd := NewForward(tc.Name, tc.LocalPort, tc.RemoteHost, tc.RemotePort, m.logger)
		fwd.LogFunc = m.LogFunc
		if err := fwd.Start(client); err != nil {
			m.log("[%s] Failed to start: %v", tc.Name, err)
		} else {
			m.log("[%s] Public: %s:%d", tc.Name, m.cfg.SSH.Host, tc.LocalPort)
		}
		m.mu.Lock()
		m.forwards = append(m.forwards, fwd)
		m.mu.Unlock()
	}

	m.log("All tunnels started")
	m.startKeepAlive()
	return nil
}

// sshHandshakeTimeout bounds the whole SSH handshake (TCP + key exchange +
// authentication). Unlike the per-connection deadline set in dialWithTimeout,
// the handshake timeout lives inside ssh.ClientConfig and is honoured by the
// x/crypto/ssh package for the entire handshake even though it may refresh the
// underlying conn deadline during key exchange. Without it an unresponsive
// server that accepts the TCP connection but stalls mid-handshake would hang
// the connect goroutine forever, leaving the UI stuck on "Connecting...".
const sshHandshakeTimeout = 15 * time.Second

// dialWithTimeout establishes the TCP connection and completes the SSH
// handshake under an overall deadline, so an unreachable or unresponsive
// server fails fast instead of hanging the connect goroutine (which would
// otherwise leave the UI stuck on "Connecting..." with no way to recover).
func dialWithTimeout(addr string, sshConfig *ssh.ClientConfig) (*ssh.Client, error) {
	const timeout = sshHandshakeTimeout
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return nil, err
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// Clear the handshake deadline so subsequent operations (keepalive,
	// tunnel dial, data transfer) are not killed by the 15s deadline.
	conn.SetDeadline(time.Time{})
	return ssh.NewClient(sshConn, chans, reqs), nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}

	m.running = false

	// Capture values before releasing lock
	forwards := m.forwards
	sshClient := m.sshClient
	m.forwards = nil
	m.sshClient = nil
	// Signal the keepalive goroutine to exit (reliably, instead of relying on
	// the sshClient becoming nil).
	close(m.stopCh)
	m.mu.Unlock()

	// Stop forwards outside lock
	for _, f := range forwards {
		f.Stop()
	}

	// Close SSH client outside lock
	if sshClient != nil {
		sshClient.Close()
	}

	m.updateStatus("Disconnected")
	m.log("All tunnels stopped")
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Manager) RemoveTunnel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, f := range m.forwards {
		if f.Name == name {
			f.Stop()
			m.forwards = append(m.forwards[:i], m.forwards[i+1:]...)
			return
		}
	}
}

// StartTunnel starts a single tunnel by config
func (m *Manager) StartTunnel(tc config.TunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sshClient == nil {
		return fmt.Errorf("not connected")
	}

	// Check if already running
	for _, f := range m.forwards {
		if f.Name == tc.Name {
			return fmt.Errorf("tunnel %s already running", tc.Name)
		}
	}

	fwd := NewForward(tc.Name, tc.LocalPort, tc.RemoteHost, tc.RemotePort, m.logger)
	fwd.LogFunc = m.LogFunc
	if err := fwd.Start(m.sshClient); err != nil {
		return err
	}

	m.forwards = append(m.forwards, fwd)
	return nil
}

// StopTunnel stops a single tunnel by name
func (m *Manager) StopTunnel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, f := range m.forwards {
		if f.Name == name {
			f.Stop()
			m.forwards = append(m.forwards[:i], m.forwards[i+1:]...)
			return
		}
	}
}

func (m *Manager) buildSSHConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	switch m.cfg.SSH.Auth.Method {
	case "password":
		m.log("Auth method: password")
		m.log("Password file: %s", m.cfg.SSH.Auth.PasswordFile)
		pass, err := m.cfg.ReadPassword()
		if err != nil {
			return nil, err
		}
		if pass == "" {
			return nil, fmt.Errorf("password required but none configured (file: %s)", m.cfg.SSH.Auth.PasswordFile)
		}
		m.log("Password loaded: %d chars", len(pass))
		authMethods = append(authMethods, ssh.Password(pass))

	case "key":
		keyData, err := m.cfg.ReadKeyFile()
		if err != nil {
			return nil, err
		}
		if keyData == nil {
			return nil, fmt.Errorf("key file required but none configured")
		}

		var signer ssh.Signer
		passphrase, _ := m.cfg.ReadKeyPassphrase()
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	default:
		return nil, fmt.Errorf("unsupported auth method: %s", m.cfg.SSH.Auth.Method)
	}

	return &ssh.ClientConfig{
		User:            m.cfg.SSH.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		// Bound the entire handshake so a server that connects but stalls
		// mid-handshake fails fast instead of hanging this goroutine forever.
		Timeout: sshHandshakeTimeout,
	}, nil
}

func (m *Manager) startKeepAlive() {
	go func() {
		ticker := time.NewTicker(time.Duration(m.cfg.Options.ServerAliveInterval) * time.Second)
		defer ticker.Stop()
		failCount := 0

		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.mu.Lock()
				client := m.sshClient
				running := m.running
				m.mu.Unlock()

				if client == nil || !running {
					return
				}

				// Standard SSH keepalive: requests a reply from the server.
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					failCount++
					m.log("Keepalive failed (%d/%d): %v", failCount, m.cfg.Options.ServerAliveCountMax, err)
					if failCount >= m.cfg.Options.ServerAliveCountMax {
						m.log("Connection lost, reconnecting...")
						m.reconnect()
						return
					}
				} else {
					failCount = 0
				}
			}
		}
	}()
}

// reconnect closes the dead SSH client, re-dials, and restarts all forwards.
func (m *Manager) reconnect() {
	// Tear down old client and forwards under lock.
	m.mu.Lock()
	oldClient := m.sshClient
	oldForwards := m.forwards
	m.sshClient = nil
	m.forwards = nil
	m.mu.Unlock()

	for _, f := range oldForwards {
		f.Stop()
	}
	if oldClient != nil {
		oldClient.Close()
	}

	// Retry loop with backoff.
	const maxRetries = 10
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check if we were stopped while waiting.
		select {
		case <-m.stopCh:
			m.updateStatus("Disconnected")
			return
		default:
		}

		m.updateStatus(fmt.Sprintf("Reconnecting... (%d/%d)", attempt, maxRetries))
		m.log("Reconnect attempt %d/%d", attempt, maxRetries)

		sshConfig, err := m.buildSSHConfig()
		if err != nil {
			m.log("Reconnect build config failed: %v", err)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		addr := fmt.Sprintf("%s:%d", m.cfg.SSH.Host, m.cfg.SSH.Port)
		client, err := dialWithTimeout(addr, sshConfig)
		if err != nil {
			m.log("Reconnect dial failed: %v", err)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		// Verify the connection is still wanted.
		m.mu.Lock()
		if !m.running {
			m.mu.Unlock()
			client.Close()
			m.updateStatus("Disconnected")
			return
		}
		m.sshClient = client
		m.mu.Unlock()

		m.log("Reconnected to %s", addr)
		m.updateStatus("Connected")

		// Restart forwards.
		for _, tc := range m.cfg.Tunnels {
			fwd := NewForward(tc.Name, tc.LocalPort, tc.RemoteHost, tc.RemotePort, m.logger)
			fwd.LogFunc = m.LogFunc
			if err := fwd.Start(client); err != nil {
				m.log("[%s] Failed to restart: %v", tc.Name, err)
			} else {
				m.log("[%s] Reconnected", tc.Name)
			}
			m.mu.Lock()
			m.forwards = append(m.forwards, fwd)
			m.mu.Unlock()
		}

		m.startKeepAlive()
		return
	}

	m.log("Reconnect failed after %d attempts", maxRetries)
	m.updateStatus("Reconnect failed")
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}
