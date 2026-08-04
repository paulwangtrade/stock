/**
 * UI helpers: after generate-next, choose upcoming query trade_date.
 * Pure — no fetch / no backend changes.
 */

/**
 * When planId is present, prefer the generated trade_date so the new plan is visible
 * (upcoming server semantics unchanged for that date).
 * Without planId, keep the caller's input / default upcoming fallback.
 *
 * @param {{ planId?: number, generatedTradeDate?: string, inputTradeDate?: string }} opts
 * @returns {string|undefined}
 */
export function resolveUpcomingQueryTradeDateAfterGenerate(opts = {}) {
  const planId = Math.trunc(Number(opts.planId) || 0)
  const generated = String(opts.generatedTradeDate || '').trim()
  if (planId > 0 && generated) {
    return generated
  }
  const input = String(opts.inputTradeDate || '').trim()
  return input || undefined
}

/**
 * @param {number|undefined} preferredPlanId
 * @param {number|null|undefined} loadedPlanId
 */
export function isPreferredGeneratedPlan(preferredPlanId, loadedPlanId) {
  const pref = Math.trunc(Number(preferredPlanId) || 0)
  if (pref <= 0) return false
  return Math.trunc(Number(loadedPlanId) || 0) === pref
}
