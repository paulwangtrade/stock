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

/** 策略模板（我的策略 → 从模板创建）。冰点出坑买点仍可能 enable，对比包不放这里。 */
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

/**
 * 对比观察包：只写入「我的策略」，供手动运行后对照名单。
 * 条件只能是东财自然语言或本地技术面勾选（与指标选股 / 股票筛选同一条链路）。
 * 全部 enable:false 且不带 Cron。交易宇宙走 GetFirstEnabled()，未启用的策略不会进候选池。
 * 截面动量、均线趋势等快照引擎留在设置页预设，不进这个包。
 */
export const COMPARISON_OBSERVATION_PRESETS = [
  {
    id: 'cmp_quality_value',
    name: '价值质量对照',
    queryType: 'eastmoney_nl',
    observationOnly: true,
    queryText:
      '市盈率大于0小于25；净资产收益率大于10%；每股收益大于0；净利润增长率大于0；非ST；不要退市股；成交额大于1亿；流通市值大于50亿小于800亿',
    description:
      '对比观察。东财同时要求低市盈率、ROE>10%、每股收益与利润增长为正，并去掉 ST 与过小成交。和放量、超跌、突破名单对照同一天的基本面篮子。导入后不启用定时；勾选启用会成为交易宇宙来源，观察时不要勾。',
    pageSize: 50,
    cronExpr: '',
    enable: false,
  },
  {
    id: 'cmp_volume_price',
    name: '放量价格观察',
    queryType: 'eastmoney_nl',
    observationOnly: true,
    queryText:
      '量比大于2；换手率大于3%小于20%；涨幅大于2%小于7%；成交额大于1亿；股价在20日均线以上；非ST；不要退市股',
    description:
      '对比观察。量比与换手放大、涨幅停在 2%–7%、收在 20 日均线之上。和价值篮、趋势突破技术包对照：只是放量，还是放量并且形态突破。不启用定时，运行结果不是委托。',
    pageSize: 40,
    cronExpr: '',
    enable: false,
  },
  {
    id: 'cmp_meanrev_ma',
    name: '超跌均线观察',
    queryType: 'eastmoney_nl',
    observationOnly: true,
    queryText:
      'RSI小于35；股价在60日均线以下；非ST；不要退市股；成交额大于5000万；换手率大于0.5%小于15%',
    description:
      '对比观察。在冰点预警（仅 RSI<35）上多加「股价在 60 日均线以下」，看超跌是否还在中期均线下方。和出坑买点、价值篮对照，不代替买点标记。不启用定时。',
    pageSize: 60,
    cronExpr: '',
    enable: false,
  },
  {
    id: 'cmp_breakout_tech',
    name: '趋势突破技术包',
    queryType: 'technical',
    observationOnly: true,
    description:
      '对比观察。本地技术面同时勾选放量突破、均线多头、MACD 金叉（几项同时成立才入选）。和东财放量价格、价值篮对照形态是否共振。不启用定时，不会写入交易计划。',
    pageSize: 50,
    cronExpr: '',
    enable: false,
    applyTechnical: applyBreakoutTechnicalPreset,
  },
  {
    id: 'cmp_pullback_tech',
    name: '缩量回踩技术包',
    queryType: 'technical',
    observationOnly: true,
    description:
      '对比观察。均线多头、下跌无量、向上突破 5 日均线同时成立，表示回踩后重新站上短均线且没有放量。和趋势突破包对照「缩量回踩」与「已经放量突破」。仅观察，不启用定时。',
    pageSize: 50,
    cronExpr: '',
    enable: false,
    applyTechnical: applyPullbackTechnicalPreset,
  },
  {
    id: 'cmp_low_funds',
    name: '低位资金回流',
    queryType: 'technical',
    observationOnly: true,
    description:
      '对比观察。低位资金净流入且 KDJ 金叉同时成立。和超跌均线、趋势突破对照：资金回流落在超跌名单里，还是落在突破名单里。不启用定时。',
    pageSize: 50,
    cronExpr: '',
    enable: false,
    applyTechnical: applyLowFundsTechnicalPreset,
  },
]

/** 写入「我的策略」的创建体。定时与 enable 固定关闭，忽略预设上的 enable。 */
export function buildComparisonObservationPayload(preset) {
  if (!preset || typeof preset !== 'object') {
    throw new Error('对比策略预设无效')
  }
  const queryType = preset.queryType
  if (queryType !== 'eastmoney_nl' && queryType !== 'technical') {
    throw new Error(`不支持的策略类型: ${queryType}`)
  }
  const technical = defaultTechnicalIndicators()
  let queryJson = ''
  let queryText = ''
  if (queryType === 'technical') {
    if (typeof preset.applyTechnical !== 'function') {
      throw new Error(`${preset.id || preset.name || '技术面策略'} 缺少技术条件`)
    }
    preset.applyTechnical(technical)
    if (!hasActiveTechnicalIndicator(technical)) {
      throw new Error(`${preset.name || preset.id} 未勾选技术条件`)
    }
    queryJson = JSON.stringify(technical)
  } else {
    queryText = String(preset.queryText || '').trim()
    if (!queryText) {
      throw new Error(`${preset.name || preset.id} 缺少选股条件`)
    }
  }
  const pageSize = Number(preset.pageSize) > 0 ? Number(preset.pageSize) : 50
  return {
    id: 0,
    name: String(preset.name || '').trim(),
    queryType,
    queryText,
    queryJson,
    keyword: '',
    industry: '',
    cronExpr: '',
    enable: false,
    pageSize,
    description: String(preset.description || ''),
  }
}

/** 技术面「超跌」常用组合 */
export function applyIcePointTechnicalPreset(target) {
  resetTechnicalIndicators(target)
  target.DOWN_7DAYS = true
  target.SHORT_AVG_ARRAY = true
  target.DOWN_NARROW_VOLUME = true
  target.DOWNNDAY = 5
  target.REVERSING_HAMMER = true
}

/** 趋势里的放量突破，并要求 MACD 金叉同时成立 */
export function applyBreakoutTechnicalPreset(target) {
  resetTechnicalIndicators(target)
  target.BREAK_THROUGH = true
  target.LONG_AVG_ARRAY = true
  target.MACD_GOLDEN_FORK = true
}

/** 多头趋势中的缩量回踩，并重新站上 5 日均线 */
export function applyPullbackTechnicalPreset(target) {
  resetTechnicalIndicators(target)
  target.LONG_AVG_ARRAY = true
  target.DOWN_NARROW_VOLUME = true
  target.BREAKUP_MA_5DAYS = true
}

/** 低位资金净流入，且 KDJ 金叉 */
export function applyLowFundsTechnicalPreset(target) {
  resetTechnicalIndicators(target)
  target.LOW_FUNDS_INFLOW = true
  target.KDJ_GOLDEN_FORK = true
}
