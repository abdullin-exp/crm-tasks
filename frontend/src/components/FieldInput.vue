<template>
  <!-- Только чтение -->
  <div v-if="readonlyText !== null" class="ro">{{ readonlyText || '—' }}</div>

  <ResultButtons v-else-if="kind === 'result_id'" :model-value="modelValue" :results="results" :disabled="disabled" @update:model-value="set" />

  <el-segmented v-else-if="kind === 'priority'" :model-value="modelValue || 2" :options="priorityOptions" :disabled="disabled" @update:model-value="set" />

  <RefSelect
    v-else-if="kind === 'observer_user_ids'"
    :model-value="modelValue || []"
    kind="user"
    multiple
    :labels="task?.observers || []"
    placeholder="Добавьте наблюдателей"
    :disabled="disabled"
    @update:model-value="set"
  />

  <RefSelect v-else-if="kind === 'executor_user_id' || kind === 'user'" :model-value="modelValue" kind="user" :label="refLabel" :disabled="disabled" @update:model-value="set" />

  <RefSelect
    v-else-if="kind === 'contractor_id' || kind === 'contact_contractor_id' || kind === 'contractor'"
    :model-value="modelValue"
    kind="contractor"
    :contractor-kind="contractorKind"
    :label="refLabel"
    :disabled="disabled"
    @update:model-value="set"
  />

  <el-date-picker
    v-else-if="kind === 'solve_plan_at' || kind === 'datetime'"
    :model-value="modelValue"
    type="datetime"
    value-format="YYYY-MM-DDTHH:mm:ssZ"
    format="DD.MM.YYYY HH:mm"
    :disabled="disabled"
    style="width: 100%"
    @update:model-value="set"
  />

  <el-date-picker
    v-else-if="kind === 'date'"
    :model-value="modelValue"
    type="date"
    value-format="YYYY-MM-DD"
    format="DD.MM.YYYY"
    :disabled="disabled"
    style="width: 100%"
    @update:model-value="set"
  />

  <el-input
    v-else-if="kind === 'textarea' || kind === 'description'"
    :model-value="modelValue"
    type="textarea"
    :autosize="{ minRows: 2, maxRows: 8 }"
    :placeholder="field.placeholder"
    :disabled="disabled"
    @update:model-value="set"
  />

  <el-input-number
    v-else-if="kind === 'number' || kind === 'money'"
    :model-value="modelValue ?? undefined"
    :min="field.min ?? -Infinity"
    :max="field.max ?? Infinity"
    :precision="kind === 'money' ? 2 : undefined"
    :step="kind === 'money' ? 100 : 1"
    controls-position="right"
    :disabled="disabled"
    style="width: 100%; max-width: 260px"
    @update:model-value="(v) => set(v ?? null)"
  />

  <el-switch v-else-if="kind === 'checkbox'" :model-value="!!modelValue" :disabled="disabled" active-text="Да" inactive-text="Нет" @update:model-value="set" />

  <el-select
    v-else-if="kind === 'select' || kind === 'multiselect'"
    :model-value="modelValue ?? (kind === 'multiselect' ? [] : null)"
    :multiple="kind === 'multiselect'"
    clearable
    filterable
    :placeholder="field.placeholder || 'Выберите…'"
    :disabled="disabled"
    style="width: 100%"
    @update:model-value="set"
  >
    <el-option v-for="o in field.options" :key="o.value" :value="o.value" :label="o.label" />
  </el-select>

  <el-radio-group v-else-if="kind === 'radio'" :model-value="modelValue" :disabled="disabled" @update:model-value="set">
    <el-radio-button v-for="o in field.options" :key="o.value" :value="o.value">{{ o.label }}</el-radio-button>
  </el-radio-group>

  <TableField v-else-if="kind === 'table'" :model-value="modelValue" :columns="field.columns || []" :disabled="disabled" @update:model-value="set" />

  <el-input
    v-else
    :model-value="modelValue"
    :placeholder="field.placeholder || (kind === 'phone' ? '+7 (___) ___-__-__' : '')"
    :disabled="disabled"
    @update:model-value="set"
  >
    <template v-if="kind === 'phone' && modelValue" #append>
      <a :href="'tel:+' + String(modelValue).replace(/\D/g, '')"><el-icon><Phone /></el-icon></a>
    </template>
    <template v-else-if="kind === 'email' && modelValue" #append>
      <a :href="'mailto:' + modelValue"><el-icon><Message /></el-icon></a>
    </template>
    <template v-else-if="kind === 'url' && modelValue" #append>
      <a :href="modelValue" target="_blank" rel="noopener"><el-icon><Link /></el-icon></a>
    </template>
  </el-input>
</template>

<script setup>
import { computed } from 'vue'
import RefSelect from './RefSelect.vue'
import ResultButtons from './ResultButtons.vue'
import TableField from './TableField.vue'
import { displayValue, fmtDateTime, priorityName } from '../utils/format'

const props = defineProps({
  field: { type: Object, required: true },
  modelValue: { default: null },
  task: { type: Object, default: null }, // для системных readonly-полей и имён справочников
  results: { type: Array, default: () => [] },
  disabled: Boolean,
})
const emit = defineEmits(['update:modelValue'])
const set = (v) => emit('update:modelValue', v)

const kind = computed(() => (props.field.type === 'system' ? props.field.system : props.field.type))

const priorityOptions = [1, 2, 3].map((v) => ({ value: v, label: priorityName(v) }))

const contractorKind = computed(() => {
  if (kind.value === 'contractor_id') return 'juristic'
  if (kind.value === 'contact_contractor_id') return 'physical'
  return props.field.contractor_kind || ''
})

const refLabel = computed(() => {
  const t = props.task
  if (!t) return ''
  return (
    {
      executor_user_id: t.executor_name,
      contractor_id: t.contractor_name,
      contact_contractor_id: t.contact_contractor_name,
    }[kind.value] || ''
  )
})

// Системные поля только для чтения
const readonlyText = computed(() => {
  const t = props.task
  switch (kind.value) {
    case 'number':
      if (props.field.type !== 'system') return null
      return t ? `№${t.number}` : 'будет присвоен'
    case 'status_id':
      return t ? t.status_name : 'Новая'
    case 'created_at':
      return t ? fmtDateTime(t.created_at) : 'при сохранении'
    case 'solve_fact_at':
      return t?.solve_fact_at ? fmtDateTime(t.solve_fact_at) : ''
  }
  if (props.field.readonly && props.field.type !== 'system' && props.field.type !== 'table') {
    return displayValue(props.field, props.modelValue)
  }
  return null
})
</script>

<style scoped>
.ro { line-height: 32px; min-height: 32px; }
</style>
