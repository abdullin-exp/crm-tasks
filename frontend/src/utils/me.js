import { reactive, watch } from 'vue'

// «Текущий пользователь» без авторизации: выбирается в шапке и хранится в браузере.
const KEY = 'crm-tasks.me'
export const me = reactive(JSON.parse(localStorage.getItem(KEY) || 'null') || { id: null, name: '' })
watch(me, (v) => localStorage.setItem(KEY, JSON.stringify(v)), { deep: true })
