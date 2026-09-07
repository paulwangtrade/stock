<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { GetFollowRealtimeList, GetStockEastMoneyKLine } from '../../wailsjs/go/main/App'
import { buildIntradayChartModel } from '../utils/chartMarkers'
import { getOrFetch, klineCacheKey } from '../utils/klineCache'

const KLINE_TIMEFRAME = '5'
const KLINE_BARS = 120
const KLINE_CACHE_TTL_MS = 5 * 60 * 1000

const message = useMessage()
const holdings = ref([])
const selectedCode = ref('')
const loadingList = ref(false)
const loadingKline = ref(false)
const klineError = ref('')
const klineBarsByCode = ref({})
/** @type {import('vue').Ref<Record<string, import('../utils/chartMarkers.js').ChartMarker[]>>} */
const chartMarkersByCode = ref({})
let refreshTimer = null

function field(row, ...keys) {
  for (const key of keys) {
    if (row?.[key] !== undefined && row?.[key] !== null && row?.[key] !== '') return row[key]
  }
  return undefined
}

function normalizeHolding(row) {
  const code = String(field(row, '股票代码', 'StockCode', 'stockCode', 'code', 'Code') || '').trim()
  const costVolume = Number(field(row, 'costVolume', 'CostVolume', 'Volume', 'volume')) || 0
  if (!code || costVolume <= 0) return null
  const costPrice = Number(field(row, 'costPrice', 'CostPrice')) || 0
  const price = Number(field(row, '当前价格', 'Price', 'price', 'close')) || 0
  const profit = (price - costPrice) * costVolume
  const profitRate = costPrice > 0 ? ((price / costPrice) - 1) * 100 : 0
  return {
    ...row,
    code,
    name: String(field(row, '股票名称', 'StockName', 'stockName', 'name', 'Name') || code),
    costVolume,
    costPrice,
    price,
    profit,
    profitRate,
  }
}

const selected = computed(() => holdings.value.find((item) => item.code === selectedCode.value) || null)
const klineBars = computed(() => klineBarsByCode.value[selectedCode.value] || [])
const chartMarkers = computed(() => chartMarkersByCode.value[selectedCode.value] || [])
const chartModel = computed(() => buildIntradayChartModel(klineBars.value.slice(-36), chartMarkers.value))
const latestBars = computed(() => klineBars.value.slice(-18).reverse())

function formatPrice(value) {
  return Number.isFinite(Number(value)) ? Number(value).toFixed(2) : '--'
}

function profitClass(value) {
  const n = Number(value)
  if (!Number.isFinite(n) || n === 0) return ''
  return n > 0 ? 'profit-up' : 'profit-down'
}

function formatProfit(value, rate) {
  const n = Number(value)
  const r = Number(rate)
  if (!Number.isFinite(n)) return '--'
  const sign = n >= 0 ? '+' : ''
  const pct = Number.isFinite(r) ? `${sign}${r.toFixed(2)}%` : ''
  return `${pct} ${sign}${Math.round(n)}元`.trim()
}

async function fetchKlineRows(code, name) {
  const key = klineCacheKey(code, KLINE_TIMEFRAME)
  return getOrFetch(
    key,
    () => GetStockEastMoneyKLine(code, name, KLINE_TIMEFRAME, KLINE_BARS),
    KLINE_CACHE_TTL_MS,
    { shouldCache: (data) => Array.isArray(data) && data.length > 0 },
  )
}

async function refreshHoldings() {
  loadingList.value = true
  try {
    const raw = await GetFollowRealtimeList(0)
    const next = (Array.isArray(raw) ? raw : []).map(normalizeHolding).filter(Boolean)
    holdings.value = next
    if (!next.some((item) => item.code === selectedCode.value)) {
      selectedCode.value = next[0]?.code || ''
    }
    if (selectedCode.value) {
      loadKlineForSelected({ silent: true })
    }
  } catch (error) {
    message.error(error?.message || String(error))
  } finally {
    loadingList.value = false
  }
}

async function loadKlineForSelected(opts = {}) {
  const holding = selected.value
  if (!holding) return
  loadingKline.value = true
  if (!opts.silent) klineError.value = ''
  try {
    const raw = await fetchKlineRows(holding.code, holding.name)
    klineBarsByCode.value = {
      ...klineBarsByCode.value,
      [holding.code]: Array.isArray(raw) ? raw : [],
    }
  } catch (error) {
    klineError.value = `${holding.name} 行情加载失败：${error?.message || String(error)}`
    if (!opts.silent) message.error(klineError.value)
  } finally {
    loadingKline.value = false
  }
}

