/** 量化自动化全局状态 */

import { ref } from 'vue'

export const quantEntriesByCode = ref({})
export const quantPlansByCode = ref({})
export const quantChecklistsByCode = ref({})
export const quantAutomationRunning = ref(false)
export const quantLastScanAt = ref(null)
export const watchlistSignalsByCode = ref({})

export function setQuantEntry(code, entry, plan, checklist) {
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
