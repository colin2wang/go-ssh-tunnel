<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { TunnelConfig } from '../types'

const props = defineProps<{
  visible: boolean
  title: string
  initial: TunnelConfig | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', t: TunnelConfig): void
}>()

const form = reactive({
  name: '',
  localPort: '',
  remoteHost: '127.0.0.1',
  remotePort: '',
})

watch(
  () => props.visible,
  (v) => {
    if (v) {
      const t = props.initial
      form.name = t?.name ?? ''
      form.localPort = t ? String(t.local_port) : ''
      form.remoteHost = t?.remote_host ?? '127.0.0.1'
      form.remotePort = t ? String(t.remote_port) : ''
    }
  },
)

function save() {
  const localPort = parseInt(form.localPort, 10)
  const remotePort = parseInt(form.remotePort, 10)
  if (!Number.isInteger(localPort) || !Number.isInteger(remotePort) || localPort <= 0 || remotePort <= 0) {
    alert('Invalid port number')
    return
  }
  emit('save', {
    name: form.name || `Tunnel ${localPort}->${remotePort}`,
    local_port: localPort,
    remote_host: form.remoteHost || '127.0.0.1',
    remote_port: remotePort,
  })
}
</script>

<template>
  <div v-if="visible" class="overlay" @click.self="emit('close')">
    <div class="dialog">
      <div class="dialog-title">{{ title }}</div>
      <form @submit.prevent="save">
        <label>Name</label>
        <input v-model="form.name" placeholder="My Tunnel" />
        <label>Local Port</label>
        <input v-model="form.localPort" type="number" min="1" max="65535" placeholder="3306" />
        <label>Remote Host</label>
        <input v-model="form.remoteHost" placeholder="127.0.0.1" />
        <label>Remote Port</label>
        <input v-model="form.remotePort" type="number" min="1" max="65535" placeholder="3306" />
        <div class="buttons">
          <button type="button" @click="emit('close')">Cancel</button>
          <button type="submit" class="primary">Save</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.dialog {
  width: 380px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 18px;
}

.dialog-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 14px;
}

label {
  display: block;
  font-size: 12px;
  color: var(--text-dim);
  margin: 10px 0 4px;
}

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 18px;
}
</style>