async function selectHolding(code) {
  selectedCode.value = code
  if (!klineBarsByCode.value[code]) {
    await loadKlineForSelected()
  }
}

onMounted(() => {
  refreshHoldings()
  refreshTimer = window.setInterval(() => {
    if (selectedCode.value) loadKlineForSelected({ silent: true })
  }, 30_000)
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
})
</script>

<template>
  <div class="holding-t-panel">
    <div class="toolbar">
      <div>
        <n-text strong>持仓辅助决策工具</n-text>
        <n-text depth="3" class="subtitle">观察用 · 不自动下单 · 等待 T 策略信号模型接入</n-text>
      </div>
      <n-button size="small" :loading="loadingList" @click="refreshHoldings">刷新持仓</n-button>
    </div>

    <div class="layout">
      <aside class="holding-list">
        <n-spin :show="loadingList">
          <button
            v-for="item in holdings"
            :key="item.code"
            class="holding-item"
            :class="{ active: selectedCode === item.code }"
            type="button"
            @click="selectHolding(item.code)"
          >
            <span><b>{{ item.name }}</b><small>{{ item.code }}</small></span>
            <span class="holding-nums">
              <em>{{ formatPrice(item.price) }}</em>
              <small>{{ item.costVolume }}股 · 成本{{ formatPrice(item.costPrice) }}</small>
              <small :class="profitClass(item.profit)">{{ formatProfit(item.profit, item.profitRate) }}</small>
            </span>
          </button>
          <n-empty v-if="!holdings.length && !loadingList" description="未找到 costVolume > 0 的持仓" />
        </n-spin>
      </aside>

      <main class="detail">
        <n-empty v-if="!selected" description="请选择持仓" />
        <template v-else>
          <n-card size="small" class="holding-summary" title="当前持仓">
            <div class="summary-grid">
              <div><small>成本</small><b>{{ formatPrice(selected.costPrice) }}</b></div>
              <div><small>数量</small><b>{{ selected.costVolume }} 股</b></div>
              <div><small>当前价格</small><b>{{ formatPrice(selected.price) }}</b></div>
              <div>
                <small>浮盈</small>
                <b :class="profitClass(selected.profit)">
                  {{ formatProfit(selected.profit, selected.profitRate) }}
                </b>
              </div>
            </div>
          </n-card>

          <n-card size="small" class="t-observe-card" title="T 操作观察区">
            <n-text depth="3" style="display: block; margin-bottom: 8px">
              定位：持仓辅助决策工具，用于观察与复盘；不产生自动买卖指令。
            </n-text>
            <div class="t-strategy-placeholder">
              <n-text strong>T 策略观察</n-text>
              <n-tag size="small" type="default" :bordered="false">等待信号模型接入</n-tag>
              <n-text depth="3" style="display: block; margin-top: 8px">
                当前：未生成交易建议
              </n-text>
              <n-text depth="3" style="display: block; margin-top: 4px; font-size: 12px">
                未来可在此展示 ↑T买 / ↓T卖 标记与策略解释（ChartMarker.reason）。
              </n-text>
            </div>
          </n-card>

          <n-card size="small" title="5 分钟 K 线" class="intraday-chart-card">
            <n-space align="center" :wrap="true" style="margin-bottom: 8px">
              <n-button size="tiny" :loading="loadingKline" @click="loadKlineForSelected()">刷新行情</n-button>
              <n-text v-if="loadingKline" depth="3" style="font-size: 12px">正在加载行情…</n-text>
            </n-space>
            <n-tag v-if="klineError" type="warning" :bordered="false" style="margin-bottom: 8px">{{ klineError }}</n-tag>
            <n-spin :show="loadingKline && !klineBars.length">
              <svg
                v-if="chartModel.points"
                class="intraday-chart"
                :viewBox="`0 0 ${chartModel.width} ${chartModel.height}`"
                role="img"
                aria-label="5分钟价格曲线"
              >
                <line x1="24" y1="196" x2="696" y2="196" class="chart-axis" />
                <polyline :points="chartModel.points" class="chart-line" />
                <g v-for="(marker, index) in chartModel.markers" :key="`${marker.type}-${marker.time}-${index}`">
                  <circle :cx="marker.x" :cy="marker.y" r="6" :class="marker.className" />
                  <text :x="marker.x + 8" :y="marker.y - 8" class="chart-label">
                    {{ marker.label }} {{ formatPrice(marker.price) }}
                  </text>
                </g>
                <text x="26" y="18" class="chart-scale">{{ formatPrice(chartModel.max) }}</text>
                <text x="26" y="214" class="chart-scale">{{ formatPrice(chartModel.min) }}</text>
              </svg>
              <n-empty v-else-if="!loadingKline" size="small" description="暂无可绘制的分钟数据" />
            </n-spin>
          </n-card>

          <n-card v-if="latestBars.length" size="small" title="最近 5 分钟价位">
            <div class="price-table">
              <div class="price-row header"><span>时间</span><span>收盘</span><span>成交量</span></div>
              <div v-for="bar in latestBars" :key="bar.day" class="price-row">
                <span>{{ String(bar.day).match(/\d{1,2}:\d{2}/)?.[0] || bar.day }}</span>
                <span>{{ formatPrice(bar.close) }}</span>
                <span>{{ Math.round(bar.volume) }}</span>
              </div>
            </div>
          </n-card>
        </template>
      </main>
    </div>
  </div>
