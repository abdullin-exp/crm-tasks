<template>
  <el-form label-position="top" size="default" class="settings" @submit.prevent>
    <template v-if="field.type === 'system'">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px" :title="`Системное поле: ${systemDef?.label}`"
        description="Значение хранится в колонке задачи — по нему работают фильтры списка и интеграции. Код фиксирован." />
    </template>

    <el-form-item :label="field.type === 'info' ? 'Заголовок (необязательно)' : 'Название поля'">
      <el-input v-model="field.label" @input="onLabel" />
    </el-form-item>

    <el-form-item v-if="field.type !== 'system' && field.type !== 'info'" label="Код (ключ в данных, Kafka, API)">
      <el-input v-model="field.code" class="mono" @input="$emit('code-touched', field.id)" />
    </el-form-item>
    <el-form-item v-else-if="field.type === 'system'" label="Код">
      <el-input :model-value="field.system" disabled class="mono" />
    </el-form-item>

    <el-form-item v-if="field.type !== 'system'" label="Тип поля">
      <el-select v-model="field.type" style="width: 100%" @change="onTypeChange">
        <el-option-group v-for="g in groups" :key="g.name" :label="g.name">
          <el-option v-for="t in g.items" :key="t.type" :value="t.type" :label="t.label" />
        </el-option-group>
      </el-select>
    </el-form-item>

    <el-form-item label="Ширина в блоке">
      <el-segmented v-model="field.span" :options="spans" block />
    </el-form-item>

    <el-form-item v-if="field.type === 'info'" label="Текст">
      <el-input v-model="field.text" type="textarea" :autosize="{ minRows: 3 }" />
    </el-form-item>

    <template v-if="field.type !== 'info'">
      <div class="flags">
        <el-checkbox v-if="!systemDef?.readonly" v-model="field.required">Обязательное</el-checkbox>
        <el-checkbox v-if="field.type !== 'system'" v-model="field.readonly">Только чтение</el-checkbox>
        <el-checkbox v-if="field.type !== 'system' && field.type !== 'table'" v-model="field.show_in_list">Колонка в списке задач</el-checkbox>
      </div>

      <el-form-item v-if="hasPlaceholder" label="Подсказка внутри поля (placeholder)">
        <el-input v-model="field.placeholder" />
      </el-form-item>
      <el-form-item label="Пояснение под полем">
        <el-input v-model="field.help" />
      </el-form-item>

      <!-- Варианты выбора -->
      <el-form-item v-if="['select', 'multiselect', 'radio'].includes(field.type)" label="Варианты">
        <div class="list">
          <div v-for="(o, i) in field.options" :key="i" class="row">
            <el-input v-model="o.label" placeholder="Название" @input="!o._touched && (o.value = slug(o.label) || o.value)" />
            <el-input v-model="o.value" placeholder="значение" class="mono val" @input="o._touched = true" />
            <el-button link :icon="'Top'" :disabled="i === 0" @click="move(field.options, i, -1)" />
            <el-button link type="danger" :icon="'Delete'" @click="field.options.splice(i, 1)" />
          </div>
          <el-button size="small" :icon="'Plus'" @click="addOption">Добавить вариант</el-button>
        </div>
      </el-form-item>

      <!-- Колонки таблицы -->
      <el-form-item v-if="field.type === 'table'" label="Колонки таблицы">
        <div class="list">
          <div v-for="(c, i) in field.columns" :key="i" class="row">
            <el-input v-model="c.label" placeholder="Название" @input="!c._touched && (c.code = slug(c.label) || c.code)" />
            <el-input v-model="c.code" placeholder="код" class="mono val" @input="c._touched = true" />
            <el-select v-model="c.type" style="width: 100px">
              <el-option value="text" label="Текст" />
              <el-option value="number" label="Число" />
              <el-option value="date" label="Дата" />
            </el-select>
            <el-button link type="danger" :icon="'Delete'" @click="field.columns.splice(i, 1)" />
          </div>
          <el-button size="small" :icon="'Plus'" @click="field.columns.push({ code: '', label: '', type: 'text' })">Добавить колонку</el-button>
        </div>
      </el-form-item>

      <div v-if="['number', 'money'].includes(field.type)" class="minmax">
        <el-form-item label="Минимум"><el-input-number v-model="field.min" controls-position="right" style="width: 100%" /></el-form-item>
        <el-form-item label="Максимум"><el-input-number v-model="field.max" controls-position="right" style="width: 100%" /></el-form-item>
      </div>

      <el-form-item v-if="field.type === 'contractor'" label="Вид контрагента">
        <el-radio-group v-model="field.contractor_kind">
          <el-radio-button value="">Любой</el-radio-button>
          <el-radio-button value="juristic">ЮЛ / ИП</el-radio-button>
          <el-radio-button value="physical">Физлицо</el-radio-button>
        </el-radio-group>
      </el-form-item>

      <el-form-item v-if="hasDefault" label="Значение по умолчанию">
        <FieldInput :field="{ ...field, readonly: false }" :model-value="field.default ?? null" @update:model-value="(v) => (field.default = v)" />
      </el-form-item>
    </template>

    <el-divider content-position="left">Условие показа</el-divider>
    <el-switch :model-value="!!field.visible_if" active-text="Показывать только при условии" @update:model-value="toggleCond" />
    <div v-if="field.visible_if" class="cond">
      <el-select v-model="field.visible_if.field" placeholder="Поле" filterable style="width: 100%" @change="field.visible_if.values = []">
        <el-option v-for="s in sources" :key="s.code" :value="s.code" :label="s.label" />
      </el-select>
      <el-select v-model="field.visible_if.op" style="width: 100%" @change="onOp">
        <el-option v-for="o in OPS" :key="o.value" :value="o.value" :label="o.label" />
      </el-select>
      <el-select
        v-if="!opDef?.none"
        v-model="condValue"
        :multiple="opDef?.multi"
        filterable
        :allow-create="!valueOptions"
        default-first-option
        :placeholder="valueOptions ? 'Значение' : 'Введите значение и Enter'"
        style="width: 100%"
      >
        <el-option v-for="o in valueOptions || []" :key="o.value" :value="o.value" :label="o.label" />
      </el-select>
      <div class="muted small">Скрытое поле не проверяется на обязательность, его значение сохраняется.</div>
    </div>
  </el-form>
