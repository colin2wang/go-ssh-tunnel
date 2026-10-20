// Shared frontend types mirroring the Go bindings in internal/app.

export interface SSHInfo {
  host: string
  port: number
  user: string
  authMethod: string
  hasPasswordFile: boolean
  hasKeyFile: boolean
  hasKeyPassphraseFile: boolean
}

export interface TunnelConfig {
  name: string
  local_port: number
  remote_host: string
  remote_port: number
}

export interface AppState {
  version: string
  configPath: string
  ssh: SSHInfo
  tunnels: TunnelConfig[]
}

export interface TunnelStat {
  name: string
  localPort: number
  status: string // Disconnected | Connecting | Connected | Error
  error: string
  rateUp: number
  rateDown: number
  totalUp: number
  totalDown: number
}

export interface Stats {
  running: boolean
  status: string
  upRate: number
  downRate: number
  totalUp: number
  totalDown: number
  active: number
  tunnels: TunnelStat[]
}

export interface LogEntry {
  level: string // debug|info|warn|error
  time: string
  msg: string
}

export const LOG_LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR'] as const

export function formatBytes(b: number): string {
  const KB = 1024, MB = 1024 * KB, GB = 1024 * MB
  if (b >= GB) return (b / GB).toFixed(1) + ' GB'
  if (b >= MB) return (b / MB).toFixed(1) + ' MB'
  if (b >= KB) return (b / KB).toFixed(1) + ' KB'
  return b.toFixed(0) + ' B'
}

export function formatRate(bps: number): string {
  const KB = 1024, MB = 1024 * KB, GB = 1024 * MB
  if (bps >= GB) return (bps / GB).toFixed(1) + ' GB/s'
  if (bps >= MB) return (bps / MB).toFixed(1) + ' MB/s'
  if (bps >= KB) return (bps / KB).toFixed(1) + ' KB/s'
  return bps.toFixed(0) + ' B/s'
}
