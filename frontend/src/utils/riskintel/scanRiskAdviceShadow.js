/**
 * Phase8-0 Step3-A/B / Phase8.5 shadow dual-write + observation.
 */

import { buildRiskContextBundle } from './riskContextBundle.js'
import { buildRiskAdvice, scanForbiddenAdviceKeys } from './riskAdvice.js'
import {
  recordScanRiskAdviceShadowFailed,
  recordScanRiskAdviceShadowGenerated,
} from './scanRiskAdviceShadowMetrics.js'

/**
 * @returns {{ riskContext: object|null, riskAdvice: object|null, failReason: string|null }}
 */
export function buildScanRiskAdviceShadow(bundleInput = {}) {
  if (bundleInput == null || typeof bundleInput !== 'object') {
    recordScanRiskAdviceShadowFailed('context_missing')
    return { riskContext: null, riskAdvice: null, failReason: 'context_missing' }
  }
  try {
    const riskContext = buildRiskContextBundle(bundleInput)
    const riskAdvice = buildRiskAdvice(riskContext)
    const hits = scanForbiddenAdviceKeys(riskAdvice)
    if (hits.length) {
      recordScanRiskAdviceShadowFailed('forbidden_fields', hits)
      return { riskContext: null, riskAdvice: null, failReason: 'forbidden_fields' }
    }
    recordScanRiskAdviceShadowGenerated(riskAdvice)
    return { riskContext, riskAdvice, failReason: null }
  } catch {
    recordScanRiskAdviceShadowFailed('exception')
    return { riskContext: null, riskAdvice: null, failReason: 'exception' }
  }
}

export function attachScanRiskAdviceShadow(legacyRow, bundleInput) {
  if (!legacyRow || typeof legacyRow !== 'object') return legacyRow
  const shadow = buildScanRiskAdviceShadow(bundleInput)
  if (!shadow.riskAdvice) return { ...legacyRow }
  return {
    ...legacyRow,
    riskContext: shadow.riskContext,
    riskAdvice: shadow.riskAdvice,
  }
}
