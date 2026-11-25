<template>
  <el-select
    :model-value="multiple ? modelValue || [] : modelValue"
    filterable
    remote
    clearable
    :multiple="multiple"
    :collapse-tags="multiple && collapse"
    :reserve-keyword="false"
    :remote-method="search"
    :loading="loading"
    :placeholder="placeholder"
    :disabled="disabled"
    style="width: 100%"
    @update:model-value="onChange"
    @visible-change="(v) => v && !options.length && search('')"
  >
    <el-option v-for="o in allOptions" :key="o.id" :value="o.id" :label="o.name">
      <span>{{ o.name }}</span>
      <span v-if="o.extra" class="muted small" style="float: right; margin-left: 12px">{{ kind === 'user' ? o.extra : 'ИНН ' + o.extra }}</span>
    </el-option>
  </el-select>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import api from '../api'

// Выбор из справочников CRM: сотрудники (kind=user) или контрагенты (kind=contractor).
// multiple — список id; label/labels — известные имена для уже выбранных значений.
const props = defineProps({
  modelValue: { type: [String, Array], default: null },
  kind: { type: String, default: 'user' },
  contractorKind: { type: String, default: '' },
  label: { type: String, default: '' },
  labels: { type: Array, default: () => [] }, // [{id, name}] для multiple
  multiple: Boolean,
  collapse: Boolean,
  placeholder: { type: String, default: 'Выберите значение…' },
  disabled: Boolean,
})
const emit = defineEmits(['update:modelValue', 'select'])

const options = ref([])
const known = ref([]) // выбранные/виденные элементы, чтобы показывать имена
const loading = ref(false)

const fetch = (params) =>
  props.kind === 'user' ? api.users(params) : api.contractors({ ...params, kind: props.contractorKind })

async function search(q) {
  loading.value = true
  try {
    options.value = await fetch({ q })
  } finally {
    loading.value = false
  }
}

const remember = (items) => {
  for (const it of items) if (it && !known.value.some((k) => k.id === it.id)) known.value.push(it)
}

const allOptions = computed(() => {
  const list = [...options.value]
  for (const k of known.value) if (!list.some((o) => o.id === k.id)) list.unshift(k)
  return list
})

function onChange(v) {
  if (props.multiple) {
    const ids = v || []
    remember(allOptions.value.filter((o) => ids.includes(o.id)))
    emit('update:modelValue', ids)
    emit('select', allOptions.value.filter((o) => ids.includes(o.id)))
    return
  }
  const item = allOptions.value.find((o) => o.id === v) || null
  remember([item])
  emit('update:modelValue', v || null)
  emit('select', item)
}

// Подтягиваем имена для уже выбранных значений
watch(
  () => [props.modelValue, props.label, props.labels],
  async ([val, label, labels]) => {
    const ids = (Array.isArray(val) ? val : val ? [val] : []).filter(Boolean)
    if (!ids.length) return
    if (label && !props.multiple) remember([{ id: ids[0], name: label }])
    remember(labels || [])
    const missing = ids.filter((id) => !known.value.some((k) => k.id === id))
    if (missing.length) remember(await fetch({ ids: missing.join(',') }))
  },
  { immediate: true },
)
</script>
