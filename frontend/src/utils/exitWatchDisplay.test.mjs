/**
 * Run: node frontend/src/utils/exitWatchDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'exitWatchDisplay.js')).href)
const {
  EXIT_WATCH_DISCLAIMER,
  canOfferExitWatchSell,
  filterExitWatchBySource,
  readExitWatchItem,
} = mod

{
  const item = readExitWatchItem({
    source: 'external_mirror',
    position_id: '7',
    stock_code: 'sh600519',
    class: 'FLATTEN_OBSERVE',
    reason_codes: ['LOSS'],
    sell_intent_allowed: true,
    manual_sell_draft_ref: 'draft-x',
    dedup_key: 'external_mirror|7|LOSS|2026-09-30',
    policy_ref: 'default_v1@v1',
  })
  assert.equal(item.sellIntentAllowed, false)
  assert.equal(item.manualSellDraftRef, '')
  assert.equal(canOfferExitWatchSell(item), false)
  assert.equal(item.disclaimer, EXIT_WATCH_DISCLAIMER)
  assert.equal(item.dedupKey, 'external_mirror|7|LOSS|2026-09-30')
  assert.equal(item.notAnOrder, true)
}

{
  const item = readExitWatchItem({
    source: 'paper_sim',
    class: 'REDUCE_OBSERVE',
    sell_intent_allowed: true,
    manual_sell_draft_ref: 'draft-1',
    reason_codes: ['TIME', 'LOSS'],
  })
  assert.equal(canOfferExitWatchSell(item), true)
  assert.equal(item.manualSellDraftRef, 'draft-1')
}

{
  const hold = readExitWatchItem({ source: 'paper_sim', class: 'HOLD_OBSERVE', sell_intent_allowed: true })
  assert.equal(canOfferExitWatchSell(hold), false)
  const bad = readExitWatchItem({ source: 'paper_sim', class: 'SELL_NOW', sell_intent_allowed: true })
  assert.equal(bad.class, 'DATA_INSUFFICIENT')
  assert.equal(canOfferExitWatchSell(bad), false)
  const stale = readExitWatchItem({
    source: 'paper_sim',
    class: 'DATA_INSUFFICIENT',
    reason_codes: ['STALE'],
    sell_intent_allowed: true,
  })
  assert.equal(stale.sellIntentAllowed, false)
  assert.equal(stale.reasonText, '行情过期')
  assert.equal(stale.reasonCodes.includes('LOSS'), false)
}

{
  const rows = filterExitWatchBySource(
    [
      { source: 'paper_sim', class: 'HOLD_OBSERVE' },
      { source: 'external_mirror', class: 'HOLD_OBSERVE' },
    ],
    'external_mirror',
  )
  assert.equal(rows.length, 1)
  assert.equal(rows[0].source, 'external_mirror')
}

{
  const pending = readExitWatchItem(null, 'loading')
  assert.equal(pending.pending, true)
  assert.equal(canOfferExitWatchSell(pending), false)
}

console.log('exitWatchDisplay.test.mjs ok')
