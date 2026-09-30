/**
 * 多策略对照：把已经接入的日 K 信号扫描引擎并排读同一只股票。
 * 不写快照、不改策略 enable、不生成交易计划，也不把命中行数收成建议。
 */
import { buildEntryMissObservation } from './entryMissObservation.js'
import {
  computeFullSignals,
  findRecentSignalBar,
  normalizeDayKey,
  summarizeBuySignal,
} from './icePointSignals.js'
import { resolveStrategyRole } from './multiStrategyRole.js'
import { buildSignalOptions, getScreenStrategies } from './signalSettings.js'
import { STRATEGY_PRESETS } from './technicalIndicators.js'

export const COMPARE_FOOTER_TEXT = '仅观察对照 · 不进入模拟交易计划 · 票数不等于交易信号'

export const UNWIRED_REASON = '不能在本地对单只股票求值，未接入对照'

/** 预设没有自己的求值时失败关闭，禁止借用别的策略命中文案。 */
export const PRESET_UNIMPLEMENTED_REASON = '未实现独立求值'

/**
 * 设置页会丢掉 scanKind。这些 id 不是冰点参数预设，不能走日 K 主信号。
 * 截面动量 / 均线趋势 / 突破观察等若未在对照里单独求值，一律失败关闭。
 */
const KNOWN_PRESET_SCAN_KIND = {
  default: 'ice',
  ext_xsmom_v1: 'xsmom',
  ext_ma_trend_v1: 'ma_trend',
  ext_breakout_v1: 'breakout',
  ext_vol_mom_v1: 'vol_mom',
  ext_ma_pullback_v1: 'ma_pullback',
  ext_meanrev_watch_v1: 'meanrev',
}

const UNIMPLEMENTED_PRESET_IDS = new Set([
  'ext_xsmom_v1',
  'ext_ma_trend_v1',
  'ext_breakout_v1',
  'ext_vol_mom_v1',
  'ext_ma_pullback_v1',
  'ext_meanrev_watch_v1',
])

const UNIMPLEMENTED_SCAN_KINDS = new Set([
  'xsmom',
  'ma_trend',
  'breakout',
  'vol_mom',
  'ma_pullback',
  'meanrev',
])

const PLANNED_PRESET_IDS = new Set(['ext_vol_mom_v1', 'ext_ma_pullback_v1', 'ext_meanrev_watch_v1'])

const ICE_SCAN_KINDS = new Set(['', 'ice', 'ice_point'])
const ICE_TEMPLATE_IDS = new Set(['', 'ice_point', 'default'])

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

function presetOptions(engine, fallback) {
  if (engine.settings) return scanReadOptions(buildSignalOptions(engine.settings))
  return scanReadOptions(fallback)
}

const FAMILY_BY_ID = Object.fromEntries(WIRED_SCAN_ENGINES.map((item) => [item.strategyId, item]))

export function bareStrategyId(strategyId) {
  const id = String(strategyId || '').trim()
  return id.startsWith('preset:') ? id.slice('preset:'.length) : id
}

function presetLabel(engine) {
  const name = String(engine?.name || '').trim()
  if (name) return name
  return bareStrategyId(engine?.strategyId) || '该预设'
}

function withQuoteRsi(reason, rsi) {
  const formatted = formatRsi(rsi)
  if (formatted === '—') return reason
  return `${reason} · RSI ${formatted}`
}

function appendQuotedRsi(reason, familyReason) {
  const match = String(familyReason || '').match(/RSI\s+(\d+(?:\.\d+)?)/)
  if (!match) return reason
  return `${reason} · RSI ${match[1]}`
}

/** 别的策略的命中句。只检查预设名之后的规则正文，避免预设名本身撞上这些词。 */
function usesForeignHitPhrase(reason, label) {
  const text = String(reason || '')
  const body = label && text.startsWith(label) ? text.slice(label.length) : text
  return /今日转势买点|今日强化买点|今日趋势买点|今日平台突破|今日卖后站稳|今日出冰点买点|今日减|转势买点|强化买点|趋势买点|平台突破|卖后站稳|出冰点买点|破 MA20|RSI 回落/.test(body)
}

