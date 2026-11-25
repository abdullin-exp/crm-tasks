import dayjs from 'dayjs'

export const fmtDateTime = (v) => (v ? dayjs(v).format('DD.MM.YYYY HH:mm') : '')
export const fmtDate = (v) => (v ? dayjs(v).format('DD.MM.YYYY') : '')
export const fmtMoney = (v) =>
  v === null || v === undefined || v === '' ? '' : Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0, maximumFractionDigits: 2 }) + ' ₽'

export const statusTag = (id) => ({ 1: 'info', 2: 'warning', 3: 'success' })[id] || 'info'
export const priorityTag = (p) => ({ 1: 'info', 2: '', 3: 'danger' })[p] ?? ''
export const priorityName = (p) => ({ 1: 'Низкий', 2: 'Средний', 3: 'Высокий' })[p] || ''
export const sourceName = (s) => ({ manual: 'Вручную', kafka: 'Kafka', api: 'API' })[s] || s

export const RESULT_COLORS = [
  { value: 'success', label: 'Зелёный' },
  { value: 'warning', label: 'Оранжевый' },
  { value: 'primary', label: 'Синий' },
  { value: 'danger', label: 'Красный' },
  { value: 'info', label: 'Серый' },
]

/** Отображение значения пользовательского поля в списке/истории. */
export function displayValue(field, value) {
  if (value === null || value === undefined || value === '') return ''
  switch (field?.type) {
    case 'money':
      return fmtMoney(value)
    case 'date':
      return fmtDate(value)
    case 'datetime':
      return fmtDateTime(value)
    case 'checkbox':
      return value ? 'Да' : 'Нет'
    case 'select':
    case 'radio':
      return field.options?.find((o) => o.value === value)?.label ?? value
    case 'multiselect':
      return (value || []).map((v) => field.options?.find((o) => o.value === v)?.label ?? v).join(', ')
    case 'table':
      return `${value.length} стр.`
    default:
      return String(value)
  }
}

/** Оставшееся время до срока: «0д. 00:14:36» или «просрочено …». */
export function timeLeft(planAt, now = Date.now()) {
  if (!planAt) return null
  let diff = Math.floor((new Date(planAt).getTime() - now) / 1000)
  const overdue = diff < 0
  diff = Math.abs(diff)
  const d = Math.floor(diff / 86400)
  const h = String(Math.floor((diff % 86400) / 3600)).padStart(2, '0')
  const m = String(Math.floor((diff % 3600) / 60)).padStart(2, '0')
  const s = String(diff % 60).padStart(2, '0')
  return { overdue, text: `${d}д. ${h}:${m}:${s}` }
}

export const uid = () => Math.random().toString(36).slice(2, 10)

const TR = { а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'e', ж: 'zh', з: 'z', и: 'i', й: 'y', к: 'k', л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r', с: 's', т: 't', у: 'u', ф: 'f', х: 'h', ц: 'c', ч: 'ch', ш: 'sh', щ: 'sch', ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu', я: 'ya' }

/** Транслитерация названия в код: «Сумма заказа» → summa_zakaza */
export function slug(s) {
  return (s || '')
    .toLowerCase()
    .split('')
    .map((c) => TR[c] ?? c)
    .join('')
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .replace(/^(\d)/, 'f_$1')
    .slice(0, 60)
}
