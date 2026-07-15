/**
 * 校验「弹」信号：拉东财日 K 并输出卖/弹明细
 * 用法: node scripts/verify-rebound-signals.mjs
 */
import {
  computeFullSignals,
  findRecentSignalBar,
  normalizeDayKey,
  summarizeBuySignal,
} from '../src/utils/icePointSignals.js'
import { resolveSignalLastBarIndex } from '../src/utils/tradingSession.js'

const STOCKS = [
  { code: '688143.SH', name: '长盈通', score: 84 },
  { code: '600938.SH', name: '中国海油', score: 84 },
  { code: '300611.SZ', name: '威力传动', score: 84 },
  { code: '002832.SZ', name: '比音勒芬', score: 83 },
  { code: '002298.SZ', name: '中电鑫龙', score: 83 },
  { code: '605128.SH', name: '上海沿浦', score: 83 },
  { code: '601963.SH', name: '重庆银行', score: 82 },
  { code: '688246.SH', name: '嘉和美康', score: 82 },
  { code: '603429.SH', name: '*ST艾利', score: 74 },
]

function toSecid(code) {
  const m = String(code).match(/^(\d{6})\.(SH|SZ|BJ)$/i)
  if (!m) return ''
  const num = m[1]
  const mkt = m[2].toUpperCase()
  if (mkt === 'SH') return `1.${num}`
  if (mkt === 'SZ') return `0.${num}`
  if (mkt === 'BJ') return `0.${num}`
  return ''
}

async function fetchDailyBars(code) {
  const m = String(code).match(/^(\d{6})\.(SH|SZ|BJ)$/i)
  if (!m) return { closes: [], opens: [], highs: [], lows: [], volumes: [], dayKeys: [] }
  const num = m[1]
  const prefix = m[2].toUpperCase() === 'SH' ? 'sh' : 'sz'
  const sym = `${prefix}${num}`
  const url = `https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=${sym},day,,,120,qfq`
  const res = await fetch(url, { headers: { 'User-Agent': 'Mozilla/5.0' } })
  const json = await res.json()
  const day = json?.data?.[sym]?.qfqday || json?.data?.[sym]?.day || []
  const closes = []
  const opens = []
  const highs = []
  const lows = []
  const volumes = []
  const dayKeys = []
  for (const row of day) {
    if (!row || row.length < 6) continue
    const d = String(row[0]).slice(0, 10)
    const o = Number(row[1])
    const c = Number(row[2])
    const h = Number(row[3])
    const l = Number(row[4])
    const v = Number(row[5])
    if (!d || !Number.isFinite(c)) continue
    dayKeys.push(normalizeDayKey(d))
    opens.push(o)
    closes.push(c)
    highs.push(h)
    lows.push(l)
    volumes.push(v)
  }
  return { closes, opens, highs, lows, volumes, dayKeys }
}

function fmtBar(dayKeys, i) {
  return dayKeys[i] || `#${i}`
}

function analyzeStock({ code, name, score }) {
  return fetchDailyBars(code).then((bars) => {
    if (!bars.closes.length) return { code, name, score, error: '无K线' }
    const lastIdx = resolveSignalLastBarIndex(bars.dayKeys)
    const sig = computeFullSignals(bars, { lookback: 5, requireIndexBull: true })
    const summary = summarizeBuySignal(
      { ...bars, indexMa20ByDay: null },
      { lookback: 5, recentBuyDays: 0, recentSellDays: 0, includeSell: true, signalLastIndex: lastIdx },
    )
    const reboundBar = findRecentSignalBar(sig, lastIdx, '弹', { recentBuyDays: 0 })
    const sellSet = new Set([...(sig.sellRsi || []), ...(sig.sellMa20 || [])])
    let lastSell = null
    for (let j = lastIdx - 1; j >= Math.max(0, lastIdx - 8); j--) {
      if (sellSet.has(j)) {
        lastSell = j
        break
      }
    }
    let lowSince = Infinity
    if (lastSell != null && reboundBar?.index != null) {
      for (let j = lastSell; j <= reboundBar.index; j++) {
        const l = bars.lows[j] ?? bars.closes[j]
        if (l < lowSince) lowSince = l
      }
    }
    const rb = reboundBar?.index
    const ma20 = sig.ma20
    let reclaim = false
    let reboundPct = null
    if (rb != null && rb > 0) {
      reclaim =
        ma20[rb - 1] != null &&
        ma20[rb] != null &&
        bars.closes[rb - 1] <= ma20[rb - 1] &&
        bars.closes[rb] > ma20[rb]
      if (lowSince > 0 && lowSince < Infinity) {
        reboundPct = (((bars.closes[rb] - lowSince) / lowSince) * 100).toFixed(2)
      }
    }
    return {
      code,
      name,
      listScore: score,
      tag: summary.tag,
      statusText: summary.statusText,
      effectiveDay: bars.dayKeys[lastIdx],
      reboundDay: rb != null ? bars.dayKeys[rb] : null,
      reboundDaysAgo: reboundBar?.daysAgo,
      lastSellDay: lastSell != null ? bars.dayKeys[lastSell] : null,
      sellToReboundDays: lastSell != null && rb != null ? rb - lastSell : null,
      reclaimMa20: reclaim,
      reboundPctFromLow: reboundPct,
      rsiAtRebound: rb != null ? sig.rsi[rb]?.toFixed(1) : null,
      ohlcRebound:
        rb != null
          ? `开${bars.opens[rb]} 收${bars.closes[rb]} (${bars.closes[rb] > bars.opens[rb] ? '阳' : '阴'})`
          : null,
      match弹: summary.tag === '弹' && reboundBar?.daysAgo === 0,
    }
  })
}

const results = []
for (const stock of STOCKS) {
  results.push(await analyzeStock(stock))
  await new Promise((r) => setTimeout(r, 300))
}
console.log(JSON.stringify(results, null, 2))
