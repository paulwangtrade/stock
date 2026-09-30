/**
 * Decision timeline display smoke.
 * Run: node frontend/src/utils/decisionTimelineDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'decisionTimelineDisplay.js')).href)
const {
  DECISION_TIMELINE_DISCLAIMER,
  canOpenKline,
  emptyTimelineMessage,
  filterTimelineEvents,
  laneLabel,
  sourceStatusLabel,
} = mod

assert.equal(DECISION_TIMELINE_DISCLAIMER, '回顾用，不是买卖指令')
assert.match(emptyTimelineMessage(null), /保持空白/)
assert.match(emptyTimelineMessage({ events: [] }), /不会补写买卖指令/)
assert.equal(emptyTimelineMessage({ events: [{ id: '1' }] }), '')

const events = [
  { id: 's', kind: 'signal', lane: 'observe', kline_date: '2026-08-01', title: '信号 · 冰' },
  { id: 'e', kind: 'paper_sim', lane: 'entry', kline_date: '2026-08-22', title: '模拟盘触及 · 买入成交' },
  { id: 'h', kind: 'paper_sim', lane: 'holding', kline_date: '', title: '模拟盘触及 · 持仓' },
  { id: 'x', kind: 'outcome', lane: 'exit', kline_date: '2026-09-02', title: '结果备注 · 日志' },
]

assert.equal(filterTimelineEvents(events, 'all').length, 4)
assert.deepEqual(filterTimelineEvents(events, 'entry').map((e) => e.id), ['e'])
assert.deepEqual(filterTimelineEvents(events, 'exit').map((e) => e.id), ['x'])
assert.equal(filterTimelineEvents(events, 'entry').some((e) => e.lane === 'exit'), false)
assert.equal(canOpenKline(events[0]), true)
assert.equal(canOpenKline(events[2]), false)
assert.equal(laneLabel('holding'), '持有')
assert.equal(sourceStatusLabel('empty'), '没有记录')
assert.equal(sourceStatusLabel('unavailable'), '不可用')

console.log('decisionTimelineDisplay.test.mjs ok')
