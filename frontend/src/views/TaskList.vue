<template>
  <div class="page">
    <div class="page-header">
      <h1>Задачи</h1>
      <div class="toolbar">
        <el-dropdown trigger="click" @command="(id) => $router.push(`/tasks/new/${id}`)">
          <el-button type="success" :icon="'Plus'">Создать задачу<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="t in activeTypes" :key="t.id" :command="t.id">
                <span class="dot" :style="{ background: t.color }" />{{ t.name }}
              </el-dropdown-item>
              <el-dropdown-item v-if="!activeTypes.length" disabled>Нет активных типов</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button v-if="selected.length" type="primary" plain :icon="'Switch'" @click="transferOpen = true">
          Перевести выбранные ({{ selected.length }})
        </el-button>
        <el-popover placement="bottom-end" :width="260" trigger="click">
          <template #reference><el-button :icon="'Operation'">Колонки</el-button></template>
          <el-checkbox-group v-model="visibleCols" class="cols">
            <el-checkbox v-for="c in allColumns" :key="c.key" :value="c.key">{{ c.label }}</el-checkbox>
          </el-checkbox-group>
        </el-popover>
        <el-select v-model="autoRefresh" style="width: 190px" :prefix-icon="'Refresh'">
          <el-option :value="0" label="Без автообновления" />
          <el-option :value="60" label="Автообновление 1 мин" />
          <el-option :value="300" label="Автообновление 5 мин" />
        </el-select>
      </div>
    </div>

    <el-card shadow="never" class="filters">
      <el-form :model="f" label-position="top" @submit.prevent="apply">
        <el-row :gutter="12">
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Поиск">
              <el-input v-model="f.q" clearable placeholder="Номер, цель, значения полей" :prefix-icon="'Search'" @keyup.enter="apply" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Тип задачи">
              <el-select v-model="f.type_id" clearable filterable placeholder="Все типы" style="width: 100%" @change="onTypeChange">
                <el-option v-for="t in types" :key="t.id" :value="t.id" :label="t.name" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Статус">
              <el-select v-model="f.status_id" multiple collapse-tags clearable placeholder="Все статусы" style="width: 100%">
                <el-option v-for="s in meta.statuses" :key="s.id" :value="s.id" :label="s.name" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Результат">
              <el-select v-model="f.result_id" clearable :disabled="!selectedType" :placeholder="selectedType ? 'Любой' : 'Выберите тип задачи'" style="width: 100%">
                <el-option v-for="r in selectedType?.results || []" :key="r.id" :value="r.id" :label="r.name" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Исполнитель">
              <RefSelect v-model="f.executor_user_id" kind="user" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Контрагент">
              <RefSelect v-model="f.contractor_id" kind="contractor" contractor-kind="juristic" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Наблюдатель">
              <RefSelect v-model="f.observer_user_id" kind="user" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Дата создания">
              <el-date-picker v-model="f.created" type="daterange" value-format="YYYY-MM-DD" format="DD.MM.YYYY" start-placeholder="с" end-placeholder="по" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12" :lg="6">
            <el-form-item label="Источник">
              <el-select v-model="f.source" clearable placeholder="Любой" style="width: 100%">
                <el-option value="manual" label="Вручную" />
                <el-option value="kafka" label="Kafka" />
                <el-option value="api" label="API" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <div class="toolbar">
          <el-button type="primary" native-type="submit" :icon="'Filter'">Применить</el-button>
          <el-button :icon="'RefreshLeft'" @click="reset">Сброс</el-button>
          <span class="muted small" style="margin-left: auto">Фильтр сохраняется в адресе страницы — ссылкой можно поделиться</span>
        </div>
      </el-form>
    </el-card>

    <div class="tabs-row">
      <el-radio-group :model-value="activeTab" @update:model-value="setTab">
        <el-radio-button value="all">Все <b class="cnt">{{ stats.total }}</b></el-radio-button>
        <el-radio-button value="1">Новые <b class="cnt">{{ stats.by_status[1] || 0 }}</b></el-radio-button>
        <el-radio-button value="2">В работе <b class="cnt">{{ stats.by_status[2] || 0 }}</b></el-radio-button>
        <el-radio-button value="3">Выполнены <b class="cnt">{{ stats.by_status[3] || 0 }}</b></el-radio-button>
        <el-radio-button value="overdue">Просрочены <b class="cnt danger">{{ stats.overdue }}</b></el-radio-button>
      </el-radio-group>
      <el-tooltip :disabled="!!me.id" content="Выберите себя в шапке («Я:»)">
        <el-switch :model-value="isMine" :disabled="!me.id" active-text="Мои задачи" style="margin-left: 16px" @update:model-value="toggleMine" />
      </el-tooltip>
      <el-tooltip :disabled="!!me.id" content="Выберите себя в шапке («Я:»)">
        <el-switch :model-value="isObserved" :disabled="!me.id" active-text="Я наблюдатель" style="margin-left: 16px" @update:model-value="toggleObserved" />
      </el-tooltip>
    </div>

    <el-card shadow="never" :body-style="{ padding: 0 }">
      <el-table
        ref="tableRef"
        v-loading="loading"
        row-key="id"
        :data="items"
        stripe
        :row-class-name="rowClass"
        empty-text="Ничего не найдено"
        style="width: 100%"
        :default-sort="sortState"
        @sort-change="onSort"
        @row-click="(row, column) => column?.type !== 'selection' && $router.push(`/tasks/${row.id}`)"
        @selection-change="(rows) => (selected = rows)"
      >
        <el-table-column type="selection" width="44" :selectable="(row) => row.status_id !== 3" />
        <el-table-column prop="number" label="№" width="90" sortable="custom">
          <template #default="{ row }"><router-link :to="`/tasks/${row.id}`" @click.stop>{{ row.number }}</router-link></template>
        </el-table-column>
        <el-table-column v-if="show('type')" prop="type" label="Тип задачи" min-width="170" sortable="custom">
          <template #default="{ row }"><span class="dot" :style="{ background: row.type_color }" />{{ row.type_name }}</template>
        </el-table-column>
        <el-table-column v-if="show('status')" prop="status" label="Статус" width="120" sortable="custom">
          <template #default="{ row }"><el-tag :type="statusTag(row.status_id)" size="small" disable-transitions>{{ row.status_name }}</el-tag></template>
        </el-table-column>
        <el-table-column v-if="show('result')" label="Результат" min-width="160">
          <template #default="{ row }">
            <el-tag v-if="row.result_name" :type="row.result_color" size="small" effect="plain" disable-transitions>{{ row.result_name }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column v-if="show('title')" prop="title" label="Цель задачи" min-width="220" show-overflow-tooltip />
        <el-table-column v-if="show('priority')" prop="priority" label="Приоритет" width="115" sortable="custom">
          <template #default="{ row }"><el-tag :type="priorityTag(row.priority)" size="small" effect="light" disable-transitions>{{ priorityName(row.priority) }}</el-tag></template>
        </el-table-column>
        <el-table-column v-if="show('executor')" prop="executor" label="Исполнитель" min-width="170" sortable="custom" show-overflow-tooltip>
          <template #default="{ row }">{{ row.executor_name }}</template>
        </el-table-column>
        <el-table-column v-if="show('contractor')" label="Контрагент" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.contractor_name }}</template>
        </el-table-column>
        <el-table-column v-if="show('contact')" label="Контактное лицо" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.contact_contractor_name }}</template>
        </el-table-column>
        <el-table-column v-if="show('observers')" label="Наблюдатели" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">{{ row.observers.map((o) => o.name).join(', ') }}</template>
        </el-table-column>
        <el-table-column v-for="fld in typeColumns" :key="'d_' + fld.code" :label="fld.label" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ displayValue(fld, row.data[fld.code]) }}</template>
        </el-table-column>
        <el-table-column v-if="show('solve_plan_at')" prop="solve_plan_at" label="План. завершение" width="160" sortable="custom">
          <template #default="{ row }"><span :class="{ overdue: row.overdue }">{{ fmtDateTime(row.solve_plan_at) }}</span></template>
        </el-table-column>
        <el-table-column v-if="show('created_at')" prop="created_at" label="Создана" width="150" sortable="custom">
          <template #default="{ row }">{{ fmtDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column v-if="show('source')" label="Источник" width="100">
          <template #default="{ row }"><span class="muted small">{{ sourceName(row.source) }}</span></template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="perPage"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          background
          @current-change="load"
          @size-change="apply"
        />
      </div>
    </el-card>
    <TransferDialog v-model="transferOpen" :tasks="selected" @done="onBulkDone" />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import RefSelect from '../components/RefSelect.vue'
