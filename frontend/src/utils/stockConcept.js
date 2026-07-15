/** 将 CONCEPT 字段（数组 / JSON 字符串 / 顿号或逗号分隔）解析为概念名列表 */
export function parseStockConcepts(value) {
  if (value == null || value === '') return []
  if (Array.isArray(value)) {
    return value.map((c) => String(c ?? '').trim()).filter(Boolean)
  }
  if (typeof value === 'object') {
    return Object.values(value).map((c) => String(c ?? '').trim()).filter(Boolean)
  }
  const s = String(value).trim()
  if (!s) return []
  if (s.startsWith('[')) {
    try {
      const parsed = JSON.parse(s)
      if (Array.isArray(parsed)) {
        return parsed.map((c) => String(c ?? '').trim()).filter(Boolean)
      }
    } catch {
      /* 非 JSON，继续按文本解析 */
    }
  }
  if (s.includes('、') || s.includes(',') || s.includes('，')) {
    return s.split(/[、,，]/).map((c) => c.trim()).filter(Boolean)
  }
  return [s]
}

export function formatStockConceptsText(value, sep = '、') {
  const list = parseStockConcepts(value)
  return list.length ? list.join(sep) : ''
}