/**
 * ice：用该预设自己的参数做主信号，理由必须带预设名。
 * shared：scanKind / templateId 明确指向某条已接入引擎，结论可以相同，理由必须写「与{源}同源」。
 * unimplemented：没有独立求值，失败关闭，不抄任何命中句。
 */
export function resolvePresetEvaluation(engine) {
  const id = bareStrategyId(engine?.strategyId)
  const scanKind = String(engine?.scanKind || KNOWN_PRESET_SCAN_KIND[id] || '').trim()
  const templateId = String(engine?.templateId || '').trim()
  if (engine?.engineStatus === 'planned' || PLANNED_PRESET_IDS.has(id)) {
    return { mode: 'unimplemented', source: null }
  }
  if (UNIMPLEMENTED_PRESET_IDS.has(id) || UNIMPLEMENTED_PRESET_IDS.has(templateId)) {
    return { mode: 'unimplemented', source: null }
  }
  if (UNIMPLEMENTED_SCAN_KINDS.has(scanKind) || UNIMPLEMENTED_SCAN_KINDS.has(templateId)) {
    return { mode: 'unimplemented', source: null }
  }
  const shared = FAMILY_BY_ID[scanKind] || FAMILY_BY_ID[templateId] || null
  if (shared) return { mode: 'shared', source: shared }
  if ((scanKind && !ICE_SCAN_KINDS.has(scanKind)) || (templateId && !ICE_TEMPLATE_IDS.has(templateId))) {
    return { mode: 'unimplemented', source: null }
  }
  return { mode: 'ice', source: null }
}

function concludeIcePreset(engine, summary) {
  const label = presetLabel(engine)
  const tag = String(summary?.tag || '').trim()
  const rsi = summary?.latestStatus?.rsi
  if (!tag) {
    return {
      verdict: '未命中',
      bias: '—',
      reason: withQuoteRsi(`${label} · 本预设规则未触发`, rsi),
    }
  }
  const reason = withQuoteRsi(`${label} · 本预设规则触发（${tag}）`, rsi)
  if (usesForeignHitPhrase(reason, label)) {
    return { verdict: '数据不足', bias: '—', reason: PRESET_UNIMPLEMENTED_REASON }
  }
  return {
    verdict: '命中',
    bias: biasForTag(tag),
    reason,
  }
}

function labelSharedPreset(engine, source, familyRead) {
  const label = presetLabel(engine)
  const sourceName = source?.name || source?.strategyId || '源策略'
  const head = `与${sourceName}同源 · ${label}`
  if (!familyRead) {
    return { verdict: '数据不足', bias: '—', reason: `${head} · 未取到源规则读数` }
  }
  if (familyRead.verdict === '命中') {
    const reason = appendQuotedRsi(head, familyRead.reason)
    return {
      verdict: '命中',
      bias: familyRead.bias || '—',
      reason,
    }
  }
  return {
    verdict: familyRead.verdict || '未命中',
    bias: '—',
    reason: `${head} · 源规则未触发`,
  }
}

/**
 * 预设结论。summary.statusText 是日 K 主信号的展示句，这里故意不读，避免把「今日转势买点」贴到别的策略上。
 */
export function concludePresetObservation(engine, input = {}) {
  const decision = resolvePresetEvaluation(engine)
  if (decision.mode === 'unimplemented') {
    return { verdict: '数据不足', bias: '—', reason: PRESET_UNIMPLEMENTED_REASON }
  }
  if (decision.mode === 'shared') {
    return labelSharedPreset(engine, decision.source, input.familyRead || null)
  }
  return concludeIcePreset(engine, input.summary)
}

