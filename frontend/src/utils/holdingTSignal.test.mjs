/**
 * Minimal unit checks for holdingTSignal (node --test / vite-node style).
 */
import assert from 'node:assert/strict'
import {
  buildHoldingTSignal,
  holdingTSignalsToChartMarkers,
  T_SIGNAL_BUY_WATCH,
  T_SIGNAL_SELL_WATCH,
  T_SIGNAL_REASON,
} from './holdingTSignal.js'

function barsPullback() {
  return [
    { time: '10:00', high: 10.1, low: 10.0, close: 10.05 },
    { time: '10:05', high: 10.15, low: 10.05, close: 10.12 },
    { time: '10:10', high: 10.2, low: 10.1, close: 10.18 },
    { time: '10:15', high: 10.18, low: 10.08, close: 10.1 },
    { time: '10:20', high: 10.12, low: 10.02, close: 10.04 },
    { time: '10:25', high: 10.08, low: 9.98, close: 10.0 },
    { time: '10:30', high: 10.05, low: 9.97, close: 9.99 },
    { time: '10:35', high: 10.06, low: 9.98, close: 10.03 },
  ]
}

function barsRiseFade() {
  return [
    { time: '11:00', high: 10.05, low: 9.95, close: 10.0 },
    { time: '11:05', high: 10.1, low: 10.0, close: 10.08 },
    { time: '11:10', high: 10.15, low: 10.05, close: 10.12 },
    { time: '11:15', high: 10.2, low: 10.1, close: 10.18 },
    { time: '11:20', high: 10.25, low: 10.15, close: 10.22 },
    { time: '11:25', high: 10.28, low: 10.18, close: 10.26 },
    { time: '11:30', high: 10.3, low: 10.2, close: 10.28 },
    { time: '11:35', high: 10.29, low: 10.18, close: 10.2 },
  ]
}

{
  const res = buildHoldingTSignal({
    stockCode: 'sh600000',
    hasPosition: true,
    canSell: true,
    availableQty: 1000,
    costPrice: 10,
    freshness: 'FRESH',
    suitabilityLevel: 'suitable',
    healthGrade: 'B',
    bars: barsPullback(),
  })
  assert.ok(res.signals.some((s) => s.signal_type === T_SIGNAL_BUY_WATCH))
}

{
  const res = buildHoldingTSignal({
    stockCode: 'sz000001',
    hasPosition: true,
    canSell: true,
    availableQty: 500,
    costPrice: 10,
    freshness: 'FRESH',
    bars: barsRiseFade(),
  })
  assert.ok(res.signals.some((s) => s.signal_type === T_SIGNAL_SELL_WATCH))
}

{
  const res = buildHoldingTSignal({
    hasPosition: true,
    canSell: false,
    availableQty: 0,
    costPrice: 10,
    freshness: 'FRESH',
    bars: barsPullback(),
  })
  assert.ok(res.notes.includes(T_SIGNAL_REASON.CANNOT_SELL))
  assert.ok(!res.signals.some((s) => s.signal_type === T_SIGNAL_BUY_WATCH))
}

{
  const res = buildHoldingTSignal({
    hasPosition: true,
    canSell: true,
    costPrice: 10,
    freshness: 'STALE',
    bars: barsPullback(),
  })
  assert.equal(res.signals.length, 0)
  assert.ok(res.notes.includes(T_SIGNAL_REASON.PRICE_STALE))
}

{
  const res = buildHoldingTSignal({
    hasPosition: true,
    canSell: true,
    costPrice: 10,
    freshness: 'FRESH',
    bars: [{ close: 10, high: 10.1, low: 9.9 }],
  })
  assert.ok(res.notes.includes(T_SIGNAL_REASON.NO_5M_BARS))
}

{
  const res = buildHoldingTSignal({ hasPosition: false, bars: barsPullback(), costPrice: 10 })
  assert.ok(res.notes.includes(T_SIGNAL_REASON.NO_POSITION))
}

{
  const markers = holdingTSignalsToChartMarkers([
    { signal_type: T_SIGNAL_BUY_WATCH, signal_time: '10:35', signal_price: 10.03, reasons: ['NEAR_COST'] },
  ])
  assert.equal(markers[0].type, 'BUY')
  assert.equal(markers[0].time, '10:35')
}

console.log('holdingTSignal.test.mjs: ok')
