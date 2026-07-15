/** 买点信号 → 买入交易记录草稿 */

import { UpsertBuySignalDraft } from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { EventsEmit } from '../../wailsjs/runtime'
import { BUY_ENTRY_TAGS } from './buyPriceRange'
import { formatSignalTagLabel } from './signalBuyGuide'
import { evaluateTradeRiskGate } from './riskGate'

/**
 * @param {object} params
 * @param {object} params.entry - 扫描条目（含 tag / buyPriceRange）
 * @param {object} [params.plan] - calcBuyPositionPlan 结果
 * @param {object} [params.checklist]
 * @param {string} [params.marketModeKey]
 * @param {number} [params.livePrice]
 * @param {boolean} [params.skipRiskGate] - 仅写草稿时默认仍做级别检查
 */
export async function upsertBuySignalDraft(params) {
  const { entry, plan, checklist, marketModeKey, livePrice } = params || {}
  if (!entry?.code || !entry?.tag || !BUY_ENTRY_TAGS.has(entry.tag)) {
    return { ok: false, reason: '非买点信号' }
  }
  if (entry.daysAgo != null && Number(entry.daysAgo) > 0) {
    if (entry.recentSignalDaysAgo == null || Number(entry.recentSignalDaysAgo) > 0) {
      return { ok: false, reason: '非当日买点' }
    }
  }

  const gate = evaluateTradeRiskGate({
    marketModeKey,
    checklist,
    direction: '买入',
  })
  // 草稿阶段：级别 4/5 仍禁止写入买入草稿；清单未就绪允许写草稿但确认时再拦
  if (gate.code === 'defense_block') {
    return { ok: false, reason: gate.reason, code: gate.code }
  }

  if (!plan?.ok || !(plan.suggestedAddShares > 0 || plan.suggestedShares > 0)) {
    return { ok: false, reason: plan?.reason || '无有效仓位建议' }
  }

  const volume = plan.suggestedAddShares > 0 ? plan.suggestedAddShares : plan.suggestedShares
  const price =
    Number(livePrice) ||
    Number(plan.entryPrice) ||
    Number(entry.buyPriceRange?.instantPrice) ||
    0
  if (!(volume > 0) || !(price > 0)) {
    return { ok: false, reason: '数量或价格无效' }
  }

  const tagLabel = formatSignalTagLabel(entry.tag)
  const reason = [
    '[量化草稿]',
    `${tagLabel}信号`,
    entry.statusText || '',
    `建议买入 ${volume} 股`,
    plan.positionPct != null ? `目标仓位约 ${plan.positionPct}%` : '',
    plan.stopPrice ? `参考止损 ${Number(plan.stopPrice).toFixed(2)}` : '',
  ]
    .filter(Boolean)
    .join(' · ')

  const record = models.TradingRecord.createFrom({
    StockCode: entry.code,
    StockName: entry.name || entry.code,
    Direction: '买入',
    Status: 'draft',
    Price: price,
    Volume: volume,
    TradingTime: new Date(),
    Reason: reason,
    StopLossPrice: plan.stopPrice || 0,
    TakeProfitPrice: 0,
    Mindset: '量化系统自动生成，请在交易日志中确认后生效；确认受大盘级别与清单风控约束',
  })

  try {
    const id = await UpsertBuySignalDraft(record)
    EventsEmit('tradingRecordDraftUpdated', {
      id,
      code: entry.code,
      tag: entry.tag,
      volume,
      direction: '买入',
    })
    return { ok: true, id, volume, price, reason }
  } catch (e) {
    return { ok: false, reason: e?.message || String(e) }
  }
}
