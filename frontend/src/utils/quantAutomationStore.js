/** 量化自动化全局状态 */

import { ref } from 'vue'
import { resetQuantDecisionObservability } from './quantDecisionObservability'

export const quantEntriesByCode = ref({})
export const quantPlansByCode = ref({})
export const quantChecklistsByCode = ref({})
/** QuantDecision Phase1-A：assembleWatchlistDecision 结果（producer=js_legacy） */
export const quantDecisionsByCode = ref({})
export const quantAutomationRunning = ref(false)
export const quantLastScanAt = ref(null)
export const watchlistSignalsByCode = ref({})

export function setQuantEntry(code, entry, plan, checklist, decision) {
  if (!code) return
  quantEntriesByCode.value = {
    ...quantEntriesByCode.value,
    [code]: entry,
  }
  if (plan !== undefined) {
    quantPlansByCode.value = { ...quantPlansByCode.value, [code]: plan }
  }
  if (checklist !== undefined) {
    quantChecklistsByCode.value = { ...quantChecklistsByCode.value, [code]: checklist }
  }
  if (decision !== undefined) {
    quantDecisionsByCode.value = { ...quantDecisionsByCode.value, [code]: decision }
  }
}

export function clearQuantAutomationCaches() {
  quantEntriesByCode.value = {}
  quantPlansByCode.value = {}
  quantChecklistsByCode.value = {}
  quantDecisionsByCode.value = {}
  resetQuantDecisionObservability()
}

export function setWatchlistSignalSnapshot(byCode) {
  watchlistSignalsByCode.value = { ...(byCode || {}) }
}

export function quantPlanFor(code) {
  return quantPlansByCode.value[code] ?? null
}

export function quantChecklistFor(code) {
  return quantChecklistsByCode.value[code] ?? null
}

export function quantEntryFor(code) {
  return quantEntriesByCode.value[code] ?? null
}

export function quantDecisionFor(code) {
  if (!code) return null
  return quantDecisionsByCode.value[code]
    ?? quantDecisionsByCode.value[String(code).trim().toLowerCase()]
    ?? null
}
