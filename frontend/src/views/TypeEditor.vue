<template>
  <div v-loading="loading" class="page wide">
    <el-breadcrumb separator="/" style="margin-bottom: 12px">
      <el-breadcrumb-item :to="{ path: '/admin/types' }">Типы задач</el-breadcrumb-item>
      <el-breadcrumb-item>{{ type?.name }}</el-breadcrumb-item>
    </el-breadcrumb>

    <template v-if="type && meta">
      <div class="page-header">
        <div>
          <h1><span class="dot" :style="{ background: settings.color }" />{{ type.name }}</h1>
          <div class="muted small" style="margin-top: 4px">
            Код <span class="mono">{{ type.code }}</span> · форма v{{ type.form_version }} · задач: {{ type.tasks_count }}
          </div>
        </div>
        <div class="toolbar">
          <el-tag v-if="dirtyForm || dirtySettings" type="warning">Есть несохранённые изменения</el-tag>
          <el-button :icon="'Plus'" @click="$router.push(`/tasks/new/${type.id}`)" :disabled="!type.is_active">Создать задачу этого типа</el-button>
          <el-button type="primary" :icon="'Check'" :loading="saving" :disabled="!dirtyForm && !dirtySettings" @click="saveAll">Сохранить</el-button>
        </div>
      </div>

      <el-alert v-if="formErrors.length" type="error" :closable="true" show-icon style="margin-bottom: 12px" title="Форма не сохранена" @close="formErrors = []">
        <ul style="margin: 4px 0 0; padding-left: 18px"><li v-for="(e, i) in formErrors" :key="i">{{ e }}</li></ul>
      </el-alert>

      <el-tabs v-model="tab">
        <!-- ================= КОНСТРУКТОР ================= -->
        <el-tab-pane label="Конструктор формы" name="form">
          <div class="builder">
            <!-- Палитра -->
            <el-card shadow="never" class="palette">
              <div class="pal-title">Блоки</div>
              <draggable :list="meta.block_kinds" :group="{ name: 'blocks', pull: 'clone', put: false }" :sort="false" :clone="makeBlock" item-key="kind" class="pal-list">
                <template #item="{ element }">
                  <div class="pal-item" :title="element.description" @click="addBlock(element)">
                    <el-icon><Files /></el-icon>{{ element.label }}
                  </div>
                </template>
              </draggable>

              <div class="pal-title">Системные поля</div>
              <draggable :list="freeSystemFields" :group="{ name: 'fields', pull: 'clone', put: false }" :sort="false" :clone="makeSystemField" item-key="key" class="pal-list">
                <template #item="{ element }">
                  <div class="pal-item sys" @click="addField(makeSystemField(element))"><el-icon><Key /></el-icon>{{ element.label }}</div>
                </template>
              </draggable>
              <div v-if="!freeSystemFields.length" class="muted small">Все системные поля размещены</div>

              <template v-for="g in fieldGroups" :key="g.name">
                <div class="pal-title">{{ g.name }}</div>
                <draggable :list="g.items" :group="{ name: 'fields', pull: 'clone', put: false }" :sort="false" :clone="makeField" item-key="type" class="pal-list">
                  <template #item="{ element }">
                    <div class="pal-item" @click="addField(makeField(element))">
                      <el-icon><component :is="element.icon" /></el-icon>{{ element.label }}
                    </div>
                  </template>
                </draggable>
              </template>
              <div class="muted small" style="margin-top: 12px">Перетащите элемент на форму или кликните — он добавится в выбранный блок.</div>
            </el-card>

            <!-- Холст -->
            <div class="canvas">
              <draggable v-model="schema.blocks" item-key="id" group="blocks" handle=".block-handle" class="blocks" @add="(e) => select('block', schema.blocks[e.newIndex])">
                <template #item="{ element: block }">
                  <div class="cblock" :style="{ width: pct(block.span) }">
                    <div class="cblock-inner" :class="{ active: selected === block }" @click.stop="select('block', block)">
                      <div class="cblock-head">
                        <el-icon class="block-handle" title="Перетащить блок"><Rank /></el-icon>
                        <b class="title">{{ block.title }}</b>
                        <el-tag size="small" type="info" effect="plain">{{ kindLabel(block.kind) }}</el-tag>
                        <span class="spacer" />
                        <el-button-group size="small">
                          <el-button :type="block.span === 12 ? 'primary' : ''" @click.stop="block.span = 12">½</el-button>
                          <el-button :type="block.span === 24 ? 'primary' : ''" @click.stop="block.span = 24">Вся</el-button>
                        </el-button-group>
                        <el-button link type="danger" :icon="'Delete'" title="Удалить блок" @click.stop="removeBlock(block)" />
                      </div>

                      <draggable
                        v-if="block.kind === 'fields'"
                        v-model="block.fields"
                        item-key="id"
                        group="fields"
                        class="cfields"
                        :class="{ empty: !block.fields.length }"
                        @add="(e) => select('field', block.fields[e.newIndex], block)"
                      >
                        <template #item="{ element: f }">
                          <div class="cfield" :style="{ width: pct(f.span) }" @click.stop="select('field', f, block)">
                            <div class="cfield-inner" :class="{ active: selected === f, sys: f.type === 'system' }">
                              <div class="cfield-label">
                                <span>{{ f.label || (f.type === 'info' ? 'Информационный текст' : 'Без названия') }}</span>
                                <span v-if="f.required" class="req">*</span>
                                <el-icon v-if="f.show_in_list" title="Колонка в списке" class="muted"><Grid /></el-icon>
                                <el-tooltip v-if="f.visible_if" :content="condText(f)"><el-icon class="cond-icon"><View /></el-icon></el-tooltip>
                              </div>
                              <div class="cfield-meta">
                                <span>{{ typeLabel(f) }}</span>
                                <span v-if="f.type !== 'info'" class="mono">{{ f.type === 'system' ? f.system : f.code }}</span>
                              </div>
                              <el-button class="cfield-del" link type="danger" :icon="'Close'" @click.stop="removeField(block, f)" />
                            </div>
                          </div>
                        </template>
                        <template #footer>
                          <div v-if="!block.fields.length" class="drop-hint">Перетащите сюда поля из палитры</div>
                        </template>
                      </draggable>
                      <div v-else class="drop-hint static">{{ kindDescription(block.kind) }}</div>
                    </div>
                  </div>
                </template>
                <template #footer>
                  <div v-if="!schema.blocks.length" class="drop-hint big">Перетащите блок из палитры слева</div>
                </template>
              </draggable>
            </div>

            <!-- Настройки выбранного -->
            <el-card shadow="never" class="props">
              <template #header>
                <b>{{ selectedKind === 'field' ? 'Настройки поля' : selectedKind === 'block' ? 'Настройки блока' : 'Свойства' }}</b>
              </template>
              <FieldSettings
                v-if="selectedKind === 'field'"
                :key="selected.id"
                :field="selected"
                :meta="meta"
                :auto-code="newFieldIds.has(selected.id)"
                :schema="schema"
                :results="settings.results"
                @code-touched="(id) => newFieldIds.delete(id)"
              />
              <el-form v-else-if="selectedKind === 'block'" label-position="top" @submit.prevent>
                <el-form-item label="Заголовок блока"><el-input v-model="selected.title" /></el-form-item>
                <el-form-item label="Вид"><el-input :model-value="kindLabel(selected.kind)" disabled /></el-form-item>
                <el-form-item label="Ширина">
                  <el-segmented v-model="selected.span" :options="[{ value: 8, label: '⅓' }, { value: 12, label: '½' }, { value: 16, label: '⅔' }, { value: 24, label: 'Вся' }]" block />
                </el-form-item>
                <el-checkbox v-model="selected.collapsible">Можно сворачивать</el-checkbox><br />
                <el-checkbox v-model="selected.collapsed" :disabled="!selected.collapsible">Свёрнут по умолчанию</el-checkbox>
              </el-form>
              <div v-else class="muted small">Выберите блок или поле на форме, чтобы изменить его свойства.</div>
            </el-card>
          </div>
        </el-tab-pane>

        <!-- ================= ПРЕДПРОСМОТР ================= -->
        <el-tab-pane label="Предпросмотр" name="preview">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px" title="Так форма будет выглядеть для менеджера. Можно заполнять поля — данные не сохраняются." />
          <FormRenderer v-if="tab === 'preview'" :schema="schema" :model="previewModel" :results="settings.results.map((r, i) => ({ ...r, id: r.id || -i - 1, is_active: true }))" preview />
        </el-tab-pane>

        <!-- ================= НАСТРОЙКИ И РЕЗУЛЬТАТЫ ================= -->
        <el-tab-pane label="Настройки и результаты" name="settings">
          <el-row :gutter="16">
            <el-col :lg="10">
              <el-card shadow="never" header="Основные настройки">
                <el-form :model="settings" label-position="top" @submit.prevent>
                  <el-form-item label="Название" :error="settingsErrors.name" required><el-input v-model="settings.name" /></el-form-item>
                  <el-form-item label="Код" :error="settingsErrors.code" required>
                    <el-input v-model="settings.code" class="mono" />
                    <div class="muted small">Используется в Kafka-событиях (<span class="mono">type_code</span>) и API</div>
                  </el-form-item>
                  <el-form-item label="Описание"><el-input v-model="settings.description" type="textarea" /></el-form-item>
                  <el-row :gutter="12">
                    <el-col :span="12">
                      <el-form-item label="Срок решения, минут">
                        <el-input-number v-model="settings.solve_minutes" :min="1" controls-position="right" style="width: 100%" />
                        <div class="muted small">План. дата = создание + срок</div>
                      </el-form-item>
                    </el-col>
                    <el-col :span="12">
                      <el-form-item label="Приоритет по умолчанию">
                        <el-select v-model="settings.default_priority" style="width: 100%">
                          <el-option v-for="p in meta.priorities" :key="p.id" :value="p.id" :label="p.name" />
                        </el-select>
                      </el-form-item>
                    </el-col>
                    <el-col :span="12">
                      <el-form-item label="Цвет"><el-color-picker v-model="settings.color" /></el-form-item>
                    </el-col>
                    <el-col :span="12">
                      <el-form-item label="ID типа в CRM (enum_entity)">
                        <el-input-number v-model="settings.legacy_entity_id" :min="1" controls-position="right" style="width: 100%" />
                        <div class="muted small">Для миграции старых задач</div>
                      </el-form-item>
                    </el-col>
                  </el-row>
                  <el-form-item label="Активность">
                    <el-switch v-model="settings.is_active" active-text="Активен — можно создавать задачи" inactive-text="Отключён" />
                  </el-form-item>
                </el-form>
              </el-card>
            </el-col>
            <el-col :lg="14">
              <el-card shadow="never" header="Результаты задачи">
                <div class="muted small" style="margin-bottom: 10px">Кнопки результата на форме задачи. Порядок меняется перетаскиванием.</div>
                <div v-if="settingsErrors.results" class="el-form-item__error" style="position: static; margin-bottom: 8px">{{ settingsErrors.results }}</div>
                <ResultsEditor :results="settings.results" />
              </el-card>
              <el-card shadow="never" header="Опасная зона" style="margin-top: 16px">
                <el-popconfirm title="Удалить тип задачи?" @confirm="removeType">
                  <template #reference>
                    <el-button type="danger" plain :icon="'Delete'" :disabled="type.tasks_count > 0">Удалить тип</el-button>
                  </template>
                </el-popconfirm>
                <span v-if="type.tasks_count > 0" class="muted small" style="margin-left: 8px">По типу есть задачи — его можно только отключить</span>
              </el-card>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- ================= ИНТЕГРАЦИЯ ================= -->
        <el-tab-pane label="Kafka / API" name="integration">
          <IntegrationHelp :type="type" :schema="schema" :kafka="meta.kafka" />
        </el-tab-pane>
      </el-tabs>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import draggable from 'vuedraggable'
