import axios from 'axios'
import { ElMessage } from 'element-plus'

const http = axios.create({ baseURL: '/api', timeout: 30000 })

// Ошибки валидации (422) возвращаются вызывающему коду с полями, остальные показываются тостом.
http.interceptors.response.use(
  (r) => r.data,
  (err) => {
    const status = err.response?.status
    const body = err.response?.data || {}
    const e = new Error(body.error || err.message)
    e.status = status
    e.fields = body.fields || null
    if (status !== 422) ElMessage.error(e.message)
    return Promise.reject(e)
  },
)

let metaPromise = null

export default {
  meta: () => (metaPromise ||= http.get('/meta')),

  types: (activeOnly = false) => http.get('/task-types', { params: activeOnly ? { active: 1 } : {} }),
  type: (id) => http.get(`/task-types/${id}`),
  createType: (body) => http.post('/task-types', body),
  updateType: (id, body) => http.put(`/task-types/${id}`, body),
  deleteType: (id) => http.delete(`/task-types/${id}`),
  saveForm: (id, schema) => http.put(`/task-types/${id}/form`, schema),

  tasks: (params) => http.get('/tasks', { params }),
  stats: (params) => http.get('/tasks/stats', { params }),
  task: (id) => http.get(`/tasks/${id}`),
  createTask: (body) => http.post('/tasks', body),
  updateTask: (id, body) => http.put(`/tasks/${id}`, body),
  setStatus: (id, statusId) => http.post(`/tasks/${id}/status`, { status_id: statusId }),
  bulkTransfer: (body) => http.post('/tasks/bulk-transfer', body),
  transfer: (id, body) => http.post(`/tasks/${id}/transfer`, body),
  events: (id) => http.get(`/tasks/${id}/events`),
  comments: (id) => http.get(`/tasks/${id}/comments`),
  addComment: (id, text, authorUserId = null) => http.post(`/tasks/${id}/comments`, { text, author_user_id: authorUserId }),
  updateComment: (id, body) => http.patch(`/comments/${id}`, body),
  deleteComment: (id) => http.delete(`/comments/${id}`),
  contractorTasks: (id) => http.get(`/tasks/${id}/contractor-tasks`),
  clientActivity: (id) => http.get(`/tasks/${id}/client-activity`),

  users: (params) => http.get('/refs/users', { params }),
  contractors: (params) => http.get('/refs/contractors', { params }),
}

/** Первое сообщение валидации — для тоста. */
export function firstFieldError(e) {
  if (!e?.fields) return e?.message
  return Object.values(e.fields)[0]
}
