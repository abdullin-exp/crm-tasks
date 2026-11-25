<template>
  <div class="results">
    <el-button
      v-for="r in visible"
      :key="r.id"
      :type="r.color"
      :plain="modelValue !== r.id"
      size="small"
      :disabled="disabled"
      @click="toggle(r.id)"
    >
      <el-icon v-if="modelValue === r.id" style="margin-right: 4px"><Select /></el-icon>
      {{ r.name }}
    </el-button>
    <span v-if="!visible.length" class="muted small">Результаты не настроены для типа</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: Number, default: null },
  results: { type: Array, default: () => [] },
  disabled: Boolean,
})
const emit = defineEmits(['update:modelValue'])

// Неактивные результаты показываем, только если они уже выбраны в задаче
const visible = computed(() => props.results.filter((r) => r.is_active || r.id === props.modelValue))
const toggle = (id) => emit('update:modelValue', props.modelValue === id ? null : id)
</script>

<style scoped>
.results { display: flex; flex-wrap: wrap; gap: 6px; }
.results .el-button { margin-left: 0; }
</style>