import api from '../api'
import FormRenderer from '../components/FormRenderer.vue'
import FieldSettings from '../components/constructor/FieldSettings.vue'
import ResultsEditor from '../components/constructor/ResultsEditor.vue'
import IntegrationHelp from '../components/constructor/IntegrationHelp.vue'
import { uid } from '../utils/format'
import { OPS } from '../utils/visibility'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const saving = ref(false)
const meta = ref(null)
const type = ref(null)
const tab = ref('form')
const schema = reactive({ blocks: [] })
const settings = reactive({ results: [] })
const formSnapshot = ref('')
const settingsSnapshot = ref('')
const formErrors = ref([])
const settingsErrors = ref({})
const previewModel = reactive({ priority: 2, data: {} })

const selected = ref(null)
const selectedKind = ref(null)
const newFieldIds = reactive(new Set())

const dirtyForm = computed(() => JSON.stringify(schema) !== formSnapshot.value)
const dirtySettings = computed(() => JSON.stringify(settings) !== settingsSnapshot.value)

function applyType(t) {
  type.value = t
  schema.blocks = JSON.parse(JSON.stringify(t.form.blocks))
  formSnapshot.value = JSON.stringify(schema)
  Object.assign(settings, {
    name: t.name,
    code: t.code,
    description: t.description,
    color: t.color,
    is_active: t.is_active,
    default_priority: t.default_priority,
    solve_minutes: t.solve_minutes,
    legacy_entity_id: t.legacy_entity_id,
    results: t.results.filter((r) => r.is_active).map((r) => ({ ...r, _key: String(r.id) })),
  })
  settingsSnapshot.value = JSON.stringify(settings)
}

