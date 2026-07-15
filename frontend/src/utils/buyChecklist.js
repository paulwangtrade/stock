/** 买入清单：规则化逐项校验（自动 + 可展示） */

import { BUY_ENTRY_TAGS } from './buyPriceRange'
import { isSellSignalTag } from './icePointSignals'
import { formatSignalTagLabel, isStStockRow } from './signalBuyGuide'

function priceInBuyZone(price, bp, tolPct = 0.003) {
  if (!bp || !Number.isFinite(price)) return false
  const lo = bp.low
  const hi = bp.high ?? bp.rangeHigh ?? bp.instantPrice
  if (!Number.isFinite(lo) || !Number.isFinite(hi)) return false
  return price >= lo * (1 - tolPct) && price <= hi * (1 + tolPct)
}

function checkRsiForTag(tag, rsi) {
  if (!Number.isFinite(rsi)) return null
  if (tag === '买' || tag === '强') return rsi >= 30 && rsi <= 75
  if (tag === '趋') return rsi >= 45 && rsi <= 68
  if (tag === '转') return rsi >= 45 && rsi <= 70
  if (tag === '突') return rsi >= 40 && rsi <= 80
  if (tag === '弹') return rsi <= 60
  return rsi >= 25 && rsi <= 80
}

function checkMaStructure(summary, livePrice, tag) {
  const ma20 = summary?.latestStatus?.ma20
  if (ma20 == null || !Number.isFinite(livePrice)) return null
  if (tag === '强' || tag === '突') return livePrice >= ma20
  if (tag === '趋' || tag === '弹') return livePrice >= ma20 * 0.995
  if (tag === '转') return livePrice >= ma20 * 0.98
  return true
}

/**
 * @param {object} ctx
 * @returns {{ items, score, ready, requiredPassed, blockers, passCount, totalCount }}
 */
export function evaluateBuyChecklist(ctx) {
  const {
    summary,
    buyPriceRange,
    livePrice,
    stockRow,
    automation,
  } = ctx

  const tag = summary?.tag
  const tol = automation?.zoneTouchTolerancePct ?? 0.003
  const readyScore = automation?.checklistReadyScore ?? 0.85
  const requireRequired = automation?.checklistRequireRequired !== false

  const rsi = summary?.latestStatus?.rsi
  const items = []

  items.push({
    id: 'buy_signal',
    label: '有效买点信号（买/强/趋/转/突/弹）',
    passed: BUY_ENTRY_TAGS.has(tag),
    required: true,
    detail: tag ? `当前信号：${formatSignalTagLabel(tag)}` : '无买点',
  })

  items.push({
    id: 'no_sell_override',
    label: '无优先卖信号（止/减）',
    passed: !isSellSignalTag(tag),
    required: true,
    detail: isSellSignalTag(tag) ? '当前为止/减，先处理风控' : '通过',
  })

  items.push({
    id: 'not_st',
    label: '非 ST 标的',
    passed: !isStStockRow(stockRow || {}),
    required: true,
    detail: isStStockRow(stockRow || {}) ? 'ST 排除' : '通过',
  })

  const inZone = priceInBuyZone(livePrice, buyPriceRange, tol)
  items.push({
    id: 'price_zone',
    label: '现价落入买入参考区间',
    passed: inZone,
    required: true,
    detail: buyPriceRange?.text
      ? `区间 ${buyPriceRange.text} · 现价 ${livePrice ?? '—'}`
      : '无区间数据',
  })

  const rsiOk = checkRsiForTag(tag, rsi)
  items.push({
    id: 'rsi',
    label: 'RSI 结构符合该信号',
    passed: rsiOk === null ? true : rsiOk,
    required: false,
    detail: Number.isFinite(rsi) ? `RSI ${rsi.toFixed(1)}` : '—',
  })

  const maOk = checkMaStructure(summary, livePrice, tag)
  items.push({
    id: 'ma20',
    label: '收盘在 MA20 之上（强/趋/突/弹）',
    passed: maOk === null ? true : maOk,
    required: tag === '强' || tag === '突',
    detail: maOk === false ? '低于 MA20' : '通过',
  })

  items.push({
    id: 'signal_fresh',
    label: '信号在有效回溯窗口内',
    passed: summary?.recentSignalDaysAgo == null || summary.recentSignalDaysAgo <= 15,
    required: false,
    detail:
      summary?.recentSignalDaysAgo === 0
        ? '今日信号'
        : `${summary?.recentSignalDaysAgo ?? '—'} 日前`,
  })

  if (tag === '强') {
    items.push({
      id: 'strong_confirm',
      label: '强化买已确认完成',
      passed: summary?.hasRecentStrictBuy === true || tag === '强',
      required: true,
      detail: '须 2 日不破位确认',
    })
  }

  if (tag === '突') {
    items.push({
      id: 'breakout_confirm',
      label: '突破已 3 日站稳',
      passed: summary?.hasRecentBreakout === true || tag === '突',
      required: true,
      detail: '箱顶突破确认',
    })
  }

  const requiredItems = items.filter((i) => i.required)
  const requiredPassed = requiredItems.every((i) => i.passed)
  const passCount = items.filter((i) => i.passed).length
  const totalCount = items.length
  const score = totalCount ? passCount / totalCount : 0
  const blockers = items.filter((i) => !i.passed).map((i) => i.label)
  const ready =
    (requireRequired ? requiredPassed : true) && score >= readyScore

  return {
    items,
    score,
    ready,
    requiredPassed,
    blockers,
    passCount,
    totalCount,
  }
}

export function formatChecklistScore(checklist) {
  if (!checklist) return '—'
  const pct = Math.round((checklist.score || 0) * 100)
  return checklist.ready ? `就绪 ${pct}%` : `${pct}%`
}
