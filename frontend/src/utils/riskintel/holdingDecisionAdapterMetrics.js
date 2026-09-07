/**
 * Phase9-A/B/B.5: Holding Decision Adapter observation metrics (read-only).
 * B.5 adds accumulation dimensions + rolling stability samples.
 */

export const HOLDING_DECISION_COMPARISON = Object.freeze({
  LEGACY_ONLY: 'LEGACY_ONLY',
  PROJECTION_ONLY: 'PROJECTION_ONLY',
  EXACT_MATCH: 'EXACT_MATCH',
  ACTION_DIFF: 'ACTION_DIFF',
  LEVEL_DIFF: 'LEVEL_DIFF',
  TARGET_DIFF: 'TARGET_DIFF',
  MULTI_FIELD_DIFF: 'MULTI_FIELD_DIFF',
})

export const ROLLING_WINDOW_SIZE = 50
export const MAX_SYMBOL_KEYS = 200

function emptyComparisonFrequency() {
  return {
    LEGACY_ONLY: 0,
    PROJECTION_ONLY: 0,
    EXACT_MATCH: 0,
    ACTION_DIFF: 0,
    LEVEL_DIFF: 0,
    TARGET_DIFF: 0,
    MULTI_FIELD_DIFF: 0,
  }
}

function createEmptyState() {
  return {
    enabled: true,
    total: 0,
    same: 0,
    different: 0,
    projectionUnavailable: 0,
    sourceFrequency: {
      legacy: 0,
      riskAdvice_projection: 0,
    },
    /** Phase9-C.3: controlled-switch runtime counters */
    switchMetrics: {
      projectionSelectedCount: 0,
      legacyFallbackCount: 0,
      preferredLegacyCount: 0,
    },
    shadowMigrationMetrics: {
      comparisonFrequency: emptyComparisonFrequency(),
      simulatedSwitchCount: 0,
      diffSymbolFrequency: {},
      actionFrequency: {},
      levelFrequency: {},
      reasonFrequency: {},
      projectionUnavailableReasons: {},
      recentOutcomes: [], // { match: boolean, unavailable: boolean, comparisonCode }
    },
  }
}

let state = createEmptyState()

function bump(map, key) {
  const k = String(key)
  map[k] = (map[k] || 0) + 1
}

function topEntries(map, limit = 10) {
  return Object.entries(map || {})
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, limit)
    .map(([key, count]) => ({ key, count }))
}

export function resetHoldingDecisionAdapterMetrics() {
  state = createEmptyState()
}

export function setHoldingDecisionAdapterMetricsEnabled(on) {
  state.enabled = !!on
}

/**
 * @param {{
 *   source: string,
 *   hasLegacy: boolean,
 *   hasProjection: boolean,
 *   decisionDiff: 'same'|'different'|'projectionUnavailable'|string,
 *   comparisonCode?: string,
 *   symbol?: string,
 *   action?: string,
 *   level?: string|number|null,
 *   unavailableReason?: string,
 *   fallbackUsed?: boolean,
 * }} row
 */