import TransferDialog from '../components/TransferDialog.vue'
import { me } from '../utils/me'
import { displayValue, fmtDateTime, priorityName, priorityTag, sourceName, statusTag } from '../utils/format'

const route = useRoute()
const router = useRouter()

const meta = ref({ statuses: [] })
const types = ref([])
const activeTypes = computed(() => types.value.filter((t) => t.is_active))
const items = ref([])
const total = ref(0)
const loading = ref(false)

const emptyFilter = () => ({ q: '', type_id: null, status_id: [], result_id: null, executor_user_id: null, observer_user_id: null, contractor_id: null, created: null, source: null, overdue: false })
const f = reactive(emptyFilter())
const page = ref(1)
const perPage = ref(20)
const sort = ref({ prop: 'created_at', order: 'descending' })
const sortState = computed(() => sort.value)

const selectedType = computed(() => types.value.find((t) => t.id === f.type_id))
// Колонки пользовательских полей (флаг «показывать в списке») — при фильтре по типу
const typeColumns = computed(() => {
  if (!selectedType.value) return []
  return selectedType.value.form.blocks.flatMap((b) => b.fields).filter((x) => x.show_in_list && x.type !== 'system' && x.type !== 'info')
})

const allColumns = [
  { key: 'type', label: 'Тип задачи' },
  { key: 'status', label: 'Статус' },
  { key: 'result', label: 'Результат' },
  { key: 'title', label: 'Цель задачи' },
  { key: 'priority', label: 'Приоритет' },
  { key: 'executor', label: 'Исполнитель' },
  { key: 'contractor', label: 'Контрагент' },
  { key: 'contact', label: 'Контактное лицо' },
  { key: 'observers', label: 'Наблюдатели' },
  { key: 'solve_plan_at', label: 'Плановое завершение' },
  { key: 'created_at', label: 'Создана' },
  { key: 'source', label: 'Источник' },
]
const COLS_KEY = 'crm-tasks.list.columns'
const visibleCols = ref(JSON.parse(localStorage.getItem(COLS_KEY) || 'null') || allColumns.map((c) => c.key).filter((k) => k !== 'contact' && k !== 'observers'))
watch(visibleCols, (v) => localStorage.setItem(COLS_KEY, JSON.stringify(v)))
const show = (k) => visibleCols.value.includes(k)

