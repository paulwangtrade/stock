import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const modUrl = pathToFileURL(join(root, 'src/utils/signalScanTaskLoading.js')).href
const { resolveSignalScanTaskPollOutcome, resolveStockScreenTableLoading } = await import(modUrl)

for (const status of ['completed', 'failed', 'cancelled']) {
  const out = resolveSignalScanTaskPollOutcome(status)
  assert.equal(out.releaseSignalScanLoading, true, `${status} must release signalScanLoading`)
}

assert.equal(
  resolveStockScreenTableLoading(false),
  false,
  'snapshot rows visible: table must not spin when only background scan is active',
)

const vue = readFileSync(join(root, 'src/components/allStockList.vue'), 'utf8')
assert.match(vue, /:loading="tableLoading"/, 'NDataTable must use tableLoading, not signalScanLoading')
assert.doesNotMatch(
  vue,
  /:loading="loadingRef \|\| signalScanLoading"/,
  'NDataTable must not OR-merge signalScanLoading',
)

console.log('verify-opportunity-loading: ok')