</template>

<script setup>
import { computed } from 'vue'
import FieldInput from '../FieldInput.vue'
import { priorityName, slug } from '../../utils/format'
import { OPS } from '../../utils/visibility'

const props = defineProps({
  field: { type: Object, required: true },
  meta: { type: Object, required: true },
  autoCode: Boolean, // код ещё не правили руками — генерируем из названия
  schema: { type: Object, required: true },
  results: { type: Array, default: () => [] },
})
defineEmits(['code-touched'])

const spans = [
  { value: 6, label: '¼' },
  { value: 8, label: '⅓' },
  { value: 12, label: '½' },
  { value: 16, label: '⅔' },
  { value: 24, label: 'Вся' },
]

const systemDef = computed(() => props.meta.system_fields.find((s) => s.key === props.field.system))
const groups = computed(() => {
  const res = []
  for (const t of props.meta.field_types) {
    let g = res.find((x) => x.name === t.group)
    if (!g) res.push((g = { name: t.group, items: [] }))
    g.items.push(t)
  }
  return res
})
const hasPlaceholder = computed(() => ['text', 'textarea', 'phone', 'email', 'url', 'select'].includes(props.field.type))
const hasDefault = computed(() => ['text', 'textarea', 'number', 'money', 'select', 'radio', 'checkbox', 'multiselect'].includes(props.field.type))

// ---- условие показа ----
const COND_SYSTEM = ['observer_user_ids', 'result_id', 'priority', 'executor_user_id', 'contractor_id', 'contact_contractor_id', 'solve_plan_at', 'title', 'description']
const allFields = computed(() => props.schema.blocks.flatMap((b) => b.fields))
const sources = computed(() =>
  allFields.value
    .filter((f) => f.id !== props.field.id && f.type !== 'info' && f.type !== 'table')
    .filter((f) => f.type !== 'system' || COND_SYSTEM.includes(f.system))
    .map((f) => ({ code: f.type === 'system' ? f.system : f.code, label: f.label || f.code, field: f })),
)
const sourceField = computed(() => sources.value.find((s) => s.code === props.field.visible_if?.field)?.field)
const opDef = computed(() => OPS.find((o) => o.value === props.field.visible_if?.op))
const valueOptions = computed(() => {
  const f = sourceField.value
  if (!f) return null
  if (f.type === 'system' && f.system === 'result_id')
    return props.results.filter((r) => r.id).map((r) => ({ value: String(r.id), label: r.name }))
  if (f.type === 'system' && f.system === 'priority') return [1, 2, 3].map((p) => ({ value: String(p), label: priorityName(p) }))
  if (f.type === 'checkbox') return [{ value: 'true', label: 'Да' }, { value: 'false', label: 'Нет' }]
  if (['select', 'radio', 'multiselect'].includes(f.type)) return f.options || []
  return null
})
const condValue = computed({
  get: () => (opDef.value?.multi ? props.field.visible_if.values || [] : (props.field.visible_if.values || [])[0] ?? null),
  set: (v) => {
    props.field.visible_if.values = Array.isArray(v) ? v.map(String) : v === null || v === '' ? [] : [String(v)]
  },
})
function toggleCond(on) {
  props.field.visible_if = on ? { field: sources.value[0]?.code || '', op: 'eq', values: [] } : undefined
}
function onOp() {
  if (opDef.value?.none) props.field.visible_if.values = []
  else if (!opDef.value?.multi) props.field.visible_if.values = (props.field.visible_if.values || []).slice(0, 1)
}

function onLabel() {
  if (props.autoCode && props.field.type !== 'system' && props.field.type !== 'info') props.field.code = slug(props.field.label) || props.field.code
}
function onTypeChange(t) {
  const f = props.field
  f.default = undefined
  if (['select', 'multiselect', 'radio'].includes(t) && !f.options?.length) f.options = [{ value: 'option_1', label: 'Вариант 1' }]
  if (t === 'table' && !f.columns?.length) f.columns = [{ code: 'name', label: 'Наименование', type: 'text' }]
}
function addOption() {
  if (!props.field.options) props.field.options = []
  const n = props.field.options.length + 1
  props.field.options.push({ value: `option_${n}`, label: `Вариант ${n}` })
}
function move(arr, i, d) {
  const [x] = arr.splice(i, 1)
  arr.splice(i + d, 0, x)
}
</script>

<style scoped>
.flags { display: flex; flex-direction: column; gap: 2px; margin-bottom: 12px; }
.list { width: 100%; display: flex; flex-direction: column; gap: 6px; }
.row { display: flex; gap: 4px; align-items: center; }
.row .val { width: 110px; flex-shrink: 0; }
.minmax { display: flex; gap: 8px; }
.cond { display: flex; flex-direction: column; gap: 8px; margin-top: 10px; }
.settings :deep(.el-form-item) { margin-bottom: 12px; }
</style>
