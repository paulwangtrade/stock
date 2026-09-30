/**
 * 多策略对照：把已经接入的日 K 信号扫描引擎并排读同一只股票。
 * 不写快照、不改策略 enable、不生成交易计划，也不把命中行数收成建议。
 */
import {
  computeFullSignals,
  findRecentSignalBar,
  normalizeDayKey,
  summarizeBuySignal,
} from './icePointSignals.js'
import { buildSignalOptions, getScreenStrategies } from './signalSettings.js'
import { STRATEGY_PRESETS } from './technicalIndicators.js'

export const COMPARE_FOOTER_TEXT = '仅观察对照 · 不进入模拟交易计划 · 票数不等于交易信号'

export const UNWIRED_REASON = '不能在本地对单只股票求值，未接入对照'

/** 信号扫描包里已经会算、并能对单票日 K 求值的观察引擎。 */
export const WIRED_SCAN_ENGINES = [
  { strategyId: 'scan_strong_v1', name: '强化买点', tag: '强', kind: 'scan_family', wired: true },
  { strategyId: 'scan_trend_v1', name: '趋势买点', tag: '趋', kind: 'scan_family', wired: true },
  { strategyId: 'scan_reversal_v1', name: '转势买点', tag: '转', kind: 'scan_family', wired: true },
  { strategyId: 'scan_breakout_v1', name: '平台突破', tag: '突', kind: 'scan_family', wired: true },
  { strategyId: 'scan_rebound_v1', name: '卖后站稳', tag: '弹', kind: 'scan_family', wired: true },
  { strategyId: 'scan_ice_buy_v1', name: '出冰点买点', tag: '买', kind: 'scan_family', wired: true },
  { strategyId: 'scan_reduce_v1', name: '破位减仓观察', tag: '减', kind: 'scan_family', wired: true },
  { strategyId: 'scan_take_profit_v1', name: '止盈观察', tag: '止', kind: 'scan_family', wired: true },
  { strategyId: 'scan_ice_v1', name: '冰点区', tag: '冰', kind: 'scan_family', wired: true },
]

const BULLISH_TAGS = new Set(['强', '趋', '转', '突', '弹', '买'])
const BEARISH_TAGS = new Set(['减', '止'])

const HIT_REASON = {
  强: (rsi) => `强化买点 · RSI ${rsi}`,
  趋: (rsi, options) => `趋势买点${options?.trendRequireNextDayYang ? ' · 次日确认' : ''} · RSI ${rsi}`,
  转: (rsi) => `转势买点 · RSI ${rsi}`,
  突: (rsi) => `平台突破 · RSI ${rsi}`,
  弹: (rsi) => `卖后站稳MA20 · RSI ${rsi}`,
  买: (rsi) => `出冰点买点 · RSI ${rsi}`,
  减: (rsi) => `破 MA20 · RSI ${rsi}`,
  止: (rsi) => `RSI 回落 · RSI ${rsi}`,
}

function scanReadOptions(options) {
  return {
    ...(options || {}),
    recentBuyDays: 0,
    recentSellDays: 0,
    includeSell: true,
  }
}

function formatRsi(rsi) {
  return Number.isFinite(Number(rsi)) ? Number(rsi).toFixed(1) : '—'
}

export function biasForTag(tag) {
  if (BULLISH_TAGS.has(tag)) return '偏多'
  if (BEARISH_TAGS.has(tag)) return '偏空'
  if (tag === '冰') return '中性'
  return '—'
}

function emptyRow(engine, patch) {
  return {
    strategyId: engine.strategyId,
    strategyName: engine.name,
    verdict: '未命中',
    bias: '—',
    reason: '',
    dataDay: '',
    ...patch,
  }
}

function minBarsFor(engine, options) {
  if (engine.tag === '转') {
    return Math.max(20, Math.floor(Number(options?.reversalCrossMaPeriod) || 60) + 1)
  }
  return 20
}

