import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import {
  DAILY_DISCIPLINE_DISCLAIMER,
  DAILY_DISCIPLINE_STORAGE_KEY,
  DAILY_TIPS,
  TIP_CATEGORIES,
  advanceDailyDiscipline,
  calendarDaysBetween,
  dismissDailyDiscipline,
  ensureDailyAssignment,
  emptyDailyDisciplineState,
  presentDailyDiscipline,
  rankDailyTips,
} from './dailyDisciplineTips.js'

function memoryStorage(initial) {
  const data = { ...initial }
  return {
    getItem(key) {
      return Object.prototype.hasOwnProperty.call(data, key) ? data[key] : null
    },
    setItem(key, value) {
      data[key] = String(value)
    },
    dump() {
      return data
    },
  }
}

const miniTips = [
  { id: 'a', category: '心态', title: 'A', summary: 'sa', body: 'ba' },
  { id: 'b', category: '纪律', title: 'B', summary: 'sb', body: 'bb' },
  { id: 'c', category: '仓位', title: 'C', summary: 'sc', body: 'bc' },
  { id: 'd', category: '失效条件', title: 'D', summary: 'sd', body: 'bd' },
  { id: 'e', category: '观察≠下单', title: 'E', summary: 'se', body: 'be' },
]

test('catalog is a static curated set with required categories and disclaimer', () => {
  assert.ok(DAILY_TIPS.length >= 40 && DAILY_TIPS.length <= 60)
  const ids = new Set(DAILY_TIPS.map((tip) => tip.id))
  assert.equal(ids.size, DAILY_TIPS.length)
  const categories = new Set(DAILY_TIPS.map((tip) => tip.category))
  for (const category of TIP_CATEGORIES) {
    assert.ok(categories.has(category), `missing category ${category}`)
  }
  for (const tip of DAILY_TIPS) {
    assert.ok(TIP_CATEGORIES.includes(tip.category))
    assert.ok(tip.title && tip.summary && tip.body)
    assert.equal(tip.body.includes('建议买入'), false)
  }
  const corpus = DAILY_TIPS.map((tip) => `${tip.title}\n${tip.summary}\n${tip.body}`).join('\n')
  assert.match(corpus, /信号\s*≠\s*订单/)
  assert.match(corpus, /观察\s*≠\s*TradePlan/)
  assert.match(corpus, /不追回本|不追恢复/)
  assert.match(DAILY_DISCIPLINE_DISCLAIMER, /非投资建议/)
  assert.match(DAILY_DISCIPLINE_DISCLAIMER, /不是交易信号/)
})

test('same Shanghai calendar day keeps the same tip', () => {
  const storage = memoryStorage()
  const opts = { storage, tips: miniTips, today: '2026-09-30', poolSize: 3 }
  const first = presentDailyDiscipline(opts)
  const second = presentDailyDiscipline(opts)
  assert.equal(first.tip.id, 'a')
  assert.equal(second.tip.id, 'a')
  assert.equal(second.index, 0)
  assert.deepEqual(second.disclaimer, DAILY_DISCIPLINE_DISCLAIMER)
  assert.equal(first.visible, true)
})

test('stored assignment wins over a fresh ranking on the same day', () => {
  const storage = memoryStorage({
    [DAILY_DISCIPLINE_STORAGE_KEY]: JSON.stringify({
      version: 1,
      history: [],
      assignment: { date: '2026-09-30', pool: ['e', 'c', 'a'], index: 1 },
      dismissedDate: null,
    }),
  })
  const view = presentDailyDiscipline({
    storage,
    tips: miniTips,
    today: '2026-09-30',
  })
  assert.equal(view.tip.id, 'c')
  assert.equal(view.positionLabel, '2/3')
})

test('next calendar day prefers a tip not shown recently', () => {
  const storage = memoryStorage()
  const opts = { storage, tips: miniTips, poolSize: 3 }
  const day1 = presentDailyDiscipline({ ...opts, today: '2026-09-30' })
  const day2 = presentDailyDiscipline({ ...opts, today: '2026-10-01' })
  assert.equal(day1.tip.id, 'a')
  assert.equal(day2.tip.id, 'b')
  assert.notEqual(day1.tip.id, day2.tip.id)
})

test('Shanghai midnight rolls the calendar day', () => {
  const storage = memoryStorage()
  const before = presentDailyDiscipline({
    storage,
    tips: miniTips,
    now: new Date('2026-09-30T15:59:00Z'),
    poolSize: 3,
  })
  const after = presentDailyDiscipline({
    storage,
    tips: miniTips,
    now: new Date('2026-09-30T16:01:00Z'),
    poolSize: 3,
  })
  assert.equal(before.date, '2026-09-30')
  assert.equal(after.date, '2026-10-01')
  assert.equal(before.tip.id, 'a')
  assert.equal(after.tip.id, 'b')
})

