/** 股票交易实战 5 级模型：大盘级别 -> 组合仓位控制（参考，非投资建议） */

import { getQuantAutomationFromSettings } from './quantAutomationSettings'
import { signalSettingsState } from './signalSettingsStore'
import { calculateModelPositionCap, toDisplayTradingLevel } from './tradingLevelRules'

export {
  MARKET_MODE_EXPOSURE_FACTOR,
  TRADING_LEVELS,
  resolveTradingLevel,
} from './tradingLevelRules'

/**
 * @returns {{ pct: number, pctDisplay: number, rangeLabel: string, level: number|null,
 *   levelName: string, modelMinPct: number, modelMaxPct: number, hint: string }}
 */
export function resolveMarketPositionCap(modeKey, automation) {
  const auto = automation || getQuantAutomationFromSettings(signalSettingsState.value)
  const { rule, maxExposure, pct } = calculateModelPositionCap(modeKey, auto.maxTotalExposurePct)
  const pctDisplay = Math.round(pct * 100)
  const configuredText = Math.round(maxExposure * 100)
  const ruleMinDisplay = Math.round(rule.minPct * 100)
  const effectiveRangeLabel = rule.level === 5 && pct >= rule.minPct && pct < rule.maxPct
    ? `${ruleMinDisplay}%-${pctDisplay}%`
    : pct < rule.minPct
      ? `不高于${pctDisplay}%（设置约束）`
      : rule.rangeLabel
  const effectiveText = pctDisplay < Math.round(rule.maxPct * 100)
    ? `；受设置中的总敞口上限 ${configuredText}% 约束，实际不超过 ${pctDisplay}%`
    : ''

  return {
    pct,
    pctDisplay,
    factor: rule.maxPct,
    maxExposure,
    rangeLabel: effectiveRangeLabel,
    modelRangeLabel: rule.rangeLabel,
    level: rule.level,
    levelName: rule.name,
    modelMinPct: rule.minPct,
    modelMaxPct: rule.maxPct,
    action: rule.action,
    hint: `${rule.level ? `${toDisplayTradingLevel(rule.level)}级 ${rule.name}` : rule.name}：模型总仓位 ${rule.rangeLabel}${effectiveText}。${rule.action} · 参考非投资建议`,
  }
}
