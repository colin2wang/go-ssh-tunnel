<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import type { LogEntry } from '../types'
import { LOG_LEVELS } from '../types'

const props = defineProps<{ logs: LogEntry[] }>()

const minLevel = ref<Number>(0)
const selected = ref('DEBUG')
const body = ref<HTMLElement | null>(null)

const levelIndex = (l: string) => LOG_LEVELS.indexOf(l.toUpperCase() as any)

const visible = computed(() =>
  props.logs.filter((e) => levelIndex(e.level) >= (minLevel.value as number)),
)

watch(selected, (v) => {
  minLevel.value = levelIndex(v)
})

watch(
  () => props.logs.length,
  async () => {
    await nextTick()
    if (body.value) body.value.scrollTop = body.value.scrollHeight
  },
)

function classFor(e: LogEntry): string {
  switch (e.level) {
    case 'error': return 'lvl-error'
    case 'warn': return 'lvl-warn'
    case 'debug': return 'lvl-debug'
    default: return 'lvl-info'
  }
}
</script>

<template>
  <div class="log-panel">
    <div class="log-header">
      <span class="section">Logs</span>
      <select v-model="selected">
        <option v-for="l in LOG_LEVELS" :key="l">{{ l }}</option>
      </select>
    </div>
    <div ref="body" class="log-body mono">
      <div v-for="(e, i) in visible" :key="i" :class="classFor(e)">
        {{ e.time }} | {{ e.level.toUpperCase() }} | {{ e.msg }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
}

.section {
  font-weight: 600;
}

.log-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  user-select: text;
}

.lvl-info { color: var(--text); }
.lvl-debug { color: var(--text-dim); }
.lvl-warn { color: var(--yellow); }
.lvl-error { color: var(--red); }
</style>
