/** 止/减信号 → 卖出交易记录草稿 */

import { UpsertSellSignalDraft } from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { EventsEmit } from '../../wailsjs/runtime'
import { isSellSignalTag } from './icePointSignals'
import { formatSellPositionPct } from './sellPositionRatio'
import { formatSignalTagLabel } from './signalBuyGuide'

function roundSellVolume(holding, pct, minLot = 100) {
  const h = Math.floor(Number(holding) || 0)
  if (h <= 0 || !Number.isFinite(pct) || pct <= 0) return 0
  let v = Math.floor((h * pct) / minLot) * minLot
  if (v <= 0 && h >= minLot) v = minLot
  if (v > h) v = Math.floor(h / minLot) * minLot
  return v
}

function resolveLivePrice(entry, livePrice) {
  const p = Number(livePrice)
  if (Number.isFinite(p) && p > 0) return p
  const bp = entry?.buyPriceRange
  return Number(bp?.instantPrice) || Number(entry?.latestStatus?.close) || 0
}

/**
 * @param {object} params
 * @param {object} params.entry - 扫描条目
 * @param {number} [params.livePrice]
 * @param {number} [params.holdingVolume] - 自选持仓
 * @param {number} [params.minLotSize]
 */
export async function upsertSellSignalDraft(params) {
  const { entry, livePrice, holdingVolume, minLotSize = 100 } = params || {}
  if (!entry?.code || !entry?.tag || !isSellSignalTag(entry.tag)) {
    return { ok: false, reason: '非卖信号' }
  }
  if (entry.daysAgo !== 0) {
    return { ok: false, reason: '非当日信号' }
  }

  const pct = entry.sellPositionPct
  const volume = roundSellVolume(holdingVolume, pct, minLotSize)
  if (volume <= 0) {
    return { ok: false, reason: '无持仓或建议卖出数量为0' }
  }

  const price = resolveLivePrice(entry, livePrice)
  if (!price || price <= 0) {
    return { ok: false, reason: '无有效价格' }
  }

  const pctLabel = formatSellPositionPct(pct) || ''
  const tagLabel = formatSignalTagLabel(entry.tag)
  const reason = [
    '[量化草稿]',
    `${tagLabel}信号`,
    entry.statusText || '',
    pctLabel ? `建议${tagLabel}${pctLabel}` : '',
    `持仓${holdingVolume}股 → 卖${volume}股`,
  ]
    .filter(Boolean)
    .join(' · ')

  const record = models.TradingRecord.createFrom({
    StockCode: entry.code,
    StockName: entry.name || entry.code,
    Direction: '卖出',
    Status: 'draft',
    Price: price,
    Volume: volume,
    TradingTime: new Date(),
    Reason: reason,
    Mindset: '量化系统自动生成，请在交易日志中确认后生效',
  })

  try {
    const id = await UpsertSellSignalDraft(record)
    EventsEmit('tradingRecordDraftUpdated', { id, code: entry.code, tag: entry.tag, volume })
    return { ok: true, id, volume, price, reason }
  } catch (e) {
    return { ok: false, reason: e?.message || String(e) }
  }
}
