/**
 * K-line chart marker framework for future T-strategy overlays.
 * Observation-only: markers may be empty; no trading signals generated here.
 */

/** @typedef {'BUY'|'SELL'} ChartMarkerType */

/**
 * @typedef {object} ChartMarker
 * @property {string} time  HH:mm or bar timestamp label
 * @property {number} price
 * @property {ChartMarkerType} type
 * @property {string} [reason] future strategy explanation
 */

/**
 * @param {ChartMarkerType} type
 * @returns {string}
 */
export function chartMarkerLabel(type) {
  const t = String(type || '').toUpperCase()
  if (t === 'BUY') return 'T买'
  if (t === 'SELL') return 'T卖'
  return t || '—'
}

/**
 * @param {ChartMarkerType} type
 * @returns {'marker-buy'|'marker-sell'|'marker-neutral'}
 */
export function chartMarkerClass(type) {
  const t = String(type || '').toUpperCase()
  if (t === 'BUY') return 'marker-buy'
  if (t === 'SELL') return 'marker-sell'
  return 'marker-neutral'
}

/**
 * Build SVG chart model from minute bars + optional markers.
 * @param {Array<{ day?: string, close?: number, low?: number, high?: number }>} bars
 * @param {ChartMarker[]} [markers]
 * @param {{ width?: number, height?: number, pad?: number }} [opts]
 */
export function buildIntradayChartModel(bars, markers = [], opts = {}) {
  const width = opts.width ?? 720
  const height = opts.height ?? 220
  const pad = opts.pad ?? 24
  const list = Array.isArray(bars) ? bars : []
  if (!list.length) {
    return { points: '', markers: [], min: 0, max: 0, width, height }
  }

  const lows = list.map((bar) => Number(bar.low ?? bar.close ?? 0))
  const highs = list.map((bar) => Number(bar.high ?? bar.close ?? 0))
  const min = Math.min(...lows)
  const max = Math.max(...highs)
  const span = Math.max(max - min, max * 0.002, 0.01)
  const xAt = (index) => pad + index * ((width - pad * 2) / Math.max(1, list.length - 1))
  const yAt = (price) => height - pad - ((Number(price) - min) / span) * (height - pad * 2)
  const points = list.map((bar, index) => `${xAt(index)},${yAt(bar.close)}`).join(' ')

  const placed = (Array.isArray(markers) ? markers : []).map((marker) => {
    const hhmm = String(marker.time || '').slice(0, 5)
    let index = list.findLastIndex((bar) => String(bar.day || '').includes(hhmm))
    if (index < 0) index = list.length - 1
    return {
      ...marker,
      label: chartMarkerLabel(marker.type),
      className: chartMarkerClass(marker.type),
      x: xAt(index),
      y: yAt(marker.price),
    }
  })

  return { points, markers: placed, min, max, width, height }
}
