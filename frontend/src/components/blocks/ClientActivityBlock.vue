<template>
  <div>
    <el-radio-group v-model="filter" size="small" style="margin-bottom: 10px">
      <el-radio-button value="">Все</el-radio-button>
      <el-radio-button v-for="(k, key) in KINDS" :key="key" :value="key">{{ k.label }}</el-radio-button>
    </el-radio-group>
    <el-scrollbar max-height="380px">
      <el-timeline v-if="visible.length" style="padding: 4px 0 0 2px">
        <el-timeline-item v-for="a in visible" :key="a.kind + a.id" :timestamp="fmtDateTime(a.at)" :color="KINDS[a.kind]?.color" placement="top">
          <el-tag size="small" effect="plain" style="margin-right: 6px">{{ KINDS[a.kind]?.label }}</el-tag>
          <router-link v-if="a.kind === 'task'" :to="`/tasks/${a.id}`">{{ a.title }}</router-link>
          <span v-else>{{ a.title }}</span>
          <div v-if="a.detail" class="muted small" style="margin-top: 2px">{{ a.detail }}</div>
        </el-timeline-item>
      </el-timeline>
      <div v-else class="empty-hint">Нет активности: укажите контрагента или контактное лицо</div>
    </el-scrollbar>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import api from '../../api'
import { fmtDateTime } from '../../utils/format'

const KINDS = {
  call: { label: 'Звонки', color: '#67c23a' },
  order: { label: 'Заказы', color: '#e6a23c' },
  task: { label: 'Задачи', color: '#409eff' },
  crm_task: { label: 'Задачи CRM', color: '#909399' },
}

const props = defineProps({ taskId: { type: String, required: true }, reloadKey: Number })
const items = ref([])
const filter = ref('')
const visible = computed(() => (filter.value ? items.value.filter((a) => a.kind === filter.value) : items.value))
const load = async () => {
  items.value = await api.clientActivity(props.taskId)
}
onMounted(load)
watch(() => props.reloadKey, load)
</script>
