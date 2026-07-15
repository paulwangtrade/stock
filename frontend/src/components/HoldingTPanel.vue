<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NProgress,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { GetFollowRealtimeList, GetStockEastMoneyKLine } from '../../wailsjs/go/main/App'
import {
  evaluateIntradayTSignal,
  roundToBoardLot,
} from '../utils/intradayTSignals'

const STORAGE_PREFIX = 'go-stock:intraday-t:'
const message = useMessage()
const holdings = ref([])
const selectedCode = ref('')
const loadingList = ref(false)
const loadingDetail = ref(false)
const detailByCode = ref({})
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
  return {
    ...row,
    code,
    name: String(field(row, '股票名称', 'StockName', 'stockName', 'name', 'Name') || code),
    costVolume,
    costPrice: Number(field(row, 'costPrice', 'CostPrice')) || 0,
    price: Number(field(row, '当前价格', 'Price', 'price', 'close')) || 0,
  }
}

function stateKey(code, date) {
  return `${STORAGE_PREFIX}${date || 'unknown'}:${code}`
}

function emptyState(date) {
  return { date, rounds: 0, soldQty: 0, boughtBackQty: 0, events: [] }
}

function readState(code, date) {
  try {
    const saved = JSON.parse(localStorage.getItem(stateKey(code, date)) || 'null')
    return saved?.date === date ? { ...emptyState(date), ...saved } : emptyState(date)
  } catch {
    return emptyState(date)
  }
}

function writeState(code, state) {
  localStorage.setItem(stateKey(code, state.date), JSON.stringify(state))
}

const selected = computed(() => holdings.value.find((item) => item.code === selectedCode.value) || null)
const detail = computed(() => detailByCode.value[selectedCode.value] || null)
const signal = computed(() => detail.value?.signal || null)
const localState = computed(() => detail.value?.state || emptyState(''))
const outstandingQty = computed(() =>
  Math.max(0, Number(localState.value.soldQty || 0) - Number(localState.value.boughtBackQty || 0)),
)
const latestBars = computed(() => (signal.value?.metrics?.todayBars || []).slice(-18).reverse())
const chartModel = computed(() => {
  const bars = (signal.value?.metrics?.todayBars || []).slice(-36)
  if (!bars.length) return { points: '', markers: [] }
  const width = 720
  const height = 220
  const pad = 24
  const lows = bars.map(bar => Number(bar.low))
  const highs = bars.map(bar => Number(bar.high))
  const min = Math.min(...lows)
  const max = Math.max(...highs)
  const span = Math.max(max - min, max * 0.002, 0.01)
  const xAt = index => pad + index * ((width - pad * 2) / Math.max(1, bars.length - 1))
  const yAt = price => height - pad - ((Number(price) - min) / span) * (height - pad * 2)
  const points = bars.map((bar, index) => `${xAt(index)},${yAt(bar.close)}`).join(' ')
  const stateMarkers = (localState.value.events || []).map((event) => {
    const hhmm = String(event.time || '').slice(0, 5)
    let index = bars.findLastIndex(bar => String(bar.day).includes(hhmm))
    if (index < 0) index = bars.length - 1
    return { ...event, x: xAt(index), y: yAt(event.price) }
  })
  const live = signal.value?.action !== '观察'
    ? [{
        action: signal.value.action,
        time: signal.value.triggerTime,
        price: signal.value.referencePrice,
        x: xAt(bars.length - 1),
        y: yAt(signal.value.referencePrice),
        live: true,
      }]
    : []
  return { points, markers: [...stateMarkers, ...live], min, max, width, height }
})

function formatPrice(value) {
  return Number.isFinite(Number(value)) ? Number(value).toFixed(2) : '--'
}

function formatMetric(value, digits = 2) {
  return Number.isFinite(Number(value)) ? Number(value).toFixed(digits) : '--'
}

function signalType(action) {
  if (action === 'T出') return 'error'
  if (action === 'T入') return 'success'
  return 'default'
}

