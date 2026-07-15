/** 账户概览：自选持仓市值 vs 权益与 5 级仓位上限 */

import { getQuantAutomationFromSettings } from './quantAutomationSettings'
import { signalSettingsState } from './signalSettingsStore'
import { resolveMarketPositionCap } from './positionPolicy'

function num(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

/**
 * @param {Array<object>} stocks - 自选行情行（含 costVolume / 当前价格）
 * @param {{ marketModeKey?: string, automation?: object }} [opts]
 */
export function buildAccountOverview(stocks = [], opts = {}) {
  const auto = opts.automation || getQuantAutomationFromSettings(signalSettingsState.value)
  const equity = Math.max(0, num(auto.accountEquity) || 500000)
  const cap = resolveMarketPositionCap(opts.marketModeKey || 'unknown', auto)
  const maxExposureValue = equity * cap.pct

  let positionValue = 0
  let holdingCount = 0
  for (const row of stocks) {
    const vol = num(row.costVolume ?? row.CostVolume ?? row.Volume)
    if (vol <= 0) continue
    const price =
      num(row['当前价格']) ||
      num(row['买一报价']) ||
      num(row.costPrice) ||
      num(row.CostPrice) ||
      0
    if (price <= 0) continue
    positionValue += vol * price
    holdingCount += 1
  }

  const exposurePct = equity > 0 ? positionValue / equity : 0
  const roomValue = Math.max(0, maxExposureValue - positionValue)
  const roomPct = equity > 0 ? roomValue / equity : 0
  const overCap = positionValue > maxExposureValue + 1

  return {
    equity,
    positionValue: Math.round(positionValue * 100) / 100,
    holdingCount,
    exposurePct,
    exposurePctDisplay: Math.round(exposurePct * 1000) / 10,
    maxExposurePct: cap.pct,
    maxExposurePctDisplay: cap.pctDisplay,
    maxExposureValue: Math.round(maxExposureValue * 100) / 100,
    roomValue: Math.round(roomValue * 100) / 100,
    roomPctDisplay: Math.round(roomPct * 1000) / 10,
    overCap,
    level: cap.level,
    levelName: cap.levelName,
    rangeLabel: cap.rangeLabel,
    hint: overCap
      ? `当前总仓位 ${Math.round(exposurePct * 100)}% 已超过纪律上限 ${cap.pctDisplay}%`
      : `可用仓位约 ${Math.round(roomPct * 100)}%（距上限 ${cap.rangeLabel}）`,
  }
}
