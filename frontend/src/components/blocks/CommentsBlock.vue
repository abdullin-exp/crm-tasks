<template>
  <div>
    <div class="new">
      <el-input v-model="text" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" placeholder="Новый комментарий… (Ctrl+Enter — отправить)" @keydown.ctrl.enter="add" @keydown.meta.enter="add" />
      <el-button type="success" :icon="'Plus'" :disabled="!text.trim()" :loading="saving" @click="add">Добавить</el-button>
    </div>
    <el-scrollbar max-height="420px">
      <div v-for="c in items" :key="c.id" class="comment" :class="{ fav: c.is_favorite, sys: c.is_system }">
        <template v-if="editing === c.id">
          <el-input v-model="editText" type="textarea" :autosize="{ minRows: 2 }" />
          <div class="actions">
            <el-button size="small" type="primary" @click="saveEdit(c)">Сохранить</el-button>
            <el-button size="small" @click="editing = null">Отмена</el-button>
          </div>
        </template>
        <template v-else>
          <div class="text">{{ c.text }}</div>
          <div class="meta">
            <span class="muted small">{{ c.is_system ? 'Система' : c.author_name || 'Без автора' }} · {{ fmtDateTime(c.created_at) }}</span>
            <span class="actions">
              <el-button link :icon="c.is_favorite ? 'StarFilled' : 'Star'" :type="c.is_favorite ? 'warning' : ''" title="В избранное" @click="fav(c)" />
              <el-button link :icon="'Edit'" title="Редактировать" @click="startEdit(c)" />
              <el-popconfirm title="Удалить комментарий?" @confirm="remove(c)">
                <template #reference><el-button link :icon="'Delete'" type="danger" title="Удалить" /></template>
              </el-popconfirm>
            </span>
          </div>
        </template>
      </div>
      <div v-if="!items.length" class="empty-hint">Комментариев пока нет</div>
    </el-scrollbar>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import api from '../../api'
import { me } from '../../utils/me'
import { fmtDateTime } from '../../utils/format'

const props = defineProps({ taskId: { type: String, required: true }, reloadKey: Number })
const items = ref([])
const text = ref('')
const saving = ref(false)
const editing = ref(null)
const editText = ref('')

const load = async () => {
  items.value = await api.comments(props.taskId)
}
onMounted(load)
watch(() => props.reloadKey, load)

async function add() {
  if (!text.value.trim()) return
  saving.value = true
  try {
    await api.addComment(props.taskId, text.value, me.id)
    text.value = ''
    await load()
  } finally {
    saving.value = false
  }
}
async function fav(c) {
  await api.updateComment(c.id, { is_favorite: !c.is_favorite })
  await load()
}
function startEdit(c) {
  editing.value = c.id
  editText.value = c.text
}
async function saveEdit(c) {
  await api.updateComment(c.id, { text: editText.value })
  editing.value = null
  await load()
}
async function remove(c) {
  await api.deleteComment(c.id)
  await load()
}
</script>

<style scoped>
.new { display: flex; gap: 8px; align-items: flex-start; margin-bottom: 12px; }
.comment { border-left: 3px solid var(--el-border-color); background: var(--el-fill-color-lighter); padding: 8px 12px; margin-bottom: 8px; border-radius: 4px; }
.comment.sys { border-left-color: var(--el-color-info); background: var(--el-color-info-light-9); }
.comment.fav { border-left-color: var(--el-color-warning); }
.text { white-space: pre-wrap; font-weight: 500; }
.meta { display: flex; justify-content: space-between; align-items: center; margin-top: 4px; }
.actions { display: flex; gap: 2px; margin-top: 4px; }
</style>
