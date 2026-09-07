/**
 * Unit tests: Phase16.14 StockDisplayModel
 * Run: node frontend/src/utils/stockDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'stockDisplay.js')).href)
const adapters = await import(pathToFileURL(join(__dir, 'stockDisplayAdapters.js')).href)

const {
  toStockDisplayModel,
  pickRealStockName,
  buildStockDisplayTitle,
  looksLikeInternalStockCode,
} = mod

const { adaptTradePlanOrigin, adaptOpportunity, adaptPortfolioPosition } = adapters

// --- name 正常 ---
{
  const m = toStockDisplayModel({ stock_code: 'sz300274', stock_name: '阳光电源' })
  assert.equal(m.code, 'sz300274')
  assert.equal(m.name, '阳光电源')
  assert.equal(m.displayText, '阳光电源(sz300274)')
  assert.equal(m.klineKey, '300274.SZ')
  assert.equal(buildStockDisplayTitle(m, '解释'), '阳光电源(sz300274) · 解释')
}

// --- name 为空：禁止 code(code)，仅显示 code ---
{
  const m = toStockDisplayModel({ stock_code: 'sz300274', stock_name: '' })
  assert.equal(m.code, 'sz300274')
  assert.equal(m.name, '')
  assert.equal(m.displayText, 'sz300274')
  assert.notEqual(m.displayText, 'sz300274(sz300274)')
  assert.ok(!m.displayText.includes('('))
}

// --- name 被写成 code ---
{
  const m = toStockDisplayModel({ stock_code: 'sz300274', stock_name: 'sz300274' })
  assert.equal(m.name, '')
  assert.equal(m.displayText, 'sz300274')
}

// --- Origin 仅 code（历史 code/code 缺陷）---
{
  const m = adaptTradePlanOrigin({ stock_code: 'sz300274' })
  assert.equal(m.displayText, 'sz300274')
  assert.equal(buildStockDisplayTitle(m), 'sz300274 · 解释')
}

// --- Opportunity 投影有名 ---
{
  const m = adaptOpportunity(
    { stockCode: 'sz300274', stockName: '' },
    { stock_code: 'sz300274', stock_name: '阳光电源' },
  )
  assert.equal(m.displayText, '阳光电源(sz300274)')
}

// --- Portfolio ---
{
  const m = adaptPortfolioPosition({ stockCode: 'sh600519', stockName: '贵州茅台' })
  assert.equal(m.displayText, '贵州茅台(sh600519)')
  assert.equal(m.klineKey, '600519.SH')
}

// --- code 异常格式：仍可展示 raw，klineKey 空 ---
{
  const m = toStockDisplayModel({ stock_code: 'NOT_A_CODE', stock_name: '测试名' })
  assert.equal(m.code, 'NOT_A_CODE')
  assert.equal(m.name, '测试名')
  assert.equal(m.displayText, '测试名(NOT_A_CODE)')
  assert.equal(m.klineKey, '')
}

{
  const m = toStockDisplayModel({ stock_code: '??', stock_name: '' })
  assert.equal(m.code, '??')
  assert.equal(m.name, '')
  assert.equal(m.displayText, '??')
  assert.equal(m.klineKey, '')
}

{
  const m = toStockDisplayModel(null)
  assert.deepEqual(m, { code: '', name: '', displayText: '', klineKey: '' })
}

assert.equal(looksLikeInternalStockCode('sz300274'), true)
assert.equal(pickRealStockName({ stock_name: 'sz300274' }, 'sz300274'), '')

// --- K-line open uses klineKey ---
{
  const { applyStockClickAction, toStockKlineLinkModel } = mod
  const m = toStockDisplayModel({ stock_code: 'sz300274', stock_name: '阳光电源' })
  const link = toStockKlineLinkModel(m)
  assert.equal(link.klineKey, '300274.SZ')
  assert.equal(link.click_action.chart_code, '300274.SZ')
  const target = { visible: false, title: '', chartCode: '', stockName: '' }
  assert.equal(applyStockClickAction(m, target), true)
  assert.equal(target.chartCode, '300274.SZ')
  assert.equal(target.visible, true)
  assert.ok(String(target.title).includes('阳光电源(sz300274)'))
}

{
  const { applyStockClickAction } = mod
  const m = toStockDisplayModel({ stock_code: 'NOT_A_CODE', stock_name: 'X' })
  const target = { visible: false, title: '', chartCode: '', stockName: '' }
  assert.equal(applyStockClickAction(m, target), false)
  assert.equal(target.visible, false)
}

console.log('stockDisplay.test.mjs: ok')