function currentMarketClock() {
  const now = new Date()
  const weekday = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai', weekday: 'short',
  }).format(now)
  if (weekday === 'Sat' || weekday === 'Sun') return undefined
  const text = new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai', hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(now)
  const [hour, minute] = text.match(/\d+/g)?.map(Number) || []
  const total = hour * 60 + minute
  return total >= 9 * 60 + 30 && total <= 15 * 60 ? now : undefined
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
    if (selectedCode.value) await refreshSelected()
  } catch (error) {
    message.error(error?.message || String(error))
  } finally {
    loadingList.value = false
  }
}

async function refreshSelected() {
  const holding = selected.value
  if (!holding || loadingDetail.value) return
  loadingDetail.value = true
  try {
    // 当前 Wails 绑定及项目其他调用均为四参数签名。
    const raw = await GetStockEastMoneyKLine(holding.code, holding.name, '5', 120)
    const provisional = evaluateIntradayTSignal({
      rows: raw,
      holdingVolume: holding.costVolume,
      costPrice: holding.costPrice,
    })
    const date = provisional.metrics?.date || new Date().toISOString().slice(0, 10)
    const state = readState(holding.code, date)
    const nextSignal = evaluateIntradayTSignal({
      rows: raw,
      holdingVolume: holding.costVolume,
      costPrice: holding.costPrice,
      state,
      // 盘后按最后一根 5 分钟线保留结果；盘中（含午休）使用真实时钟约束。
      now: currentMarketClock(),
    })
    detailByCode.value = {
      ...detailByCode.value,
      [holding.code]: { signal: nextSignal, state, refreshedAt: new Date().toLocaleTimeString('zh-CN') },
    }
  } catch (error) {
    message.error(`${holding.name} 刷新失败：${error?.message || String(error)}`)
  } finally {
    loadingDetail.value = false
  }
}

async function selectHolding(code) {
  selectedCode.value = code
  await refreshSelected()
}

async function recordAction(action) {
  const holding = selected.value
  const current = detail.value
  const hint = signal.value
  if (!holding || !current || !hint) return
  const qty = roundToBoardLot(hint.suggestedVolume)
  if (qty <= 0 || hint.action !== action) {
    message.warning(`当前没有可记录的${action}整手提示`)
    return
  }
  const state = { ...current.state, events: [...(current.state.events || [])] }
  if (action === 'T出') {
    if (state.rounds >= 2 || state.soldQty > state.boughtBackQty) {
      message.warning('须先回补上一轮，且每日最多两轮')
      return
    }
    state.rounds += 1
    state.soldQty += qty
  } else {
    state.boughtBackQty += Math.min(qty, Math.max(0, state.soldQty - state.boughtBackQty))
  }
  state.events.unshift({
    action,
    time: new Date().toLocaleTimeString('zh-CN', {
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    }),
    price: hint.referencePrice,
    volume: qty,
  })
  writeState(holding.code, state)
  detailByCode.value = {
    ...detailByCode.value,
    [holding.code]: {
      ...current,
      state,
      signal: evaluateIntradayTSignal({
        rows: hint.metrics?.bars || [],
        holdingVolume: holding.costVolume,
        costPrice: holding.costPrice,
        state,
      }),
    },
  }
  message.success(`已在本机记录${action} ${qty} 股（未下单）`)
}

onMounted(() => {
  refreshHoldings()
  refreshTimer = window.setInterval(refreshSelected, 30_000)
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
})
</script>