function catalogPresetMeta(item) {
  const id = String(item?.id || '').trim()
  const scanKind = String(item?.scanKind || KNOWN_PRESET_SCAN_KIND[id] || '').trim()
  let templateId = String(item?.templateId || '').trim()
  if (!templateId && id === 'default') templateId = 'ice_point'
  if (!templateId && KNOWN_PRESET_SCAN_KIND[id] && id !== 'default') templateId = id
  const engineStatus = item?.engineStatus === 'planned' || PLANNED_PRESET_IDS.has(id) ? 'planned' : String(item?.engineStatus || '')
  return { scanKind, templateId, engineStatus }
}

/**
 * 设置里的信号参数预设。冰点参数预设按自己的阈值求值。
 * 其他 scanKind 没有独立求值时不会借用冰点主信号文案。
 */
export function presetEnginesFromSettings(settings) {
  return getScreenStrategies(settings).map((item) => {
    const meta = catalogPresetMeta(item)
    return {
      strategyId: `preset:${item.id}`,
      name: item.name,
      kind: 'scan_preset',
      wired: true,
      settings: item.settings,
      scanKind: meta.scanKind,
      templateId: meta.templateId,
      engineStatus: meta.engineStatus,
    }
  })
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
  const needFamily = selected.some((item) => {
    if (!item?.wired) return false
    if (item.kind === 'scan_family') return true
    return item.kind === 'scan_preset' && resolvePresetEvaluation(item).mode === 'shared'
  })
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

  const rows = selected.map((engine) => {
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
      const decision = resolvePresetEvaluation(engine)
      if (decision.mode === 'unimplemented') {
        return emptyRow(engine, { ...concludePresetObservation(engine), dataDay })
      }
      if (decision.mode === 'shared') {
        const need = minBarsFor(decision.source, activeOptions)
        if (closes.length < need || !familySig) {
          const sourceName = decision.source.name || decision.source.strategyId
          return emptyRow(engine, {
            verdict: '数据不足',
            bias: '—',
            reason: `与${sourceName}同源 · ${presetLabel(engine)} · 日 K 不足，无法计算`,
            dataDay,
          })
        }
        const familyRead = readFamily(decision.source, familySig, last, activeOptions)
        return emptyRow(engine, { ...concludePresetObservation(engine, { familyRead }), dataDay })
      }
      if (closes.length < 20) {
        return emptyRow(engine, {
          verdict: '数据不足',
          bias: '—',
          reason: `${presetLabel(engine)} · 日 K 不足，无法计算`,
          dataDay,
        })
      }
      const options = presetOptions(engine, activeOptions)
      const summary = summarizeBuySignal(bars, { ...options, signalLastIndex: last })
      return emptyRow(engine, { ...concludePresetObservation(engine, { summary }), dataDay })
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

  return rows.map((row, index) => {
    const engine = selected[index]
    const spec = missSpec(engine)
    return {
      ...row,
      entryMiss: buildEntryMissObservation({
        roleId: spec.roleId,
        tag: spec.tag,
        composite: spec.composite,
        bars: noBars ? null : bars,
        signalOptions: optionsForMiss(engine, activeOptions),
        familySig: spec.sigSource === 'family' ? familySig : null,
        verdict: row.verdict,
        lookbackDays: activeSignalOptions?.entryMissLookbackDays,
      }),
    }
  })
}

function missSpec(engine) {
  const roleId = resolveStrategyRole(engine?.strategyId, engine || {}).id
  if (engine?.kind !== 'scan_preset') {
    return { roleId, tag: engine?.tag || '', composite: false, sigSource: 'family' }
  }
  const decision = resolvePresetEvaluation(engine)
  if (decision.mode === 'shared') {
    return { roleId, tag: decision.source?.tag || '', composite: false, sigSource: 'family' }
  }
  if (decision.mode === 'ice') {
    return { roleId, tag: '', composite: true, sigSource: 'preset' }
  }
  return { roleId, tag: '', composite: false, sigSource: 'none' }
}

function optionsForMiss(engine, activeOptions) {
  if (engine?.kind === 'scan_preset' && resolvePresetEvaluation(engine).mode === 'ice') {
    return presetOptions(engine, activeOptions)
  }
  return activeOptions
}
