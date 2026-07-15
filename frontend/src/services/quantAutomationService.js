/**
 * 量化自动化服务：全局扫描、实时区间监控、告警推送
 * 在 App.vue 启动，不依赖自选页是否打开
 */

import { EventsEmit, EventsOn, EventsOff, SendNotification } from '../../wailsjs/runtime'
import {
  GetFollowList,
  SendDingDingMessage,
} from '../../wailsjs/go/main/App'
import { scanWatchlistSignals, isScannableWatchlistCode } from '../utils/watchlistSignalScan'
import { getQuantAutomationFromSettings } from '../utils/quantAutomationSettings'
import { signalSettingsState, getSignalOptions } from '../utils/signalSettingsStore'
import { quantAlertEngine } from '../utils/quantAlertEngine'
import { calcBuyPositionPlan } from '../utils/buyPositionSizing'
import { evaluateBuyChecklist } from '../utils/buyChecklist'
import { upsertSellSignalDraft } from '../utils/sellRecordDraft'
import { isSellSignalTag } from '../utils/icePointSignals'
import { pushWatchlistActionAlerts } from '../utils/watchlistActionAlert'
import { getLastMarketModeKey } from '../utils/marketStatusBar'
import {
  quantAutomationRunning,
  quantLastScanAt,
  setQuantEntry,
  quantEntriesByCode,
  quantChecklistFor,
  quantEntryFor,
  setWatchlistSignalSnapshot,
} from '../utils/quantAutomationStore'
import { measurePerformance } from './performanceMetrics'

let scanTimer = null
let initialScanTimer = null
let scanInFlight = null
let priceListenerReady = false
let followListCache = []

function getAutomation() {
  return getQuantAutomationFromSettings(signalSettingsState.value)
}

function shouldRunBackgroundScan() {
  const automation = getAutomation()
  const actionPopup = signalSettingsState.value?.display?.watchlistActionPopup !== false
  return automation.enabled || actionPopup
}

function normalizeCode(code) {
  return String(code || '').trim().toLowerCase()
}

function matchFollowRow(code) {
  const c = normalizeCode(code)
  return followListCache.find((f) => {
    const fc = normalizeCode(f.StockCode)
    return fc === c || fc === `sh${c}` || fc === `sz${c}` || c.endsWith(fc) || fc.endsWith(c.replace(/^(sh|sz)/, ''))
  })
}

async function buildScanTargets() {
  try {
    const list = await GetFollowList()
    followListCache = Array.isArray(list) ? list : []
    const seen = new Set()
    const out = []
    for (const f of followListCache) {
      const code = f.StockCode
      if (!code || seen.has(code) || !isScannableWatchlistCode(code)) continue
      seen.add(code)
      out.push({ code, name: f.Name || f.StockName || code })
    }
    return out
  } catch {
    return []
  }
}

function buildPositionsByCode() {
  const positions = {}
  for (const row of followListCache) {
    const code = row?.StockCode
    const costPrice = Number(row?.CostPrice) || 0
    const costVolume = Number(row?.Volume) || 0
    if (!code || costPrice <= 0 || costVolume <= 0) continue
    positions[code] = {
      costPrice,
      costVolume,
      industry: row?.Industry || row?.['所属行业'] || '',
      bkName: row?.BkName || row?.['所属板块'] || '',
      profitPct: Number(row?.Profit ?? row?.profit),
    }
  }
  return positions
}

function dispatchNotify(payload) {
  EventsEmit('quantPush', payload)
  try {
    SendNotification?.({ title: payload.title || '量化提醒', body: payload.content || '' })
  } catch {
    /* optional */
  }
  const title = payload.title || '量化提醒'
  const md = `### ${title}\n\n${(payload.content || '').replace(/\n/g, '\n\n')}`
  SendDingDingMessage(md, payload.code || '').catch(() => {})
}

/** 自选操作提示（自选页 / 后台扫描共用） */
export function dispatchWatchlistActionPush(byCode) {
  pushWatchlistActionAlerts(byCode, dispatchNotify, {
    settings: signalSettingsState.value,
    quantChecklistFor,
    quantEntryFor,
  })
}

function enrichEntry(entry, livePrice, stockRow) {
  const automation = getAutomation()
  const follow = matchFollowRow(entry.code)
  const existingVolume = Number(stockRow?.costVolume || follow?.Volume || 0)
  const existingCostPrice = Number(stockRow?.costPrice || follow?.CostPrice || 0)
  const totalExposureValue = followListCache.reduce((sum, row) => {
    const volume = Number(row?.Volume || row?.volume || 0)
    const price = Number(row?.Price || row?.price || row?.CostPrice || row?.costPrice || 0)
    return sum + Math.max(0, volume) * Math.max(0, price)
  }, 0)

  const fullSummary = {
    ...entry,
    latestStatus: entry.latestStatus,
    hasRecentStrictBuy: entry.hasRecentStrictBuy,
    hasRecentBreakout: entry.hasRecentBreakout,
    recentSignalDaysAgo: entry.daysAgo,
  }

  const checklist = evaluateBuyChecklist({
    summary: fullSummary,
    buyPriceRange: entry.buyPriceRange,
    livePrice,
    stockRow: stockRow || { SECURITY_NAME_ABBR: entry.name },
    automation,
  })

  const plan = calcBuyPositionPlan({
    summary: fullSummary,
    buyPriceRange: entry.buyPriceRange,
    livePrice,
    automation,
    existingVolume,
    existingCostPrice,
    totalExposureValue,
    marketModeKey: getLastMarketModeKey(),
  })

  return { checklist, plan }
}

