/** 与「股票信息筛选」一致的技术面字段，供策略与筛选共用 */

export function defaultTechnicalIndicators() {
  return {
    MACD_GOLDEN_FORK: false,
    KDJ_GOLDEN_FORK: false,
    BREAK_THROUGH: false,
    LOW_FUNDS_INFLOW: false,
    HIGH_FUNDS_OUTFLOW: false,
    BREAKUP_MA_5DAYS: false,
    LONG_AVG_ARRAY: false,
    SHORT_AVG_ARRAY: false,
    UPPER_LARGE_VOLUME: false,
    DOWN_NARROW_VOLUME: false,
    ONE_DAYANG_LINE: false,
    TWO_DAYANG_LINES: false,
    RISE_SUN: false,
    POWER_FULGUN: false,
    RESTORE_JUSTICE: false,
    DOWN_7DAYS: false,
    UPPER_8DAYS: false,
    UPPER_9DAYS: false,
    UPPER_4DAYS: false,
    HEAVEN_RULE: false,
    UPSIDE_VOLUME: false,
    BEARISH_ENGULFING: false,
    REVERSING_HAMMER: false,
    SHOOTING_STAR: false,
    EVENING_STAR: false,
    FIRST_DAWN: false,
    PREGNANT: false,
    BLACK_CLOUD_TOPS: false,
    MORNING_STAR: false,
    NARROW_FINISH: false,
    UPP_DAYS: 0,
    CONCERN_RANK_7DAYS: 0,
    UPNDAY: 0,
    DOWNNDAY: 0,
  }
}

export function resetTechnicalIndicators(target) {
  const d = defaultTechnicalIndicators()
  Object.keys(d).forEach((k) => {
    target[k] = d[k]
  })
}

export function applyTechnicalIndicators(target, source) {
  if (!source || typeof source !== 'object') return
  const d = defaultTechnicalIndicators()
  Object.keys(d).forEach((k) => {
    if (source[k] !== undefined && source[k] !== null) {
      target[k] = source[k]
    }
  })
}

export function hasActiveTechnicalIndicator(ind) {
  const d = defaultTechnicalIndicators()
  for (const k of Object.keys(d)) {
    if (typeof d[k] === 'boolean' && ind[k]) return true
    if (typeof d[k] === 'number' && Number(ind[k]) > 0) return true
  }
  return false
}

/** 冰点超跌类策略推荐的自然语言条件 */
export const ICE_POINT_NL_QUERY =
  'RSI小于30；60日新低；非ST；成交额大于5000万'

/** 冰点观察池（略放宽 RSI，作备选） */
export const ICE_POINT_WATCH_NL_QUERY =
  'RSI小于35；非ST；成交额大于3000万'

/** 策略模板（我的策略 → 一键创建） */
export const STRATEGY_PRESETS = [
  {
    id: 'ice_buy',
    name: '冰点超跌·出坑买点',
    queryType: 'eastmoney_nl',
    queryText: ICE_POINT_NL_QUERY,
    description:
      '东财筛选 RSI<30 且 60 日新低；运行后自动扫描 K 线「冰/买」标记，勾选「仅显示买点候选」即可选股。',
    pageSize: 50,
    cronExpr: '0 35 9 * * 1-5',
    enable: true,
  },
  {
    id: 'ice_watch',
    name: '冰点预警观察池',
    queryType: 'eastmoney_nl',
    queryText: ICE_POINT_WATCH_NL_QUERY,
    description: 'RSI<35 的观察池，配合 K 线扫描找即将出冰点的标的。',
    pageSize: 80,
    cronExpr: '',
    enable: false,
  },
  {
    id: 'ice_technical',
    name: '冰点形态·技术面',
    queryType: 'technical',
    description: '连跌、空头排列、缩量、倒锤头等超跌技术形态。',
    pageSize: 50,
    cronExpr: '0 5 15 * * 1-5',
    enable: false,
    applyTechnical: applyIcePointTechnicalPreset,
  },
]

/** 技术面「超跌」常用组合 */
export function applyIcePointTechnicalPreset(target) {
  resetTechnicalIndicators(target)
  target.DOWN_7DAYS = true
  target.SHORT_AVG_ARRAY = true
  target.DOWN_NARROW_VOLUME = true
  target.DOWNNDAY = 5
  target.REVERSING_HAMMER = true
}
