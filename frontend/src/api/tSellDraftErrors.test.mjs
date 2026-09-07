/**
 * Unit tests: T-sell draft error mapping (Phase14-A-R1-A).
 * Run: node frontend/src/api/tSellDraftErrors.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'tSellDraftErrors.js')).href
const { mapTSellDraftErrorMessage, T_SELL_DRAFT_ERROR_CODES } = await import(modUrl)

{
  assert.match(
    mapTSellDraftErrorMessage(T_SELL_DRAFT_ERROR_CODES.insufficient_available),
    /可卖数量不足/,
  )
  assert.match(
    mapTSellDraftErrorMessage(T_SELL_DRAFT_ERROR_CODES.cannot_sell),
    /不可卖/,
  )
  assert.match(
    mapTSellDraftErrorMessage(T_SELL_DRAFT_ERROR_CODES.invalid_stock),
    /无此持仓/,
  )
  assert.match(
    mapTSellDraftErrorMessage(T_SELL_DRAFT_ERROR_CODES.invalid_quantity),
    /正整数/,
  )
}

console.log('tSellDraftErrors.test.mjs: all passed')
