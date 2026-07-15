import { ref } from 'vue'
import { EventsOn } from '../../wailsjs/runtime'
import { GetConfig } from '../../wailsjs/go/main/App'
import {
  buildSignalOptions,
  cloneDefaultSignalSettings,
  getActiveScreenStrategy,
  getActiveScreenStrategySettings,
  getFollowDateGroupSettings as resolveFollowDateGroupSettings,
  getReboundScreenMaxRsi,
  getScreenStrategies,
  getScreenStrategySettingsById,
  parseSignalParams,
} from './signalSettings'

export const signalSettingsState = ref(cloneDefaultSignalSettings())

let listenerReady = false

export function applySignalParamsFromConfig(raw) {
  signalSettingsState.value = parseSignalParams(raw)
}

export function getSignalOptions(extra = {}) {
  return { ...buildSignalOptions(signalSettingsState.value), ...extra }
}

export function getScreenSignalOptions(strategyId, extra = {}) {
  const settings = strategyId
    ? getScreenStrategySettingsById(signalSettingsState.value, strategyId)
    : getActiveScreenStrategySettings(signalSettingsState.value)
  return { ...buildSignalOptions(settings), ...extra }
}

export function getCurrentScreenStrategy() {
  return getActiveScreenStrategy(signalSettingsState.value)
}

export function getScreenStrategyOptions() {
  return getScreenStrategies(signalSettingsState.value).map((item) => ({ label: item.name, value: item.id }))
}

export function getReboundScreenMaxRsiValue() {
  return getReboundScreenMaxRsi(signalSettingsState.value)
}

export function getFollowDateGroupSettings() {
  return resolveFollowDateGroupSettings(signalSettingsState.value)
}

export async function loadSignalSettingsFromBackend() {
  try {
    const res = await GetConfig()
    applySignalParamsFromConfig(res?.signalParams)
  } catch {
    signalSettingsState.value = cloneDefaultSignalSettings()
  }
}

export function initSignalSettingsSync() {
  if (listenerReady) return
  listenerReady = true
  EventsOn('updateSettings', (config) => {
    applySignalParamsFromConfig(config?.signalParams)
  })
}
