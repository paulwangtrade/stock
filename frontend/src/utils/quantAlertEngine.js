/** 量化告警：信号 / 区间触达 / 清单就绪 / 卖信号 — 去重与推送 */

import { BUY_ENTRY_TAGS } from './buyPriceRange'
import { isSellSignalTag } from './icePointSignals'
import { formatPositionPlanText } from './buyPositionSizing'
import { formatChecklistScore } from './buyChecklist'
import { formatSignalTagLabel } from './signalBuyGuide'

const STATE_KEY = 'quantAlertStateV1'

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

function alertKey(code, type, extra = '') {
  return `${code}:${type}:${extra}:${dayKey()}`
}

function canSend(state, key, cooldownMs) {
  const last = state[key] || 0
  return Date.now() - last >= cooldownMs
}

function markSent(state, key) {
  state[key] = Date.now()
}

function priceInZone(price, bp, tolPct = 0.003) {
  if (!bp || !Number.isFinite(price)) return false
  const lo = bp.low
  const hi = bp.high ?? bp.instantPrice
  if (!Number.isFinite(lo) || !Number.isFinite(hi)) return false
  return price >= lo * (1 - tolPct) && price <= hi * (1 + tolPct)
}

export function createQuantAlertEngine() {
  let state = loadState()

  function persist() {
    saveState(state)
  }

  function emitAlert(onNotify, payload) {
    onNotify?.(payload)
  }

  /** 扫描完成后：新信号、卖信号、清单就绪 */
  function processScanEntry(code, entry, plan, checklist, automation, onNotify) {
    if (!automation?.enabled) return
    const cooldown = (automation.alertCooldownMinutes ?? 15) * 60 * 1000
    const name = entry.name || code
    const tag = entry.tag
    const tagLabel = formatSignalTagLabel(tag)
    const alerts = automation.alerts || {}

    if (alerts.signalNew && tag && entry.daysAgo === 0) {
      if (BUY_ENTRY_TAGS.has(tag)) {
        const sigKey = alertKey(code, 'signal', tag)
        const barKey = `${code}:signalBar:${entry.signalBar ?? tag}:${dayKey()}`
        if (!state[barKey] && canSend(state, sigKey, cooldown)) {
          markSent(state, sigKey)
          state[barKey] = 1
          emitAlert(onNotify, {
            type: 'signal',
            isRed: false,
            title: `【量化买点】${name}`,
            content: `${name}(${code})\n信号：${tagLabel}\n${entry.statusText || ''}\n${entry.buyPriceRange?.text ? `参考区间 ${entry.buyPriceRange.text}` : ''}`,
            code,
            tag,
          })
        }
      }
    }

    if (alerts.sellSignal && tag && isSellSignalTag(tag) && entry.daysAgo === 0) {
      const sellKey = alertKey(code, 'sell', tag)
      const barKey = `${code}:sellBar:${entry.daysAgo}:${tag}:${dayKey()}`
      if (!state[barKey] && canSend(state, sellKey, cooldown)) {
        markSent(state, sellKey)
        state[barKey] = 1
        const pct = entry.sellPositionPct != null
          ? `${Math.round(entry.sellPositionPct * 100)}%`
          : ''
        emitAlert(onNotify, {
          type: 'sell',
          isRed: true,
          title: `【量化风控】${name}`,
          content: `${name}(${code})\n信号：${tagLabel}${pct ? ` · 建议${tagLabel}${pct}` : ''}\n${entry.statusText || ''}`,
          code,
          tag,
        })
      }
    }

    if (alerts.checklistReady && checklist?.ready && BUY_ENTRY_TAGS.has(tag)) {
      const ckKey = alertKey(code, 'checklist')
      if (canSend(state, ckKey, cooldown)) {
        markSent(state, ckKey)
        emitAlert(onNotify, {
          type: 'checklist',
          isRed: false,
          title: `【清单就绪】${name}`,
          content: `${name}(${code})\n${formatChecklistScore(checklist)} · 信号 ${tagLabel}\n${plan?.ok ? formatPositionPlanText(plan) : ''}`,
          code,
          tag,
        })
      }
    }

    if (alerts.positionPlan && plan?.ok && plan.suggestedAddShares > 0 && BUY_ENTRY_TAGS.has(tag)) {
      const pk = alertKey(code, 'plan', String(plan.suggestedShares))
      if (canSend(state, pk, cooldown * 2)) {
        markSent(state, pk)
        emitAlert(onNotify, {
          type: 'plan',
          isRed: false,
          title: `【仓位建议】${name}`,
          content: `${name}(${code})\n${formatPositionPlanText(plan)}\n止损≈${plan.stopPrice?.toFixed(2)} · 风险≈${plan.riskAmount}元`,
          code,
          tag,
        })
      }
    }

    persist()
  }

  /** 实时价：区间触达 */
  function processPriceTick(code, price, entry, automation, onNotify) {
    if (!automation?.enabled || !entry?.buyPriceRange) return
    const alerts = automation.alerts || {}
    if (!alerts.zoneTouch || !Number.isFinite(price)) return

    const tol = automation.zoneTouchTolerancePct ?? 0.003
    const inZone = priceInZone(price, entry.buyPriceRange, tol)
    const zoneStateKey = `${code}:inZone`
    const wasInZone = !!state[zoneStateKey]

    if (inZone && !wasInZone && BUY_ENTRY_TAGS.has(entry.tag)) {
      const zk = alertKey(code, 'zone')
      const cooldown = (automation.alertCooldownMinutes ?? 15) * 60 * 1000
      if (canSend(state, zk, cooldown)) {
        markSent(state, zk)
        emitAlert(onNotify, {
          type: 'zone',
          isRed: false,
          title: `【区间触达】${entry.name || code}`,
          content: `${entry.name || code}(${code})\n现价 ${price} 落入 ${entry.buyPriceRange.text || '买入区间'}\n信号 ${entry.tag}`,
          code,
          tag: entry.tag,
        })
      }
    }

    state[zoneStateKey] = inZone ? 1 : 0
    persist()
  }

  return { processScanEntry, processPriceTick, resetState: () => { state = {}; persist() } }
}

export const quantAlertEngine = createQuantAlertEngine()