onMounted(async () => {
  try {
    const [m, t] = await Promise.all([api.meta(), api.type(route.params.id)])
    meta.value = m
    applyType(t)
  } finally {
    loading.value = false
  }
})

// ---- палитра ----
const fieldGroups = computed(() => {
  const res = []
  for (const t of meta.value?.field_types || []) {
    let g = res.find((x) => x.name === t.group)
    if (!g) res.push((g = { name: t.group, items: [] }))
    g.items.push(t)
  }
  return res
})
const usedSystem = computed(() => new Set(schema.blocks.flatMap((b) => b.fields).filter((f) => f.type === 'system').map((f) => f.system)))
const freeSystemFields = computed(() => (meta.value?.system_fields || []).filter((s) => !usedSystem.value.has(s.key)))

const kindLabel = (k) => meta.value.block_kinds.find((b) => b.kind === k)?.label || k
const kindDescription = (k) => meta.value.block_kinds.find((b) => b.kind === k)?.description || ''
const typeLabel = (f) => (f.type === 'system' ? 'Системное' : meta.value.field_types.find((t) => t.type === f.type)?.label || f.type)
const condText = (f) => {
  const c = f.visible_if
  const src = schema.blocks.flatMap((b) => b.fields).find((x) => (x.type === 'system' ? x.system : x.code) === c.field)
  const op = OPS.find((o) => o.value === c.op)?.label || c.op
  const label = (v) =>
    c.field === 'result_id' ? settings.results.find((r) => String(r.id) === v)?.name || v : src?.options?.find((o) => o.value === v)?.label || v
  return `Показывается, если «${src?.label || c.field}» ${op} ${(c.values || []).map(label).join(', ')}`.trim()
}
const pct = (span) => `${(span / 24) * 100}%`

