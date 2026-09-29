<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NAutoComplete,
  NButton,
  NForm,
  NFormItem,
  NInputNumber,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import * as echarts from 'echarts'
import { GetConfig, GetStockEastMoneyKLine, GetStockList } from '../../wailsjs/go/main/App'
import { eastMoneyKLinesToBars, runDailySignalBacktest } from '../utils/backtestEngine'
import {
  buildEquityChartOption,
  buildPriceTradeChartOption,
  formatBacktestMoney,
} from '../utils/signalBacktestCharts'
import {
  identityAllowsRun,
  resolveStockIdentity,
  toStockSuggestions,
} from '../utils/signalBacktestIdentity'
import { resolveMarketMode } from '../utils/tradingLevelRules'

const message = useMessage()
const stockInput = ref('600519')
const limit = ref(250)
const initialCash = ref(1000000)
const useDiscipline = ref(true)
const loading = ref(false)
const result = ref(null)
const compare = ref(null)
const identity = ref(null)
const suggestions = ref([])
const activeStock = ref(null)
const dark = ref(true)
const equityRef = ref(null)
const priceRef = ref(null)
const chartsWrap = ref(null)

let lookupSeq = 0
let lookupTimer = 0
let skipNextLookup = false
let equityChart = null
let priceChart = null
let resizeObserver = null

function queryKey(value) {
  return String(value || '').replace(/\s+/g, '').trim().toLowerCase()
}

const ma = (closes, i, n) => {
  if (i + 1 < n) return null
  let s = 0
  for (let j = i - n + 1; j <= i; j++) s += closes[j]
  return s / n
}

function asList(raw) {
  if (Array.isArray(raw)) return raw
  if (raw && typeof raw.length === 'number') return Array.from(raw)
  return []
}

async function lookup(raw) {
  const seq = ++lookupSeq
  const query = String(raw || '').trim()
  let basics = []
  if (query) {
    try {
      basics = asList(await GetStockList(query))
    } catch {
      basics = []
    }
  }
  if (seq !== lookupSeq) return null
  let resolved = resolveStockIdentity(query, basics)
  const previous = identity.value
  if (
    resolved.status === 'code_only' &&
    previous?.status === 'unique' &&
    previous.name &&
    previous.klineCode === resolved.klineCode
  ) {
    resolved = { ...previous, query: queryKey(query) }
  } else {
    resolved = { ...resolved, query: queryKey(query) }
  }
  identity.value = resolved
  suggestions.value = toStockSuggestions(resolved)
  return resolved
}

function pickMatchFromValue(value) {
  const code = typeof value === 'object' && value
    ? String(value.value || value.label || '')
    : String(value || '')
  const row = (identity.value?.matches || []).find((item) => {
    const itemCode = item.symbol || item.klineCode
    return itemCode === code || item.label === code
  })
  if (row) pickMatch(row)
}

function scheduleLookup(value) {
  window.clearTimeout(lookupTimer)
  lookupTimer = window.setTimeout(() => {
    void lookup(value)
  }, 280)
}

function pickMatch(row) {
  const code = row?.symbol || row?.klineCode || ''
  if (!code) return
  if (stockInput.value !== code) skipNextLookup = true
  stockInput.value = code
  identity.value = {
    status: 'unique',
    name: row.name || '',
    symbol: row.symbol || code,
    tsCode: row.tsCode || '',
    klineCode: row.klineCode || code,
    label: row.label || code,
    matches: [row],
    matchCount: 1,
    message: '',
    query: queryKey(code),
  }
  suggestions.value = toStockSuggestions(identity.value)
}

function disposeCharts() {
  equityChart?.dispose()
  priceChart?.dispose()
  equityChart = null
  priceChart = null
}

function renderCharts() {
  if (!result.value || !equityRef.value || !priceRef.value) return
  if (!equityChart) equityChart = echarts.init(equityRef.value)
  if (!priceChart) priceChart = echarts.init(priceRef.value)
  equityChart.setOption(
    buildEquityChartOption(result.value, compare.value, {
      dark: dark.value,
      showCompare: useDiscipline.value,
    }),
    true,
  )
  priceChart.setOption(buildPriceTradeChartOption(result.value, { dark: dark.value }), true)
  equityChart.resize()
  priceChart.resize()
  requestAnimationFrame(() => {
    equityChart?.resize()
    priceChart?.resize()
  })
}

