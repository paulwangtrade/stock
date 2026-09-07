/**
 * Unit tests: portfolio sell entry helpers (Phase14-A-R1-B).
 * Run: node frontend/src/utils/portfolioSellEntry.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'portfolioSellEntry.js')).href
const {
  canShowSellButton,
  isPaperSimPosition,
  maxSellQuantity,
  sellRowStockCode,
  validateSellQuantity,
} = await import(modUrl)

{
  assert.equal(maxSellQuantity({ availableQty: 500 }), 500)
  assert.equal(maxSellQuantity({ sellable: 300 }), 300)
  assert.equal(maxSellQuantity({ availableQty: 0, sellable: 0 }), 0)
  assert.equal(maxSellQuantity(null), 0)
}

{
  assert.equal(isPaperSimPosition({ availableQty: 100 }), true)
  assert.equal(isPaperSimPosition({ source: 'paper_sim' }), true)
  assert.equal(isPaperSimPosition({ accountType: 'real' }), false)
  assert.equal(isPaperSimPosition({ isSelfHolding: true }), false)
}

{
  assert.equal(canShowSellButton({ availableQty: 100, totalQty: 200 }), true)
  assert.equal(canShowSellButton({ availableQty: 0, totalQty: 200 }), false)
  assert.equal(canShowSellButton({ availableQty: 100, totalQty: 0 }), false)
  assert.equal(canShowSellButton({ availableQty: 100, totalQty: 200, canSell: false }), false)
  assert.equal(canShowSellButton({ availableQty: 100, totalQty: 200, accountType: 'real' }), false)
}

{
  assert.equal(validateSellQuantity(100, 500), 100)
  assert.equal(validateSellQuantity(501, 500), null)
  assert.equal(validateSellQuantity(0, 500), null)
  assert.equal(validateSellQuantity(100.7, 500), 100)
}

{
  assert.equal(sellRowStockCode({ stockCode: 'sz000001' }), 'sz000001')
  assert.equal(sellRowStockCode({ code: 'sh600000' }), 'sh600000')
}

console.log('portfolioSellEntry.test.mjs: all passed')
