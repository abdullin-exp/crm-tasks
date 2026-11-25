<template>
  <div style="width: 100%">
    <el-table :data="rows" border size="small" empty-text="Нет строк">
      <el-table-column type="index" width="44" />
      <el-table-column v-for="c in columns" :key="c.code" :label="c.label" :min-width="c.type === 'text' ? 180 : 110">
        <template #default="{ row }">
          <el-input-number
            v-if="c.type === 'number'"
            v-model="row[c.code]"
            :disabled="disabled"
            controls-position="right"
            size="small"
            style="width: 100%"
            @change="emitRows"
          />
          <el-date-picker
            v-else-if="c.type === 'date'"
            v-model="row[c.code]"
            type="date"
            value-format="YYYY-MM-DD"
            format="DD.MM.YYYY"
            size="small"
            :disabled="disabled"
            style="width: 100%"
            @change="emitRows"
          />
          <el-input v-else v-model="row[c.code]" size="small" :disabled="disabled" @change="emitRows" />
        </template>
      </el-table-column>
      <el-table-column v-if="!disabled" width="50" align="center">
        <template #default="{ $index }">
          <el-button link type="danger" :icon="'Delete'" @click="remove($index)" />
        </template>
      </el-table-column>
    </el-table>
    <el-button v-if="!disabled" size="small" :icon="'Plus'" style="margin-top: 6px" @click="add">Добавить строку</el-button>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Array, default: null },
  columns: { type: Array, default: () => [] },
  disabled: Boolean,
})
const emit = defineEmits(['update:modelValue'])

const rows = ref([])
watch(
  () => props.modelValue,
  (v) => {
    rows.value = (v || []).map((r) => ({ ...r }))
  },
  { immediate: true },
)
const emitRows = () => emit('update:modelValue', rows.value.map((r) => ({ ...r })))
const add = () => {
  rows.value.push({})
  emitRows()
}
const remove = (i) => {
  rows.value.splice(i, 1)
  emitRows()
}
</script>