async function run() {
  window.clearTimeout(lookupTimer)
  const resolved = await lookup(stockInput.value)
  if (!resolved) return
  if (!identityAllowsRun(resolved)) {
    if (resolved.status === 'ambiguous') message.warning(resolved.message || '请先点选一只股票')
    else if (resolved.status === 'empty') message.warning('请输入股票代码或名称')
    else message.warning(resolved.message || '没有找到这只股票')
    return
  }

  loading.value = true
  result.value = null
  compare.value = null
  try {
    const barsLimit = Math.trunc(Number(limit.value))
    const raw = await GetStockEastMoneyKLine(
      resolved.klineCode,
      resolved.name || '',
      '101',
      Number.isFinite(barsLimit) && barsLimit > 0 ? barsLimit : 250,
    )
    const bars = eastMoneyKLinesToBars(raw)
    if (bars.length < 60) {
      message.warning('K 线不足，请换代码或加大条数')
      return
    }
    const closes = bars.map((b) => b.close)
    const volumes = bars.map((b) => b.volume || 0)

    const signalAt = (i) => {
      if (i < 60) return null
      const ma5 = ma(closes, i, 5)
      const ma10 = ma(closes, i, 10)
      const ma20 = ma(closes, i, 20)
      const prevMa5 = ma(closes, i - 1, 5)
      const prevMa10 = ma(closes, i - 1, 10)
      if (ma5 && ma10 && prevMa5 && prevMa10 && prevMa5 < prevMa10 && ma5 >= ma10 && closes[i] > ma20) {
        return { action: 'buy', strength: 0.7, tag: '趋' }
      }
      if (ma5 && ma10 && prevMa5 && prevMa10 && prevMa5 > prevMa10 && ma5 <= ma10) {
        return { action: 'sell', sellPct: 1, tag: '止' }
      }
      return null
    }

    const marketModeAt = (i) => {
      const ma5 = ma(closes, i, 5)
      const ma10 = ma(closes, i, 10)
      const ma20 = ma(closes, i, 20)
      const ma60 = ma(closes, i, 60)
      let volSum = 0
      let cnt = 0
      for (let j = Math.max(0, i - 5); j < i; j++) {
        if (volumes[j] > 0) {
          volSum += volumes[j]
          cnt++
        }
      }
      const avg = cnt ? volSum / cnt : 0
      const volumeRatio = avg > 0 ? volumes[i] / avg : 1
      const ma20Prev = ma(closes, i - 1, 20)
      return resolveMarketMode({
        close: closes[i],
        ma5,
        ma10,
        ma20,
        ma60,
        volumeRatio,
        volumeExpanding: volumeRatio >= 1.05,
        ma20Rising: ma20Prev != null && ma20 != null && ma20 >= ma20Prev,
      }).key
    }

    const withDisc = runDailySignalBacktest({
      bars,
      initialCash: initialCash.value,
      usePositionDiscipline: useDiscipline.value,
      signalAt,
      marketModeAt,
    })
    const noDisc = runDailySignalBacktest({
      bars,
      initialCash: initialCash.value,
      usePositionDiscipline: false,
      signalAt,
      marketModeAt,
    })
    activeStock.value = {
      name: resolved.name || '',
      code: resolved.symbol || resolved.klineCode,
    }
    result.value = withDisc
    compare.value = noDisc
    message.success('回测完成')
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

const summaryText = computed(() => {
  if (!result.value) return ''
  const a = result.value
  const b = compare.value
  const primaryName = useDiscipline.value ? '纪律模型' : '本次回测'
  return [
    `${primaryName}：收益 ${a.totalReturnPct}% · 回撤 ${a.maxDrawdownPct}% · 夏普 ${a.sharpe} · 交易 ${a.tradeCount}`,
    useDiscipline.value && b
      ? `无纪律对照：收益 ${b.totalReturnPct}% · 回撤 ${b.maxDrawdownPct}% · 夏普 ${b.sharpe} · 交易 ${b.tradeCount}`
      : '',
  ]
    .filter(Boolean)
    .join('\n')
})

const activeTitle = computed(() => {
  const stock = activeStock.value
  if (!stock?.code) return ''
  return stock.name ? `${stock.name}（${stock.code}）` : stock.code
})

const shownIdentity = computed(() => {
  const current = identity.value
  if (!current || current.status === 'empty') return null
  if (current.query !== queryKey(stockInput.value)) return null
  return current
})

const identityTone = computed(() => {
  if (shownIdentity.value?.status === 'unknown') return 'error'
  if (shownIdentity.value?.status === 'unique') return 'info'
  return 'warning'
})

watch(stockInput, (value) => {
  if (skipNextLookup) {
    skipNextLookup = false
    return
  }
  suggestions.value = []
  scheduleLookup(value)
})

watch(result, (value) => {
  if (!value) {
    disposeCharts()
    return
  }
  renderCharts()
}, { flush: 'post' })

watch(dark, () => {
  if (!result.value) return
  renderCharts()
})

onMounted(() => {
  void lookup(stockInput.value)
  GetConfig()
    .then((cfg) => {
      if (cfg && typeof cfg.darkTheme === 'boolean') dark.value = cfg.darkTheme
    })
    .catch(() => {})
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => {
      equityChart?.resize()
      priceChart?.resize()
    })
  }
})

