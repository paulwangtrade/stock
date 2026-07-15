/** 信号图例与买入时机对照（非投资建议） */

import {
  isSellSignalTag,
  SELL_TAG_REDUCE,
  SELL_TAG_TAKE_PROFIT,
  SIGNAL_PRIORITY_SUMMARY,
  SIGNAL_TAG_PRIORITY,
} from './icePointSignals'
import { formatSellPositionPct } from './sellPositionRatio'
import { formatAddPositionPct } from './addPositionRatio'
import { formatRushReducePct } from './rushReduceRatio'
import { isAddPositionTag } from './addPositionSignals'
import { isRushReduceTag } from './rushReduceSignals'
import { getReboundScreenMaxRsiValue } from './signalSettingsStore'
import {
  normalizeScreenSignalTag,
  SCREEN_SNAPSHOT_SIGNAL_TAGS,
  SCREEN_SNAPSHOT_SIGNAL_TAG_SET,
} from './signalTagConstants'

export { normalizeScreenSignalTag, SCREEN_SNAPSHOT_SIGNAL_TAGS, SCREEN_SNAPSHOT_SIGNAL_TAG_SET }

const LEGEND_BY_TAG = {
  减: '破MA20(1日)/破MA5走弱/大阴/破近端低点',
  止: 'RSI≥72停留3日回落+浮盈≥5%+自高点回撤≥4%',
  冲: '有仓浮盈·近端新高阳后首阴',
  加: '有持仓且浮盈≥3%·趋势回踩/强化/突破(距上次买点≥5日)',
  强: '强化买(2日确认)',
  趋: '趋势回踩(收盘距MA5≤3%)',
  转: '转势上穿MA60(前段多数在均线下)',
  突: '平台突破(3日确认)',
  弹: '卖后连续2日收≥MA20确认(第2日标·RSI≤60)',
  买: '出冰点收阳>MA5(MA20下须MA20不走弱·MA5≥MA10)',
  冰: '进入冰点',
}

export { SIGNAL_PRIORITY_SUMMARY, SIGNAL_TAG_PRIORITY }

/** 筛选/图例展示顺序（与 SIGNAL_PRIORITY_SUMMARY 一致；同日仍优先「减」） */
export const SIGNAL_TAG_DISPLAY_ORDER = ['止', '减', '冲', '加', '强', '趋', '转', '突', '弹', '买', '冰']

export const SIGNAL_TAG_LEGEND = [
  `优先级 ${SIGNAL_PRIORITY_SUMMARY}`,
  ...SIGNAL_TAG_DISPLAY_ORDER.map((t) => `${formatSignalTagLabel(t)}=${LEGEND_BY_TAG[t]}`),
].join(' · ')

