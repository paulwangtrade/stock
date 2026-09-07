/**
 * Phase8.5 observation reason taxonomy (read-only).
 */

export const OBSERVATION_CATEGORY = Object.freeze({
  MATCH: 'MATCH',
  MISMATCH: 'MISMATCH',
  FAILED: 'FAILED',
})

export const OBSERVATION_REASON = Object.freeze({
  EXACT_MATCH: 'EXACT_MATCH',
  BOTH_ABSENT: 'BOTH_ABSENT',
  LEVEL_DIFF: 'LEVEL_DIFF',
  ACTION_DIFF: 'ACTION_DIFF',
  PRIORITY_DIFF: 'PRIORITY_DIFF',
  TARGET_DIFF: 'TARGET_DIFF',
  LEGACY_ONLY: 'LEGACY_ONLY',
  RISK_ADVICE_ONLY: 'RISK_ADVICE_ONLY',
  RISK_ADVICE_EXCEPTION: 'RISK_ADVICE_EXCEPTION',
  CONTEXT_MISSING: 'CONTEXT_MISSING',
  FORBIDDEN: 'FORBIDDEN',
  UNKNOWN_ERROR: 'UNKNOWN_ERROR',
})

/** @param {unknown} raw */
export function rawLevelProvided(raw) {
  return raw != null && raw !== ''
}

/** @param {unknown} raw */
export function rawLevelInvalid(raw) {
  if (!rawLevelProvided(raw)) return true
  const n = Number(raw)
  return !Number.isFinite(n) || n < 1 || n > 5
}

export function mapShadowFailureToObservationReason(failure) {
  const f = String(failure || '').trim().toLowerCase()
  if (f === 'forbidden' || f === 'forbidden_fields') return OBSERVATION_REASON.FORBIDDEN
  if (f === 'context_missing') return OBSERVATION_REASON.CONTEXT_MISSING
  if (f === 'exception' || f === 'risk_advice_exception') return OBSERVATION_REASON.RISK_ADVICE_EXCEPTION
  if (!f) return OBSERVATION_REASON.UNKNOWN_ERROR
  return OBSERVATION_REASON.UNKNOWN_ERROR
}

export function legacyReasonFromObservation(observationReason, levelDelta) {
  switch (observationReason) {
    case OBSERVATION_REASON.EXACT_MATCH:
      return 'LEVEL_MATCH'
    case OBSERVATION_REASON.BOTH_ABSENT:
      return 'BOTH_ABSENT'
    case OBSERVATION_REASON.LEGACY_ONLY:
      return 'MISSING_LEGACY_HOLDING_ADVICE'
    case OBSERVATION_REASON.RISK_ADVICE_ONLY:
      return 'MISSING_RISK_ADVICE'
    case OBSERVATION_REASON.LEVEL_DIFF:
      if (levelDelta != null && levelDelta !== 0) {
        const dir = levelDelta > 0 ? 'UP' : 'DOWN'
        return `LEVEL_DELTA_${dir}_${Math.abs(levelDelta)}`
      }
      return 'LEVEL_MISMATCH'
    case OBSERVATION_REASON.ACTION_DIFF:
      return 'ACTION_MISMATCH'
    case OBSERVATION_REASON.PRIORITY_DIFF:
      return 'PRIORITY_MISMATCH'
    case OBSERVATION_REASON.TARGET_DIFF:
      return 'TARGET_MISMATCH'
    default:
      return String(observationReason)
  }
}

function actionAlignedWithCategory(action, category) {
  const a = String(action || 'hold').trim()
  const c = String(category || 'none').trim()
  if (a === 'reduce' && c === 'market_discipline') return true
  if (a === 'add' && c === 'position_assist') return true
  if (a === 'hold' && (c === 'hold_observe' || c === 'none')) return true
  if (a === 'reduce' && c === 'hold_observe') return false
  if (a === 'add' && c === 'market_discipline') return false
  return a === 'hold'
}