test('when every tip is recent, pick the least-recently-shown', () => {
  const history = [
    { id: 'a', date: '2026-09-28' },
    { id: 'b', date: '2026-09-25' },
    { id: 'c', date: '2026-09-29' },
    { id: 'd', date: '2026-09-27' },
    { id: 'e', date: '2026-09-26' },
  ]
  const ranked = rankDailyTips(miniTips, history, '2026-09-30')
  assert.equal(ranked[0].id, 'b')
  assert.equal(ranked[0].tier, 3)
  const state = ensureDailyAssignment(
    { ...emptyDailyDisciplineState(), history },
    '2026-09-30',
    miniTips,
  )
  assert.equal(state.assignment.pool[0], 'b')
})

test('tips outside 14 days outrank the 7–14 day soft window', () => {
  const history = [
    { id: 'a', date: '2026-09-16' },
    { id: 'b', date: '2026-09-20' },
    { id: 'c', date: '2026-09-29' },
  ]
  assert.equal(calendarDaysBetween('2026-09-16', '2026-09-30'), 14)
  assert.equal(calendarDaysBetween('2026-09-20', '2026-09-30'), 10)
  const ranked = rankDailyTips(miniTips, history, '2026-09-30')
  assert.deepEqual(
    ranked.map((row) => row.id),
    ['d', 'e', 'a', 'b', 'c'],
  )
  assert.equal(ranked.find((row) => row.id === 'a').tier, 1)
  assert.equal(ranked.find((row) => row.id === 'b').tier, 2)
  assert.equal(ranked.find((row) => row.id === 'c').tier, 3)
})

test('下一条 stays inside the same-day pool and survives reload', () => {
  const storage = memoryStorage()
  const opts = { storage, tips: miniTips, today: '2026-09-30', poolSize: 3 }
  const first = presentDailyDiscipline(opts)
  const second = advanceDailyDiscipline(opts)
  const third = advanceDailyDiscipline(opts)
  const reloaded = presentDailyDiscipline(opts)
  assert.deepEqual(
    [first.tip.id, second.tip.id, third.tip.id],
    ['a', 'b', 'c'],
  )
  assert.equal(reloaded.tip.id, 'c')
  const wrapped = advanceDailyDiscipline(opts)
  assert.equal(wrapped.tip.id, 'a')
  assert.equal(wrapped.date, '2026-09-30')
  const nextDay = presentDailyDiscipline({ ...opts, today: '2026-10-01' })
  assert.equal(nextDay.tip.id, 'd')
  assert.equal(nextDay.index, 0)
})

test('dismiss hides the tip until the next calendar day', () => {
  const storage = memoryStorage()
  const opts = { storage, tips: miniTips, poolSize: 3 }
  const hidden = dismissDailyDiscipline({ ...opts, today: '2026-09-30' })
  const still = presentDailyDiscipline({ ...opts, today: '2026-09-30' })
  const tomorrow = presentDailyDiscipline({ ...opts, today: '2026-10-01' })
  assert.equal(hidden.visible, false)
  assert.equal(hidden.tip.id, 'a')
  assert.equal(still.visible, false)
  assert.equal(still.tip.id, 'a')
  assert.equal(tomorrow.visible, true)
  assert.equal(tomorrow.tip.id, 'b')
})

test('corrupt storage falls back to a stable assignment', () => {
  const storage = memoryStorage({
    [DAILY_DISCIPLINE_STORAGE_KEY]: '{not-json',
  })
  const view = presentDailyDiscipline({
    storage,
    tips: miniTips,
    today: '2026-09-30',
    poolSize: 3,
  })
  assert.equal(view.tip.id, 'a')
  assert.doesNotThrow(() => presentDailyDiscipline({
    storage,
    tips: miniTips,
    today: '2026-09-30',
  }))
})

test('home page mounts the dismissible daily tip under the header area', () => {
  const home = readFileSync(new URL('../components/InvestmentHome.vue', import.meta.url), 'utf8')
  const banner = readFileSync(new URL('../components/DailyDisciplineTip.vue', import.meta.url), 'utf8')
  assert.match(home, /DailyDisciplineTip/)
  assert.match(banner, /每日精进/)
  assert.match(banner, /下一条/)
  assert.match(banner, /disclaimer/)
  assert.match(banner, /今日不再显示/)
})
