/**
 * Phase8.6: RiskAdvice Projection — read-only simulation of future holding-facing shape.
 * Not a decision. Does not order, mutate holdings, write DB, or overwrite holdingAdvice.
 */

import { ADVICE_CATEGORY } from './constants.js'
import {
  recordRiskAdviceProjectionFailed,
  recordRiskAdviceProjectionGenerated,
  recordRiskAdviceProjectionMissingField,
} from './riskAdviceProjectionMetrics.js'

export const RISK_ADVICE_PROJECTION_SOURCE = 'riskAdvice_projection'

export const PROJECTED_ACTION = Object.freeze({
  REDUCE: 'reduce',
  HOLD: 'hold',
  ADD: 'add',
  NONE: 'none',
})

export function mapRiskAdviceToProjectedAction(riskAdvice) {
  if (!riskAdvice || typeof riskAdvice !== 'object') return PROJECTED_ACTION.NONE
  const level = riskAdvice.level
  const category = String(riskAdvice.category || '')
  if (
    riskAdvice.suggestedAction === 'reduce'
    || riskAdvice.suggestedAction === 'hold'
    || riskAdvice.suggestedAction === 'add'
    || riskAdvice.suggestedAction === 'none'
  ) {
    return riskAdvice.suggestedAction
  }
  if (level === 1 || level === 2 || category === ADVICE_CATEGORY.MARKET_DISCIPLINE) {
    return PROJECTED_ACTION.REDUCE
  }
  if (category === ADVICE_CATEGORY.POSITION_ASSIST) return PROJECTED_ACTION.ADD
  if (category === ADVICE_CATEGORY.HOLD_OBSERVE) return PROJECTED_ACTION.HOLD
  if (category === ADVICE_CATEGORY.NONE) return PROJECTED_ACTION.NONE
  if (level === 3 || level === 4 || level === 5) return PROJECTED_ACTION.HOLD
  return PROJECTED_ACTION.NONE
}

export function resolveCurrentHoldingState(position = {}, meta = {}) {
  const costPrice = Number(position.costPrice ?? meta.costPrice)
  const costVolume = Number(position.costVolume ?? meta.costVolume ?? position.volume)
  const hasPosition = Number.isFinite(costPrice) && costPrice > 0
    && Number.isFinite(costVolume) && costVolume > 0
  return {
    hasPosition,
    costPrice: hasPosition ? costPrice : null,
    costVolume: hasPosition ? costVolume : null,
    profitPct: Number.isFinite(Number(position.profitPct ?? meta.profitPct))
      ? Number(position.profitPct ?? meta.profitPct)
      : null,
    industry: position.industry || meta.industry || null,
    bkName: position.bkName || meta.bkName || null,
  }
}

/**
 * @returns {{ ok: boolean, projection: object|null, missingFields: string[], failReason: string|null }}
 */
export function buildRiskAdviceProjection(riskAdvice, options = {}) {
  const missingFields = []
  const position = options.position && typeof options.position === 'object' ? options.position : {}
  const riskContext = options.riskContext && typeof options.riskContext === 'object'
    ? options.riskContext
    : null
  const meta = options.meta && typeof options.meta === 'object' ? options.meta : {}

  if (!riskAdvice || typeof riskAdvice !== 'object') {
    recordRiskAdviceProjectionFailed('empty_risk_advice')
    return {
      ok: false,
      projection: null,
      missingFields: ['riskAdvice'],
      failReason: 'empty_risk_advice',
    }
  }

  const symbol =
    String(
      options.symbol
      || riskAdvice.subject?.stockCode
      || riskContext?.source?.stockCode
      || meta.stockCode
      || '',
    ).trim() || null

  if (!symbol) {
    missingFields.push('symbol')
    recordRiskAdviceProjectionMissingField('symbol')
  }
  if (riskAdvice.level == null) {
    missingFields.push('projectedLevel')
    recordRiskAdviceProjectionMissingField('projectedLevel')
  }
  if (!Array.isArray(riskAdvice.reasons) || riskAdvice.reasons.length === 0) {
    missingFields.push('riskReasons')
    recordRiskAdviceProjectionMissingField('riskReasons')
  }
  if (typeof riskAdvice.confidence !== 'number' && typeof riskContext?.confidence !== 'number') {
    missingFields.push('confidence')
    recordRiskAdviceProjectionMissingField('confidence')
  }

  try {
    const projectedAction = mapRiskAdviceToProjectedAction(riskAdvice)
    const projectedLevel = riskAdvice.level == null ? null : Number(riskAdvice.level)
    const riskReasons = Array.isArray(riskAdvice.reasons)
      ? riskAdvice.reasons.map((r) => ({
        code: String(r?.code || ''),
        message: String(r?.message || ''),
        marketDriven: !!r?.marketDriven,
      }))
      : []
    const confidence = typeof riskAdvice.confidence === 'number'
      ? riskAdvice.confidence
      : (typeof riskContext?.confidence === 'number' ? riskContext.confidence : null)

    const projection = {
      symbol,
      currentHoldingState: resolveCurrentHoldingState(position, meta),
      projectedAction,
      projectedLevel: Number.isFinite(projectedLevel) ? projectedLevel : null,
      riskReasons,
      confidence,
      source: RISK_ADVICE_PROJECTION_SOURCE,
      category: riskAdvice.category || null,
      marketDriven: !!riskAdvice.marketDriven,
      policyVersion: riskAdvice.policyVersion || riskContext?.policyVersion || null,
      asOf: riskAdvice.asOf || riskContext?.asOf || meta.asOf || null,
    }

    if (!symbol) {
      recordRiskAdviceProjectionFailed('missing_symbol')
      return {
        ok: false,
        projection,
        missingFields,
        failReason: 'missing_symbol',
      }
    }

    recordRiskAdviceProjectionGenerated(projection)
    return {
      ok: true,
      projection,
      missingFields,
      failReason: null,
    }
  } catch {
    recordRiskAdviceProjectionFailed('exception')
    return {
      ok: false,
      projection: null,
      missingFields,
      failReason: 'exception',
    }
  }
}

export function compareLegacyHoldingToProjection(holdingAdvice, projection) {
  const legacyLevel = holdingAdvice?.marketLevel == null ? null : Number(holdingAdvice.marketLevel)
  const projectedLevel = projection?.projectedLevel == null ? null : Number(projection.projectedLevel)
  const legacyAction = String(holdingAdvice?.action || '').trim() || null
  const projectedAction = String(projection?.projectedAction || '').trim() || null
  return {
    levelMatch: legacyLevel === projectedLevel,
    actionMatch: legacyAction === projectedAction,
    legacyLevel,
    projectedLevel,
    legacyAction,
    projectedAction,
    source: projection?.source || null,
  }
}