const BUY_GUIDE_BY_TAG = {
  [SELL_TAG_TAKE_PROFIT]: {
    tag: SELL_TAG_TAKE_PROFIT,
    level: '风控',
    watch: 'RSI 在 ≥72 区连续停留后下穿，且相对最近买点浮盈≥设定值、自阶段高点已有回撤',
    buy: '—',
    avoid: '整理中的小回落不标止；趋势未破 MA20 时勿当清仓',
  },
  [SELL_TAG_REDUCE]: {
    tag: SELL_TAG_REDUCE,
    level: '风控',
    watch: '连续 N 日收盘低于 MA20，或收盘跌破最近买点当日低点',
    buy: '—',
    avoid: '结构转弱，宜减仓或收紧止损',
  },
  冲: {
    tag: '冲',
    level: '持仓早减',
    watch: '近端新高大阳后首阴；有浮盈；RSI 偏高；仍可在 MA20 上方',
    buy: '—',
    avoid: '震荡小阴、非新高阳后；亏损持仓不标',
  },
  加: {
    tag: '加',
    level: '持仓加码',
    watch: '已有持仓且浮盈；强/趋/突 回踩确认；距上次买点≥5日；近端无止/减',
    buy: '落入均线附近分批加码；总仓勿超计划上限',
    avoid: '亏损摊平；RSI 超买；破 MA20 或近端刚减/止',
  },
  强: {
    tag: '强',
    level: '保守首选',
    watch: '「强」确认日出现（买+2日不破位）',
    buy: '确认后回踩 MA5/MA10 收阳；设止损于信号低点下',
    avoid: '标强当日已大涨、追阳线实体上沿',
  },
  趋: {
    tag: '趋',
    level: '均衡',
    watch: '上升趋势中回踩 MA5/MA10 收阳；收盘相对 MA5 涨幅不超过设定上限（默认 3%）',
    buy: '趋信号日或次日，价仍在均线附近',
    avoid: '横盘均线粘合、近端有止/减；收盘远离 MA5；仅下影线碰线的大阳',
  },
  转: {
    tag: '转',
    level: '积极',
    watch: '收盘上穿 MA60（可配）；前段多数日在均线下；MA20 走平/向上；收阳且 RSI 在设定区间',
    buy: '转势确认日或次日回踩 MA5/MA20 不破再进',
    avoid: '下跌中继假突破；MA20 仍下行；无上穿仅贴近均线',
  },
  突: {
    tag: '突',
    level: '均衡',
    watch: '箱体突破且 3 日收盘站稳箱顶',
    buy: '确认完成后回踩箱顶不破 + 收阳',
    avoid: '突破当日放量长上影、假突破',
  },
  弹: {
    tag: '弹',
    level: '积极',
    watch: '近 6 日内有「止/减」；连续 2 日收盘 ≥ MA20 后第 2 日标弹；卖后曾破 MA20 或自低点反弹≥5%；RSI≤60 等',
    buy: '弹确认后回踩 MA20/MA5 收阳再进（弹为确认信号，非追弹当日）',
    avoid: '仅单日收复 MA20 即追；弹日前长上影；MA20 下行；MA5<MA10；买后跟进日；ST；RSI>60',
  },
  买: {
    tag: '买',
    level: '保守',
    watch: 'RSI 上穿 30 且收阳>MA5、实体≥1%（须先有「冰」）；MA20 下须 MA20 不走弱且 MA5≥MA10',
    buy: '出买后 1～2 日不破位，回踩再小仓试',
    avoid: '仍在冰点区提前抢、无收阳；MA20 下行中 MA5<MA10 的弱反弹',
  },
}

export const SIGNAL_BUY_GUIDE_ROWS = SIGNAL_TAG_DISPLAY_ORDER.filter((t) => t !== '冰')
  .map((t) => BUY_GUIDE_BY_TAG[t])
  .map((row) => row ? { ...row, displayTag: formatSignalTagLabel(row.tag) } : row)
  .filter(Boolean)

export function buildSignalFilterOptions(prefix = []) {
  return [
    ...prefix,
    ...SIGNAL_TAG_DISPLAY_ORDER.filter((t) => t !== '冰').map((t) => ({
      label: formatSignalTagLabel(t),
      value: t,
    })),
  ]
}

/** 股票筛选页专用：仅 强/趋/转/突/弹/买 */
export function buildScreenSignalFilterOptions() {
  return SCREEN_SNAPSHOT_SIGNAL_TAGS.map((t) => ({ label: t, value: t }))
}

/** @deprecated 请用 getReboundScreenMaxRsi() */
export const REBOUND_SCREEN_MAX_RSI = 60

export function getReboundScreenMaxRsi() {
  return getReboundScreenMaxRsiValue()
}

