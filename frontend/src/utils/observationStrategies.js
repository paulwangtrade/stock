/** 内置观察策略。只产生观察名单，默认不进入模拟交易计划。 */

export const OBSERVATION_QUERY_TYPE = 'observation'
export const OBSERVATION_CRON = '0 5 15 * * 1-5'

export const OBSERVATION_STRATEGIES = [
  {
    strategyId: 'ext_ma_pullback',
    name: '均线趋势回踩',
    blurb: '收盘在抬头的20日均线之上；近3日低点贴近均线后以阳线收回（有开盘价则收盘>开盘，否则收盘>前收）。只作观察名单，不是买卖指令。K线不足、价格无效或均线未抬头时跳过。',
  },
  {
    strategyId: 'ext_vol_breakout',
    name: '放量突破确认',
    blurb: '收盘价突破此前20日最高价，且成交量不低于此前20日均量的1.5倍。只作观察名单，不是买卖指令。成交量缺失或为0、K线不足时跳过。',
  },
  {
    strategyId: 'ext_dd_bounce',
    name: '受控回撤反弹',
    blurb: '自近20日高点回撤约8%–18%后出现阳线反弹；不是60日新低，且RSI不低于30（避开冰点式新低/超卖）。只作观察名单，不是买卖指令。K线或价格不足时跳过。',
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
