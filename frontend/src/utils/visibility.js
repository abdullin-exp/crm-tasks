// Условная видимость полей — зеркало backend/internal/form/visibility.go

export const OPS = [
  { value: 'eq', label: 'равно', multi: false },
  { value: 'neq', label: 'не равно', multi: false },
  { value: 'in', label: 'одно из', multi: true },
  { value: 'not_in', label: 'ни одно из', multi: true },
  { value: 'filled', label: 'заполнено', none: true },
  { value: 'empty', label: 'не заполнено', none: true },
]

function strings(v) {
  if (v === null || v === undefined) return []
  if (Array.isArray(v)) return v.map((x) => String(x).trim()).filter(Boolean)
  if (typeof v === 'boolean') return [String(v)]
  const s = String(v).trim()
  return s ? [s] : []
}

function fieldByCode(schema, code) {
  for (const b of schema.blocks) for (const f of b.fields) if (f.type !== 'info' && (f.code === code || f.system === code)) return f
  return null
}

export function isVisible(schema, field, values, depth = 0) {
  const c = field.visible_if
  if (!c || !c.field) return true
  if (depth > 10) return false
  const src = fieldByCode(schema, c.field)
  if (src && src !== field && !isVisible(schema, src, values, depth + 1)) return false
  const vals = strings(values[c.field])
  const want = (c.values || []).map(String)
  const match = vals.some((v) => want.includes(v))
  switch (c.op) {
    case 'filled': return vals.length > 0
    case 'empty': return vals.length === 0
    case 'eq':
    case 'in': return match
    case 'neq':
    case 'not_in': return !match
  }
  return true
}

/** Значения для условий: данные формы + системные поля модели задачи. */
export function conditionValues(model) {
  const { data, ...sys } = model
  return { ...(data || {}), ...sys }
}
