/**
 * Phase14-H1: signal scan loading helpers
 * Run: node frontend/src/utils/signalScanTaskLoading.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'signalScanTaskLoading.js')).href
const {
  isSignalScanTaskTerminalStatus,
  resolveSignalScanTaskPollOutcome,
  resolveStockScreenTableLoading,
} = await import(modUrl)

// completed → release signalScanLoading
{
  const out = resolveSignalScanTaskPollOutcome('completed')
  assert.equal(out.shouldStopPoll, true)
  assert.equal(out.releaseBackendLoading, true)
  assert.equal(out.releaseSignalScanLoading, true)
  assert.equal(out.clearScanStatus, true)
}

// failed → release signalScanLoading
{
  const out = resolveSignalScanTaskPollOutcome('failed')
  assert.equal(out.shouldStopPoll, true)
  assert.equal(out.releaseSignalScanLoading, true)
}

// cancelled → release signalScanLoading
{
  const out = resolveSignalScanTaskPollOutcome('cancelled')
  assert.equal(out.shouldStopPoll, true)
  assert.equal(out.releaseSignalScanLoading, true)
}

// running → keep polling
{
  const out = resolveSignalScanTaskPollOutcome('running')
  assert.equal(out.shouldStopPoll, false)
  assert.equal(out.releaseSignalScanLoading, false)
}

// snapshot visible: table loading must not follow background scan flag
{
  assert.equal(resolveStockScreenTableLoading(false), false)
  assert.equal(resolveStockScreenTableLoading(true), true)
}

assert.equal(isSignalScanTaskTerminalStatus('COMPLETED'), true)
assert.equal(isSignalScanTaskTerminalStatus('running'), false)

console.log('signalScanTaskLoading.test.mjs: all passed')