export function recordHoldingDecisionAdapterObservation(row = {}) {
  if (!state.enabled) return
  state.total += 1
  const src = row.source === 'riskAdvice_projection' ? 'riskAdvice_projection' : 'legacy'
  bump(state.sourceFrequency, src)

  if (row.fallbackUsed) {
    state.switchMetrics.legacyFallbackCount += 1
  } else if (src === 'riskAdvice_projection') {
    state.switchMetrics.projectionSelectedCount += 1
  } else {
    state.switchMetrics.preferredLegacyCount += 1
  }

  const code = row.comparisonCode || HOLDING_DECISION_COMPARISON.LEGACY_ONLY
  const isUnavailable = row.decisionDiff === 'projectionUnavailable'
    || code === HOLDING_DECISION_COMPARISON.LEGACY_ONLY
  const isMatch = row.decisionDiff === 'same'
    || code === HOLDING_DECISION_COMPARISON.EXACT_MATCH

  if (isMatch) state.same += 1
  else if (isUnavailable) state.projectionUnavailable += 1
  else state.different += 1

  if (Object.prototype.hasOwnProperty.call(state.shadowMigrationMetrics.comparisonFrequency, code)) {
    state.shadowMigrationMetrics.comparisonFrequency[code] += 1
  } else {
    bump(state.shadowMigrationMetrics.comparisonFrequency, code)
  }
  bump(state.shadowMigrationMetrics.reasonFrequency, code)

  if (row.action) bump(state.shadowMigrationMetrics.actionFrequency, row.action)
  const levelKey = row.level == null || row.level === '' ? 'null' : String(row.level)
  bump(state.shadowMigrationMetrics.levelFrequency, levelKey)

  const isDiff = !isMatch && !isUnavailable
  if (isDiff && row.symbol) {
    const keys = Object.keys(state.shadowMigrationMetrics.diffSymbolFrequency)
    if (
      state.shadowMigrationMetrics.diffSymbolFrequency[row.symbol]
      || keys.length < MAX_SYMBOL_KEYS
    ) {
      bump(state.shadowMigrationMetrics.diffSymbolFrequency, row.symbol)
    } else {
      bump(state.shadowMigrationMetrics.diffSymbolFrequency, '__other__')
    }
  }

  if (isUnavailable) {
    bump(
      state.shadowMigrationMetrics.projectionUnavailableReasons,
      row.unavailableReason || 'missing_projection',
    )
  }

  state.shadowMigrationMetrics.recentOutcomes.push({
    match: isMatch,
    unavailable: isUnavailable,
    comparisonCode: code,
  })
  if (state.shadowMigrationMetrics.recentOutcomes.length > ROLLING_WINDOW_SIZE) {
    state.shadowMigrationMetrics.recentOutcomes.shift()
  }
}

export function recordHoldingDecisionSimulatedSwitch() {
  if (!state.enabled) return
  state.shadowMigrationMetrics.simulatedSwitchCount += 1
}

export function getHoldingDecisionAdapterMetrics() {
  const sm = state.shadowMigrationMetrics
  const sw = state.switchMetrics
  const total = state.total
  const projectionSelectedCount = sw.projectionSelectedCount
  const legacyFallbackCount = sw.legacyFallbackCount
  const preferredLegacyCount = sw.preferredLegacyCount
  const fallbackRate = total > 0 ? legacyFallbackCount / total : 0
  const projectionUsageRate = total > 0 ? projectionSelectedCount / total : 0
  const freq = sm.comparisonFrequency
  const exact = freq.EXACT_MATCH || 0
  const targetAligned = freq.TARGET_DIFF || 0
  const unavailable = freq.LEGACY_ONLY || 0
  const aligned = exact + targetAligned
  const diffCount = Math.max(0, total - aligned - unavailable)
  const decisionDiffRate = total > 0 ? diffCount / total : 0

  return {
    total,
    same: state.same,
    different: state.different,
    projectionUnavailable: state.projectionUnavailable,
    sourceFrequency: { ...state.sourceFrequency },
    switchMetrics: {
      projectionSelectedCount,
      legacyFallbackCount,
      preferredLegacyCount,
      fallbackRate: Number(fallbackRate.toFixed(4)),
      projectionUsageRate: Number(projectionUsageRate.toFixed(4)),
      decisionDiffRate: Number(decisionDiffRate.toFixed(4)),
      projectionUnavailableReasons: { ...sm.projectionUnavailableReasons },
    },
    shadowMigrationMetrics: {
      comparisonFrequency: { ...sm.comparisonFrequency },
      simulatedSwitchCount: sm.simulatedSwitchCount,
      diffSymbolFrequency: { ...sm.diffSymbolFrequency },
      actionFrequency: { ...sm.actionFrequency },
      levelFrequency: { ...sm.levelFrequency },
      reasonFrequency: { ...sm.reasonFrequency },
      projectionUnavailableReasons: { ...sm.projectionUnavailableReasons },
      highestFrequencyDiffSymbols: topEntries(sm.diffSymbolFrequency, 10),
      highestFrequencyDiffTypes: topEntries(sm.comparisonFrequency, 10)
        .filter((r) => r.key !== 'EXACT_MATCH'),
      recentSampleCount: sm.recentOutcomes.length,
    },
    enabled: state.enabled,
  }
}

export function getShadowMigrationMetrics() {
  return getHoldingDecisionAdapterMetrics().shadowMigrationMetrics
}

/** Raw rolling outcomes for stability (observation only). */
export function getHoldingDecisionRecentOutcomes() {
  return state.shadowMigrationMetrics.recentOutcomes.slice()
}

export function exportHoldingDecisionAdapterMetricsJSON() {
  return JSON.stringify(getHoldingDecisionAdapterMetrics(), null, 2)
}
