<template>
  <el-row :gutter="16">
    <el-col v-for="block in schema.blocks" :key="block.id" :xs="24" :lg="block.span">
      <el-card class="block-card" shadow="never" :body-style="{ display: isCollapsed(block) ? 'none' : 'block' }">
        <template #header>
          <div class="block-head" :class="{ clickable: block.collapsible }" @click="block.collapsible && toggle(block)">
            <span>{{ block.title }}</span>
            <el-icon v-if="block.collapsible"><component :is="isCollapsed(block) ? 'ArrowDown' : 'ArrowUp'" /></el-icon>
          </div>
        </template>

        <el-form v-if="block.kind === 'fields'" class="task-form" label-position="right" label-width="auto" @submit.prevent>
          <el-row :gutter="24">
            <el-col v-for="field in visibleFields(block)" :key="field.id" :xs="24" :md="field.span">
              <div v-if="field.type === 'info'" class="info-text">
                <el-alert :title="field.label || undefined" :description="field.text" type="info" :closable="false" show-icon />
              </div>
              <el-form-item v-else :label="field.label" :required="field.required" :error="errors[field.code]">
                <FieldInput
                  :field="field"
                  :model-value="getValue(field)"
                  :task="task"
                  :results="results"
                  :disabled="disabled || field.readonly"
                  @update:model-value="(v) => setValue(field, v)"
                />
                <div v-if="field.help" class="muted small help">{{ field.help }}</div>
              </el-form-item>
            </el-col>
          </el-row>
          <div v-if="!block.fields.length" class="empty-hint">В блоке нет полей</div>
        </el-form>

        <template v-else>
          <div v-if="!task?.id" class="empty-hint">{{ preview ? 'Содержимое блока появится в задаче' : 'Будет доступно после создания задачи' }}</div>
          <CommentsBlock v-else-if="block.kind === 'comments'" :task-id="task.id" :reload-key="reloadKey" />
          <HistoryBlock v-else-if="block.kind === 'history'" :task-id="task.id" :reload-key="reloadKey" />
          <ContractorTasksBlock v-else-if="block.kind === 'contractor_tasks'" :task-id="task.id" :reload-key="reloadKey" />
          <ClientActivityBlock v-else-if="block.kind === 'client_activity'" :task-id="task.id" :reload-key="reloadKey" />
        </template>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import { computed, reactive } from 'vue'
import { conditionValues, isVisible } from '../utils/visibility'
import FieldInput from './FieldInput.vue'
import CommentsBlock from './blocks/CommentsBlock.vue'
import HistoryBlock from './blocks/HistoryBlock.vue'
import ContractorTasksBlock from './blocks/ContractorTasksBlock.vue'
import ClientActivityBlock from './blocks/ClientActivityBlock.vue'

// Рендер формы задачи по схеме из конструктора.
// model: { title, description, priority, executor_user_id, contractor_id, contact_contractor_id, solve_plan_at, result_id, data: {} }
const props = defineProps({
  schema: { type: Object, required: true },
  model: { type: Object, required: true },
  task: { type: Object, default: null },
  results: { type: Array, default: () => [] },
  errors: { type: Object, default: () => ({}) },
  disabled: Boolean,
  preview: Boolean,
  reloadKey: { type: Number, default: 0 },
})

const condValues = computed(() => conditionValues(props.model))
const visibleFields = (block) => block.fields.filter((f) => isVisible(props.schema, f, condValues.value))

const SYSTEM_EDITABLE = ['title', 'description', 'priority', 'executor_user_id', 'contractor_id', 'contact_contractor_id', 'solve_plan_at', 'result_id', 'observer_user_ids']

function getValue(field) {
  if (field.type === 'system') return SYSTEM_EDITABLE.includes(field.system) ? props.model[field.system] : null
  return props.model.data?.[field.code] ?? null
}

function setValue(field, v) {
  if (field.type === 'system') {
    if (SYSTEM_EDITABLE.includes(field.system)) props.model[field.system] = v
    return
  }
  if (!props.model.data) props.model.data = {}
  props.model.data[field.code] = v
}

const collapsed = reactive({})
const isCollapsed = (b) => b.collapsible && (collapsed[b.id] ?? b.collapsed)
const toggle = (b) => {
  collapsed[b.id] = !isCollapsed(b)
}
</script>

<style scoped>
.help { line-height: 1.4; margin-top: 2px; width: 100%; }
.info-text { margin-bottom: 14px; }
.el-form-item :deep(.el-form-item__content) { flex-wrap: wrap; }
</style>
