package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	path := writeTemp(t, "config.yaml", `
ssh:
  host: example.com
  user: alice
  auth:
    method: password
    password_file: password.txt
tunnels:
  - name: MySQL
    local_port: 3306
    remote_host: 10.0.0.1
    remote_port: 3306
  - local_port: 6379
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// applyDefaults should fill in unset values.
	if cfg.SSH.Port != 22 {
		t.Errorf("expected default port 22, got %d", cfg.SSH.Port)
	}
	if cfg.SSH.Auth.Method != "password" {
		t.Errorf("expected default auth method password, got %q", cfg.SSH.Auth.Method)
	}
	if got := cfg.Tunnels[0].RemoteHost; got != "10.0.0.1" {
		t.Errorf("expected remote_host 10.0.0.1, got %q", got)
	}
	if got := cfg.Tunnels[1].RemoteHost; got != "127.0.0.1" {
		t.Errorf("expected default remote_host 127.0.0.1, got %q", got)
	}
	if got := cfg.Tunnels[1].RemotePort; got != 6379 {
		t.Errorf("expected default remote_port 6379, got %d", got)
	}
	if got := cfg.Tunnels[1].Name; got != "Tunnel 6379->6379" {
		t.Errorf("expected default name 'Tunnel 6379->6379', got %q", got)
	}
	if cfg.Options.ServerAliveInterval != 15 {
		t.Errorf("expected default ServerAliveInterval 15, got %d", cfg.Options.ServerAliveInterval)
	}
	if cfg.Options.ServerAliveCountMax != 3 {
		t.Errorf("expected default ServerAliveCountMax 3, got %d", cfg.Options.ServerAliveCountMax)
	}
}

func TestLoadMissingHost(t *testing.T) {
	path := writeTemp(t, "config.yaml", `
ssh:
  user: alice
tunnels:
  - local_port: 3306
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing ssh.host")
	}
}

func TestLoadMissingUser(t *testing.T) {
	path := writeTemp(t, "config.yaml", `
ssh:
  host: example.com
tunnels:
  - local_port: 3306
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing ssh.user")
	}
}

func TestLoadMissingLocalPort(t *testing.T) {
	path := writeTemp(t, "config.yaml", `
ssh:
  host: example.com
  user: alice
tunnels:
  - name: Bad
    remote_port: 80
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing tunnel.local_port")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeTemp(t, "config.yaml", "ssh: [unbalanced")
	if _, err := Load(path); err == nil {
		t.Fatal("expected parse error for invalid YAML")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateDoesNotMutate(t *testing.T) {
	cfg := &Config{
		SSH:     SSHConfig{Host: "h", User: "u"},
		Tunnels: []TunnelConfig{{LocalPort: 1}},
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate unexpected error: %v", err)
	}
	// Defaults must not be applied by validate.
	if cfg.SSH.Port != 0 {
		t.Errorf("validate should not set port, got %d", cfg.SSH.Port)
	}
}

func TestReadPassword(t *testing.T) {
	dir := t.TempDir()
	pwPath := filepath.Join(dir, "password.txt")
	if err := os.WriteFile(pwPath, []byte(" secret123 \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{baseDir: dir}
	cfg.SSH.Auth.PasswordFile = "password.txt"

	got, err := cfg.ReadPassword()
	if err != nil {
		t.Fatalf("ReadPassword failed: %v", err)
	}
	if got != "secret123" {
		t.Errorf("expected trimmed password 'secret123', got %q", got)
	}
}

func TestReadPasswordEmptyFile(t *testing.T) {
	cfg := &Config{}
	if got, err := cfg.ReadPassword(); err != nil || got != "" {
		t.Errorf("expected empty password and nil err, got %q, %v", got, err)
	}
}

func TestReadPasswordMissingFile(t *testing.T) {
	cfg := &Config{baseDir: t.TempDir()}
	cfg.SSH.Auth.PasswordFile = "does-not-exist.txt"
	if _, err := cfg.ReadPassword(); err == nil {
		t.Fatal("expected error for missing password file")
	}
}

func TestReadKeyPassphrase(t *testing.T) {
	dir := t.TempDir()
	ppPath := filepath.Join(dir, "pp.txt")
	if err := os.WriteFile(ppPath, []byte("pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{baseDir: dir}
	cfg.SSH.Auth.KeyPassphraseFile = "pp.txt"

	got, err := cfg.ReadKeyPassphrase()
	if err != nil {
		t.Fatalf("ReadKeyPassphrase failed: %v", err)
	}
	if got != "pass" {
		t.Errorf("expected 'pass', got %q", got)
	}
}

func TestReadKeyPassphraseEmpty(t *testing.T) {
	cfg := &Config{}
	if got, err := cfg.ReadKeyPassphrase(); err != nil || got != "" {
		t.Errorf("expected empty passphrase and nil err, got %q, %v", got, err)
	}
}

func TestReadKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(keyPath, []byte("KEYDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{baseDir: dir}
	cfg.SSH.Auth.KeyFile = "id_rsa"

	data, err := cfg.ReadKeyFile()
	if err != nil {
		t.Fatalf("ReadKeyFile failed: %v", err)
	}
	if string(data) != "KEYDATA" {
		t.Errorf("expected KEYDATA, got %q", string(data))
	}
}

func TestReadKeyFileEmpty(t *testing.T) {
	cfg := &Config{}
	data, err := cfg.ReadKeyFile()
	if err != nil || data != nil {
		t.Errorf("expected nil data and nil err, got %q, %v", string(data), err)
	}
}

func TestResolvePathRelative(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{baseDir: dir}
	cfg.SSH.Auth.PasswordFile = "password.txt"
	got := cfg.resolvePath("password.txt")
	want := filepath.Join(dir, "password.txt")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolvePathAbsolute(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "password.txt")
	cfg := &Config{baseDir: t.TempDir()}
	got := cfg.resolvePath(abs)
	if got != abs {
		t.Errorf("expected absolute path %q unchanged, got %q", abs, got)
	}
}

func TestExpandPathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got := expandPath("~/foo")
	want := filepath.Join(home, "foo")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExpandPathNoTilde(t *testing.T) {
	got := expandPath("/abs/path")
	if got != "/abs/path" {
		t.Errorf("expected /abs/path unchanged, got %q", got)
	}
}
