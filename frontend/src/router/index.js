import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/tasks' },
  { path: '/tasks', name: 'tasks', component: () => import('../views/TaskList.vue'), meta: { title: 'Задачи' } },
  { path: '/tasks/new/:typeId', name: 'task-new', component: () => import('../views/TaskForm.vue'), meta: { title: 'Новая задача' } },
  { path: '/tasks/:id', name: 'task', component: () => import('../views/TaskForm.vue'), meta: { title: 'Задача' } },
  { path: '/admin/types', name: 'types', component: () => import('../views/TypeList.vue'), meta: { title: 'Типы задач' } },
  { path: '/admin/types/:id', name: 'type-edit', component: () => import('../views/TypeEditor.vue'), meta: { title: 'Конструктор типа' } },
]

const router = createRouter({ history: createWebHistory(), routes })
router.afterEach((to) => {
  document.title = `${to.meta.title || 'Задачи'} · CRM`
})
export default router
