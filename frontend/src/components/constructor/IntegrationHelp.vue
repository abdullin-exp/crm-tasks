<template>
  <el-row :gutter="16">
    <el-col :lg="12">
      <el-card shadow="never" header="Создание задачи событием Kafka">
        <p class="muted small" style="margin-top: 0">
          Топик <span class="mono">{{ kafka?.topic }}</span>. Повторная доставка с тем же <span class="mono">event_id</span> не создаёт дубль.
          Невалидные сообщения уходят в <span class="mono">{{ kafka?.dlq_topic }}</span> с заголовком <span class="mono">error</span>.
        </p>
        <pre class="code">{{ message }}</pre>
        <el-button size="small" :icon="'CopyDocument'" @click="copy(message)">Скопировать JSON</el-button>
        <div class="muted small" style="margin: 14px 0 6px">Отправить из контейнера брокера CRM:</div>
        <pre class="code">{{ kafkaCmd }}</pre>
        <el-button size="small" :icon="'CopyDocument'" @click="copy(kafkaCmd)">Скопировать команду</el-button>
      </el-card>
    </el-col>
    <el-col :lg="12">
      <el-card shadow="never" header="Создание задачи через REST API">
        <p class="muted small" style="margin-top: 0">С <span class="mono">external_id</span> вызов идемпотентен (источник «API»).</p>
        <pre class="code">{{ curl }}</pre>
        <el-button size="small" :icon="'CopyDocument'" @click="copy(curl)">Скопировать</el-button>
      </el-card>
      <el-card shadow="never" header="Поля данных (data)" style="margin-top: 16px">
        <el-table :data="fields" size="small" border empty-text="Пользовательских полей нет">
          <el-table-column prop="code" label="Код" width="170"><template #default="{ row }"><span class="mono">{{ row.code }}</span></template></el-table-column>
          <el-table-column prop="label" label="Название" />
          <el-table-column label="Тип / формат" width="220"><template #default="{ row }">{{ formatOf(row) }}</template></el-table-column>
          <el-table-column label="Обяз." width="70" align="center"><template #default="{ row }">{{ row.required ? '✔' : '' }}</template></el-table-column>
        </el-table>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import { computed } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps({ type: Object, schema: Object, kafka: Object })

const fields = computed(() => props.schema.blocks.flatMap((b) => b.fields).filter((f) => f.type !== 'system' && f.type !== 'info'))
const sys = computed(() => new Set(props.schema.blocks.flatMap((b) => b.fields).filter((f) => f.type === 'system').map((f) => f.system)))

function sample(f) {
  switch (f.type) {
    case 'number': return 1
    case 'money': return 5099
    case 'date': return '2026-11-25'
    case 'datetime': return '2026-11-25T12:00:00+03:00'
    case 'checkbox': return true
    case 'select':
    case 'radio': return f.options?.[0]?.value ?? ''
    case 'multiselect': return f.options?.slice(0, 1).map((o) => o.value) ?? []
    case 'phone': return '79270000000'
    case 'email': return 'client@example.com'
    case 'url': return 'https://example.com'
    case 'user':
    case 'contractor': return '00000000-0000-0000-0000-000000000000'
    case 'table': return [Object.fromEntries((f.columns || []).map((c) => [c.code, c.type === 'number' ? 1 : c.type === 'date' ? '2026-11-25' : 'текст']))]
    default: return 'текст'
  }
}
const formatOf = (f) =>
  ({ number: 'число', money: 'число', date: 'ГГГГ-ММ-ДД', datetime: 'RFC3339', checkbox: 'true/false', multiselect: 'массив значений', table: 'массив объектов', user: 'uuid ref_user', contractor: 'uuid ref_contractor', select: 'одно из: ' + (f.options || []).map((o) => o.value).join(', '), radio: 'одно из: ' + (f.options || []).map((o) => o.value).join(', ') })[f.type] || 'строка'

const payload = computed(() => {
  const p = { event_id: 'evt-' + Date.now(), type_code: props.type.code, title: props.type.description || props.type.name }
  if (sys.value.has('description')) p.description = 'Описание задачи'
  if (sys.value.has('executor_user_id')) p.executor_user_id = null
  if (sys.value.has('contractor_id')) p.contractor_id = null
  if (sys.value.has('contact_contractor_id')) p.contact_contractor_id = null
  if (sys.value.has('solve_plan_at')) p.solve_plan_at = null
  if (sys.value.has('observer_user_ids')) p.observer_user_ids = []
  p.priority = props.type.default_priority
  p.data = Object.fromEntries(fields.value.map((f) => [f.code, sample(f)]))
  return p
})
const message = computed(() => JSON.stringify(payload.value, null, 2))
const kafkaCmd = computed(
  () =>
    `echo '${JSON.stringify(payload.value)}' | docker exec -i broker \\\n  /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 --topic ${props.kafka?.topic || 'crm-tasks.task.create'}`,
)
const curl = computed(() => {
  const { event_id, type_code, ...rest } = payload.value
  const body = { ...rest, type_code, external_id: event_id }
  return `curl -X POST ${location.origin}/api/tasks \\\n  -H 'Content-Type: application/json' \\\n  -d '${JSON.stringify(body)}'`
})

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('Скопировано')
  } catch {
    ElMessage.warning('Не удалось скопировать — выделите текст вручную')
  }
}
</script>

<style scoped>
.code { background: #1e1e1e; color: #d4d4d4; padding: 12px; border-radius: 6px; overflow: auto; font-size: 12px; max-height: 420px; white-space: pre-wrap; word-break: break-all; }
</style>