watch(chartsWrap, (el) => {
  resizeObserver?.disconnect()
  if (el) resizeObserver?.observe(el)
})

onBeforeUnmount(() => {
  window.clearTimeout(lookupTimer)
  lookupSeq += 1
  resizeObserver?.disconnect()
  disposeCharts()
})
</script>

<template>
  <div class="backtest-panel">
    <div class="head">
      <span class="title">信号回测</span>
      <n-tag size="small" type="warning" :bordered="false">示意沙盒</n-tag>
    </div>
    <n-text depth="3" class="hint">
      示意沙盒：固定 MA5 上穿 MA10 且收盘站上 MA20 为买入，MA5 下穿 MA10 为卖出。含佣金、印花税、滑点与 T+1，可选 5 级仓位纪律。不是实盘策略，不构成投资建议。
    </n-text>
    <n-form label-placement="top" :show-feedback="false" class="form-grid">
      <n-form-item label="股票" class="span-stock">
        <div class="stock-field">
          <n-auto-complete
            v-model:value="stockInput"
            class="stock-input"
            :options="suggestions"
            :input-props="{ autocomplete: 'off' }"
            placeholder="代码或名称，如 688137 / 晶合集成"
            clearable
            @select="pickMatchFromValue"
          />
          <n-text v-if="shownIdentity && shownIdentity.status !== 'unique'" :type="identityTone" class="identity">
            {{ shownIdentity.message }}
          </n-text>
          <n-tag v-else-if="shownIdentity?.status === 'unique'" size="small" type="info" :bordered="false" class="identity-tag">
            {{ shownIdentity.name || '名称未收录' }} · {{ shownIdentity.symbol || shownIdentity.klineCode }}
          </n-tag>
          <n-space v-if="shownIdentity?.status === 'ambiguous'" size="small" class="picks">
            <n-button
              v-for="row in shownIdentity.matches"
              :key="row.symbol || row.tsCode || row.label"
              size="tiny"
              secondary
              @click="pickMatch(row)"
            >
              {{ row.label }}
            </n-button>
          </n-space>
        </div>
      </n-form-item>
      <n-form-item label="K线条数">
        <n-input-number v-model:value="limit" :min="120" :max="1000" :step="10" class="field-num" />
      </n-form-item>
      <n-form-item label="初始资金">
        <n-input-number v-model:value="initialCash" :min="100000" :step="100000" class="field-num" />
      </n-form-item>
      <n-form-item label="5级纪律">
        <n-switch v-model:value="useDiscipline" />
      </n-form-item>
      <n-form-item class="span-action" :show-label="false">
        <n-button type="primary" :loading="loading" :disabled="!String(stockInput || '').trim()" @click="run">
          运行回测
        </n-button>
      </n-form-item>
    </n-form>

    <template v-if="result">
      <div v-if="activeTitle" class="run-title">{{ activeTitle }}</div>
      <div class="metrics">
        <div class="metric">
          <div class="metric-label">最终权益</div>
          <div class="metric-value">{{ formatBacktestMoney(result.finalEquity) }}</div>
        </div>
        <div class="metric">
          <div class="metric-label">总收益%</div>
          <div class="metric-value" :class="result.totalReturnPct >= 0 ? 'up' : 'down'">{{ result.totalReturnPct }}</div>
        </div>
        <div class="metric">
          <div class="metric-label">最大回撤%</div>
          <div class="metric-value down">{{ result.maxDrawdownPct }}</div>
        </div>
        <div class="metric">
          <div class="metric-label">夏普(简)</div>
          <div class="metric-value">{{ result.sharpe }}</div>
        </div>
        <div class="metric">
          <div class="metric-label">交易次数</div>
          <div class="metric-value">{{ result.tradeCount }}</div>
        </div>
      </div>

      <div ref="chartsWrap" class="charts">
        <div class="chart-card">
          <div class="chart-title">权益曲线</div>
          <div ref="equityRef" class="chart-host"></div>
        </div>
        <div class="chart-card">
          <div class="chart-title">收盘价与买卖点</div>
          <div ref="priceRef" class="chart-host"></div>
        </div>
      </div>
      <n-text depth="3" class="chart-note">
        权益按收盘价盯市。买卖点是本次示意信号的成交（含滑点），不是实盘委托。
      </n-text>
      <pre class="summary">{{ summaryText }}</pre>
    </template>
  </div>