<template>
  <div class="holding-t-panel">
    <div class="toolbar">
      <div>
        <n-text strong>分钟做 T 提示</n-text>
        <n-text depth="3" class="subtitle">仅提示，不自动下单 · 10:00-14:30 · 每日最多两轮</n-text>
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
            </span>
          </button>
          <n-empty v-if="!holdings.length && !loadingList" description="未找到 costVolume > 0 的持仓" />
        </n-spin>
      </aside>

      <main class="detail">
        <n-empty v-if="!selected" description="请选择持仓" />
        <n-spin v-else :show="loadingDetail">
          <template v-if="signal">
            <div class="signal-head">
              <div>
                <n-tag :type="signalType(signal.action)" size="large">{{ signal.action }}</n-tag>
                <strong class="reference">{{ formatPrice(signal.referencePrice) }}</strong>
                <span class="time">触发 {{ signal.triggerTime }}</span>
              </div>
              <div class="actions">
                <n-button size="small" @click="refreshSelected">刷新分钟线</n-button>
                <n-button
                  v-if="signal.action === 'T出'"
                  size="small"
                  type="error"
                  secondary
                  @click="recordAction('T出')"
                >记录已T出</n-button>
                <n-button
                  v-if="signal.action === 'T入'"
                  size="small"
                  type="success"
                  secondary
                  @click="recordAction('T入')"
                >记录已T入</n-button>
              </div>
            </div>

            <div class="metric-grid">
              <n-card size="small"><small>VWAP</small><b>{{ formatPrice(signal.metrics.vwap) }}</b></n-card>
              <n-card size="small"><small>MA5</small><b>{{ formatPrice(signal.metrics.ma5) }}</b></n-card>
              <n-card size="small"><small>RSI14</small><b>{{ formatMetric(signal.metrics.rsi14, 1) }}</b></n-card>
              <n-card size="small"><small>日内高 / 低</small><b>{{ formatPrice(signal.metrics.intradayHigh) }} / {{ formatPrice(signal.metrics.intradayLow) }}</b></n-card>
              <n-card size="small"><small>量比</small><b>{{ formatMetric(signal.metrics.volumeRatio) }}</b></n-card>
              <n-card size="small"><small>建议整手量</small><b>{{ signal.suggestedVolume }} 股</b></n-card>
            </div>

            <n-card size="small" class="decision-card">
              <div class="confidence">
                <span>置信度 {{ signal.confidence }}%</span>
                <n-progress type="line" :percentage="signal.confidence" :show-indicator="false" />
              </div>
              <ul><li v-for="reason in signal.reasons" :key="reason">{{ reason }}</li></ul>
              <p><b>失效条件：</b>{{ signal.invalidation }}</p>
              <p v-if="signal.requiresSellableConfirmation" class="sellable-warning">
                无法从手工自持仓确认真实可卖数量，请在券商端人工核对后再操作。
              </p>
              <p class="state-line">
                本地当日状态：第 {{ localState.rounds }}/2 轮 · 已T出 {{ localState.soldQty }} 股 ·
                已T入 {{ localState.boughtBackQty }} 股 · 待回补 {{ outstandingQty }} 股
              </p>
            </n-card>

            <div class="lower-grid">
              <n-card size="small" title="5 分钟价格与 T 点标记" class="intraday-chart-card">
                <svg
                  v-if="chartModel.points"
                  class="intraday-chart"
                  :viewBox="`0 0 ${chartModel.width} ${chartModel.height}`"
                  role="img"
                  aria-label="5分钟价格曲线和T点标记"
                >
                  <line x1="24" y1="196" x2="696" y2="196" class="chart-axis" />
                  <polyline :points="chartModel.points" class="chart-line" />
                  <g v-for="(marker, index) in chartModel.markers" :key="`${marker.action}-${marker.time}-${index}`">
                    <circle
                      :cx="marker.x"
                      :cy="marker.y"
                      r="6"
                      :class="marker.action === 'T出' ? 'marker-sell' : 'marker-buy'"
                    />
                    <text :x="marker.x + 8" :y="marker.y - 8" class="chart-label">
                      {{ marker.action }} {{ formatPrice(marker.price) }}
                    </text>
                  </g>
                  <text x="26" y="18" class="chart-scale">{{ formatPrice(chartModel.max) }}</text>
                  <text x="26" y="214" class="chart-scale">{{ formatPrice(chartModel.min) }}</text>
                </svg>
                <n-empty v-else size="small" description="暂无可绘制的分钟数据" />
              </n-card>
              <n-card size="small" title="最近 5 分钟价位">
                <div class="price-table">
                  <div class="price-row header"><span>时间</span><span>收盘</span><span>成交量</span></div>
                  <div v-for="bar in latestBars" :key="bar.day" class="price-row">
                    <span>{{ String(bar.day).match(/\d{1,2}:\d{2}/)?.[0] || bar.day }}</span>
                    <span>{{ formatPrice(bar.close) }}</span>
                    <span>{{ Math.round(bar.volume) }}</span>
                  </div>
                </div>
              </n-card>
              <n-card size="small" title="本地信号时间线">
                <div v-if="localState.events.length" class="timeline">
                  <div v-for="(event, index) in localState.events" :key="`${event.time}-${index}`">
                    <n-tag size="small" :type="signalType(event.action)">{{ event.action }}</n-tag>
                    <span>{{ event.time }} · {{ formatPrice(event.price) }} · {{ event.volume }}股</span>
                  </div>
                </div>
                <n-empty v-else size="small" description="今日尚未记录执行" />
              </n-card>
            </div>
            <n-text depth="3" class="refreshed">最近刷新 {{ detail.refreshedAt }}，盘后保留最后结果</n-text>
          </template>
        </n-spin>
      </main>
    </div>
  </div>
