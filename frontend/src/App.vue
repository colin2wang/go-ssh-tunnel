<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { GetState, Start, Stop, Reload, AddTunnel, UpdateTunnel, DeleteTunnel, ToggleTunnel, GetStats } from '../wailsjs/go/app/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import type { AppState, Stats, LogEntry, TunnelConfig } from './types'
import StatsBar from './components/StatsBar.vue'
import TunnelPanel from './components/TunnelPanel.vue'
import LogPanel from './components/LogPanel.vue'
import TunnelForm from './components/TunnelForm.vue'
import PasswordPrompt from './components/PasswordPrompt.vue'

const state = ref<AppState | null>(null)
const stats = ref<Stats | null>(null)
const logs = ref<LogEntry[]>([])
const starting = ref(false)

// per-tunnel enabled checkboxes (session state only)
const enabled = reactive<Record<string, boolean>>({})

// tunnel add/edit dialog
const formVisible = ref(false)
const formTitle = ref('Add Tunnel')
const formInitial = ref<TunnelConfig | null>(null)
const editingName = ref<string | null>(null) // null = add

// password / passphrase prompt
const promptVisible = ref(false)
const promptTitle = ref('SSH Password')

let pollTimer: number | undefined

onMounted(async () => {
  state.value = await GetState()
  await refreshStats()

  EventsOn('log', (entry: LogEntry) => {
    logs.value.push(entry)
    if (logs.value.length > 500) logs.value.splice(0, logs.value.length - 500)
  })
  EventsOn('status', (msg: string) => {
    if (stats.value) stats.value.status = msg
  })

  pollTimer = window.setInterval(refreshStats, 1000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  EventsOff('log')
  EventsOff('status')
})

async function refreshStats() {
  try {
    stats.value = await GetStats()
  } catch {
    /* backend may be shutting down */
  }
}

async function onStart() {
  const ssh = state.value!.ssh
  const needPassword = ssh.authMethod === 'password' && !ssh.hasPasswordFile
  const needPassphrase = ssh.authMethod === 'key' && !ssh.hasKeyPassphraseFile
  if (needPassword || needPassphrase) {
    promptTitle.value = needPassword ? 'SSH Password' : 'Key Passphrase'
    promptVisible.value = true
    return
  }
  await doStart('')
}

async function doStart(secret: string) {
  starting.value = true
  try {
    await Start(secret)
  } catch (e: any) {
    // logged by the backend
  } finally {
    starting.value = false
    refreshStats()
  }
}

async function onStop() {
  Stop()
  refreshStats()
}

async function onReload() {
  await Reload()
  state.value = await GetState()
  refreshStats()
}

function onToggle(t: TunnelConfig, on: boolean) {
  enabled[t.name] = on
  ToggleTunnel(t, on).catch(() => {})
}

function openAdd() {
  editingName.value = null
  formTitle.value = 'Add Tunnel'
  formInitial.value = null
  formVisible.value = true
}

function openEdit(t: TunnelConfig) {
  editingName.value = t.name
  formTitle.value = 'Edit Tunnel'
  formInitial.value = t
  formVisible.value = true
}

async function onDelete(t: TunnelConfig) {
  if (!confirm(`Delete tunnel "${t.name}"? This also stops it if running.`)) return
  await DeleteTunnel(t.name)
  state.value = await GetState()
  refreshStats()
}

async function saveTunnel(t: TunnelConfig) {
  formVisible.value = false
  if (editingName.value !== null) {
    await UpdateTunnel(editingName.value, t)
  } else {
    enabled[t.name] = true
    await AddTunnel(t)
  }
  state.value = await GetState()
  refreshStats()
}

const statusColor = () => {
  const s = stats.value?.status ?? ''
  if (s === 'Connected') return 'var(--green)'
  if (s === 'Connecting...') return 'var(--yellow)'
  if (s === 'Connection failed' || s === 'Auth failed') return 'var(--red)'
  return 'var(--gray)'
}
</script>

<template>
  <div class="app">
    <header class="toolbar">
      <button class="primary" :disabled="stats?.running || starting" @click="onStart">
        {{ starting ? 'Connecting…' : '▶ Start' }}
      </button>
      <button :disabled="!stats?.running" @click="onStop">■ Stop</button>
      <button @click="onReload">⟳ Reload</button>
      <button @click="openAdd">＋ Add</button>
      <div class="spacer"></div>
      <span class="status-label">Status:</span>
      <span class="dot" :style="{ background: statusColor() }"></span>
      <span class="status mono">{{ stats?.status ?? 'Disconnected' }}</span>
    </header>

    <StatsBar :stats="stats" />

    <main class="split">
      <section class="left">
        <div class="section-header"><span class="section">Tunnels</span></div>
        <TunnelPanel
          v-if="state"
          :tunnels="state.tunnels"
          :stats="stats"
          :enabled="enabled"
          @toggle="onToggle"
          @edit="openEdit"
          @delete="onDelete"
        />
      </section>
      <section class="right">
        <LogPanel :logs="logs" />
      </section>
    </main>

    <TunnelForm
      :visible="formVisible"
      :title="formTitle"
      :initial="formInitial"
      @close="formVisible = false"
      @save="saveTunnel"
    />

    <PasswordPrompt
      :visible="promptVisible"
      :title="promptTitle"
      @close="promptVisible = false"
      @confirm="(v) => { promptVisible = false; doStart(v) }"
    />
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
}

.spacer {
  flex: 1;
}

.status-label {
  color: var(--text-dim);
}

.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.status {
  min-width: 120px;
}

.split {
  flex: 1;
  display: flex;
  min-height: 0;
}

.left {
  width: 42%;
  min-width: 340px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
}

.right {
  flex: 1;
  min-width: 0;
}

.section-header {
  padding: 8px 12px 0;
}

.section {
  font-weight: 600;
  text-transform: uppercase;
  font-size: 11px;
  letter-spacing: 0.5px;
  color: var(--text-dim);
}
</style>
