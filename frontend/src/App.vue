<template>
  <el-container direction="vertical" style="min-height: 100%">
    <el-header class="app-header">
      <router-link to="/tasks" class="logo">
        <el-icon :size="20"><Tickets /></el-icon>
        <span>CRM · Задачи</span>
      </router-link>
      <el-menu mode="horizontal" :default-active="active" router :ellipsis="false" class="app-menu">
        <el-menu-item index="/tasks">Задачи</el-menu-item>
        <el-menu-item index="/admin/types">Конструктор типов</el-menu-item>
      </el-menu>
      <div class="spacer" />
      <div class="me">
        <span class="muted small">Я:</span>
        <RefSelect
          :model-value="me.id"
          kind="user"
          :label="me.name"
          placeholder="Выберите себя"
          style="width: 230px"
          @select="(u) => Object.assign(me, u ? { id: u.id, name: u.name } : { id: null, name: '' })"
        />
      </div>
      <el-tooltip :content="kafkaHint" placement="bottom">
        <el-tag :type="kafkaOk ? 'success' : 'info'" size="small" effect="plain">Kafka: {{ kafkaOk ? 'подключена' : 'нет связи' }}</el-tag>
      </el-tooltip>
    </el-header>
    <el-main style="padding: 0">
      <router-view :key="$route.fullPath.split('?')[0]" />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import RefSelect from './components/RefSelect.vue'
import { me } from './utils/me'

const route = useRoute()
const active = computed(() => (route.path.startsWith('/admin') ? '/admin/types' : '/tasks'))

const kafka = ref(null)
const kafkaOk = computed(() => kafka.value?.enabled && !kafka.value?.last_error)
const kafkaHint = computed(() =>
  kafka.value?.enabled
    ? `Топик ${kafka.value.topic}. Обработано: ${kafka.value.processed}, отклонено: ${kafka.value.failed}` +
      (kafka.value.last_error ? `. Ошибка: ${kafka.value.last_error}` : '')
    : 'Консьюмер выключен',
)
let timer
const poll = async () => {
  try {
    kafka.value = (await axios.get('/api/health')).data.kafka
  } catch (e) {
    kafka.value = e.response?.data?.kafka || null
  }
}
onMounted(() => {
  poll()
  timer = setInterval(poll, 15000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.app-header { display: flex; align-items: center; gap: 24px; background: #fff; border-bottom: 1px solid var(--el-border-color-light); height: 56px; }
.logo { display: flex; align-items: center; gap: 8px; font-weight: 700; font-size: 16px; color: var(--el-color-primary); text-decoration: none; }
.app-menu { border-bottom: none; height: 55px; }
.spacer { flex: 1; }
.me { display: flex; align-items: center; gap: 6px; }
</style>
