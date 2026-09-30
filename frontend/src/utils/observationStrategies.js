/** 内置观察策略。只产生观察名单，默认不进入模拟交易计划。 */

export const OBSERVATION_QUERY_TYPE = 'observation'
export const OBSERVATION_CRON = '0 5 15 * * 1-5'

export const OBSERVATION_STRATEGIES = [
  {
    strategyId: 'ext_ma_pullback',
    name: '均线趋势回踩',
    blurb: '收盘价在过去 20 日均线之上，且近 3 日收盘价回踩均线附近。只作观察名单，不是买卖指令。K 线不足、价格失败或结果为空时跳过。',
  },
  {
    strategyId: 'ext_vol_breakout',
    name: '放量突破确认',
    blurb: '收盘价突破近 20 日最高价，且成交量不低于此前 20 日均量的 1.5 倍。只作观察名单，不是买卖指令。K 线不足、成交量失败或结果为空时跳过。',
  },
  {
    strategyId: 'ext_dd_bounce',
    name: '受控回撤反弹',
    blurb: '自 60 日高点回撤约 8%–18% 后出现反弹，且 RSI 低于 30。只作观察名单，不是买卖指令。K 线或价格不足时跳过。',
  },
]

export function parseObservationMeta(queryJson) {
  const empty = { strategyId: '', feedsTradePlan: false }
  const raw = String(queryJson || '').trim()
  if (!raw) return empty
  try {
    const parsed = JSON.parse(raw)
    return {
      strategyId: typeof parsed.strategyId === 'string' ? parsed.strategyId : '',
      feedsTradePlan: parsed.feedsTradePlan === true,
    }
  } catch {
    return empty
  }
}

export function observationStrategyIdFromRow(row) {
  if (!row) return ''
  const meta = parseObservationMeta(row.queryJson)
  if (meta.strategyId) return meta.strategyId
  const text = String(row.queryText || '').trim()
  if (OBSERVATION_STRATEGIES.some((def) => def.strategyId === text)) return text
  return ''
}

/**
 * 定时观察与纳入模拟交易计划分开保存。
 * enable 不会改 feedsTradePlan；feedsTradePlan 只有布尔 true 才写入。
 */
export function buildObservationStrategyPayload(card, row, patch = {}) {
  const enable = patch.enable === true
  const feedsTradePlan = patch.feedsTradePlan === true
  const existingCron = String(row?.cronExpr || '').trim()
  return {
    id: row?.id || 0,
    name: card.name,
    queryType: OBSERVATION_QUERY_TYPE,
    queryText: card.strategyId,
    queryJson: JSON.stringify({
      strategyId: card.strategyId,
      feedsTradePlan,
      observationOnly: true,
    }),
    keyword: '',
    industry: '',
    cronExpr: enable ? existingCron || OBSERVATION_CRON : '',
    enable,
    pageSize: Number(row?.pageSize) > 0 ? Number(row.pageSize) : 40,
    description: card.blurb,
  }
}
