<template>
  <div>
    <draggable :list="results" item-key="_key" handle=".handle" class="list">
      <template #item="{ element: r, index }">
        <div class="row">
          <el-icon class="handle"><Rank /></el-icon>
          <el-input v-model="r.name" placeholder="Название результата" />
          <el-select v-model="r.color" style="width: 150px">
            <el-option v-for="c in RESULT_COLORS" :key="c.value" :value="c.value" :label="c.label">
              <el-tag :type="c.value" size="small">{{ c.label }}</el-tag>
            </el-option>
          </el-select>
          <el-tooltip content="Закрывающий результат переводит задачу в статус «Выполнена». Незакрывающий (например «Не дозвонился») оставляет её в работе.">
            <el-switch v-model="r.is_closing" active-text="закрывает" inactive-text="в работе" inline-prompt style="--el-switch-on-color: #67c23a; width: 100px" />
          </el-tooltip>
          <el-button :type="r.color" size="small" plain style="min-width: 120px">{{ r.name || 'Пример' }}</el-button>
          <el-button link type="danger" :icon="'Delete'" @click="results.splice(index, 1)" />
        </div>
      </template>
    </draggable>
    <el-button size="small" :icon="'Plus'" style="margin-top: 8px" @click="add">Добавить результат</el-button>
  </div>
</template>

<script setup>
import draggable from 'vuedraggable'
import { RESULT_COLORS, uid } from '../../utils/format'

const props = defineProps({ results: { type: Array, required: true } })
const add = () => props.results.push({ _key: uid(), name: '', color: 'primary', is_closing: true })
</script>

<style scoped>
.list { display: flex; flex-direction: column; gap: 6px; }
.row { display: flex; gap: 8px; align-items: center; }
.handle { cursor: grab; color: var(--el-text-color-secondary); }
</style>
