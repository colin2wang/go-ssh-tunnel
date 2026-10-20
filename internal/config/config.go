package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type AuthConfig struct {
	Method            string `yaml:"method"`
	PasswordFile      string `yaml:"password_file"`
	KeyFile           string `yaml:"key_file"`
	KeyPassphraseFile string `yaml:"key_passphrase_file"`
}

type TunnelConfig struct {
	Name       string `yaml:"name" json:"name"`
	LocalPort  int    `yaml:"local_port" json:"local_port"`
	RemoteHost string `yaml:"remote_host" json:"remote_host"`
	RemotePort int    `yaml:"remote_port" json:"remote_port"`
}

type OptionsConfig struct {
	ServerAliveInterval int `yaml:"server_alive_interval"`
	ServerAliveCountMax int `yaml:"server_alive_count_max"`
}

type SSHConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	User string     `yaml:"user"`
	Auth AuthConfig `yaml:"auth"`
}

type Config struct {
	SSH     SSHConfig      `yaml:"ssh"`
	Tunnels []TunnelConfig `yaml:"tunnels"`
	Options OptionsConfig  `yaml:"options"`
	baseDir string         // directory of config file, for resolving relative paths
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.baseDir = filepath.Dir(path)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	return cfg, nil
}

// Default returns an empty config filled with defaults. Used when the config
// file cannot be loaded but the application should still start (the error is
// surfaced in the UI and a Reload can pick up a fixed config).
func Default() *Config {
	c := &Config{}
	c.applyDefaults()
	return c
}

// validate checks that all required fields are present. It does not mutate
// the config; default values are filled by applyDefaults.
func (c *Config) validate() error {
	if c.SSH.Host == "" {
		return fmt.Errorf("ssh.host is required")
	}
	if c.SSH.User == "" {
		return fmt.Errorf("ssh.user is required")
	}
	for i, t := range c.Tunnels {
		if t.LocalPort == 0 {
			return fmt.Errorf("tunnel[%d]: local_port is required", i)
		}
	}
	return nil
}

// applyDefaults fills in optional values that were left unset, so the rest of
// the program can rely on them being populated.
func (c *Config) applyDefaults() {
	if c.SSH.Port == 0 {
		c.SSH.Port = 22
	}
	if c.SSH.Auth.Method == "" {
		c.SSH.Auth.Method = "password"
	}
	for i := range c.Tunnels {
		if c.Tunnels[i].RemoteHost == "" {
			c.Tunnels[i].RemoteHost = "127.0.0.1"
		}
		if c.Tunnels[i].RemotePort == 0 {
			c.Tunnels[i].RemotePort = c.Tunnels[i].LocalPort
		}
		if c.Tunnels[i].Name == "" {
			c.Tunnels[i].Name = fmt.Sprintf("Tunnel %d->%d", c.Tunnels[i].LocalPort, c.Tunnels[i].RemotePort)
		}
	}
	if c.Options.ServerAliveInterval == 0 {
		c.Options.ServerAliveInterval = 15
	}
	if c.Options.ServerAliveCountMax == 0 {
		c.Options.ServerAliveCountMax = 3
	}
}

func (c *Config) ReadPassword() (string, error) {
	if c.SSH.Auth.PasswordFile == "" {
		return "", nil
	}
	path := c.resolvePath(c.SSH.Auth.PasswordFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read password file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (c *Config) ReadKeyPassphrase() (string, error) {
	if c.SSH.Auth.KeyPassphraseFile == "" {
		return "", nil
	}
	path := c.resolvePath(c.SSH.Auth.KeyPassphraseFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read key passphrase file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (c *Config) ReadKeyFile() ([]byte, error) {
	if c.SSH.Auth.KeyFile == "" {
		return nil, nil
	}
	path := c.resolvePath(c.SSH.Auth.KeyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file %s: %w", path, err)
	}
	return data, nil
}

func (c *Config) resolvePath(path string) string {
	path = expandPath(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.baseDir, path)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}

// Save marshals the config back to YAML and writes it to the given path. The
// unexported baseDir field is omitted automatically by the yaml encoder, so
// unchanged fields (SSH, options, other tunnels) are preserved.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
