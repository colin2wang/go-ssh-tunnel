package tunnel

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

type ForwardStatus int

const (
	StatusDisconnected ForwardStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
)

func (s ForwardStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "Disconnected"
	case StatusConnecting:
		return "Connecting"
	case StatusConnected:
		return "Connected"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

type Forward struct {
	Name       string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Status     ForwardStatus
	Stats      *TrafficStats
	Error      string

	mu      sync.Mutex
	ln      net.Listener
	clients []net.Conn
	stopCh  chan struct{}
	logger  *log.Logger
}

func NewForward(name string, localPort int, remoteHost string, remotePort int, logger *log.Logger) *Forward {
	return &Forward{
		Name:       name,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Status:     StatusDisconnected,
		Stats:      NewTrafficStats(),
		stopCh:     make(chan struct{}),
		logger:     logger,
	}
}

func (f *Forward) Start(sshClient *ssh.Client) error {
	f.mu.Lock()
	f.Status = StatusConnecting
	f.mu.Unlock()

	listenAddr := fmt.Sprintf("127.0.0.1:%d", f.LocalPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		f.mu.Lock()
		f.Status = StatusError
		f.Error = err.Error()
		f.mu.Unlock()
		return fmt.Errorf("listen on %s: %w", listenAddr, err)
	}

	f.mu.Lock()
	f.ln = ln
	f.Status = StatusConnected
	f.Error = ""
	f.mu.Unlock()

	f.logger.Printf("[%s] Listening on %s -> %s:%d", f.Name, listenAddr, f.RemoteHost, f.RemotePort)

	go f.acceptLoop(sshClient)
	return nil
}

func (f *Forward) acceptLoop(sshClient *ssh.Client) {
	for {
		f.mu.Lock()
		ln := f.ln
		f.mu.Unlock()

		if ln == nil {
			return
		}

		conn, err := ln.Accept()
		if err != nil {
			f.mu.Lock()
			stopped := f.ln == nil
			f.mu.Unlock()
			if stopped {
				return
			}
			f.logger.Printf("[%s] Accept error: %v", f.Name, err)
			continue
		}

		f.mu.Lock()
		f.clients = append(f.clients, conn)
		f.mu.Unlock()

		go f.handleConn(sshClient, conn)
	}
}

func (f *Forward) handleConn(sshClient *ssh.Client, localConn net.Conn) {
	defer localConn.Close()

	remoteAddr := fmt.Sprintf("%s:%d", f.RemoteHost, f.RemotePort)
	remoteConn, err := sshClient.Dial("tcp", remoteAddr)
	if err != nil {
		f.logger.Printf("[%s] Dial remote %s failed: %v", f.Name, remoteAddr, err)
		f.removeClient(localConn)
		return
	}
	defer remoteConn.Close()

	f.logger.Printf("[%s] Connection established to %s", f.Name, remoteAddr)

	done := make(chan struct{}, 2)

	go func() {
		n, _ := io.Copy(remoteConn, localConn)
		f.Stats.AddUpload(n)
		done <- struct{}{}
	}()

	go func() {
		n, _ := io.Copy(localConn, remoteConn)
		f.Stats.AddDownload(n)
		done <- struct{}{}
	}()

	<-done
	f.removeClient(localConn)
	f.logger.Printf("[%s] Connection to %s closed", f.Name, remoteAddr)
}

func (f *Forward) removeClient(conn net.Conn) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, c := range f.clients {
		if c == conn {
			f.clients = append(f.clients[:i], f.clients[i+1:]...)
			return
		}
	}
}

func (f *Forward) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.stopCh != nil {
		close(f.stopCh)
		f.stopCh = make(chan struct{})
	}

	if f.ln != nil {
		f.ln.Close()
		f.ln = nil
	}

	for _, c := range f.clients {
		c.Close()
	}
	f.clients = nil

	f.Status = StatusDisconnected
	f.Error = ""
}
