import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const panel = readFileSync(join(root, 'components/MultiStrategyComparePanel.vue'), 'utf8')
const modal = readFileSync(join(root, 'components/StockKlineModal.vue'), 'utf8')
const list = readFileSync(join(root, 'components/allStockList.vue'), 'utf8')
const research = readFileSync(join(root, 'components/researchIndex.vue'), 'utf8')

assert.match(panel, /stockCode:\s*\{\s*type:\s*String/)
assert.match(panel, /stockName:\s*\{\s*type:\s*String/)
assert.match(panel, /placeholder="代码或名称，如 600519 \/ 贵州茅台"/)
assert.match(panel, /v-if="!embedded"/)
assert.match(panel, /runCompare\(\{\s*code:\s*embeddedCode\.value,\s*name:\s*embeddedName\.value\s*\}\)/)
assert.equal(panel.includes('TradePlan'), false)
assert.equal(panel.includes('CreateOrder'), false)

assert.match(modal, /tab="多策略对照"/)
assert.match(modal, /display-directive="if"/)
assert.match(modal, /defineAsyncComponent\(\(\) => import\('\.\/MultiStrategyComparePanel\.vue'\)\)/)
assert.match(modal, /v-else-if="show && activeTab === 'compare'"/)
assert.match(modal, /:stock-code="resolvedCode"/)
assert.match(modal, /:stock-name="resolvedStockName"/)
assert.equal(modal.includes('TradePlan'), false)
assert.equal(modal.includes('CreateOrder'), false)

assert.match(list, /onClick: \(\) => showKline\(row, '', 'compare'\)/)
assert.match(list, /\{ default: \(\) => '多策略' \}/)
assert.match(list, /:initial-tab="modalDataRef\.initialTab"/)
assert.match(list, /:open-token="modalDataRef\.openToken"/)
assert.match(list, /initialTab: "kline"/)

assert.match(research, /<MultiStrategyComparePanel \/>/)
assert.equal(/<MultiStrategyComparePanel[^>]*stock-code/.test(research), false)

console.log('klineCompareEmbed.test.mjs ok')
