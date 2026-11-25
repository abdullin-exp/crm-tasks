<template>
  <div v-loading="loading" class="page">
    <el-breadcrumb separator="/" style="margin-bottom: 12px">
      <el-breadcrumb-item :to="{ path: '/tasks' }">Задачи</el-breadcrumb-item>
      <el-breadcrumb-item>{{ heading }}</el-breadcrumb-item>
    </el-breadcrumb>

    <template v-if="type">
      <div class="page-header sticky">
        <div>
          <h1>
            <span class="dot" :style="{ background: type.color }" />{{ heading }}
            <el-tag v-if="task" :type="statusTag(task.status_id)" style="margin-left: 8px; vertical-align: middle">{{ task.status_name }}</el-tag>
            <el-tag v-if="task?.result_name" :type="task.result_color" effect="plain" style="margin-left: 6px; vertical-align: middle">{{ task.result_name }}</el-tag>
          </h1>
          <div v-if="task" class="muted small" style="margin-top: 4px">
            Создана {{ fmtDateTime(task.created_at) }} · {{ sourceName(task.source) }}<template v-if="task.external_id"> · event <span class="mono">{{ task.external_id }}</span></template>
            · изменена {{ fmtDateTime(task.updated_at) }}
          </div>
          <div v-else class="muted small" style="margin-top: 4px">{{ type.description }}</div>
        </div>
        <div class="toolbar">
          <span v-if="left && task?.status_id !== 3" :class="left.overdue ? 'overdue' : 'left'">
            <el-icon><Timer /></el-icon> {{ left.overdue ? 'Просрочено на' : 'Осталось' }} {{ left.text }}
          </span>
          <el-button v-if="task && task.status_id === 1" type="warning" :icon="'VideoPlay'" @click="setStatus(2)">Взять в работу</el-button>
          <el-button v-if="task && task.status_id === 3" :icon="'RefreshLeft'" @click="setStatus(2)">Вернуть в работу</el-button>
          <el-button v-if="task && task.status_id !== 3" type="info" plain :icon="'Switch'" @click="openTransfer">Перевести задачу</el-button>
          <el-button type="primary" :icon="'Check'" :loading="saving" :disabled="!dirty && !!task" @click="save">
            {{ task ? 'Сохранить' : 'Создать задачу' }}
          </el-button>
          <el-button v-if="!task" @click="$router.push('/tasks')">Отмена</el-button>
        </div>
      </div>

      <el-alert v-if="task && task.form_version !== type.form_version" type="info" :closable="false" show-icon style="margin-bottom: 12px"
        title="Форма типа изменилась после создания задачи — задача показана по актуальной форме, данные удалённых полей сохранены." />
      <el-alert v-if="!type.is_active && !task" type="warning" :closable="false" show-icon style="margin-bottom: 12px" title="Тип задачи отключён — создание недоступно" />

      <TransferDialog v-if="task" v-model="transferOpen" :task="task" @done="onTransferred" />
      <FormRenderer :schema="type.form" :model="model" :task="task" :results="type.results" :errors="errors" :reload-key="reloadKey" />
    </template>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api, { firstFieldError } from '../api'
import FormRenderer from '../components/FormRenderer.vue'
import TransferDialog from '../components/TransferDialog.vue'
import { fmtDateTime, sourceName, statusTag, timeLeft } from '../utils/format'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const saving = ref(false)
const type = ref(null)
const task = ref(null)
const errors = ref({})
const reloadKey = ref(0)
const model = reactive({})
const snapshot = ref('')

const heading = computed(() => (task.value ? `${type.value?.name} №${task.value.number}` : `Новая задача «${type.value?.name || ''}»`))

function fillModel(t) {
  Object.assign(model, {
    title: t?.title ?? '',
    description: t?.description ?? '',
    priority: t?.priority ?? type.value.default_priority,
    executor_user_id: t?.executor_user_id ?? null,
    contractor_id: t?.contractor_id ?? null,
    contact_contractor_id: t?.contact_contractor_id ?? null,
    solve_plan_at: t?.solve_plan_at ?? null,
    result_id: t?.result_id ?? null,
    observer_user_ids: (t?.observers || []).map((o) => o.id),
    data: JSON.parse(JSON.stringify(t?.data ?? defaults())),
  })
  snapshot.value = JSON.stringify(model)
}

// Значения по умолчанию пользовательских полей для новой задачи
function defaults() {
  const d = {}
  for (const b of type.value.form.blocks) for (const fl of b.fields) if (fl.default !== undefined && fl.default !== null && fl.type !== 'system') d[fl.code] = fl.default
  return d
}

const dirty = computed(() => JSON.stringify(model) !== snapshot.value)

async function load() {
  loading.value = true
  try {
    if (route.params.id) {
      const res = await api.task(route.params.id)
      type.value = res.type
      task.value = res.task
      fillModel(res.task)
    } else {
      type.value = await api.type(route.params.typeId)
      task.value = null
      fillModel(null)
    }
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  errors.value = {}
  try {
    const body = { ...model, task_type_id: type.value.id }
    if (task.value) {
      task.value = await api.updateTask(task.value.id, body)
      fillModel(task.value)
      reloadKey.value++
      ElMessage.success('Задача сохранена')
    } else {
      const t = await api.createTask(body)
      snapshot.value = JSON.stringify(model)
      ElMessage.success(`Задача №${t.number} создана`)
      router.replace(`/tasks/${t.id}`)
    }
  } catch (e) {
    if (e.fields) {
      errors.value = e.fields
      ElMessage.error(firstFieldError(e))
    }
  } finally {
    saving.value = false
  }
}

const transferOpen = ref(false)
function openTransfer() {
  if (dirty.value) {
    ElMessage.warning('Сначала сохраните изменения')
    return
  }
  transferOpen.value = true
}
function onTransferred(t) {
  task.value = t
  fillModel(t)
  reloadKey.value++
}

async function setStatus(s) {
  if (dirty.value) {
    ElMessage.warning('Сначала сохраните изменения')
    return
  }
  task.value = await api.setStatus(task.value.id, s)
  fillModel(task.value)
  reloadKey.value++
}

// Таймер срока
const now = ref(Date.now())
const left = computed(() => timeLeft(model.solve_plan_at, now.value))
let tick
onMounted(() => {
  load()
  tick = setInterval(() => (now.value = Date.now()), 1000)
})
onBeforeUnmount(() => clearInterval(tick))

// Защита от потери несохранённых изменений
onBeforeRouteLeave(async () => {
  if (!dirty.value) return true
  try {
    await ElMessageBox.confirm('Есть несохранённые изменения. Уйти со страницы?', 'Внимание', { type: 'warning', confirmButtonText: 'Уйти', cancelButtonText: 'Остаться' })
    return true
  } catch {
    return false
  }
})
watch(
  () => route.params.id,
  (id, old) => id && id !== old && task.value?.id !== id && load(),
)
</script>

<style scoped>
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 10px; vertical-align: middle; }
.sticky { position: sticky; top: 0; z-index: 10; background: var(--el-bg-color-page); padding: 8px 0; }
.left { color: var(--el-color-warning-dark-2); font-weight: 600; display: inline-flex; align-items: center; gap: 4px; }
.overdue { display: inline-flex; align-items: center; gap: 4px; }
</style>
