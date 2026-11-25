<template>
  <el-dialog :model-value="modelValue" :title="bulk ? `Перевод задач (${tasks.length})` : 'Перевод задачи'" width="520px" @update:model-value="(v) => emit('update:modelValue', v)" @open="reset">
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item v-if="!bulk" label="Текущий исполнитель">
        <div>{{ task.executor_name || 'не назначен' }}</div>
      </el-form-item>
      <el-form-item v-else label="Задачи">
        <div class="picked">
          <el-tag v-for="t in tasks.slice(0, 12)" :key="t.id" size="small" effect="plain">№{{ t.number }} · {{ t.executor_name || 'без исполнителя' }}</el-tag>
          <span v-if="tasks.length > 12" class="muted small">и ещё {{ tasks.length - 12 }}</span>
        </div>
      </el-form-item>
      <el-form-item label="Новый исполнитель" required :error="errors.executor_user_id">
        <RefSelect v-model="form.executor_user_id" kind="user" placeholder="Начните вводить ФИО или логин" />
      </el-form-item>
      <el-form-item label="Причина перевода" required :error="errors.reason">
        <el-input v-model="form.reason" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="Например: клиент закреплён за другим менеджером" />
        <div class="quick">
          <el-tag v-for="r in QUICK" :key="r" class="clickable" effect="plain" @click="form.reason = r">{{ r }}</el-tag>
        </div>
      </el-form-item>
      <el-form-item label="Новая плановая дата завершения (необязательно)">
        <el-date-picker v-model="form.solve_plan_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" format="DD.MM.YYYY HH:mm" style="width: 100%" />
      </el-form-item>
      <el-alert v-if="errors.status_id" :title="errors.status_id" type="error" :closable="false" show-icon />
      <el-alert v-if="errors.task_ids" :title="errors.task_ids" type="error" :closable="false" show-icon />
      <div class="muted small">Задача станет «Новой» у нового исполнителя. Перевод и причина попадут в историю и комментарии.</div>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">Отмена</el-button>
      <el-button type="primary" :loading="saving" @click="submit">Перевести</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, h, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import RefSelect from './RefSelect.vue'

const props = defineProps({ modelValue: Boolean, task: { type: Object, default: null }, tasks: { type: Array, default: null } })
const bulk = computed(() => !!props.tasks)
const emit = defineEmits(['update:modelValue', 'done'])

const QUICK = ['Клиент закреплён за другим менеджером', 'Отпуск / больничный', 'Перераспределение нагрузки', 'Нужна компетенция другого отдела']
const form = reactive({ executor_user_id: null, reason: '', solve_plan_at: null })
const errors = ref({})
const saving = ref(false)

function reset() {
  Object.assign(form, { executor_user_id: null, reason: '', solve_plan_at: null })
  errors.value = {}
}

async function submit() {
  saving.value = true
  errors.value = {}
  try {
    const body = { ...form, solve_plan_at: form.solve_plan_at || null }
    if (bulk.value) {
      const res = await api.bulkTransfer({ ...body, task_ids: props.tasks.map((t) => t.id) })
      const failed = Object.entries(res.errors || {})
      if (res.transferred) ElMessage.success(`Переведено задач: ${res.transferred}`)
      if (failed.length) {
        const num = (id) => props.tasks.find((t) => t.id === id)?.number ?? id
        ElMessageBox.alert(h('div', failed.map(([id, msg]) => h('div', `№${num(id)}: ${msg}`))), `Не переведено: ${failed.length}`, { type: 'warning' })
      }
      emit('done', res)
    } else {
      const t = await api.transfer(props.task.id, body)
      ElMessage.success(`Задача переведена на ${t.executor_name}`)
      emit('done', t)
    }
    emit('update:modelValue', false)
  } catch (e) {
    if (e.fields) errors.value = e.fields
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.picked { display: flex; flex-wrap: wrap; gap: 4px; align-items: center; }
.quick { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
</style>
