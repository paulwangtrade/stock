/**
 * 多策略对照：股票身份解析
 * Run: node frontend/src/utils/signalBacktestIdentity.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const { isStockCodeQuery, resolveStockIdentity } = await import(
  pathToFileURL(join(__dir, 'signalBacktestIdentity.js')).href
)

assert.equal(isStockCodeQuery('600519'), true)
assert.equal(isStockCodeQuery('sh600519'), true)
assert.equal(isStockCodeQuery('600519.SH'), true)
assert.equal(isStockCodeQuery('贵州茅台'), false)

{
  const out = resolveStockIdentity('600519', [
    { ts_code: '600519.SH', symbol: '600519', name: '贵州茅台' },
  ])
  assert.equal(out.status, 'resolved')
  assert.equal(out.code, '600519.SH')
  assert.equal(out.name, '贵州茅台')
}

{
  const out = resolveStockIdentity('sh600519', [])
  assert.equal(out.status, 'code_only')
  assert.equal(out.code, '600519.SH')
  assert.equal(out.name, '')
}

{
  const out = resolveStockIdentity('贵州茅台', [
    { ts_code: '600519.SH', symbol: '600519', name: '贵州茅台' },
  ])
  assert.equal(out.status, 'resolved')
  assert.equal(out.code, '600519.SH')
  assert.equal(out.name, '贵州茅台')
}

{
  const out = resolveStockIdentity('平安', [
    { ts_code: '601318.SH', symbol: '601318', name: '中国平安' },
    { ts_code: '000001.SZ', symbol: '000001', name: '平安银行' },
  ])
  assert.equal(out.status, 'ambiguous')
  assert.equal(out.candidates.length, 2)
  assert.equal(out.code, '')
}

{
  const out = resolveStockIdentity('宁德', [
    { ts_code: '300750.SZ', symbol: '300750', name: '宁德时代' },
  ])
  assert.equal(out.status, 'resolved')
  assert.equal(out.name, '宁德时代')
  assert.equal(out.code, '300750.SZ')
}

{
  const out = resolveStockIdentity('不存在的公司', [])
  assert.equal(out.status, 'missing')
}

{
  const out = resolveStockIdentity('   ', [{ name: '贵州茅台', ts_code: '600519.SH' }])
  assert.equal(out.status, 'missing')
}

{
  const out = resolveStockIdentity('同名', [
    { ts_code: '600000.SH', symbol: '600000', name: '同名' },
    { ts_code: '000001.SZ', symbol: '000001', name: '同名' },
  ])
  assert.equal(out.status, 'ambiguous')
  assert.equal(out.candidates.length, 2)
}

console.log('signalBacktestIdentity.test.mjs ok')
