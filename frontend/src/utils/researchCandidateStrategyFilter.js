/**
 * 研究候选列表的策略筛选（只影响当前页可见行）。
 * strategy_id 缺失时视为未标注，不把空值猜成 default 或其他预设。
 */

export function researchCandidateStrategyId(row) {
  if (!row || typeof row !== 'object') return ''
  const raw = row.strategy_id != null ? row.strategy_id : row.strategyId
  if (raw == null) return ''
  return String(raw).trim()
}

export function researchCandidateStrategyName(row) {
  if (!row || typeof row !== 'object') return ''
  const raw = row.strategy_name != null ? row.strategy_name : row.strategyName
  if (raw == null) return ''
  return String(raw).trim()
}

/** 仅收录调用方给出的 id→名称；不做模糊匹配，也不回退到列表第一项。 */
export function strategyNameMapFromPresets(presets) {
  const map = {}
  for (const item of presets || []) {
    const id = String(item?.id || '').trim()
    const name = String(item?.name || '').trim()
    if (id && name) map[id] = name
  }
  return map
}

/**
 * 展示名优先级：快照上已有的 strategy_name → 预设表精确 id → 原始 id。
 * 没有 strategy_id 时固定为「未标注」。
 */
export function researchCandidateStrategyLabel(row, nameById) {
  const id = researchCandidateStrategyId(row)
  if (!id) return '未标注'
  const recorded = researchCandidateStrategyName(row)
  if (recorded) return recorded
  if (nameById && typeof nameById === 'object') {
    const known = nameById[id]
    if (typeof known === 'string' && known.trim()) return known.trim()
  }
  return id
}

function labelForStrategyId(rows, id, nameById) {
  for (const row of rows || []) {
    if (researchCandidateStrategyId(row) !== id) continue
    const recorded = researchCandidateStrategyName(row)
    if (recorded) return recorded
  }
  if (nameById && typeof nameById[id] === 'string' && nameById[id].trim()) {
    return nameById[id].trim()
  }
  return id
}

/** null = 全部；'' = 未标注（仅当当前加载里确有空 strategy_id 时出现）。 */
export function buildResearchStrategyFilterOptions(rows, nameById) {
  const ids = []
  const seen = new Set()
  let unlabeled = false
  for (const row of rows || []) {
    const id = researchCandidateStrategyId(row)
    if (!id) {
      unlabeled = true
      continue
    }
    if (seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  ids.sort((a, b) =>
    labelForStrategyId(rows, a, nameById).localeCompare(labelForStrategyId(rows, b, nameById), 'zh'),
  )
  const options = [{ label: '全部', value: null }]
  for (const id of ids) {
    options.push({ label: labelForStrategyId(rows, id, nameById), value: id })
  }
  if (unlabeled) options.push({ label: '未标注', value: '' })
  return options
}

/** strategyFilter: null 全部，'' 未标注，其它为精确 strategy_id。 */
export function filterResearchCandidatesByStrategy(rows, strategyFilter) {
  const list = Array.isArray(rows) ? rows : []
  if (strategyFilter == null) return list
  if (strategyFilter === '') {
    return list.filter((row) => researchCandidateStrategyId(row) === '')
  }
  const want = String(strategyFilter)
  return list.filter((row) => researchCandidateStrategyId(row) === want)
}

/** 详情里同时给出名称和原始 id；未标注不补 id。 */
export function researchCandidateStrategyDetailText(row, nameById) {
  const id = researchCandidateStrategyId(row)
  const label = researchCandidateStrategyLabel(row, nameById)
  if (!id) return label
  if (label === id) return id
  return `${label}（${id}）`
}

export function researchCandidateRowKey(row) {
  const id = String(row?.id || row?.stock_code || '')
  return `${id}|${researchCandidateStrategyId(row)}`
}