function makeBlock(kind) {
  return { id: 'b_' + uid(), title: kind.label, kind: kind.kind, span: kind.kind === 'fields' ? 24 : 12, collapsible: false, collapsed: false, fields: [] }
}
function uniqueCode(base) {
  const used = new Set(schema.blocks.flatMap((b) => b.fields).map((f) => f.code))
  let code = base
  for (let i = 2; used.has(code); i++) code = `${base}_${i}`
  return code
}
function makeField(t) {
  const f = { id: 'f_' + uid(), code: uniqueCode(t.type === 'info' ? 'info' : 'field'), type: t.type, label: t.type === 'info' ? '' : t.label, required: false, readonly: false, span: 12, show_in_list: false }
  if (['select', 'multiselect', 'radio'].includes(t.type)) f.options = [{ value: 'option_1', label: 'Вариант 1' }, { value: 'option_2', label: 'Вариант 2' }]
  if (t.type === 'table') {
    f.span = 24
    f.columns = [{ code: 'name', label: 'Наименование', type: 'text' }, { code: 'qty', label: 'Кол-во', type: 'number' }]
  }
  if (t.type === 'textarea' || t.type === 'info') f.span = 24
  if (t.type === 'info') f.text = 'Текст подсказки для менеджера'
  newFieldIds.add(f.id)
  return f
}
function makeSystemField(s) {
  return { id: 'f_' + uid(), code: s.key, type: 'system', system: s.key, label: s.label, required: s.key === 'title', readonly: s.readonly, span: s.key === 'description' ? 24 : 12, show_in_list: false }
}

