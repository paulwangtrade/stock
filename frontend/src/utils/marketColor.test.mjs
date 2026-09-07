/**
 * Unit tests: marketColor (Phase16.26-A).
 * Run: node frontend/src/utils/marketColor.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'marketColor.js')).href)
const tokens = await import(pathToFileURL(join(__dir, 'designTokens.js')).href)

const {
  changeRateColor,
  marketColor,
  marketColorCssVar,
  marketColorStyle,
  marketDirection,
  marketToneClass,
  parseMarketNumber,
  pnlColor,
} = mod

{
  assert.equal(parseMarketNumber(1.2), 1.2)
  assert.equal(parseMarketNumber('-3.5%'), -3.5)
  assert.equal(parseMarketNumber('—'), null)
  assert.equal(parseMarketNumber(''), null)
}

{
  assert.equal(marketDirection(12), 'up')
  assert.equal(marketDirection(-0.01), 'down')
  assert.equal(marketDirection(0), 'flat')
  assert.equal(marketDirection(null), 'empty')
}

{
  // A-share: profit/up = red, loss/down = green
  assert.equal(marketColor(100), tokens.DESIGN_TOKENS.marketUp)
  assert.equal(marketColor(-50), tokens.DESIGN_TOKENS.marketDown)
  assert.equal(marketColor(0), tokens.DESIGN_TOKENS.marketFlat)
  assert.equal(marketColor(null), undefined)
}

{
  assert.equal(marketColorCssVar(1), 'var(--market-up)')
  assert.equal(marketColorCssVar(-1), 'var(--market-down)')
  assert.equal(pnlColor(2), 'var(--market-up)')
  assert.equal(changeRateColor(-2), 'var(--market-down)')
  assert.deepEqual(marketColorStyle(3), { color: 'var(--market-up)' })
  assert.equal(marketToneClass(-1), 'gs-market-down')
}

{
  // Brand is blue, not Naive default green
  assert.equal(tokens.DESIGN_TOKENS.brandPrimary, '#2080f0')
  assert.equal(tokens.naiveThemeOverrides.common.primaryColor, '#2080f0')
  assert.equal(tokens.naiveThemeOverrides.common.successColor, '#18a058')
}

console.log('marketColor.test.mjs: all passed')
