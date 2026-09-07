import assert from 'node:assert/strict'
import test from 'node:test'
import {
  calcReturnSinceSignal,
  enrichHitToRow,
  formatPctWithSign,
  isDerivedSignalPriceStatus,
  isMissingSignalPriceStatus,
  OPPORTUNITY_PRICE_FOOTER,
  resolveLiveChangeRate,
  resolveSignalFields,
  resolveSnapshotChange,
  SIGNAL_PRICE_DERIVED_TOOLTIP,
  SIGNAL_PRICE_TOOLTIP,
  signalPriceTooltipForStatus,
  vsSignalReturnMissingHint,
} from './opportunityListMetrics.js'

test('calcReturnSinceSignal', () => {
  assert.equal(calcReturnSinceSignal(12, 10), 20)
  assert.equal(calcReturnSinceSignal(null, 10), null)
})

test('formatPctWithSign', () => {
  assert.equal(formatPctWithSign(19.98), '+19.98%')
  assert.equal(formatPctWithSign(-2.5), '-2.50%')
})

test('resolveSnapshotChange with date', () => {
  const snap = resolveSnapshotChange(
    { SNAPSHOT_CHANGE_RATE: '19.98', SNAPSHOT_TRADE_DATE: '2026-08-28' },
    '2026-08-29',
  )
  assert.equal(snap.rate, 19.98)
  assert.equal(snap.date, '2026-08-28')
})

test('resolveSignalFields prefers row signal_price over quote', () => {
  const fields = resolveSignalFields(
    { SIGNAL_PRICE: 14.2, SIGNAL_TIME: '2026-08-27', NEW_PRICE: '15.88' },
    null,
  )
  assert.equal(fields.signalPrice, 14.2)
  assert.equal(fields.signalTime, '2026-08-27')
  assert.equal(fields.signalPriceStatus, 'frozen')
})

test('resolveSignalFields derived from buyPriceRange fallback', () => {
  const fields = resolveSignalFields(
    { NEW_PRICE: '15.88' },
    { buyPriceRange: { instantPrice: 14.5, daysAgo: 2 }, recentSignalDaysAgo: 2 },
  )
  assert.equal(fields.signalPrice, 14.5)
  assert.equal(fields.signalPriceStatus, 'derived')
  assert.ok(isDerivedSignalPriceStatus(fields.signalPriceStatus))
})

test('resolveSignalFields falls back to signalSummary.signalPrice', () => {
  const fields = resolveSignalFields(
    { NEW_PRICE: '14.07', CHANGE_RATE: '10.09', SECUCODE: '301125.SZ' },
    {
      signalPrice: 14.07,
      signalTime: '2026-08-31',
      signalPriceStatus: 'frozen',
      recentSignalDaysAgo: 0,
    },
  )
  assert.equal(fields.signalPrice, 14.07)
  assert.equal(fields.signalTime, '2026-08-31')
  assert.equal(fields.signalPriceStatus, 'frozen')
})

test('resolveSignalFields prefers row price over signalSummary', () => {
  const fields = resolveSignalFields(
    { SIGNAL_PRICE: 14.2, SIGNAL_TIME: '2026-08-27' },
    { signalPrice: 13.0, signalTime: '2026-08-20' },
  )
  assert.equal(fields.signalPrice, 14.2)
  assert.equal(fields.signalTime, '2026-08-27')
})

test('resolveSignalFields missing when no signal price', () => {
  const fields = resolveSignalFields({ NEW_PRICE: '10' }, null)
  assert.equal(fields.signalPrice, null)
  assert.ok(isMissingSignalPriceStatus(fields.signalPriceStatus))
})

test('signalPriceTooltipForStatus', () => {
  assert.match(signalPriceTooltipForStatus('frozen'), /非买入价/)
  assert.match(signalPriceTooltipForStatus('frozen'), /非委托价/)
  assert.match(signalPriceTooltipForStatus('frozen'), /非成交价/)
  assert.equal(signalPriceTooltipForStatus('derived'), SIGNAL_PRICE_DERIVED_TOOLTIP)
  assert.equal(signalPriceTooltipForStatus('frozen'), SIGNAL_PRICE_TOOLTIP)
})

test('vsSignalReturnMissingHint', () => {
  assert.match(vsSignalReturnMissingHint(null, 'missing'), /未记录/)
  assert.equal(vsSignalReturnMissingHint(10, 'frozen'), null)
})

test('OPPORTUNITY_PRICE_FOOTER mentions disclaimer', () => {
  assert.match(OPPORTUNITY_PRICE_FOOTER, /不构成买卖建议/)
  assert.match(OPPORTUNITY_PRICE_FOOTER, /委托价/)
})

test('resolveLiveChangeRate separates snapshot view', () => {
  assert.equal(
    resolveLiveChangeRate({ CHANGE_RATE: '3.3' }, { changeRate: 1.1 }, { isSnapshotView: true }),
    1.1,
  )
  assert.equal(
    resolveLiveChangeRate({ CHANGE_RATE: '3.3' }, null, { isSnapshotView: true }),
    null,
  )
})

test('enrichHitToRow carries signal fields', () => {
  const row = enrichHitToRow(
    {
      SECUCODE: '600036.SH',
      SECURITY_NAME_ABBR: '招商银行',
      NEW_PRICE: '38.65',
      CHANGE_RATE: '1.2',
      signal_price: 38.12,
      signal_time: '2026-08-27',
    },
    '2026-08-29',
  )
  assert.equal(row.SIGNAL_PRICE, 38.12)
  assert.equal(row.SNAPSHOT_TRADE_DATE, '2026-08-29')
  assert.equal(row.SNAPSHOT_CHANGE_RATE, '1.2')
})
