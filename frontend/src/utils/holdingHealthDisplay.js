/**
 * Holding Health display helpers (Phase17-C4).
 * Observation-only labels — not sell / buy recommendations.
 */

export const HEALTH_GRADE = {
  A: 'A',
  B: 'B',
  C: 'C',
  D: 'D',
}

/** Short list label: A 健康 / B 观察 / C 关注 / D 风险 */
export function healthGradeListLabel(grade) {
  switch (String(grade || '').trim().toUpperCase()) {
    case HEALTH_GRADE.A:
      return 'A 健康'
    case HEALTH_GRADE.B:
      return 'B 观察'
    case HEALTH_GRADE.C:
      return 'C 关注'
    case HEALTH_GRADE.D:
      return 'D 风险'
    default:
      return '—'
  }
}

export function healthGradeTagType(grade) {
  switch (String(grade || '').trim().toUpperCase()) {
    case HEALTH_GRADE.A:
      return 'success'
    case HEALTH_GRADE.B:
      return 'info'
    case HEALTH_GRADE.C:
      return 'warning'
    case HEALTH_GRADE.D:
      return 'error'
    default:
      return 'default'
  }
}

export function healthFactorLabelZH(code) {
  switch (String(code || '').trim()) {
    case 'PROFIT':
      return '已盈利'
    case 'PROFIT_EXPANDING':
      return '盈利扩大'
    case 'SIGNAL_ACTIVE':
      return '信号有效'
    case 'TREND_SUPPORT':
      return '趋势支持'
    case 'PROFIT_PROTECTION':
      return '利润保护'
    case 'SIGNAL_EXPIRED':
      return '信号过期'
    case 'LOSS_CONTROL':
      return '亏损扩大'
    case 'PRICE_STALE':
      return '数据过期'
    case 'NO_SOURCE_TRACE':
      return '无来源信息'
    default:
      return String(code || '').trim() || '—'
  }
}

/**
 * Compact reason summary lines for table cell.
 * @returns {{ kind: 'ok'|'warn', text: string }[]}
 */
export function buildHealthReasonSummary(health) {
  if (!health || typeof health !== 'object') return []
  const out = []
  const support = Array.isArray(health.supportingFactors)
    ? health.supportingFactors
    : Array.isArray(health.supporting_factors)
      ? health.supporting_factors
      : []
  const risks = Array.isArray(health.riskFactors)
    ? health.riskFactors
    : Array.isArray(health.risk_factors)
      ? health.risk_factors
      : []
  for (const code of support) {
    const label = healthFactorLabelZH(code)
    if (label && label !== '—') out.push({ kind: 'ok', text: `✓ ${label}` })
  }
  for (const code of risks) {
    const label = healthFactorLabelZH(code)
    if (label && label !== '—') out.push({ kind: 'warn', text: `⚠ ${label}` })
  }
  return out
}

export function formatHealthEvalTime(raw) {
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return String(raw)
  return d.toLocaleString('zh-CN', { hour12: false })
}

export function normalizeHealthStockCode(code) {
  return String(code || '')
    .trim()
    .toLowerCase()
    .replace(/^(sh|sz|bj)/, '')
    .replace(/\.(sh|sz|bj)$/i, '')
}
