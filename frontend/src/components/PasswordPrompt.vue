<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  visible: boolean
  title: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', value: string): void
}>()

const value = ref('')

watch(
  () => props.visible,
  (v) => {
    if (v) value.value = ''
  },
)

function confirm() {
  if (value.value) emit('confirm', value.value)
}
</script>

<template>
  <div v-if="visible" class="overlay" @click.self="emit('close')">
    <div class="dialog">
      <div class="dialog-title">{{ title }}</div>
      <input v-model="value" type="password" autofocus @keyup.enter="confirm" />
      <div class="buttons">
        <button @click="emit('close')">Cancel</button>
        <button class="primary" :disabled="!value" @click="confirm">Connect</button>
      </div>
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
  width: 320px;
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

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 18px;
}
</style>
