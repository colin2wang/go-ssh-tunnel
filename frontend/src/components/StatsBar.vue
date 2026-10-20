<script setup lang="ts">
import { computed } from 'vue'
import type { Stats } from '../types'
import { formatRate, formatBytes } from '../types'

const props = defineProps<{ stats: Stats | null }>()

const blocks = computed(() => {
  const s = props.stats
  return [
    { title: 'UP RATE', value: formatRate(s?.upRate ?? 0) },
    { title: 'DOWN RATE', value: formatRate(s?.downRate ?? 0) },
    { title: 'TOTAL UP', value: formatBytes(s?.totalUp ?? 0) },
    { title: 'TOTAL DOWN', value: formatBytes(s?.totalDown ?? 0) },
    { title: 'ACTIVE', value: String(s?.active ?? 0) },
    { title: 'TUNNELS', value: String(s?.tunnels?.length ?? 0) },
  ]
})
</script>

<template>
  <div class="stats-bar">
    <div v-for="b in blocks" :key="b.title" class="stat-block">
      <div class="stat-title">{{ b.title }}</div>
      <div class="stat-value mono">{{ b.value }}</div>
    </div>
  </div>
</template>

<style scoped>
.stats-bar {
  display: flex;
  justify-content: center;
  gap: 40px;
  padding: 8px 16px;
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
}

.stat-block {
  text-align: center;
  min-width: 84px;
}

.stat-title {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.5px;
  color: var(--text-dim);
}

.stat-value {
  font-size: 14px;
  margin-top: 2px;
}
</style>