function addBlock(kind) {
  const b = makeBlock(kind)
  schema.blocks.push(b)
  select('block', b)
}
function targetBlock() {
  if (selectedKind.value === 'block' && selected.value.kind === 'fields') return selected.value
  if (selectedKind.value === 'field') return selectedBlock.value
  return schema.blocks.find((b) => b.kind === 'fields')
}
function addField(f) {
  let b = targetBlock()
  if (!b) {
    b = makeBlock(meta.value.block_kinds[0])
    b.title = 'Основная информация'
    schema.blocks.unshift(b)
  }
  b.fields.push(f)
  select('field', f, b)
}

const selectedBlock = ref(null)
function select(kind, item, block = null) {
  selectedKind.value = kind
  selected.value = item
  selectedBlock.value = kind === 'block' ? item : block
}
function removeBlock(b) {
  schema.blocks.splice(schema.blocks.indexOf(b), 1)
  if (selected.value === b || selectedBlock.value === b) select(null, null)
}
function removeField(b, f) {
  b.fields.splice(b.fields.indexOf(f), 1)
  if (selected.value === f) select('block', b)
}

// ---- сохранение ----
async function saveAll() {
  saving.value = true
  formErrors.value = []
  settingsErrors.value = {}
  try {
    let t = type.value
    if (dirtySettings.value) {
      const body = { ...settings, results: settings.results.map(({ id, name, color, is_closing }) => ({ id: id || 0, name, color, is_closing })) }
      try {
        t = await api.updateType(t.id, body)
      } catch (e) {
        if (e.fields) {
          settingsErrors.value = e.fields
          tab.value = 'settings'
          ElMessage.error('Проверьте настройки типа')
        }
        return
      }
    }
    if (dirtyForm.value) {
      try {
        t = await api.saveForm(t.id, JSON.parse(JSON.stringify(schema)))
      } catch (e) {
        if (e.fields) {
          formErrors.value = Object.values(e.fields)
          tab.value = 'form'
          ElMessage.error('Форма содержит ошибки')
        }
        // настройки уже сохранены — обновим снимок, чтобы не потерять состояние
        if (t !== type.value) {
          const blocks = JSON.stringify(schema)
          applyType(t)
          schema.blocks = JSON.parse(blocks)
        }
        return
      }
    }
    const keepSel = selected.value?.id
    applyType(t)
    newFieldIds.clear()
    if (keepSel) {
      for (const b of schema.blocks) {
        if (b.id === keepSel) select('block', b)
        const f = b.fields.find((x) => x.id === keepSel)
        if (f) select('field', f, b)
      }
    } else select(null, null)
    ElMessage.success('Сохранено')
  } finally {
    saving.value = false
  }
}

async function removeType() {
  await api.deleteType(type.value.id)
  formSnapshot.value = JSON.stringify(schema)
  settingsSnapshot.value = JSON.stringify(settings)
  ElMessage.success('Тип удалён')
  router.push('/admin/types')
}

