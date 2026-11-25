<template>
  <el-table :data="items" size="small" border empty-text="Других задач по контрагенту и КЛ нет" max-height="360">
    <el-table-column label="Источник" width="110">
      <template #default="{ row }">
        <el-tag size="small" :type="row.origin === 'crm' ? 'info' : 'success'">{{ row.origin === 'crm' ? 'CRM' : 'Новый сервис' }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="Номер" width="110">
      <template #default="{ row }">
        <router-link v-if="row.origin === 'tasks'" :to="`/tasks/${row.id}`">№{{ row.number }}</router-link>
        <a v-else :href="`${crmUrl}/document/task/view?id=${row.id}`" target="_blank" rel="noopener">№{{ row.number }}</a>
      </template>
    </el-table-column>
    <el-table-column prop="type_name" label="Тип задачи" min-width="180" show-overflow-tooltip />
    <el-table-column prop="title" label="Цель / описание" min-width="200" show-overflow-tooltip />
    <el-table-column prop="status_name" label="Статус" width="110" />
    <el-table-column prop="contractor_name" label="Контрагент" min-width="160" show-overflow-tooltip />
    <el-table-column prop="contact_contractor_name" label="Контактное лицо" min-width="140" show-overflow-tooltip />
    <el-table-column prop="executor_name" label="Исполнитель" min-width="160" show-overflow-tooltip />
    <el-table-column label="Создана" width="140">
      <template #default="{ row }">{{ fmtDateTime(row.created_at) }}</template>
    </el-table-column>
  </el-table>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import api from '../../api'
import { fmtDateTime } from '../../utils/format'

const props = defineProps({ taskId: { type: String, required: true }, reloadKey: Number })
const items = ref([])
const crmUrl = ref('')
const load = async () => {
  items.value = await api.contractorTasks(props.taskId)
}
onMounted(async () => {
  crmUrl.value = (await api.meta()).crm_url
  await load()
})
watch(() => props.reloadKey, load)
</script>