// ---- вкладки со счётчиками и «Мои задачи» ----
const stats = ref({ total: 0, by_status: {}, overdue: 0 })
const activeTab = computed(() => {
  if (f.overdue) return 'overdue'
  if (f.status_id.length === 1) return String(f.status_id[0])
  return f.status_id.length ? null : 'all'
})
function setTab(tab) {
  f.overdue = tab === 'overdue'
  f.status_id = ['1', '2', '3'].includes(tab) ? [Number(tab)] : []
  apply()
}
const isMine = computed(() => !!me.id && f.executor_user_id === me.id)
function toggleMine(on) {
  f.executor_user_id = on ? me.id : null
  apply()
}

const isObserved = computed(() => !!me.id && f.observer_user_id === me.id)
function toggleObserved(on) {
  f.observer_user_id = on ? me.id : null
  apply()
}

// ---- массовый перевод ----
const tableRef = ref(null)
const selected = ref([])
const transferOpen = ref(false)
function onBulkDone() {
  tableRef.value?.clearSelection()
  load()
}

const rowClass = ({ row }) => (row.overdue ? 'row-overdue' : '')

// ---- синхронизация фильтра с URL ----
function fromQuery() {
  const q = route.query
  Object.assign(f, emptyFilter(), {
    q: q.q || '',
    type_id: q.type_id ? Number(q.type_id) : null,
    status_id: q.status_id ? String(q.status_id).split(',').map(Number) : [],
    result_id: q.result_id ? Number(q.result_id) : null,
    executor_user_id: q.executor_user_id || null,
    observer_user_id: q.observer_user_id || null,
    contractor_id: q.contractor_id || null,
    created: q.created_from ? [q.created_from, q.created_to] : null,
    source: q.source || null,
    overdue: q.overdue === '1',
  })
  page.value = Number(q.page) || 1
  perPage.value = Number(q.per_page) || 20
  if (q.sort) sort.value = { prop: q.sort, order: q.order === 'asc' ? 'ascending' : 'descending' }
}