async function performFollowListSignalScan() {
  if (!shouldRunBackgroundScan() || document.hidden) {
    quantAutomationRunning.value = false
    return
  }

  const finishMetric = measurePerformance('quant.scan')
  const automation = getAutomation()
  quantAutomationRunning.value = true
  let scannedCount = 0
  try {
    const targets = await buildScanTargets()
    if (!targets.length) {
      if (automation.enabled) quantEntriesByCode.value = {}
      setWatchlistSignalSnapshot({})
      return
    }

    const signalOpts = getSignalOptions()
    const { byCode } = await scanWatchlistSignals(targets, {
      concurrency: 3,
      positionsByCode: buildPositionsByCode(),
      recentBuyDays: signalOpts.recentBuyDays ?? 15,
      recentSellDays: signalOpts.recentSellDays ?? 5,
    })
    scannedCount = Object.keys(byCode || {}).length
    setWatchlistSignalSnapshot(byCode)

    if (automation.enabled) {
      for (const [code, entry] of Object.entries(byCode || {})) {
        if (!entry?.ok) continue
        const enriched = { ...entry, code }
        const { checklist, plan } = enrichEntry(enriched, null, null)
        setQuantEntry(code, enriched, plan, checklist)
        quantAlertEngine.processScanEntry(code, enriched, plan, checklist, automation, dispatchNotify)

        if (
          automation.alerts?.sellDraft !== false
          && isSellSignalTag(enriched.tag)
          && enriched.daysAgo === 0
        ) {
          const follow = matchFollowRow(code)
          upsertSellSignalDraft({
            entry: enriched,
            holdingVolume: follow?.Volume,
            minLotSize: automation.minLotSize ?? 100,
          }).then((draft) => {
            if (draft.ok) {
              dispatchNotify({
                type: 'sellDraft',
                isRed: true,
                title: `【交易草稿】${enriched.name || code}`,
                content: `${enriched.name || code}(${code})\n卖出草稿已更新\n${draft.volume} 股 @ ${draft.price?.toFixed?.(2) ?? draft.price}\n${draft.reason || ''}`,
                code,
                tag: enriched.tag,
              })
            }
          }).catch(() => {})
        }
      }
    }

    dispatchWatchlistActionPush(byCode)

    quantLastScanAt.value = Date.now()
    EventsEmit('quantScanDone', { count: scannedCount })
  } catch (e) {
    console.error('[quantAutomation]', e)
  } finally {
    quantAutomationRunning.value = false
    finishMetric()
  }
}

function runFollowListSignalScan() {
  if (scanInFlight) return scanInFlight
  scanInFlight = performFollowListSignalScan().finally(() => {
    scanInFlight = null
  })
  return scanInFlight
}

function scheduleScan() {
  clearInterval(scanTimer)
  clearTimeout(initialScanTimer)
  scanTimer = null
  initialScanTimer = null
  if (!shouldRunBackgroundScan() || document.hidden) return
  const automation = getAutomation()
  const mins = Math.max(5, automation.scanIntervalMinutes ?? 20)
  scanTimer = setInterval(runFollowListSignalScan, mins * 60 * 1000)
  initialScanTimer = setTimeout(runFollowListSignalScan, 45 * 1000)
}

function onVisibilityChange() {
  if (document.hidden) {
    clearInterval(scanTimer)
    clearTimeout(initialScanTimer)
    scanTimer = null
    initialScanTimer = null
    return
  }
  scheduleScan()
  const staleFor = Date.now() - Number(quantLastScanAt.value || 0)
  if (staleFor >= 60_000) runFollowListSignalScan()
}

function onStockPrice(data) {
  const automation = getAutomation()
  if (!automation.enabled) return
  const code = data?.code || data?.['股票代码'] || data?.stockCode
  const price = Number(data?.price ?? data?.['当前价格'] ?? data?.currentPrice)
  if (!code || !Number.isFinite(price)) return

  const entry =
    quantEntriesByCode.value[code] ||
    quantEntriesByCode.value[normalizeCode(code)]
  if (!entry) return

  quantAlertEngine.processPriceTick(code, price, entry, automation, dispatchNotify)

  const follow = matchFollowRow(code)
  const { checklist, plan } = enrichEntry(entry, price, {
    costVolume: follow?.Volume,
    costPrice: follow?.CostPrice,
  })
  setQuantEntry(code, entry, plan, checklist)
}

export function startQuantAutomation() {
  if (priceListenerReady) {
    scheduleScan()
    return
  }
  priceListenerReady = true
  EventsOn('stock_price', onStockPrice)
  EventsOn('updateSettings', () => scheduleScan())
  document.addEventListener('visibilitychange', onVisibilityChange)
  scheduleScan()
}

export function stopQuantAutomation() {
  clearInterval(scanTimer)
  clearTimeout(initialScanTimer)
  EventsOff('stock_price')
  document.removeEventListener('visibilitychange', onVisibilityChange)
  priceListenerReady = false
}

export function triggerQuantScanNow() {
  return runFollowListSignalScan()
}
