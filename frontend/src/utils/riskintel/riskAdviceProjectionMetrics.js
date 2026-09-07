/**
 * Phase8.6: RiskAdvice Projection observation metrics (read-only).
 */

function createEmptyState() {
  return {
    enabled: true,
    generated: 0,
    failed: 0,
    missingFieldCount: 0,
    missingFieldFrequency: {},
    failReasons: {
      empty_risk_advice: 0,
      missing_symbol: 0,
      exception: 0,
    },
    projectedActionFrequency: {},
    projectedLevelFrequency: {},
  }
}

let state = createEmptyState()

function bump(map, key) {
  const k = String(key)
  map[k] = (map[k] || 0) + 1
}

export function resetRiskAdviceProjectionMetrics() {
  state = createEmptyState()
}

export function setRiskAdviceProjectionMetricsEnabled(on) {
  state.enabled = !!on
}

export function recordRiskAdviceProjectionGenerated(projection) {
  if (!state.enabled) return
  state.generated += 1
  if (projection?.projectedAction) bump(state.projectedActionFrequency, projection.projectedAction)
  const level = projection?.projectedLevel
  bump(state.projectedLevelFrequency, level == null || level === '' ? 'null' : level)
}

export function recordRiskAdviceProjectionFailed(reason = 'exception') {
  if (!state.enabled) return
  state.failed += 1
  const key = ['empty_risk_advice', 'missing_symbol', 'exception'].includes(reason)
    ? reason
    : 'exception'
  state.failReasons[key] = (state.failReasons[key] || 0) + 1
}

export function recordRiskAdviceProjectionMissingField(field) {
  if (!state.enabled) return
  state.missingFieldCount += 1
  bump(state.missingFieldFrequency, field || 'unknown')
}

export function getRiskAdviceProjectionMetrics() {
  return {
    generated: state.generated,
    failed: state.failed,
    missingFieldCount: state.missingFieldCount,
    missingFieldFrequency: { ...state.missingFieldFrequency },
    failReasons: { ...state.failReasons },
    projectedActionFrequency: { ...state.projectedActionFrequency },
    projectedLevelFrequency: { ...state.projectedLevelFrequency },
    enabled: state.enabled,
  }
}

export function exportRiskAdviceProjectionMetricsJSON() {
  return JSON.stringify(getRiskAdviceProjectionMetrics(), null, 2)
}
