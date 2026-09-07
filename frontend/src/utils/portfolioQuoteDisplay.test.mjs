/**
 * Unit tests: Portfolio quote display (Phase15-B1).
 * Run: node frontend/src/utils/portfolioQuoteDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'portfolioQuoteDisplay.js')).href
const {
  buildQuotePriceTooltip,
  isLiveQuoteSource,
  quoteSourceShortLabel,
  resolveDisplayPrice,
  resolveMarkPrice,
} = await import(modUrl)

{
  assert.equal(resolveDisplayPrice({ displayPrice: 20.5, markPrice: 12 }), 20.5)
  assert.equal(resolveDisplayPrice({ displayPrice: null, markPrice: 12 }), 12)
  assert.equal(resolveDisplayPrice({ markPrice: 12 }), 12)
  assert.equal(resolveDisplayPrice({ displayPrice: 0, markPrice: 11.2 }), 11.2)
}

{
  assert.equal(isLiveQuoteSource('live'), true)
  assert.equal(isLiveQuoteSource('open_fallback'), true)
  assert.equal(isLiveQuoteSource('persisted'), false)
  assert.equal(isLiveQuoteSource(''), false)
}

{
  assert.equal(quoteSourceShortLabel('live'), 'Quote')
  assert.equal(quoteSourceShortLabel('open_fallback'), 'Quote')
  assert.equal(quoteSourceShortLabel('persisted'), 'Settlement')
  assert.equal(quoteSourceShortLabel(''), 'Settlement')
}

{
  assert.equal(resolveMarkPrice({ markPrice: 10.63 }), 10.63)
  const tip = buildQuotePriceTooltip(
    { displayPrice: 11, markPrice: 10.63, displayQuoteSource: 'live' },
    (v) => String(v),
  )
  assert.match(tip, /展示价格：11/)
  assert.match(tip, /账本价/)
  assert.match(tip, /Quote/)
}

console.log('portfolioQuoteDisplay.test.mjs: all passed')
