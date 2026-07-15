/** 股票交易实战 5 级模型的纯规则：1=最防守，5=最进攻。 */

export const LEVEL_SEMANTICS_VERSION = 2

export const MARKET_LEVEL_IMPACT = Object.freeze({
  1: -35,
  2: -18,
  3: -5,
  4: 4,
  5: 10,
})

export function isNewEntryBlockedLevel(level) {
  if (level == null || level === '') return false
  const n = Number(level)
  return Number.isFinite(n) && n >= 1 && n <= 2
}

export const TRADING_LEVELS = {
  level1: {
    key: 'level1', level: 1, name: '空仓防守', minPct: 0, maxPct: 0, rangeLabel: '0%',
    action: '清空风险仓位，不抄底，至少观察三个交易日',
  },
  level2: {
    key: 'level2', level: 2, name: '战略撤退', minPct: 0, maxPct: 0.1, rangeLabel: '0%-10%',
    action: '逢反弹减仓，只留底仓，不再补仓',
  },
  level3: {
    key: 'level3', level: 3, name: '中性观望', minPct: 0, maxPct: 0.2, rangeLabel: '0%-20%',
    action: '原则上不开新仓，仅保留观察仓或空仓',
  },
  level4: {
    key: 'level4', level: 4, name: '轻仓试探', minPct: 0.4, maxPct: 0.6, rangeLabel: '40%-60%',
    action: '只做强势回踩或确认突破，次日不及预期及时退出',
  },
  level5: {
    key: 'level5', level: 5, name: '重仓攻击', minPct: 0.8, maxPct: 1, rangeLabel: '80%-100%',
    action: '主线明确时进攻，分批建仓，不在尾盘追高',
  },
  unknown: {
    key: 'unknown', level: null, name: '数据不足', minPct: 0, maxPct: 0.2, rangeLabel: '不高于20%',
    action: '关键数据不足时按观望级处理',
  },
}

const LEGACY_LEVEL_MAP = { attack: 'level5', neutral: 'level3', defense: 'level2' }

/** 语义版本 2 中内部与展示恒等：1=最防守，5=最进攻。 */
export function toDisplayTradingLevel(level) {
  const n = Number(level)
  return Number.isFinite(n) && n >= 1 && n <= 5 ? n : null
}

export function formatDisplayTradingLevel(level, name = '') {
  const display = toDisplayTradingLevel(level)
  return display == null ? (name || '数据不足') : `${display}级 ${name}`.trim()
}

export function resolveTradingLevel(modeKey) {
  const key = LEGACY_LEVEL_MAP[modeKey] || modeKey || 'unknown'
  return TRADING_LEVELS[key] || TRADING_LEVELS.unknown
}

export const MARKET_MODE_EXPOSURE_FACTOR = Object.fromEntries(
  Object.entries(TRADING_LEVELS).map(([key, row]) => [key, row.maxPct]),
)
Object.assign(MARKET_MODE_EXPOSURE_FACTOR, {
  attack: TRADING_LEVELS.level5.maxPct,
  neutral: TRADING_LEVELS.level3.maxPct,
  defense: TRADING_LEVELS.level2.maxPct,
})

export function calculateModelPositionCap(modeKey, maxExposure = 0.85) {
  const rule = resolveTradingLevel(modeKey)
  const configuredMax = Number.isFinite(Number(maxExposure)) ? Number(maxExposure) : 0.85
  return { rule, maxExposure: configuredMax, pct: Math.min(configuredMax, rule.maxPct) }
}

/** 用指数均线和量能做代理条件；无法稳定取得的主线、涨跌家数不参与硬判定。 */
export function resolveMarketMode(metrics = {}) {
  const { close, ma5, ma10, ma20, ma60, volumeRatio, volumeExpanding, ma20Rising } = metrics
  const required = [close, ma5, ma10, ma20, ma60]
  if (required.some((v) => !Number.isFinite(v) || v <= 0)) {
    return {
      key: 'unknown', level: null, name: '数据不足', label: '数据不足',
      reason: 'MA5/MA10/MA20/MA60 数据不足，按观望级控制仓位',
    }
  }

  const aboveAll = close >= Math.max(ma5, ma10, ma20, ma60)
  const bullishStack = ma5 >= ma10 && ma10 >= ma20
  const volumeConfirmed = Number.isFinite(volumeRatio) && volumeRatio >= 1.05 && volumeExpanding

  if (aboveAll && bullishStack && ma20Rising && volumeConfirmed) {
    return {
      key: 'level5', level: 5, name: '重仓攻击', label: formatDisplayTradingLevel(5, '重仓攻击'),
      reason: `站上四条均线，多头排列，量比约 ${volumeRatio.toFixed(2)}`,
    }
  }
  if (close < ma60) {
    return {
      key: 'level1', level: 1, name: '空仓防守', label: formatDisplayTradingLevel(1, '空仓防守'),
      reason: '指数跌破 MA60 生命线',
    }
  }
  if (close > ma20 && close > ma60) {
    return {
      key: 'level4', level: 4, name: '轻仓试探', label: formatDisplayTradingLevel(4, '轻仓试探'),
      reason: '指数位于 MA20、MA60 上方，但量价或均线排列尚未满足5级',
    }
  }
  if (close < ma20 && ma5 < ma10) {
    return {
      key: 'level2', level: 2, name: '战略撤退', label: formatDisplayTradingLevel(2, '战略撤退'),
      reason: '指数跌破 MA20，且 MA5 死叉 MA10',
    }
  }
  return {
    key: 'level3', level: 3, name: '中性观望', label: formatDisplayTradingLevel(3, '中性观望'),
    reason: '指数在 MA20、MA60 区域纠缠，或破位条件尚未形成共振',
  }
}