function priorityAligned(holdingAdvice, riskAdvice, legacyLevel) {
  const expectsMarketDriven = legacyLevel === 1 || legacyLevel === 2
  if (!expectsMarketDriven) return true
  return riskAdvice?.marketDriven === true
}

function targetAligned(holdingAdvice, riskAdvice) {
  const pct = holdingAdvice?.suggestPct
  if (pct == null || !Number.isFinite(Number(pct)) || Number(pct) <= 0) return true
  // RiskAdvice intentionally has no authoritative pct in shadow phase.
  return false
}

/**
 * @param {object|null|undefined} holdingAdvice
 * @param {object|null|undefined} riskAdvice
 * @param {{ shadowFailure?: string|null }} [meta]
 */
export function classifyLegacyRiskAdviceObservation(holdingAdvice, riskAdvice, meta = {}) {
  const legacyRaw = holdingAdvice?.marketLevel
  const riskRaw = riskAdvice?.level
  const legacyLevel = normalizeAdviceLevel(legacyRaw)
  const riskLevel = normalizeAdviceLevel(riskRaw)

  if (!holdingAdvice && !riskAdvice) {
    return buildObservation({
      match: true,
      observationCategory: OBSERVATION_CATEGORY.MATCH,
      observationReason: OBSERVATION_REASON.BOTH_ABSENT,
      legacyLevel: null,
      riskLevel: null,
      levelDelta: null,
    })
  }

  if (meta?.shadowFailure) {
    const observationReason = mapShadowFailureToObservationReason(meta.shadowFailure)
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.FAILED,
      observationReason,
      legacyLevel,
      riskLevel: null,
      levelDelta: null,
    })
  }

  if (!holdingAdvice) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.LEGACY_ONLY,
      legacyLevel: null,
      riskLevel,
      levelDelta: null,
    })
  }

  if (!riskAdvice) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.RISK_ADVICE_ONLY,
      legacyLevel,
      riskLevel: null,
      levelDelta: null,
    })
  }

  if (rawLevelInvalid(legacyRaw) && rawLevelInvalid(riskRaw)) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.LEVEL_DIFF,
      legacyLevel: null,
      riskLevel: null,
      levelDelta: null,
    })
  }

  const levelDelta = legacyLevel != null && riskLevel != null ? riskLevel - legacyLevel : null

  if (legacyLevel !== riskLevel) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.LEVEL_DIFF,
      legacyLevel,
      riskLevel,
      levelDelta,
    })
  }

  if (!actionAlignedWithCategory(holdingAdvice?.action, riskAdvice?.category)) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.ACTION_DIFF,
      legacyLevel,
      riskLevel,
      levelDelta: 0,
    })
  }

  if (!priorityAligned(holdingAdvice, riskAdvice, legacyLevel)) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.PRIORITY_DIFF,
      legacyLevel,
      riskLevel,
      levelDelta: 0,
    })
  }

  if (!targetAligned(holdingAdvice, riskAdvice)) {
    return buildObservation({
      match: false,
      observationCategory: OBSERVATION_CATEGORY.MISMATCH,
      observationReason: OBSERVATION_REASON.TARGET_DIFF,
      legacyLevel,
      riskLevel,
      levelDelta: 0,
    })
  }

  return buildObservation({
    match: true,
    observationCategory: OBSERVATION_CATEGORY.MATCH,
    observationReason: OBSERVATION_REASON.EXACT_MATCH,
    legacyLevel,
    riskLevel,
    levelDelta: 0,
  })
}

/** @param {unknown} level */
export function normalizeAdviceLevel(level) {
  if (level == null || level === '') return null
  const n = Number(level)
  if (!Number.isFinite(n) || n < 1 || n > 5) return null
  return n
}

function buildObservation(row) {
  const reason = legacyReasonFromObservation(row.observationReason, row.levelDelta)
  return { ...row, reason }
}