function familyMissReason(engine, options, rsi) {
  if (engine.tag === '转' && options?.reversalEnabled === false) {
    return '当前参数未启用转势信号'
  }
  const rsiNote = Number.isFinite(Number(rsi)) ? ` · RSI ${formatRsi(rsi)}` : ''
  return `最新数据日未形成${engine.name}${rsiNote}`
}

function readFamily(engine, sig, last, options) {
  const rsi = sig?.latestStatus?.rsi
  if (engine.tag === '冰') {
    const threshold = Number(options?.iceThreshold ?? 30)
    const inIce = Number.isFinite(Number(rsi)) && Number(rsi) < threshold
    if (!inIce) {
      return {
        verdict: '未命中',
        bias: '—',
        reason: familyMissReason(engine, options, rsi),
      }
    }
    return {
      verdict: '命中',
      bias: '中性',
      reason: `冰点区内 · RSI ${formatRsi(rsi)}`,
    }
  }
  const hit = findRecentSignalBar(sig, last, engine.tag, options)
  if (!hit || hit.daysAgo !== 0) {
    return {
      verdict: '未命中',
      bias: '—',
      reason: familyMissReason(engine, options, rsi),
    }
  }
  const phrase = HIT_REASON[engine.tag]
  return {
    verdict: '命中',
    bias: biasForTag(engine.tag),
    reason: phrase ? phrase(formatRsi(rsi), options) : engine.name,
  }
}

function readPreset(summary) {
  const tag = summary?.tag || ''
  if (!tag) {
    const text = String(summary?.latestStatus?.text || summary?.statusText || '').trim()
    return {
      verdict: '未命中',
      bias: '—',
      reason: text && text !== '—' ? text : '最新数据日无观察信号',
    }
  }
  return {
    verdict: '命中',
    bias: biasForTag(tag),
    reason: String(summary.statusText || '').trim() || tag,
  }
}

function presetOptions(engine, fallback) {
  if (engine.settings) return scanReadOptions(buildSignalOptions(engine.settings))
  return scanReadOptions(fallback)
}

/**
 * 设置里的信号参数预设。它们走同一套日 K 扫描，只是参数不同，可以本地求值。
 */
export function presetEnginesFromSettings(settings) {
  return getScreenStrategies(settings).map((item) => ({
    strategyId: `preset:${item.id}`,
    name: item.name,
    kind: 'scan_preset',
    wired: true,
    settings: item.settings,
    // 只给角色标签辨认同源预设；不参与求值。
    scanKind: item.scanKind || '',
    templateId: item.templateId || '',
  }))
}

/**
 * 东财自然语言 / 技术面选股包：只能扫股票池，不能对单票本地求值。
 * 模板和已保存策略都列出来，默认不勾选；若被勾选，结果固定为未接入。
 */
export function unwiredPackEngines(stockStrategies) {
  const templates = (STRATEGY_PRESETS || []).map((item) => ({
    strategyId: `pack:${item.id}`,
    name: item.name,
    kind: 'unwired',
    wired: false,
    queryType: item.queryType || '',
  }))
  const saved = (Array.isArray(stockStrategies) ? stockStrategies : []).map((item) => ({
    strategyId: `ss:${item.id}`,
    name: String(item.name || `策略${item.id}`),
    kind: 'unwired',
    wired: false,
    queryType: item.queryType || item.query_type || '',
  }))
  return [...templates, ...saved.filter((item) => item.strategyId && item.name)]
}

export function buildStrategyCatalog({ settings, stockStrategies } = {}) {
  return [
    ...WIRED_SCAN_ENGINES.map((item) => ({ ...item })),
    ...presetEnginesFromSettings(settings),
    ...unwiredPackEngines(stockStrategies),
  ]
}

export function defaultSelectedStrategyIds(catalog) {
  return (catalog || []).filter((item) => item.wired).map((item) => item.strategyId)
}

