/**
 * Phase8-0 Step3-B: RiskAdvice shadow observation metrics only.
 * Does not change trading behavior or UI contracts.
 */

function createEmptyState() {
  return {
    enabled: true,
    generated: 0,
    failed: 0,
    failReasons: {
      exception: 0,
      forbidden_fields: 0,
    },
    /** @type {Record<string, number>} level key "1".."5" or "null" */
    levelDistribution: {},
    /** @type {Record<string, number>} Advice.category frequency */
    categoryFrequency: {},
    /** @type {Record<string, number>} reasons[].code frequency ("risk tags") */
    reasonCodeFrequency: {},
    forbiddenBlocked: 0,
  }
}

/** @type {ReturnType<typeof createEmptyState>} */
let state = createEmptyState()

export function resetScanRiskAdviceShadowMetrics() {
  state = createEmptyState()
}

export function setScanRiskAdviceShadowMetricsEnabled(on) {
  state.enabled = !!on
}

function bump(map, key) {
  const k = String(key)
  map[k] = (map[k] || 0) + 1
}

/**
 * Record a successful RiskAdvice shadow generation (observation only).
 * @param {object} riskAdvice
 */
export function recordScanRiskAdviceShadowGenerated(riskAdvice) {
  if (!state.enabled) return
  state.generated += 1
  const level = riskAdvice?.level
  bump(state.levelDistribution, level == null || level === '' ? 'null' : level)
  if (riskAdvice?.category) bump(state.categoryFrequency, riskAdvice.category)
  const reasons = Array.isArray(riskAdvice?.reasons) ? riskAdvice.reasons : []
  for (const r of reasons) {
    if (r?.code) bump(state.reasonCodeFrequency, r.code)
  }
}

/**
 * Record a failed shadow attempt (observation only).
 * @param {'exception'|'forbidden_fields'|string} reason
 * @param {string[]} [forbiddenKeys]
 */
export function recordScanRiskAdviceShadowFailed(reason = 'exception', forbiddenKeys = []) {
  if (!state.enabled) return
  state.failed += 1
  const key =
    reason === 'forbidden_fields'
      ? 'forbidden_fields'
      : reason === 'context_missing'
        ? 'context_missing'
        : 'exception'
  state.failReasons[key] = (state.failReasons[key] || 0) + 1
  if (reason === 'forbidden_fields') {
    state.forbiddenBlocked += 1
    for (const k of forbiddenKeys || []) bump(state.reasonCodeFrequency, `FORBIDDEN:${k}`)
  }
}

/**
 * Snapshot for dashboards / reports (plain JSON-serializable).
 * riskTagFrequency merges category + reason codes for Step3-B requirement wording.
 */
export function getScanRiskAdviceShadowMetrics() {
  const riskTagFrequency = {
    ...state.categoryFrequency,
  }
  for (const [code, n] of Object.entries(state.reasonCodeFrequency)) {
    riskTagFrequency[`reason:${code}`] = n
  }
  return {
    generated: state.generated,
    failed: state.failed,
    failReasons: { ...state.failReasons },
    levelDistribution: { ...state.levelDistribution },
    categoryFrequency: { ...state.categoryFrequency },
    reasonCodeFrequency: { ...state.reasonCodeFrequency },
    riskTagFrequency,
    forbiddenBlocked: state.forbiddenBlocked,
    enabled: state.enabled,
  }
}

export function exportScanRiskAdviceShadowMetricsJSON() {
  return JSON.stringify(getScanRiskAdviceShadowMetrics(), null, 2)
}
