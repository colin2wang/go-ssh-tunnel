# SSH Tunnel Manager

A GUI-based SSH tunnel manager built with **Wails v2 + Vue 3 + TypeScript** (Go backend). Manage multiple SSH port-forwarding rules with real-time traffic monitoring and structured logs. Windows-first.

**[简体中文](README.zh-CN.md)** | English

## Features

- **Multi-tunnel Management** - Run multiple SSH port forwards simultaneously
- **Individual Tunnel Control** - Enable/disable each tunnel with a checkbox
- **Real-time Traffic** - Per-tunnel up/down rates, total bytes, active tunnel count
- **Auto-reconnect** - Keepalive mechanism with automatic reconnection
- **Flexible Authentication** - Password file or SSH key (with optional key passphrase)
- **YAML Configuration** - Easy to edit `config.yaml`, hot-reload from the UI
- **Structured Logs** - Level-filtered logs (DEBUG/INFO/WARN/ERROR), persisted to `logs/ssh-tunnel-YYYY-MM-DD.log`

## Screenshot

![SSH Tunnel Manager main interface](docs/images/main-interface.png)

*Disconnected state — tunnels are configured but stopped.*

![SSH Tunnel Manager connected](docs/images/main-interface-connected.png)

*Connected state — all enabled tunnels are running and listening.*

The main interface features:
- **Toolbar** - Start, Stop, Reload Config, and Add Tunnel buttons
- **Stats Cards** - UP RATE, DOWN RATE, TOTAL UP, TOTAL DOWN, ACTIVE, and TUNNELS counters
- **Tunnel List** - Each tunnel shows checkbox, name, port mapping, status, and live traffic stats
- **Log Panel** - Structured logs with timestamp, level, and message; filterable by level
- **Status Bar** - Current connection status (Disconnected/Connected)

## Prerequisites

- [Go](https://go.dev/) (1.21+)
- [Node.js](https://nodejs.org/) (LTS) and [pnpm](https://pnpm.io/) (`npm install -g pnpm`)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Build

### Development

```bash
wails dev
```

Runs the app with frontend hot-reload.

### Release Build

```powershell
.\build.ps1 -Version "1.1.0"
```

Requires `pnpm` to be available. The output stays in `build\bin\`:

```
build\bin\
├── ssh-tunnel.exe    # Application
├── config.yaml       # Configuration (kept between builds)
└── password.txt      # SSH password (edit this!)
```

> `build.ps1` never deletes an existing `config.yaml` / `password.txt`, so your settings survive rebuilds.

### Testing

```bash
go test ./...
```

## Usage

1. Edit `config.yaml` (next to the exe) to configure your SSH server and tunnels
2. Put your SSH password into `password.txt` (or configure key authentication)
3. Run `ssh-tunnel.exe`
4. Click **Start** in the toolbar to connect and start all enabled tunnels

### Command Line Options

```
ssh-tunnel.exe [options]

Options:
  -config string    Path to config file (default "config.yaml")
  -version          Show version information
```

### Runtime Files

| Path | Purpose |
| --- | --- |
| `config.yaml` | SSH server and tunnel configuration |
| `password.txt` | SSH password (plain text, keep it private) |
| `logs/ssh-tunnel-YYYY-MM-DD.log` | Daily log file |
| `secrets/` | Temporary password/key-passphrase files (auto-deleted, 0700) |

## Configuration

Edit `config.yaml` (see `configs/config-example.yaml`):

```yaml
ssh:
  host: "your-server.com"
  port: 22
  user: "your-username"
  auth:
    method: "password"           # "password" or "key"
    password_file: "password.txt"
    key_file: "~/.ssh/id_rsa"
    key_passphrase_file: ""

tunnels:
  - name: "MySQL"
    local_port: 3306
    remote_host: "127.0.0.1"
    remote_port: 3306

  - name: "Redis"
    local_port: 6379
    remote_host: "127.0.0.1"
    remote_port: 6379

options:
  server_alive_interval: 15
  server_alive_count_max: 3
```

### Password Authentication

Set `auth.method` to `"password"` and put your SSH password in the file named by `password_file` (default `password.txt`).

### Key Authentication

```yaml
auth:
  method: "key"
  key_file: "~/.ssh/id_rsa"
  key_passphrase_file: ""  # Leave empty if the key has no passphrase
```

### Adding Tunnels

- **From the UI**: click **Add Tunnel** in the toolbar
- **From disk**: edit `config.yaml` directly, then click **Reload Config**

## UI Controls

### Toolbar

- **Start** - Connect to the SSH server and start all enabled tunnels
- **Stop** - Disconnect and stop all tunnels
- **Reload Config** - Reload `config.yaml` and reset connections
- **Add Tunnel** - Add a new tunnel entry

### Tunnel List

Each tunnel displays:
- Checkbox to enable/disable the individual tunnel
- Tunnel name and port mapping (`local -> remote`)
- Status indicator (Gray = disabled/disconnected, Green = connected, Red = error)
- Real-time traffic stats: `↑ rate (total) | ↓ rate (total)`

### Log Panel

- Filter logs by level (DEBUG, INFO, WARN, ERROR)
- Auto-scrolls to the latest entries
- Format: `YYYY-MM-DD HH:MM:SS | LEVEL | Message`

## Project Structure

```
go-ssh-tunnel/
├── main.go               # Wails entry point
├── internal/
│   ├── app/              # Wails binding layer (frontend API + events)
│   ├── tunnel/           # SSH connection & forwarding logic
│   └── config/           # YAML config read/write
├── frontend/             # Vue 3 + Vite + TypeScript UI
├── configs/              # Example configuration
├── docs/images/          # Screenshots
└── build.ps1             # Release build script
```

## License

MIT License