export function klineRowsToScanBars(raw) {
  const list = Array.isArray(raw) ? raw : []
  const sorted = [...list].sort((a, b) => daySortKey(a?.Day || a?.day) - daySortKey(b?.Day || b?.day))
  const closes = []
  const opens = []
  const highs = []
  const lows = []
  const volumes = []
  const dayKeys = []
  for (const row of sorted) {
    const close = Number(row?.Close ?? row?.close)
    if (!Number.isFinite(close) || close <= 0) continue
    const open = Number(row?.Open ?? row?.open)
    const high = Number(row?.High ?? row?.high)
    const low = Number(row?.Low ?? row?.low)
    const volume = Number(row?.Volume ?? row?.volume)
    closes.push(close)
    opens.push(Number.isFinite(open) && open > 0 ? open : close)
    highs.push(Number.isFinite(high) && high > 0 ? high : close)
    lows.push(Number.isFinite(low) && low > 0 ? low : close)
    volumes.push(Number.isFinite(volume) ? volume : 0)
    dayKeys.push(normalizeDayKey(row?.Day || row?.day))
  }
  if (!closes.length) return null
  return { closes, opens, highs, lows, volumes, dayKeys, indexMa20ByDay: null }
}

function daySortKey(day) {
  const text = normalizeDayKey(day).replace(/-/g, '')
  const n = Number(text)
  return Number.isFinite(n) ? n : 0
}

export function sliceBarsToIndex(bars, lastIdx) {
  if (!bars || lastIdx == null || lastIdx < 0) return null
  const end = lastIdx + 1
  if (!bars.closes || end > bars.closes.length) return null
  return {
    closes: bars.closes.slice(0, end),
    opens: (bars.opens || []).slice(0, end),
    highs: (bars.highs || []).slice(0, end),
    lows: (bars.lows || []).slice(0, end),
    volumes: (bars.volumes || []).slice(0, end),
    dayKeys: (bars.dayKeys || []).slice(0, end),
    indexMa20ByDay: bars.indexMa20ByDay || null,
  }
}

/**
 * 对已选策略做只读对照。返回行数组，不含命中计数或方向建议。
 */
export function evaluateObservationCompare({ bars, engines, activeSignalOptions } = {}) {
  const selected = Array.isArray(engines) ? engines : []
  const activeOptions = scanReadOptions(activeSignalOptions)
  const closes = bars?.closes || []
  const last = closes.length - 1
  const dataDay = last >= 0 ? normalizeDayKey(bars?.dayKeys?.[last] || '') : ''
  const noBars = last < 0

  let familySig = null
  const needFamily = selected.some((item) => item.kind === 'scan_family' && item.wired)
  if (!noBars && needFamily && closes.length >= 20) {
    familySig = computeFullSignals(
      {
        closes,
        opens: bars.opens || [],
        highs: bars.highs || [],
        lows: bars.lows || [],
        volumes: bars.volumes || [],
        dayKeys: bars.dayKeys || [],
        indexMa20ByDay: bars.indexMa20ByDay || null,
      },
      activeOptions,
    )
  }

  return selected.map((engine) => {
    if (!engine?.wired || engine.kind === 'unwired') {
      return emptyRow(engine, {
        verdict: '未接入',
        bias: '—',
        reason: UNWIRED_REASON,
        dataDay: '',
      })
    }

    if (noBars) {
      return emptyRow(engine, {
        verdict: '数据不足',
        bias: '—',
        reason: '未取到日 K',
        dataDay: '',
      })
    }

    if (engine.kind === 'scan_preset') {
      const options = presetOptions(engine, activeOptions)
      if (closes.length < 20) {
        return emptyRow(engine, {
          verdict: '数据不足',
          bias: '—',
          reason: '日 K 不足，无法计算',
          dataDay,
        })
      }
      const summary = summarizeBuySignal(bars, { ...options, signalLastIndex: last })
      return emptyRow(engine, { ...readPreset(summary), dataDay })
    }

    const options = activeOptions
    const need = minBarsFor(engine, options)
    if (closes.length < need || !familySig) {
      return emptyRow(engine, {
        verdict: '数据不足',
        bias: '—',
        reason: closes.length < need ? '日 K 不足，无法计算' : '未取到日 K',
        dataDay: closes.length ? dataDay : '',
      })
    }
    return emptyRow(engine, { ...readFamily(engine, familySig, last, options), dataDay })
  })
}
