/** 信号操作提示：可买 / 等回踩 / 仅观察 / 先风控（参考，非投资建议） */

import { BUY_ENTRY_TAGS, calcBuyPriceRange, formatPriceTick } from './buyPriceRange'
import { isSellSignalTag } from './icePointSignals'
import {
  calcSellPositionPct,
  formatSellPositionPct,
  sellPositionHint,
} from './sellPositionRatio'
import {
  calcAddPositionPct,
  formatAddPositionPct,
  addPositionHint,
} from './addPositionRatio'
import { isAddPositionTag } from './addPositionSignals'
import { isRushReduceTag } from './rushReduceSignals'
import {
  calcRushReducePct,
  formatRushReducePct,
  rushReduceHint,
} from './rushReduceRatio'

/**
 * @returns {{ label: string, type: string, tag?: string, lines: string[], tooltip: string, buyPriceRange?: object } | null}
 */
export function resolveSignalActionHint(ctx = {}) {
  const {
    tag,
    buyPriceRange,
    sellPositionPct,
    daysAgo = 0,
    isHistorical = false,
    checklistReady,
    holdingAdvice,
  } = ctx

  if (!tag) {
    if (holdingAdvice?.action && holdingAdvice.action !== 'hold') {
      const pct = holdingAdvice.suggestPctDisplay
      const lines = [
        `持仓辅助 · ${holdingAdvice.actionLabel}`,
        holdingAdvice.summaryLine,
        pct > 0 ? `参考比例约 ${pct}%` : '综合量价与板块，参考非投资建议',
      ]
      return {
        label: holdingAdvice.actionLabel,
        type: holdingAdvice.action === 'add' ? 'success' : holdingAdvice.action === 'reduce' ? 'error' : 'default',
        lines,
        tooltip: lines.join('\n'),
      }
    }
    return null
  }

  if (isSellSignalTag(tag)) {
    const pct = formatSellPositionPct(sellPositionPct)
    const action = tag === '减' ? '减仓' : '止盈'
    const hint = sellPositionHint(tag, sellPositionPct)
    const sellVol = ctx.sellVolume
    const lines = [
      `${tag} · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
      hint || (pct ? `建议${action} ${pct}` : `建议${action}`),
    ]
    if (sellVol > 0 && ctx.costPrice > 0) {
      lines.push(`成本 ${Number(ctx.costPrice).toFixed(2)} · 卖 ${sellVol} 股`)
    }
    if (tag === '止') lines.push('须已有浮盈且自高点回撤；趋势未破 MA20 勿当清仓')
    if (tag === '减') lines.push('收盘跌破 MA20，结构转弱')
    if (isHistorical) lines.push('历史信号，仅供对照')
    return {
      label: isHistorical ? '历史参考' : '先风控',
      type: 'error',
      tag,
      lines,
      tooltip: lines.join('\n'),
    }
  }

  if (tag === '冰') {
    return {
      label: '仅观察',
      type: 'info',
      tag,
      lines: ['冰点区 · 等待 RSI 上穿 30 出「买」', '参考，非投资建议'],
      tooltip: '冰点区 · 等待 RSI 上穿 30 出「买」\n参考，非投资建议',
    }
  }

  if (isRushReduceTag(tag)) {
    const pctVal = ctx.rushReducePct ?? ctx.sellPositionPct
    const pct = formatRushReducePct(pctVal)
    const hint = rushReduceHint(pctVal)
    const lines = [
      `早减 · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
      hint || (pct ? `建议早减 ${pct}` : '新高阳后首阴，宜提前减仓'),
      '有浮盈时早于「减」触发，参考非投资建议',
    ]
    if (isHistorical) lines.push('历史信号，仅供对照')
    return {
      label: isHistorical ? '历史参考' : '早减',
      type: 'warning',
      tag,
      lines,
      tooltip: lines.join('\n'),
    }
  }

  if (isAddPositionTag(tag)) {
    const { addPositionPct, sourceTag } = ctx
    const pct = formatAddPositionPct(addPositionPct)
    const hint = addPositionHint(addPositionPct)
    const lines = [
      `加仓${sourceTag ? `(${sourceTag})` : ''} · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
      hint || (pct ? `建议加仓 ${pct}` : '趋势未破，可考虑分批加码'),
      '须已有持仓且浮盈，参考非投资建议',
    ]
    if (isHistorical) lines.push('历史信号，仅供对照')
    return {
      label: isHistorical ? '历史参考' : '可加仓',
      type: 'success',
      tag,
      lines,
      tooltip: lines.join('\n'),
    }
  }

  if (!BUY_ENTRY_TAGS.has(tag)) return null

  const rangeText = buyPriceRange?.text || '—'
  const instant =
    buyPriceRange?.instantText ||
    (buyPriceRange?.instantPrice != null ? formatPriceTick(buyPriceRange.instantPrice) : '—')
  const lines = [
    `${tag} · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
    `出信号价 ${instant}`,
    `买入参考 ${rangeText}`,
  ]

  let label = '仅观察'
  let type = 'info'

  if (isHistorical) {
    label = '历史参考'
    type = 'default'
    lines.push('历史信号，仅供对照')
  } else if (checklistReady === true) {
    label = '可买'
    type = 'success'
    lines.push('清单已通过，落入区间可考虑')
  } else if (buyPriceRange?.mode === 'above' || buyPriceRange?.deferMode === 'wait') {
    label = '等回踩'
    type = 'warning'
    lines.push('现价偏高，等落入参考区间')
  } else if (buyPriceRange?.mode === 'inZone' || buyPriceRange?.mode === 'near') {
    if (tag === '买' && daysAgo === 0) {
      label = '仅观察'
      type = 'info'
      lines.push('出买当日宜观察，1～2 日回踩再试')
    } else {
      label = '可买'
      type = 'success'
      lines.push('现价接近参考区间，可考虑')
    }
  } else if (buyPriceRange?.mode === 'below') {
    label = '可买'
    type = 'success'
    lines.push('现价低于出信号价，区间仍有效')
  } else if (tag === '买') {
    label = '仅观察'
    lines.push('等回踩确认后再小仓试')
  } else if (tag === '强' || tag === '突' || tag === '弹' || tag === '转') {
    label = '等回踩'
    type = 'warning'
    lines.push(tag === '转' ? '转势确认后等回踩再进' : '不宜追信号当日大阳线')
  }

  lines.push('参考区间，非投资建议')
  return {
    label,
    type,
    tag,
    lines,
    tooltip: lines.join('\n'),
    buyPriceRange,
  }
}

/** K 线某根上的信号 → 操作提示 */
export function buildBarSignalHint({ sig, bars, barIndex, tag, options = {}, livePrice } = {}) {
  if (!tag || barIndex == null || barIndex < 0) return null
  const closes = bars?.closes ?? []
  const last = closes.length - 1
  if (last < 0) return null
  const daysAgo = Math.max(0, last - barIndex)
  const isHistorical = daysAgo > 0
  const nowPrice = livePrice ?? closes[last]

  if (isSellSignalTag(tag)) {
    const sellPositionPct = calcSellPositionPct(tag, sig, barIndex, bars, options)
    return resolveSignalActionHint({
      tag,
      sellPositionPct,
      daysAgo,
      isHistorical,
    })
  }

  if (isRushReduceTag(tag)) {
    const rushReducePct =
      options.rushReducePct ?? calcRushReducePct(barIndex, bars, sig, options)
    return resolveSignalActionHint({
      tag,
      rushReducePct,
      sellPositionPct: rushReducePct,
      daysAgo,
      isHistorical,
    })
  }

  if (isAddPositionTag(tag)) {
    const addPositionPct =
      options.addPositionPct ?? calcAddPositionPct(sourceTag, barIndex, bars, sig, options)
    return resolveSignalActionHint({
      tag,
      addPositionPct,
      sourceTag,
      daysAgo,
      isHistorical,
    })
  }

  if (BUY_ENTRY_TAGS.has(tag)) {
    const summary = {
      tag,
      recentSignalBar: barIndex,
      recentSignalDaysAgo: daysAgo,
      signalLastIndex: last,
      recentSignalConfirmBar: tag === '强' || tag === '突' ? barIndex : undefined,
    }
    const buyPriceRange = calcBuyPriceRange(summary, bars, options)
    return resolveSignalActionHint({
      tag,
      buyPriceRange,
      daysAgo,
      isHistorical,
      livePrice: nowPrice,
    })
  }

  if (tag === '冰') {
    return resolveSignalActionHint({ tag, daysAgo, isHistorical })
  }

  return null
}

export function formatSignalActionTooltip(hint) {
  if (!hint) return ''
  return hint.tooltip || hint.lines?.join('\n') || ''
}