function params() {
  const p = {
    q: f.q || undefined,
    type_id: f.type_id || undefined,
    status_id: f.status_id.length ? f.status_id.join(',') : undefined,
    result_id: f.result_id || undefined,
    executor_user_id: f.executor_user_id || undefined,
    observer_user_id: f.observer_user_id || undefined,
    contractor_id: f.contractor_id || undefined,
    created_from: f.created?.[0],
    created_to: f.created?.[1] ? nextDay(f.created[1]) : undefined,
    source: f.source || undefined,
    overdue: f.overdue ? '1' : undefined,
    sort: sort.value.prop,
    order: sort.value.order === 'ascending' ? 'asc' : 'desc',
    page: page.value,
    per_page: perPage.value,
  }
  return p
}
const nextDay = (d) => {
  const x = new Date(d)
  x.setDate(x.getDate() + 1)
  return x.toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  try {
    const p = params()
    const q = { ...p, created_to: f.created?.[1] }
    router.replace({ query: Object.fromEntries(Object.entries(q).filter(([, v]) => v !== undefined && v !== null)) })
    const [res, st] = await Promise.all([api.tasks(p), api.stats(p)])
    items.value = res.items
    total.value = res.total
    stats.value = st
  } finally {
    loading.value = false
  }
}

function apply() {
  page.value = 1
  load()
}
function reset() {
  Object.assign(f, emptyFilter())
  apply()
}
function onTypeChange() {
  f.result_id = null
}
function onSort({ prop, order }) {
  sort.value = order ? { prop, order } : { prop: 'created_at', order: 'descending' }
  apply()
}

const autoRefresh = ref(Number(localStorage.getItem('crm-tasks.list.refresh') || 0))
let timer
watch(
  autoRefresh,
  (v) => {
    localStorage.setItem('crm-tasks.list.refresh', v)
    clearInterval(timer)
    if (v) timer = setInterval(load, v * 1000)
  },
  { immediate: true },
)
onUnmounted(() => clearInterval(timer))

onMounted(async () => {
  fromQuery()
  ;[meta.value, types.value] = await Promise.all([api.meta(), api.types()])
  await load()
})
</script>

<style scoped>
.filters { margin-bottom: 16px; }
.tabs-row { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.cnt { margin-left: 6px; font-weight: 600; opacity: 0.8; }
.cnt.danger { color: var(--el-color-danger); }
.tabs-row .is-active .cnt.danger { color: #fff; }
.pager { display: flex; justify-content: flex-end; padding: 12px; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 8px; vertical-align: middle; }
.cols { display: flex; flex-direction: column; }
:deep(.el-table__row) { cursor: pointer; }
:deep(.row-overdue td:first-child) { box-shadow: inset 3px 0 0 var(--el-color-danger); }
</style>
