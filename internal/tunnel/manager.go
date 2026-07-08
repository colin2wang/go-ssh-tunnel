package tunnel

import (
	"fmt"
	"log"
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

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("already running")
	}
	m.running = true
	m.mu.Unlock()

	m.updateStatus("Connecting...")
	m.log("Connecting to %s:%d as %s...", m.cfg.SSH.Host, m.cfg.SSH.Port, m.cfg.SSH.User)

	sshConfig, err := m.buildSSHConfig()
	if err != nil {
		m.updateStatus("Auth failed")
		return fmt.Errorf("build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.SSH.Host, m.cfg.SSH.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		m.updateStatus("Connection failed")
		return fmt.Errorf("SSH dial: %w", err)
	}

	m.mu.Lock()
	m.sshClient = client
	m.mu.Unlock()

	m.log("Connected to %s", addr)
	m.updateStatus("Connected")

	for _, tc := range m.cfg.Tunnels {
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

func (m *Manager) AddTunnel(tc config.TunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sshClient == nil {
		return fmt.Errorf("not connected")
	}

	fwd := NewForward(tc.Name, tc.LocalPort, tc.RemoteHost, tc.RemotePort, m.logger)
	if err := fwd.Start(m.sshClient); err != nil {
		return err
	}

	m.forwards = append(m.forwards, fwd)
	return nil
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
	}, nil
}

func (m *Manager) startKeepAlive() {
	go func() {
		ticker := time.NewTicker(time.Duration(m.cfg.Options.ServerAliveInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.mu.Lock()
				client := m.sshClient
				m.mu.Unlock()

				if client == nil {
					return
				}

				// Send keepalive by opening a new session
				session, err := client.NewSession()
				if err != nil {
					m.log("Keepalive failed: %v", err)
					return
				}
				session.Close()
			}
		}
	}()
}
