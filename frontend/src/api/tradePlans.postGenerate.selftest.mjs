/**
 * Selftest: post-generate upcoming trade_date preference helpers.
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/api/tradePlans.postGenerate.selftest.mjs
 */
import assert from 'node:assert/strict'
import {
  isPreferredGeneratedPlan,
  resolveUpcomingQueryTradeDateAfterGenerate,
} from './tradePlansPostGenerate.js'

{
  assert.equal(
    resolveUpcomingQueryTradeDateAfterGenerate({
      planId: 17,
      generatedTradeDate: '2026-08-05',
      inputTradeDate: '',
    }),
    '2026-08-05',
  )
  assert.equal(
    resolveUpcomingQueryTradeDateAfterGenerate({
      planId: 17,
      generatedTradeDate: '2026-08-05',
      inputTradeDate: '2026-08-04',
    }),
    '2026-08-05',
  )
  assert.equal(
    resolveUpcomingQueryTradeDateAfterGenerate({
      planId: 0,
      generatedTradeDate: '2026-08-05',
      inputTradeDate: '',
    }),
    undefined,
  )
  assert.equal(
    resolveUpcomingQueryTradeDateAfterGenerate({
      planId: 0,
      generatedTradeDate: '2026-08-05',
      inputTradeDate: '2026-08-04',
    }),
    '2026-08-04',
  )
  assert.equal(
    resolveUpcomingQueryTradeDateAfterGenerate({
      planId: 17,
      generatedTradeDate: '  ',
      inputTradeDate: '2026-08-04',
    }),
    '2026-08-04',
  )
}

{
  assert.equal(isPreferredGeneratedPlan(17, 17), true)
  assert.equal(isPreferredGeneratedPlan(17, 15), false)
  assert.equal(isPreferredGeneratedPlan(0, 15), false)
  assert.equal(isPreferredGeneratedPlan(undefined, 15), false)
  assert.equal(isPreferredGeneratedPlan(17, null), false)
}

console.log('tradePlans.postGenerate.selftest: OK')

