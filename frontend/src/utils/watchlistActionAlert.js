/** 自选页：今日有操作提示（可买/等回踩/先风控）时的弹窗去重与文案 */

import { BUY_ENTRY_TAGS } from './buyPriceRange'
import { isSellSignalTag } from './icePointSignals'
import { getQuantAutomationFromSettings } from './quantAutomationSettings'
import { resolveSignalActionHint } from './signalActionHint'
import { formatSignalTagLabel } from './signalBuyGuide'

const STATE_KEY = 'watchlistActionAlertStateV1'

/** 需要弹窗提醒的操作标签 */
export const WATCHLIST_ACTIONABLE_LABELS = new Set(['可买', '等回踩', '可加仓', '早减', '先风控'])

function loadState() {
  try {
    const raw = localStorage.getItem(STATE_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch {
    return {}
  }
}

function saveState(state) {
  try {
    localStorage.setItem(STATE_KEY, JSON.stringify(state))
  } catch {
    /* ignore */
  }
}

function dayKey() {
  const d = new Date()
  return `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
}

function alertKey(code, tag, actionLabel) {
  return `${String(code || '').toLowerCase()}:${tag}:${actionLabel}:${dayKey()}`
}

function stockLabel(name, code) {
  const n = String(name || '').trim()
  const c = String(code || '').trim()
  if (n && c && n !== c) return `${n} (${c})`
  return n || c || '未知股票'
}

/**
 * 量化自动化已覆盖的提醒，自选页不再重复弹
 */
export function isCoveredByQuantAutomation(entry, actionLabel, automation) {
  if (!automation?.enabled) return false
  const alerts = automation.alerts || {}
  const tag = entry?.tag
  if (!tag || entry.daysAgo !== 0) return false
  if (actionLabel === '先风控' && alerts.sellSignal && isSellSignalTag(tag)) return true
  if (actionLabel === '可买' && alerts.signalNew && BUY_ENTRY_TAGS.has(tag)) return true
  return false
}

/**
 * @param {Record<string, object>} byCode scanWatchlistSignals 的 byCode
 * @param {object} ctx
 * @param {Record<string, object>} [ctx.liveByCode] 代码 → 行情行
 * @param {function} [ctx.quantChecklistFor]
 * @param {function} [ctx.quantEntryFor]
 * @param {object} [ctx.automation] 量化自动化配置
 * @returns {Array<{ code, name, tag, actionLabel, actionHint, title, content, isRed, sortRank }>}
 */
export function collectWatchlistActionAlerts(byCode, ctx = {}) {
  const {
    liveByCode = {},
    quantChecklistFor,
    quantEntryFor,
    automation,
  } = ctx
  const out = []
  for (const [code, entry] of Object.entries(byCode || {})) {
    if (!entry?.ok || !entry.tag || entry.daysAgo !== 0) continue
    const qEntry = quantEntryFor?.(code)
    const actionHint = resolveSignalActionHint({
      tag: entry.tag,
      buyPriceRange: qEntry?.buyPriceRange ?? entry.buyPriceRange,
      sellPositionPct: entry.sellPositionPct,
      addPositionPct: entry.addPositionPct,
      rushReducePct: entry.rushReducePct,
      sourceTag: entry.sourceTag,
      daysAgo: 0,
      checklistReady: quantChecklistFor?.(code)?.ready,
    })
    if (!actionHint || !WATCHLIST_ACTIONABLE_LABELS.has(actionHint.label)) continue
    if (isCoveredByQuantAutomation(entry, actionHint.label, automation)) continue

    const name = entry.name || code
    const label = stockLabel(name, code)
    const tagLabel = formatSignalTagLabel(entry.tag, entry.sellPositionPct ?? entry.addPositionPct ?? entry.rushReducePct)
    const isRed = actionHint.label === '先风控'
    out.push({
      code,
      name,
      tag: entry.tag,
      actionLabel: actionHint.label,
      actionHint,
      sortRank: entry.sortRank ?? 99,
      isRed,
      title: `${name} · ${actionHint.label}`,
      content: [label, ...(actionHint.lines || [])].join('\n'),
      watchlistAlert: {
        code,
        name,
        tag: entry.tag,
        actionLabel: actionHint.label,
        actionHint,
        isRed,
      },
    })
  }
  out.sort((a, b) => a.sortRank - b.sortRank || a.name.localeCompare(b.name, 'zh-CN'))
  return out
}

/**
 * 过滤当日已提醒过的条目
 * @returns {{ fresh: Array, state: object }}
 */
export function filterFreshWatchlistAlerts(alerts) {
  const state = loadState()
  const dk = dayKey()
  const fresh = []
  for (const item of alerts || []) {
    const key = alertKey(item.code, item.tag, item.actionLabel)
    if (state[key] === dk) continue
    fresh.push(item)
  }
  return { fresh, state }
}

export function markWatchlistAlertsSent(alerts, prevState) {
  const state = { ...prevState }
  const dk = dayKey()
  for (const item of alerts || []) {
    state[alertKey(item.code, item.tag, item.actionLabel)] = dk
  }
  saveState(state)
}

/** 多条合并为一条摘要（避免首屏连弹） */
export function buildWatchlistActionSummary(alerts) {
  if (!alerts?.length) return null
  if (alerts.length === 1) return alerts[0]
  const buy = alerts.filter((a) => a.actionLabel === '可买').length
  const wait = alerts.filter((a) => a.actionLabel === '等回踩').length
  const risk = alerts.filter((a) => a.actionLabel === '先风控').length
  const parts = []
  if (buy) parts.push(`可买 ${buy}`)
  if (wait) parts.push(`等回踩 ${wait}`)
  if (risk) parts.push(`先风控 ${risk}`)
  const lines = alerts.slice(0, 8).map((a) => `${stockLabel(a.name, a.code)} ${a.tag}·${a.actionLabel}`)
  if (alerts.length > 8) lines.push(`…共 ${alerts.length} 只`)
  const hasRisk = risk > 0
  return {
    code: alerts[0].code,
    name: alerts[0].name,
    tag: alerts[0].tag,
    isRed: hasRisk && buy === 0 && wait === 0,
    title: `自选 ${alerts.length} 只有操作提示（${parts.join(' · ')}）`,
    content: lines.join('\n'),
    batch: alerts,
    watchlistAlert: {
      batch: alerts,
      isRed: hasRisk && buy === 0 && wait === 0,
      actionLabel: hasRisk ? '先风控' : buy ? '可买' : wait ? '等回踩' : alerts[0].actionLabel,
    },
  }
}

/**
 * 推送自选操作提示（全局 / 自选页共用，内部去重）
 * @param {Record<string, object>} byCode
 * @param {function} dispatchNotify
 * @param {object} ctx
 */
export function pushWatchlistActionAlerts(byCode, dispatchNotify, ctx = {}) {
  const settings = ctx.settings
  if (settings?.display?.watchlistActionPopup === false) return

  const automation = getQuantAutomationFromSettings(settings)
  const alerts = collectWatchlistActionAlerts(byCode, {
    quantChecklistFor: ctx.quantChecklistFor,
    quantEntryFor: ctx.quantEntryFor,
    automation,
  })
  const { fresh, state } = filterFreshWatchlistAlerts(alerts)
  if (!fresh.length) return

  markWatchlistAlertsSent(fresh, state)

  const payload = fresh.length >= 3 ? buildWatchlistActionSummary(fresh) : fresh[0]
  if (!payload) return

  dispatchNotify({
    type: 'watchlistAction',
    isRed: !!payload.isRed,
    title: payload.title,
    content: payload.content,
    code: payload.code,
    tag: payload.tag,
    batch: payload.batch,
    watchlistAlert: payload.watchlistAlert || payload,
  })
}
