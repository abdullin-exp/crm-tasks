<template>
  <el-scrollbar max-height="400px">
    <el-timeline v-if="items.length" style="padding: 4px 0 0 2px">
      <el-timeline-item v-for="e in items" :key="e.id" :timestamp="fmtDateTime(e.created_at)" :type="typeOf(e.kind)" placement="top">
        <div>{{ e.message }}</div>
        <ul v-if="e.payload?.changes" class="changes">
          <li v-for="c in e.payload.changes" :key="c.field">
            <b>{{ c.label }}:</b>
            <span class="muted"> {{ show(c.old) }}</span> → <span>{{ show(c.new) }}</span>
          </li>
        </ul>
      </el-timeline-item>
    </el-timeline>
    <div v-else class="empty-hint">Событий нет</div>
  </el-scrollbar>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import api from '../../api'
import { fmtDateTime } from '../../utils/format'

const props = defineProps({ taskId: { type: String, required: true }, reloadKey: Number })
const items = ref([])
const load = async () => {
  items.value = await api.events(props.taskId)
}
onMounted(load)
watch(() => props.reloadKey, load)

const typeOf = (k) => ({ created: 'primary', status: 'warning', transfer: 'info', updated: '' })[k] || ''
const show = (v) => {
  if (v === null || v === undefined || v === '') return '—'
  if (Array.isArray(v)) return v.length && typeof v[0] === 'object' ? `${v.length} стр.` : v.join(', ')
  if (typeof v === 'object') return JSON.stringify(v)
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(v)) return fmtDateTime(v)
  return String(v)
}
</script>

<style scoped>
.changes { margin: 4px 0 0; padding-left: 16px; font-size: 13px; }
</style>