</template>

<style scoped>
.holding-t-panel { height: 100%; padding: 12px; overflow: auto; box-sizing: border-box; }
.toolbar, .signal-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.toolbar { margin-bottom: 12px; }
.subtitle { display: block; margin-top: 3px; font-size: 12px; }
.layout { display: grid; grid-template-columns: minmax(210px, 25%) 1fr; gap: 14px; min-height: 560px; }
.holding-list { border-right: 1px solid var(--n-border-color); padding-right: 12px; }
.holding-item { width: 100%; border: 0; border-radius: 6px; background: transparent; color: inherit; padding: 10px; margin-bottom: 6px; display: flex; justify-content: space-between; text-align: left; cursor: pointer; }
.holding-item:hover, .holding-item.active { background: rgba(128, 128, 128, .14); }
.holding-item span, .holding-nums { display: flex; flex-direction: column; gap: 3px; }
.holding-item small { opacity: .65; font-style: normal; }
.holding-nums { text-align: right; }
.holding-nums em { font-style: normal; font-weight: 700; }
.detail { min-width: 0; }
.reference { font-size: 24px; margin-left: 10px; }
.time { margin-left: 10px; opacity: .65; }
.actions { display: flex; gap: 8px; }
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(130px, 1fr)); gap: 8px; margin: 12px 0; }
.metric-grid small, .metric-grid b { display: block; }
.metric-grid b { margin-top: 5px; font-size: 16px; }
.decision-card { margin-bottom: 10px; }
.confidence { display: grid; grid-template-columns: 90px 1fr; align-items: center; gap: 10px; }
.decision-card ul { margin: 10px 0; padding-left: 20px; }
.decision-card p { margin: 7px 0 0; }
.state-line { opacity: .75; }
.sellable-warning { color: #f0a020; font-weight: 600; }
.lower-grid { display: grid; grid-template-columns: 1.1fr .9fr; gap: 10px; }
.intraday-chart-card { grid-column: 1 / -1; }
.intraday-chart { width: 100%; min-height: 220px; overflow: visible; }
.chart-axis { stroke: rgba(128, 128, 128, .35); stroke-width: 1; }
.chart-line { fill: none; stroke: #2080f0; stroke-width: 2; vector-effect: non-scaling-stroke; }
.marker-sell { fill: #d03050; stroke: #fff; stroke-width: 1.5; }
.marker-buy { fill: #18a058; stroke: #fff; stroke-width: 1.5; }
.chart-label { font-size: 11px; fill: currentColor; }
.chart-scale { font-size: 10px; fill: currentColor; opacity: .6; }
.price-table { max-height: 280px; overflow: auto; font-variant-numeric: tabular-nums; }
.price-row { display: grid; grid-template-columns: 1fr 1fr 1fr; padding: 5px 0; border-bottom: 1px solid rgba(128, 128, 128, .15); }
.price-row span:not(:first-child) { text-align: right; }
.price-row.header { opacity: .6; position: sticky; top: 0; }
.timeline > div { display: flex; gap: 8px; align-items: center; margin-bottom: 9px; }
.refreshed { display: block; text-align: right; margin-top: 8px; font-size: 12px; }
@media (max-width: 850px) {
  .layout, .lower-grid { grid-template-columns: 1fr; }
  .holding-list { border-right: 0; border-bottom: 1px solid var(--n-border-color); padding: 0 0 8px; max-height: 220px; overflow: auto; }
  .metric-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
