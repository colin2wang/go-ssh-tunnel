<script setup lang="ts">
import { computed } from 'vue'
import type { TunnelConfig, TunnelStat } from '../types'
import { formatRate, formatBytes } from '../types'

const props = defineProps<{
  tunnels: TunnelConfig[]
  stats: Stats | null
  enabled: Record<string, boolean>
}>()

const emit = defineEmits<{
  (e: 'toggle', t: TunnelConfig, enabled: boolean): void
  (e: 'edit', t: TunnelConfig): void
  (e: 'delete', t: TunnelConfig): void
}>()

function statFor(t: TunnelConfig): TunnelStat | undefined {
  return props.stats?.tunnels.find((s) => s.localPort === t.local_port)
}

function statusColor(s?: TunnelStat): string {
  switch (s?.status) {
    case 'Connected': return 'var(--green)'
    case 'Connecting': return 'var(--yellow)'
    case 'Error': return 'var(--red)'
    default: return 'var(--gray)'
  }
}

function isEnabled(t: TunnelConfig): boolean {
  return props.enabled[t.name] ?? true
}

function hotUp(t: TunnelConfig): boolean {
  return (statFor(t)?.rateUp ?? 0) > 0 || (statFor(t)?.totalUp ?? 0) > 0
}

function hotDown(t: TunnelConfig): boolean {
  return (statFor(t)?.rateDown ?? 0) > 0 || (statFor(t)?.totalDown ?? 0) > 0
}

const running = computed(() => props.stats?.running ?? false)
</script>

<template>
  <div class="tunnel-list">
    <div v-if="tunnels.length === 0" class="empty">No tunnels configured. Click "Add" to create one.</div>
    <div v-for="t in tunnels" :key="t.name + t.local_port" class="tunnel-row" :class="{ disabled: !isEnabled(t) }">
      <input
        type="checkbox"
        class="chk"
        :checked="isEnabled(t)"
        :disabled="!running"
        @change="emit('toggle', t, ($event.target as HTMLInputElement).checked)"
      />
      <span class="dot" :style="{ background: statusColor(statFor(t)) }"></span>
      <div class="info">
        <div class="name">{{ t.name }}</div>
        <div class="ports mono">
          <span>127.0.0.1:{{ t.local_port }}</span> <span class="arrow">→</span> <span>{{ t.remote_host }}:{{ t.remote_port }}</span>
        </div>
      </div>
      <div class="traffic mono">
        <div :class="{ 'up-hot': hotUp(t) }"><span class="arr-up">↑</span> {{ formatRate(statFor(t)?.rateUp ?? 0) }}</div>
        <div :class="{ 'down-hot': hotDown(t) }"><span class="arr-down">↓</span> {{ formatRate(statFor(t)?.rateDown ?? 0) }}</div>
      </div>
      <div class="totals mono">
        <div :class="{ 'up-hot': hotUp(t) }"><span class="arr-up">↑</span> {{ formatBytes(statFor(t)?.totalUp ?? 0) }}</div>
        <div :class="{ 'down-hot': hotDown(t) }"><span class="arr-down">↓</span> {{ formatBytes(statFor(t)?.totalDown ?? 0) }}</div>
      </div>
      <div class="actions">
        <button class="icon-btn" title="Edit" @click="emit('edit', t)">✎</button>
        <button class="icon-btn danger-text" title="Delete" @click="emit('delete', t)">✕</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tunnel-list {
  overflow-y: auto;
  height: 100%;
}

.empty {
  color: var(--text-dim);
  text-align: center;
  padding: 32px 12px;
}

.tunnel-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  margin: 6px 8px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 6px;
}

.tunnel-row.disabled .info,
.tunnel-row.disabled .traffic,
.tunnel-row.disabled .totals {
  opacity: 0.45;
}

.chk {
  width: auto;
  flex-shrink: 0;
}

.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
}

.info {
  flex: 1;
  min-width: 0;
}

.name {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ports {
  font-size: 11px;
  color: var(--text-dim);
}

.ports .arrow {
  color: var(--blue);
  font-weight: 600;
}

.up-hot { color: var(--green); }
.down-hot { color: var(--blue); }

.arr-up { color: var(--green); }
.arr-down { color: var(--blue); }

.traffic, .totals {
  font-size: 11px;
  color: var(--text-dim);
  text-align: right;
  flex-shrink: 0;
}

.actions {
  display: flex;
  flex-shrink: 0;
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text-dim);
  padding: 2px 6px;
}

.icon-btn:hover {
  color: var(--blue);
}

.icon-btn.danger-text:hover {
  color: var(--red);
}

.chk {
  accent-color: var(--accent);
}
</style>