</template>

<style scoped>
.holding-t-panel { height: 100%; padding: 12px; overflow: auto; box-sizing: border-box; }
.toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.subtitle { display: block; margin-top: 3px; font-size: 12px; }
.layout { display: grid; grid-template-columns: minmax(210px, 25%) 1fr; gap: 14px; min-height: 560px; }
.holding-list { border-right: 1px solid var(--n-border-color); padding-right: 12px; }
.holding-item { width: 100%; border: 0; border-radius: 6px; background: transparent; color: inherit; padding: 10px; margin-bottom: 6px; display: flex; justify-content: space-between; text-align: left; cursor: pointer; }
.holding-item:hover, .holding-item.active { background: rgba(128, 128, 128, .14); }
.holding-item span, .holding-nums { display: flex; flex-direction: column; gap: 3px; }
.holding-item small { opacity: .85; font-style: normal; font-size: 11px; }
.holding-nums { text-align: right; }
.holding-nums em { font-style: normal; font-weight: 700; }
.detail { min-width: 0; display: flex; flex-direction: column; gap: 10px; }
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(100px, 1fr)); gap: 10px; }
.summary-grid small { display: block; opacity: .65; font-size: 11px; }
.summary-grid b { display: block; margin-top: 4px; font-size: 16px; }
.t-strategy-placeholder { padding: 12px; border: 1px dashed rgba(128, 128, 128, .35); border-radius: 8px; background: rgba(128, 128, 128, .04); }
.intraday-chart { width: 100%; min-height: 220px; overflow: visible; }
.chart-axis { stroke: rgba(128, 128, 128, .35); stroke-width: 1; }
.chart-line { fill: none; stroke: #2080f0; stroke-width: 2; vector-effect: non-scaling-stroke; }
.marker-sell { fill: #d03050; stroke: #fff; stroke-width: 1.5; }
.marker-buy { fill: #18a058; stroke: #fff; stroke-width: 1.5; }
.marker-neutral { fill: #888; stroke: #fff; stroke-width: 1.5; }
.chart-label { font-size: 11px; fill: currentColor; }
.chart-scale { font-size: 10px; fill: currentColor; opacity: .6; }
.price-table { max-height: 220px; overflow: auto; font-variant-numeric: tabular-nums; }
.price-row { display: grid; grid-template-columns: 1fr 1fr 1fr; padding: 5px 0; border-bottom: 1px solid rgba(128, 128, 128, .15); }
.price-row span:not(:first-child) { text-align: right; }
.price-row.header { opacity: .6; position: sticky; top: 0; }
.profit-up { color: #d03050; }
.profit-down { color: #18a058; }
@media (max-width: 850px) {
  .layout { grid-template-columns: 1fr; }
  .holding-list { border-right: 0; border-bottom: 1px solid var(--n-border-color); padding: 0 0 8px; max-height: 220px; overflow: auto; }
  .summary-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
