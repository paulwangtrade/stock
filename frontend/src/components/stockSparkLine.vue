<script setup>
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { GetStockMinutePriceLineData } from '../../wailsjs/go/main/App'

const props = defineProps({
  idSuffix: { type: String, default: '' },
  stockCode: { type: String, default: '' },
  stockName: { type: String, default: '' },
  lastPrice: { type: Number, default: 0 },
  openPrice: { type: Number, default: 0 },
  darkTheme: { type: Boolean, default: true },
  /** 为 false 时不请求分时数据（由父级可视区/hover 控制） */
  active: { type: Boolean, default: true },
})

const hostRef = ref(null)
const chart = ref(null)
let loadSeq = 0
let disposed = false

function applyLineColor(instance) {
  if (!instance) return
  const up = Number(props.lastPrice) > Number(props.openPrice)
  const stroke = up ? 'rgba(245, 0, 0, 1)' : 'rgb(6,251,10)'
  const fillTop = up ? 'rgba(245, 0, 0, 1)' : 'rgba(6,251,10, 1)'
  const fillBottom = up ? 'rgba(245, 0, 0, 0.25)' : 'rgba(6,251,10, 0.25)'
  instance.setOption({
    series: [{
      lineStyle: { color: stroke },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: fillTop },
          { offset: 1, color: fillBottom },
        ]),
      },
    }],
  })
}

async function loadMinuteLine() {
  if (!props.active || !props.stockCode || disposed) return
  const seq = ++loadSeq
  if (!chart.value && hostRef.value) {
    chart.value = echarts.init(hostRef.value)
  }
  const instance = chart.value
  if (!instance) return
  try {
    const result = await GetStockMinutePriceLineData(props.stockCode, props.stockName)
    if (disposed || seq !== loadSeq || !props.active) return
    const priceData = result?.priceData || []
    const category = []
    const price = []
    let min = 0
    let max = 0
    for (let i = 0; i < priceData.length; i++) {
      category.push(priceData[i].time)
      price.push(priceData[i].price)
      if (min === 0 || min > priceData[i].price) min = priceData[i].price
      if (max < priceData[i].price) max = priceData[i].price
    }
    instance.setOption({
      padding: [0, 0, 0, 0],
      grid: { top: 0, left: 0, right: 0, bottom: 0 },
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross', label: { backgroundColor: '#6a7985' } },
      },
      xAxis: { show: false, type: 'category', data: category },
      yAxis: {
        show: false,
        type: 'value',
        min: Number(min).toFixed(2),
        max: Number(max).toFixed(2),
        minInterval: 0.01,
      },
      series: [{
        data: price,
        type: 'line',
        smooth: false,
        stack: '总量',
        showSymbol: false,
        lineStyle: {
          color: Number(props.lastPrice) > Number(props.openPrice)
            ? 'rgba(245, 0, 0, 1)'
            : 'rgb(6,251,10)',
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
            offset: 0,
            color: Number(props.lastPrice) > Number(props.openPrice)
              ? 'rgba(245, 0, 0, 1)'
              : 'rgba(6,251,10, 1)',
          }, {
            offset: 1,
            color: Number(props.lastPrice) > Number(props.openPrice)
              ? 'rgba(245, 0, 0, 0.25)'
              : 'rgba(6,251,10, 0.25)',
          }]),
        },
      }],
    })
  } catch {
    /* 迷你分时失败时静默，避免打断自选列表 */
  }
}

function ensureChart() {
  if (!hostRef.value || disposed) return
  if (!chart.value) chart.value = echarts.init(hostRef.value)
}

onMounted(() => {
  ensureChart()
  if (props.active) loadMinuteLine()
})

onBeforeUnmount(() => {
  disposed = true
  loadSeq += 1
  if (chart.value) {
    chart.value.dispose()
    chart.value = null
  }
})

watch(() => props.active, (active) => {
  if (active) {
    ensureChart()
    loadMinuteLine()
  } else {
    loadSeq += 1
  }
})

watch(
  () => [props.stockCode, props.stockName],
  () => {
    if (props.active) loadMinuteLine()
  },
)

watch(
  () => [props.lastPrice, props.openPrice],
  () => applyLineColor(chart.value),
)
</script>

<template>
  <div
    ref="hostRef"
    class="spark-line-host"
    :id="'sparkLine' + stockCode + idSuffix"
  />
</template>

<style scoped>
.spark-line-host {
  height: 20px;
  width: 100%;
}
</style>
