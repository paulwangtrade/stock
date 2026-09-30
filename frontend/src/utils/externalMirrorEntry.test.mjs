/**
 * 实盘镜像录入校验与三账户文案。
 * Run: node frontend/src/utils/externalMirrorEntry.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'externalMirrorEntry.js')).href
const {
  ACCOUNT_BOOK_TABS,
  EXTERNAL_MIRROR_DISCLAIMER,
  EXTERNAL_MIRROR_SOURCE,
  isExternalMirrorRow,
  validateMirrorDraft,
} = await import(modUrl)

{
  assert.deepEqual(
    ACCOUNT_BOOK_TABS.map((t) => t.label),
    ['模拟量化', '实盘镜像（观察）', '实盘券商（未启用）'],
  )
  assert.equal(EXTERNAL_MIRROR_SOURCE, 'external_mirror')
  assert.equal(EXTERNAL_MIRROR_DISCLAIMER, '仅观察 · 不进模拟账本 · 不进交易计划 · 不下单')
}

{
  assert.equal(validateMirrorDraft({ stockCode: '', quantity: 100, costPrice: 1 }).reason, 'missing_code')
  assert.equal(validateMirrorDraft({ stockCode: '600519', quantity: 0, costPrice: 1 }).reason, 'invalid_quantity')
  assert.equal(validateMirrorDraft({ stockCode: '600519', quantity: 1.5, costPrice: 1 }).reason, 'invalid_quantity')
  assert.equal(validateMirrorDraft({ stockCode: '600519', quantity: 100, costPrice: 0 }).reason, 'invalid_cost')
  assert.equal(
    validateMirrorDraft({ stockCode: '600519', quantity: 100, costPrice: 10, source: 'paper_sim' }).reason,
    'invalid_source',
  )
  assert.equal(
    validateMirrorDraft({ stockCode: '600519', quantity: 100, costPrice: 10, feedsTradePlan: true }).reason,
    'observation_only',
  )
  const ok = validateMirrorDraft({
    stockCode: '600519',
    quantity: 100,
    costPrice: 10.5,
    entryDate: '2026-09-30',
  })
  assert.equal(ok.ok, true)
}

{
  assert.equal(isExternalMirrorRow({ source: 'external_mirror', quantity: 100 }), true)
  assert.equal(isExternalMirrorRow({ source: 'paper_sim', quantity: 100 }), false)
  assert.equal(isExternalMirrorRow({ tab: 'external_mirror' }), true)
}

console.log('externalMirrorEntry.test.mjs: all passed')
