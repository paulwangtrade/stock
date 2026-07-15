/** 量化自动化：账户风控、告警、扫描参数（持久化在 signalParams.automation） */

export const DEFAULT_QUANT_AUTOMATION = {
  enabled: false,
  /** 账户总权益（元），用于仓位计算 */
  accountEquity: 500000,
  /** 单笔最大风险占权益比例 */
  riskPerTradePct: 0.01,
  /** 单票最大仓位占权益比例 */
  maxPositionPct: 0.15,
  /** 组合最大总敞口占权益比例（与 5 级模型取更严） */
  maxTotalExposurePct: 0.85,
  /** 级别 ≤2 时禁止新开仓/确认买入草稿 */
  blockNewEntriesOnDefense: true,
  /** 确认买入草稿前要求清单就绪 */
  requireChecklistForBuyConfirm: true,
  /** A 股最小交易单位 */
  minLotSize: 100,
  /** 信号置信度 → 仓位系数 */
  tagConfidence: {
    强: 1.0,
    突: 0.85,
    趋: 0.75,
    转: 0.65,
    弹: 0.65,
    买: 0.5,
  },
  alerts: {
    signalNew: true,
    zoneTouch: true,
    zoneLeave: false,
    sellSignal: true,
    sellDraft: true,
    checklistReady: true,
    positionPlan: true,
  },
  /** 同类告警冷却（分钟） */
  alertCooldownMinutes: 15,
  /** 区间触达容差 */
  zoneTouchTolerancePct: 0.003,
  /** 信号扫描间隔（分钟） */
  scanIntervalMinutes: 20,
  /** 清单就绪最低得分（0~1） */
  checklistReadyScore: 0.85,
  /** 清单必须项全部通过才推送 */
  checklistRequireRequired: true,
}

export const QUANT_AUTOMATION_FIELDS = [
  { key: 'enabled', label: '启用量化自动化', type: 'bool' },
  { key: 'accountEquity', label: '账户总权益', type: 'number', min: 10000, max: 50000000, step: 10000, suffix: '元' },
  { key: 'riskPerTradePct', label: '单笔风险', type: 'number', min: 0.002, max: 0.05, step: 0.001, scale: 100, suffix: '%' },
  { key: 'maxPositionPct', label: '单票上限', type: 'number', min: 0.05, max: 0.5, step: 0.01, scale: 100, suffix: '%' },
  { key: 'maxTotalExposurePct', label: '总敞口上限', type: 'number', min: 0.3, max: 1, step: 0.05, scale: 100, suffix: '%' },
  { key: 'blockNewEntriesOnDefense', label: '1/2级禁止新开仓', type: 'bool' },
  { key: 'requireChecklistForBuyConfirm', label: '买入须清单就绪', type: 'bool' },
  { key: 'scanIntervalMinutes', label: '信号扫描间隔', type: 'int', min: 5, max: 120, step: 5, suffix: '分钟' },
  { key: 'alertCooldownMinutes', label: '告警冷却', type: 'int', min: 5, max: 120, step: 5, suffix: '分钟' },
  { key: 'checklistReadyScore', label: '清单就绪得分', type: 'number', min: 0.6, max: 1, step: 0.05 },
]

export function mergeQuantAutomation(raw) {
  const base = JSON.parse(JSON.stringify(DEFAULT_QUANT_AUTOMATION))
  if (!raw || typeof raw !== 'object') return base
  Object.assign(base, raw)
  if (raw.tagConfidence) base.tagConfidence = { ...base.tagConfidence, ...raw.tagConfidence }
  if (raw.alerts) base.alerts = { ...base.alerts, ...raw.alerts }
  return base
}

export function getQuantAutomationFromSettings(settings) {
  return mergeQuantAutomation(settings?.automation)
}