/** 与 K 线图标记色一致（列表/卡片标签用） */
export const SIGNAL_TAG_COLORS = {
  冰: { color: 'rgba(14, 165, 233, 0.14)', textColor: '#0284c7', borderColor: 'rgba(14, 165, 233, 0.45)' },
  买: { color: 'rgba(239, 83, 80, 0.14)', textColor: '#ef5350', borderColor: 'rgba(239, 83, 80, 0.45)' },
  强: { color: 'rgba(249, 115, 22, 0.14)', textColor: '#ea580c', borderColor: 'rgba(249, 115, 22, 0.45)' },
  趋: { color: 'rgba(168, 85, 247, 0.14)', textColor: '#9333ea', borderColor: 'rgba(168, 85, 247, 0.45)' },
  转: { color: 'rgba(99, 102, 241, 0.14)', textColor: '#6366f1', borderColor: 'rgba(99, 102, 241, 0.45)' },
  突: { color: 'rgba(234, 179, 8, 0.18)', textColor: '#ca8a04', borderColor: 'rgba(234, 179, 8, 0.5)' },
  弹: { color: 'rgba(6, 182, 212, 0.14)', textColor: '#0891b2', borderColor: 'rgba(6, 182, 212, 0.45)' },
  止: { color: 'rgba(245, 158, 11, 0.16)', textColor: '#d97706', borderColor: 'rgba(245, 158, 11, 0.45)' },
  减: { color: 'rgba(38, 166, 154, 0.14)', textColor: '#26a69a', borderColor: 'rgba(38, 166, 154, 0.45)' },
  冲: { color: 'rgba(234, 88, 12, 0.16)', textColor: '#ea580c', borderColor: 'rgba(234, 88, 12, 0.45)' },
  加: { color: 'rgba(34, 197, 94, 0.14)', textColor: '#16a34a', borderColor: 'rgba(34, 197, 94, 0.45)' },
  卖: { color: 'rgba(38, 166, 154, 0.14)', textColor: '#26a69a', borderColor: 'rgba(38, 166, 154, 0.45)' },
}

export function getSignalTagColor(tag) {
  return (
    SIGNAL_TAG_COLORS[tag] || {
      color: 'rgba(148, 163, 184, 0.12)',
      textColor: '#64748b',
      borderColor: 'rgba(148, 163, 184, 0.35)',
    }
  )
}

/** 列表/自选标签：止/减/加 附带动态仓位比例 */
export function formatSignalTagLabel(tag, ratioPct) {
  if (!tag) return ''
  const displayTag = isRushReduceTag(tag) ? '早减' : tag
  if (isSellSignalTag(tag) && ratioPct != null) {
    const pct = formatSellPositionPct(ratioPct)
    if (pct) return `${displayTag}${pct}`
  }
  if (isAddPositionTag(tag) && ratioPct != null) {
    const pct = formatAddPositionPct(ratioPct)
    if (pct) return `${displayTag}${pct}`
  }
  if (isRushReduceTag(tag) && ratioPct != null) {
    const pct = formatRushReducePct(ratioPct)
    if (pct) return `${displayTag}${pct}`
  }
  return displayTag
}

export function isStStockRow(row) {
  const name = String(
    row?.SECURITY_NAME_ABBR || row?.SECURITY_NAME || row?.name || '',
  ).toUpperCase()
  return name.includes('ST')
}

/** 筛选标签是否匹配（「卖」= 止或减） */
export function signalTagMatchesFilter(filterTag, tag) {
  if (!filterTag || !tag) return false
  filterTag = normalizeScreenSignalTag(filterTag)
  tag = normalizeScreenSignalTag(tag)
  if (filterTag === tag) return true
  if (filterTag === '卖' && isSellSignalTag(tag)) return true
  return false
}

function resolveReboundFilterMaxRsi(options = {}) {
  const raw = options?.maxRsi ?? options?.reboundMaxRsi
  const n = Number(raw)
  return Number.isFinite(n) ? n : getReboundScreenMaxRsiValue()
}

/** 「弹」筛选：默认排除 ST 与 RSI 超设置上限 */
export function passesReboundScreenFilter(summary, row, options = {}) {
  if (summary?.tag !== '弹') return true
  if (isStStockRow(row)) return false
  const rsi = summary?.latestStatus?.rsi
  const maxRsi = resolveReboundFilterMaxRsi(options)
  if (Number.isFinite(rsi) && rsi > maxRsi) return false
  return true
}

export function passesSignalTagFilter(row, summary, selectedTags, options = {}) {
  if (!summary?.tag || !selectedTags?.length) return false
  const matched = selectedTags.some((t) => signalTagMatchesFilter(t, summary.tag))
  if (!matched) return false
  return passesReboundScreenFilter(summary, row, options)
}