onBeforeRouteLeave(async () => {
  if (!dirtyForm.value && !dirtySettings.value) return true
  try {
    await ElMessageBox.confirm('Есть несохранённые изменения. Уйти без сохранения?', 'Внимание', { type: 'warning', confirmButtonText: 'Уйти', cancelButtonText: 'Остаться' })
    return true
  } catch {
    return false
  }
})
</script>

<style scoped>
.wide { max-width: none; }
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 10px; vertical-align: middle; }
.builder { display: grid; grid-template-columns: 240px minmax(0, 1fr) 340px; gap: 16px; align-items: start; }
.palette, .props { position: sticky; top: 8px; max-height: calc(100vh - 40px); overflow: auto; }
.pal-title { font-size: 12px; text-transform: uppercase; color: var(--el-text-color-secondary); margin: 12px 0 6px; letter-spacing: 0.04em; }
.pal-title:first-child { margin-top: 0; }
.pal-list { display: flex; flex-direction: column; gap: 4px; }
.pal-item { display: flex; align-items: center; gap: 8px; padding: 6px 10px; border: 1px solid var(--el-border-color); border-radius: 6px; background: #fff; cursor: grab; font-size: 13px; user-select: none; }
.pal-item:hover { border-color: var(--el-color-primary); color: var(--el-color-primary); }
.pal-item.sys { background: var(--el-color-primary-light-9); }

.canvas { min-height: 400px; }
.blocks { display: flex; flex-wrap: wrap; margin: 0 -8px; min-height: 200px; align-content: flex-start; }
.cblock { padding: 0 8px 16px; box-sizing: border-box; }
.cblock-inner { background: #fff; border: 1px solid var(--el-border-color-light); border-radius: 8px; cursor: pointer; }
.cblock-inner.active { border-color: var(--el-color-primary); box-shadow: 0 0 0 2px var(--el-color-primary-light-7); }
.cblock-head { display: flex; align-items: center; gap: 8px; padding: 8px 12px; background: var(--el-color-primary-light-9); border-radius: 8px 8px 0 0; }
.cblock-head .title { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cblock-head .el-button-group { display: flex; flex-shrink: 0; }
.cblock-head .el-tag { flex-shrink: 1; min-width: 0; overflow: hidden; }
.block-handle { cursor: grab; color: var(--el-text-color-secondary); }
.spacer { flex: 1; }
.cfields { display: flex; flex-wrap: wrap; padding: 8px 6px; min-height: 64px; align-content: flex-start; }
.cfield { padding: 4px 6px; box-sizing: border-box; }
.cfield-inner { position: relative; border: 1px dashed var(--el-border-color); border-radius: 6px; padding: 6px 28px 6px 10px; background: var(--el-fill-color-blank); cursor: grab; }
.cfield-inner.sys { background: var(--el-color-primary-light-9); border-style: solid; border-color: var(--el-color-primary-light-7); }
.cfield-inner.active { border-color: var(--el-color-primary); border-style: solid; box-shadow: 0 0 0 2px var(--el-color-primary-light-7); }
.cfield-label { font-weight: 600; font-size: 13px; display: flex; gap: 4px; align-items: center; }
.cfield-meta { display: flex; justify-content: space-between; gap: 8px; font-size: 11px; color: var(--el-text-color-secondary); margin-top: 2px; }
.cfield-del { position: absolute; top: 4px; right: 4px; }
.req { color: var(--el-color-danger); }
.cond-icon { color: var(--el-color-warning); }
.drop-hint { width: 100%; text-align: center; color: var(--el-text-color-placeholder); font-size: 13px; padding: 16px; }
.drop-hint.static { padding: 20px 12px; }
.drop-hint.big { border: 2px dashed var(--el-border-color); border-radius: 8px; margin: 0 8px; padding: 60px; }
.sortable-ghost { opacity: 0.4; }
@media (max-width: 1200px) { .builder { grid-template-columns: 1fr; } .palette, .props { position: static; max-height: none; } }
</style>
