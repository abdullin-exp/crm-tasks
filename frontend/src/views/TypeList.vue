<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h1>Типы задач</h1>
        <div class="muted small" style="margin-top: 4px">Настройка типов, результатов и конструктор формы — для технического пользователя</div>
      </div>
      <el-button type="success" :icon="'Plus'" @click="openCreate">Новый тип</el-button>
    </div>

    <el-card shadow="never" :body-style="{ padding: 0 }">
      <el-table v-loading="loading" :data="types" empty-text="Типов пока нет" @row-click="(r) => $router.push(`/admin/types/${r.id}`)">
        <el-table-column label="Название" min-width="240">
          <template #default="{ row }"><span class="dot" :style="{ background: row.color }" /><b>{{ row.name }}</b></template>
        </el-table-column>
        <el-table-column prop="code" label="Код" width="200"><template #default="{ row }"><span class="mono">{{ row.code }}</span></template></el-table-column>
        <el-table-column label="Блоков / полей" width="140">
          <template #default="{ row }">{{ row.form.blocks.length }} / {{ row.form.blocks.reduce((n, b) => n + b.fields.length, 0) }}</template>
        </el-table-column>
        <el-table-column label="Результатов" width="120"><template #default="{ row }">{{ row.results.filter((r) => r.is_active).length }}</template></el-table-column>
        <el-table-column label="Срок" width="110"><template #default="{ row }">{{ row.solve_minutes ? row.solve_minutes + ' мин' : '—' }}</template></el-table-column>
        <el-table-column prop="tasks_count" label="Задач" width="90" />
        <el-table-column label="Версия формы" width="130"><template #default="{ row }">v{{ row.form_version }}</template></el-table-column>
        <el-table-column label="Статус" width="120">
          <template #default="{ row }"><el-tag :type="row.is_active ? 'success' : 'info'" size="small">{{ row.is_active ? 'Активен' : 'Отключён' }}</el-tag></template>
        </el-table-column>
        <el-table-column width="150" align="right">
          <template #default="{ row }">
            <el-button size="small" :icon="'Edit'" @click.stop="$router.push(`/admin/types/${row.id}`)">Настроить</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createOpen" title="Новый тип задачи" width="520px">
      <el-form :model="form" label-position="top" @submit.prevent="create">
        <el-form-item label="Название" :error="errors.name" required>
          <el-input v-model="form.name" placeholder="Например: Заявка с сайта" @input="autoCode" />
        </el-form-item>
        <el-form-item label="Код (латиница, используется в Kafka-событиях и API)" :error="errors.code" required>
          <el-input v-model="form.code" placeholder="site_request" @input="codeTouched = true" />
        </el-form-item>
        <el-form-item label="Описание">
          <el-input v-model="form.description" type="textarea" />
        </el-form-item>
        <el-form-item label="Скопировать форму и результаты из типа">
          <el-select v-model="copyFrom" clearable placeholder="Стандартная стартовая форма" style="width: 100%">
            <el-option v-for="t in types" :key="t.id" :value="t.id" :label="t.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createOpen = false">Отмена</el-button>
        <el-button type="primary" :loading="saving" @click="create">Создать и открыть конструктор</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { slug } from '../utils/format'

const router = useRouter()
const types = ref([])
const loading = ref(false)
const createOpen = ref(false)
const saving = ref(false)
const errors = ref({})
const form = reactive({ name: '', code: '', description: '' })
const copyFrom = ref(null)
const codeTouched = ref(false)

const load = async () => {
  loading.value = true
  try {
    types.value = await api.types()
  } finally {
    loading.value = false
  }
}
onMounted(load)

function openCreate() {
  Object.assign(form, { name: '', code: '', description: '' })
  copyFrom.value = null
  codeTouched.value = false
  errors.value = {}
  createOpen.value = true
}

function autoCode() {
  if (!codeTouched.value) form.code = slug(form.name)
}

async function create() {
  saving.value = true
  errors.value = {}
  try {
    const body = { ...form, results: [{ name: 'Выполнена', color: 'success', is_closing: true }] }
    const src = types.value.find((t) => t.id === copyFrom.value)
    if (src) {
      body.form = src.form
      body.results = src.results.filter((r) => r.is_active).map(({ name, color, is_closing }) => ({ name, color, is_closing }))
      body.solve_minutes = src.solve_minutes
      body.default_priority = src.default_priority
      body.color = src.color
    }
    const t = await api.createType(body)
    createOpen.value = false
    router.push(`/admin/types/${t.id}`)
  } catch (e) {
    if (e.fields) errors.value = e.fields
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 8px; vertical-align: middle; }
:deep(.el-table__row) { cursor: pointer; }
</style>