</template>

<style scoped>
.backtest-panel {
  padding: 12px 8px 20px;
  height: 100%;
  overflow: auto;
  box-sizing: border-box;
}
.head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.title {
  font-size: 16px;
  font-weight: 600;
}
.hint {
  display: block;
  margin-bottom: 12px;
  font-size: 13px;
  line-height: 1.55;
}
.form-grid {
  display: grid;
  grid-template-columns: minmax(200px, 1.6fr) minmax(120px, 0.7fr) minmax(150px, 0.9fr) auto auto;
  gap: 4px 12px;
  align-items: end;
}
.form-grid :deep(.n-form-item) {
  margin-bottom: 0;
  min-width: 0;
}
.form-grid :deep(.n-form-item-blank) {
  width: 100%;
}
.stock-field {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}
.stock-input {
  width: 100%;
}
.field-num {
  width: 100%;
}
.identity,
.identity-tag {
  max-width: 100%;
}
.picks {
  flex-wrap: wrap;
}
.run-title {
  margin-top: 16px;
  font-size: 14px;
  font-weight: 600;
}
.metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(128px, 1fr));
  gap: 8px;
  margin-top: 10px;
}
.metric {
  border: 1px solid rgba(128, 128, 128, 0.28);
  border-radius: 8px;
  padding: 8px 10px;
  min-width: 0;
}
.metric-label {
  font-size: 12px;
  opacity: 0.72;
}
.metric-value {
  margin-top: 2px;
  font-size: 18px;
  font-variant-numeric: tabular-nums;
  word-break: break-all;
}
.up {
  color: #e23d3d;
}
.down {
  color: #18a058;
}
.charts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 14px;
}
.chart-card {
  min-width: 0;
  border: 1px solid rgba(128, 128, 128, 0.28);
  border-radius: 8px;
  padding: 8px 8px 4px;
}
.chart-title {
  font-size: 13px;
  font-weight: 600;
}
.chart-host {
  width: 100%;
  height: 240px;
}
.chart-note {
  display: block;
  margin-top: 8px;
  font-size: 12px;
}
.summary {
  margin-top: 10px;
  white-space: pre-wrap;
  font-size: 13px;
  line-height: 1.6;
  opacity: 0.85;
}
@media (max-width: 960px) {
  .form-grid {
    grid-template-columns: 1fr 1fr;
  }
  .span-stock,
  .span-action {
    grid-column: 1 / -1;
  }
  .span-action :deep(.n-button) {
    width: 100%;
  }
  .charts {
    grid-template-columns: 1fr;
  }
  .chart-host {
    height: 210px;
  }
}
</style>
