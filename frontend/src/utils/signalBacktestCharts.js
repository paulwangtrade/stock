/**
 * 把回测引擎的 equityCurve / trades 转成 ECharts option。
 * 不负责拉行情，只消费 runDailySignalBacktest 的返回值。
 */

export function formatBacktestMoney(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '--'
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatAxisMoney(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return ''
  const abs = Math.abs(n)
  if (abs >= 100000000) return `${(n / 100000000).toFixed(2)}亿`
  if (abs >= 10000) return `${(n / 10000).toFixed(1)}万`
  return String(Math.round(n))
}

function shortDay(day) {
  const text = String(day || '')
  return text.length >= 10 ? text.slice(5) : text
}

function themeColors(dark) {
  return {
    text: dark ? '#d0d0d0' : '#555',
    split: dark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)',
    muted: dark ? '#888' : '#aaa',
  }
}

function curveOf(run) {
  return Array.isArray(run?.equityCurve) ? run.equityCurve : []
}

/**
 * @param {object|null} primary runDailySignalBacktest 结果（含 equityCurve、initialCash）
 * @param {object|null} compare 无纪律对照，结构相同
 */
export function buildEquityChartOption(primary, compare, opts = {}) {
  const dark = !!opts.dark
  const colors = themeColors(dark)
  const curve = curveOf(primary)
  const days = curve.map((point) => String(point.day || ''))
  const showCompare = opts.showCompare !== false && Array.isArray(compare?.equityCurve) && compare.equityCurve.length > 0
  const compareByDay = new Map()
  if (showCompare) {
    for (const point of compare.equityCurve) compareByDay.set(String(point.day || ''), Number(point.equity))
  }

  const series = [
    {
      name: '本次回测',
      type: 'line',
      showSymbol: false,
      smooth: false,
      data: curve.map((point) => Number(point.equity)),
      lineStyle: { width: 2, color: '#f0a020' },
      itemStyle: { color: '#f0a020' },
    },
  ]
  if (showCompare) {
    series.push({
      name: '无纪律对照',
      type: 'line',
      showSymbol: false,
      connectNulls: true,
      data: days.map((day) => {
        const value = compareByDay.get(day)
        return value == null ? null : value
      }),
      lineStyle: { width: 1.5, type: 'dashed', color: '#70c0e8' },
      itemStyle: { color: '#70c0e8' },
    })
  }

  const initial = Number(primary?.initialCash)
  if (Number.isFinite(initial) && initial > 0 && series[0]) {
    series[0].markLine = {
      symbol: 'none',
      label: { formatter: '初始资金', color: colors.text, fontSize: 11 },
      lineStyle: { type: 'dotted', color: colors.muted },
      data: [{ yAxis: initial }],
    }
  }

  return {
    animation: false,
    backgroundColor: 'transparent',
    legend: {
      top: 0,
      textStyle: { color: colors.text, fontSize: 12 },
      data: series.map((item) => item.name),
    },
    grid: { left: 8, right: 12, top: 32, bottom: 4, containLabel: true },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value) => formatBacktestMoney(value),
    },
    xAxis: {
      type: 'category',
      data: days,
      boundaryGap: false,
      axisLabel: { color: colors.text, hideOverlap: true, formatter: shortDay },
      axisLine: { lineStyle: { color: colors.split } },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: { color: colors.text, formatter: formatAxisMoney },
      splitLine: { lineStyle: { color: colors.split } },
    },
    series,
  }
}

/**
 * 用权益曲线里已有的 close，以及 trades 的买卖价，画价格与成交点。
 * @param {object|null} primary
 */
export function buildPriceTradeChartOption(primary, opts = {}) {
  const dark = !!opts.dark
  const colors = themeColors(dark)
  const curve = curveOf(primary)
  const days = curve.map((point) => String(point.day || ''))
  const daySet = new Set(days)
  const buys = []
  const sells = []
  for (const trade of primary?.trades || []) {
    const day = String(trade?.day || '')
    const price = Number(trade?.price)
    if (!daySet.has(day) || !(price > 0)) continue
    const point = { value: [day, price], qty: trade.qty, tag: trade.tag || '' }
    if (trade.side === 'buy') buys.push(point)
    else if (trade.side === 'sell') sells.push(point)
  }

  return {
    animation: false,
    backgroundColor: 'transparent',
    legend: {
      top: 0,
      textStyle: { color: colors.text, fontSize: 12 },
    },
    grid: { left: 8, right: 12, top: 32, bottom: 4, containLabel: true },
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        const list = Array.isArray(params) ? params : [params]
        const day = list[0]?.axisValueLabel || list[0]?.axisValue || ''
        const lines = [String(day)]
        for (const item of list) {
          const raw = item?.data?.value ?? item?.data
          const num = Array.isArray(raw) ? raw[1] : raw
          if (num == null || num === '' || Number.isNaN(Number(num))) continue
          if (item.seriesName === '收盘价') {
            lines.push(`${item.marker}${item.seriesName} ${Number(num).toFixed(2)}`)
          } else {
            const qty = item?.data?.qty
            lines.push(`${item.marker}${item.seriesName} ${Number(num).toFixed(2)}${qty ? ` × ${qty}` : ''}`)
          }
        }
        return lines.join('<br/>')
      },
    },
    xAxis: {
      type: 'category',
      data: days,
      boundaryGap: false,
      axisLabel: { color: colors.text, hideOverlap: true, formatter: shortDay },
      axisLine: { lineStyle: { color: colors.split } },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: {
        color: colors.text,
        formatter: (value) => {
          const n = Number(value)
          return Number.isFinite(n) ? n.toFixed(2) : ''
        },
      },
      splitLine: { lineStyle: { color: colors.split } },
    },
    series: [
      {
        name: '收盘价',
        type: 'line',
        showSymbol: false,
        data: curve.map((point) => Number(point.close)),
        lineStyle: { width: 1.5, color: '#8b8b8b' },
        itemStyle: { color: '#8b8b8b' },
      },
      {
        name: '买入',
        type: 'scatter',
        data: buys,
        symbol: 'triangle',
        symbolSize: 11,
        itemStyle: { color: '#e23d3d' },
        z: 3,
      },
      {
        name: '卖出',
        type: 'scatter',
        data: sells,
        symbol: 'triangle',
        symbolRotate: 180,
        symbolSize: 11,
        itemStyle: { color: '#18a058' },
        z: 3,
      },
    ],
  }
}
