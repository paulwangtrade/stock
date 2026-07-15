/** 纪律化交易台：风控硬门禁（参考，非投资建议） */

import { getQuantAutomationFromSettings } from './quantAutomationSettings'
import { signalSettingsState } from './signalSettingsStore'
import { isNewEntryBlockedLevel, resolveTradingLevel, toDisplayTradingLevel } from './tradingLevelRules'

/**
 * @param {{ marketModeKey?: string, checklist?: object, direction?: string, automation?: object }} ctx
 * @returns {{ ok: boolean, reason?: string, code?: string }}
 */
export function evaluateTradeRiskGate(ctx = {}) {
  const auto = ctx.automation || getQuantAutomationFromSettings(signalSettingsState.value)
  const direction = ctx.direction || '买入'
  const level = resolveTradingLevel(ctx.marketModeKey)

  if (direction === '买入' && auto.blockNewEntriesOnDefense !== false) {
    if (isNewEntryBlockedLevel(level.level)) {
      return {
        ok: false,
        code: 'defense_block',
        reason: `当前大盘 ${toDisplayTradingLevel(level.level)}级 ${level.name}，已禁止新开仓（可在量化自动化设置中关闭）`,
      }
    }
  }

  if (direction === '买入' && auto.requireChecklistForBuyConfirm !== false) {
    const ck = ctx.checklist
    if (ck && ck.ready === false) {
      return {
        ok: false,
        code: 'checklist_block',
        reason: `买入清单未就绪（得分 ${Math.round((ck.score || 0) * 100)}%），请完成必选项后再确认`,
      }
    }
  }

  return { ok: true }
}

export function canOpenNewPosition(marketModeKey, automation) {
  return evaluateTradeRiskGate({ marketModeKey, direction: '买入', automation }).ok
}
