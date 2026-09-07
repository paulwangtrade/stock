/**
 * Unit tests: Phase16.14 ExplanationTable helpers
 * Run: node frontend/src/utils/explanationTable.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'explanationTable.js')).href)

const {
  fieldsToExplanationTableRows,
  buildOriginExplanationRows,
  normalizeExplanationTableRows,
  EXPLANATION_TABLE_COLUMNS,
} = mod

assert.equal(EXPLANATION_TABLE_COLUMNS.field, '字段')
assert.equal(EXPLANATION_TABLE_COLUMNS.content, '内容')

{
  const rows = fieldsToExplanationTableRows(
    [
      { key: 'trigger_reason', label: 'trigger_reason', value: '突破', missing: false, source: 'SignalSnapshot' },
      { key: 'signal_tag', label: 'signal_tag', value: '强', missing: false, showTag: true },
    ],
    { labelFn: (k) => ({ trigger_reason: '发现依据', signal_tag: '信号标签' }[k] || k), group: 'signal' },
  )
  assert.equal(rows.length, 2)
  assert.equal(rows[0].field, '发现依据')
  assert.equal(rows[0].content, '突破')
  assert.equal(rows[0].source, 'SignalSnapshot')
  assert.equal(rows[1].kind, 'tag')
  assert.equal(rows[1].field, '信号标签')
}

{
  const rows = buildOriginExplanationRows({
    sourceChipLabel: 'Strategy',
    sourceBucket: 'strategy',
    strategyName: '动量',
    strategyNameLabel: '动量',
    strategyNameMissing: false,
    score: '0.82',
    scoreLabel: '0.82',
    scoreMissing: false,
    sourceReason: '候选命中',
    sourceReasonMissing: false,
    selectionReason: 'rank=1',
    selectionReasonMissing: false,
    signalTime: '暂无记录',
    signalTimeMissing: true,
    signalPrice: '10.00',
    signalPriceMissing: false,
    signalTag: '突',
    signalTagMissing: false,
  })
  assert.equal(rows.length, 8)
  assert.equal(rows[0].field, '来源')
  assert.equal(rows[0].content, 'Strategy')
  assert.equal(rows[3].field, '发现依据')
  assert.equal(rows[4].field, '入选说明')
  assert.equal(rows[7].kind, 'tag')
  const keys = rows.map((r) => r.key)
  assert.deepEqual(keys, [
    'source_chip',
    'strategy_name',
    'score',
    'source_reason',
    'selection_reason',
    'signal_time',
    'signal_price',
    'signal_tag',
  ])
}

{
  const rows = normalizeExplanationTableRows(
    [
      { key: 'a', field: '发现依据', content: 'x', group: 'signal' },
      { key: 'b', field: 'HEA', content: 'y', group: 'hea' },
    ],
    { groups: ['signal'] },
  )
  assert.equal(rows.length, 1)
  assert.equal(rows[0].key, 'a')
}

console.log('explanationTable.test.mjs: ok')
