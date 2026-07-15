import { readFileSync } from 'node:fs'
import { buildIndexMa20ByDay, summarizeBuySignal } from '../../frontend/src/utils/icePointSignals'
import { buildSignalOptions, cloneDefaultSignalSettings } from '../../frontend/src/utils/signalSettings'
import { resolveSignalLastBarIndex } from '../../frontend/src/utils/tradingSession'

const inputPath = process.argv[2]
const raw = inputPath ? readFileSync(inputPath, 'utf8') : readFileSync(0, 'utf8')
const { stocks, indexClose } = JSON.parse(raw)
const indexMa20ByDay = buildIndexMa20ByDay(new Map(Object.entries(indexClose || {})), 20)
const options = {
  ...buildSignalOptions(cloneDefaultSignalSettings()),
  recentBuyDays: 0,
  recentSellDays: 0,
}

const strongToday: Array<{ code: string; name: string; statusText: string; sortRank: number }> = []
const strongRecent: Array<{ code: string; name: string; daysAgo: number; statusText: string }> = []
const allToday: Array<{ code: string; name: string; tag: string; statusText: string; sortRank: number }> = []
const allRows: Array<{ code: string; name: string; tag: string; daysAgo: number | null; sortRank: number }> = []

for (const s of stocks || []) {
  const bars = {
    closes: s.closes,
    opens: s.opens,
    highs: s.highs,
    lows: s.lows,
    volumes: s.volumes,
    dayKeys: s.dayKeys,
    indexMa20ByDay,
  }
  const lastIdx = resolveSignalLastBarIndex(bars.dayKeys)
  const summary = summarizeBuySignal(bars, { ...options, signalLastIndex: lastIdx })
  const row = {
    code: s.code,
    name: s.name,
    tag: summary.tag,
    daysAgo: summary.recentSignalDaysAgo,
    statusText: summary.statusText,
    sortRank: summary.sortRank,
  }
  allRows.push({
    code: s.code,
    name: s.name,
    tag: summary.tag || '—',
    daysAgo: summary.recentSignalDaysAgo,
    sortRank: summary.sortRank,
  })
  if (summary.tag && summary.recentSignalDaysAgo === 0) allToday.push(row)
  if (summary.tag === '强') {
    if (summary.recentSignalDaysAgo === 0) strongToday.push(row)
    else if (summary.recentSignalDaysAgo <= 5) strongRecent.push(row)
  }
}

strongToday.sort((a, b) => a.sortRank - b.sortRank)
strongRecent.sort((a, b) => a.daysAgo - b.daysAgo)
allToday.sort((a, b) => a.sortRank - b.sortRank)

console.log('=== 今日「强」标签 ===')
if (!strongToday.length) console.log('（无）')
else strongToday.forEach((r) => console.log(`${r.name} (${r.code})  ${r.statusText}`))

console.log('\n=== 近5日内「强」标签 ===')
if (!strongRecent.length) console.log('（无）')
else strongRecent.forEach((r) => console.log(`${r.name} (${r.code})  ${r.daysAgo}日前  ${r.statusText}`))

console.log('\n=== 今日其它信号（参考）===')
if (!allToday.filter((x) => x.tag !== '强').length) console.log('（无）')
else allToday.filter((x) => x.tag !== '强').forEach((r) => console.log(`${r.name} (${r.code})  [${r.tag}]  ${r.statusText}`))

console.log('\n=== 自选主信号一览 ===')
allRows.sort((a, b) => a.sortRank - b.sortRank || a.name.localeCompare(b.name, 'zh-CN'))
for (const r of allRows) {
  const when = r.daysAgo === 0 ? '今日' : r.daysAgo != null && r.tag !== '—' ? `${r.daysAgo}日前` : ''
  console.log(`${r.name} (${r.code})  [${r.tag}]${when ? ' · ' + when : ''}`)
}
