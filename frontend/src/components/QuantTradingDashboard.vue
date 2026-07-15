<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NEmpty,
  NInput,
  NSelect,
  NSpin,
  NStatistic,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  useMessage,
} from 'naive-ui'
import { GetPaperAccountSnapshot, GetPaperMarginSnapshot } from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'
import {
  accountModeText,
  formatEventTime,
  formatMoney,
  formatPercent,
  formatPrice,
  formatRiskValue,
  formatVolume,
  orderTypeText,
  pickField,
  positionTypeText,
  profitClass,
  statusText,
  toNumber,
} from '../utils/tradingFormat'
import { createMarginFallback, normalizeMarginSnapshot } from '../utils/marginRiskModel'
import { createTradingStream } from '../utils/tradingStream'

const message = useMessage()
const loading = ref(false)
const snapshot = ref(null)
const activeTab = ref('trades')
const subscribed = ref(false)
const paused = ref(false)
const pendingCount = ref(0)
const venue = ref('PAPER')
const engineStatus = ref('监听中')
const stream = createTradingStream(500)
const events = ref([])
const eventTable = ref(null)

const filters = reactive({
  keyword: '',
  side: '',
  strategy: '',
  status: '',
})

const sideOptions = [
  { label: '全部方向', value: '' },
  { label: '买入', value: 'buy' },
  { label: '卖出', value: 'sell' },
]

const account = computed(() => snapshot.value?.account || {})
const positions = computed(() => snapshot.value?.positions || [])
const orders = computed(() => snapshot.value?.orders || [])
const equityPoints = computed(() => snapshot.value?.equity || [])
const margin = computed(() => snapshot.value?.margin || {})
const riskEvents = computed(() => snapshot.value?.riskEvents || [])

const positionValue = computed(() => positions.value.reduce((sum, position) => {
  return sum + toNumber(pickField(position, 'avgCost', 'AvgCost'))
    * toNumber(pickField(position, 'volume', 'Volume'))
}, 0))

const accountEquity = computed(() => toNumber(pickField(account.value, 'equity', 'Equity')))
const initialCash = computed(() => toNumber(pickField(account.value, 'initialCash', 'InitialCash')))
const cumulativeProfit = computed(() => accountEquity.value - initialCash.value)
const positionRatio = computed(() => accountEquity.value > 0
  ? positionValue.value / accountEquity.value * 100
  : 0)

const todayProfit = computed(() => {
  const today = new Date().toLocaleDateString('sv-SE')
  const points = equityPoints.value.filter(point =>
    String(pickField(point, 'dayKey', 'DayKey') || '') === today)
  if (!points.length) return 0
  return accountEquity.value - toNumber(pickField(points[0], 'equity', 'Equity'))
})

const strategyOptions = computed(() => {
  const values = new Set(events.value.map(event => event.strategy).filter(Boolean))
  orders.value.forEach(order => values.add(pickField(order, 'strategyTag', 'StrategyTag')))
  return [
    { label: '全部策略', value: '' },
    ...[...values].filter(Boolean).map(value => ({ label: value, value })),
  ]
})

const statusOptions = computed(() => {
  const values = new Set(events.value.map(event => event.status).filter(Boolean))
  orders.value.forEach(order => values.add(pickField(order, 'status', 'Status')))
  return [
    { label: '全部状态', value: '' },
    ...[...values].filter(Boolean).map(value => ({ label: statusText(value), value })),
  ]
})

const filteredEvents = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase()
  return events.value.filter((event) => {
    const stock = `${event.stockCode} ${event.stockName}`.toLowerCase()
    const side = String(event.side || '').toLowerCase()
    return (!keyword || stock.includes(keyword))
      && (!filters.side || side === filters.side)
      && (!filters.strategy || event.strategy === filters.strategy)
      && (!filters.status || event.status === filters.status)
  })
})

function scrollToLatest() {
  nextTick(() => {
    if (eventTable.value) eventTable.value.scrollTop = 0
  })
}

function handleTradingEvent(raw) {
  events.value = stream.push(raw).slice()
  const event = events.value[0]
  if (event?.venue) venue.value = event.venue
  const nextStatus = pickField(raw, 'runningStatus', 'RunningStatus', 'engineStatus', 'EngineStatus')
  if (nextStatus) engineStatus.value = nextStatus
  if (paused.value) pendingCount.value += 1
  else scrollToLatest()
}

function togglePaused(value) {
  paused.value = value
  if (!value) {
    pendingCount.value = 0
    scrollToLatest()
  }
}

function buildPositionMarks(baseSnapshot) {
  return (baseSnapshot?.positions || []).map((item) => ({
    stockCode: String(pickField(item, 'stockCode', 'StockCode') || ''),
    price: toNumber(pickField(item, 'marketPrice', 'MarketPrice', 'lastPrice', 'LastPrice', 'price', 'Price')),
  })).filter((item) => item.stockCode && item.price > 0)
}

async function refresh() {
  loading.value = true
  try {
    const base = await GetPaperAccountSnapshot(0)
    try {
      // 后端 Snapshot：Metrics + FinanceLiabilities；维保为比值(如 1.5)，normalize 内 *100 供展示
      const marks = buildPositionMarks(base)
      snapshot.value = normalizeMarginSnapshot(await GetPaperMarginSnapshot(0, marks), base)
    } catch (error) {
      snapshot.value = createMarginFallback(base, error)
    }
    events.value = stream.backfill(snapshot.value).slice()
  } catch (error) {
    message.error(error?.message || String(error))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  EventsOn('tradingEvent', handleTradingEvent)
  subscribed.value = true
  await refresh()
})

onBeforeUnmount(() => {
  EventsOff('tradingEvent')
  subscribed.value = false
})
</script>

<template>
  <section class="dashboard">
    <header class="dashboard-header">
      <div>
        <div class="title-line">
          <h2>量化交易看板</h2>
          <n-tag :type="subscribed ? 'success' : 'error'" size="small" round>
            {{ subscribed ? engineStatus : '未连接' }}
          </n-tag>
          <n-tag size="small" :bordered="false">{{ venue }}</n-tag>
          <n-tag :type="snapshot?.marginApiAvailable ? 'info' : 'default'" size="small">
            {{ accountModeText(margin.accountMode) }}
          </n-tag>
        </div>
        <p>模拟账户执行流与资产状态</p>
      </div>
      <n-button secondary :loading="loading" @click="refresh">同步快照</n-button>
    </header>

    <div class="metrics">
      <n-statistic label="现金" :value="formatMoney(pickField(account, 'cash', 'Cash'))" />
      <n-statistic label="权益" :value="formatMoney(accountEquity)" />
      <n-statistic label="仓位" :value="formatPercent(positionRatio)" />
      <n-statistic label="当日盈亏">
        <span :class="profitClass(todayProfit)">{{ formatMoney(todayProfit) }}</span>
      </n-statistic>
      <n-statistic label="累计盈亏">
        <span :class="profitClass(cumulativeProfit)">{{ formatMoney(cumulativeProfit) }}</span>
      </n-statistic>
    </div>
    <div class="metrics margin-metrics">
      <n-statistic label="融资余额" :value="formatMoney(margin.financingBalance)" />
      <n-statistic label="融券负债" :value="formatMoney(margin.securitiesLiability)" />
      <n-statistic label="可用保证金" :value="formatMoney(margin.availableMargin)" />
      <n-statistic label="维持担保比例" :value="formatPercent(margin.maintenanceRatio)" />
      <n-statistic label="净 / 总暴露" :value="`${formatMoney(margin.netExposure)} / ${formatMoney(margin.grossExposure)}`" />
    </div>
    <p v-if="snapshot && !snapshot.marginApiAvailable" class="fallback-tip">
      两融服务暂不可用，当前展示普通模拟账户快照。
    </p>

    <n-spin :show="loading">
      <n-tabs v-model:value="activeTab" type="line" animated>
        <n-tab-pane name="trades" tab="实时成交">
          <div class="toolbar">
            <n-input v-model:value="filters.keyword" clearable placeholder="股票代码 / 名称" />
            <n-select v-model:value="filters.side" :options="sideOptions" />
            <n-select v-model:value="filters.strategy" :options="strategyOptions" />
            <n-select v-model:value="filters.status" :options="statusOptions" />
            <label class="pause-control">
              <n-switch :value="paused" @update:value="togglePaused" />
              <span>暂停滚动</span>
              <n-tag v-if="pendingCount" size="small" type="warning">+{{ pendingCount }}</n-tag>
            </label>
          </div>
          <div ref="eventTable" class="table-scroll">
            <table v-if="filteredEvents.length" class="data-table">
              <thead>
                <tr>
                  <th>时间</th><th>股票</th><th>方向</th><th>策略</th>
                  <th>状态</th><th>价格</th><th>数量</th><th>费用</th><th>说明</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="event in filteredEvents" :key="event._key">
                  <td class="mono">{{ formatEventTime(event.timestamp) }}</td>
                  <td><b>{{ event.stockCode || '--' }}</b><small>{{ event.stockName }}</small></td>
                  <td :class="event.side === 'buy' ? 'profit-up' : event.side === 'sell' ? 'profit-down' : ''">
                    {{ orderTypeText(event.orderType, event.side) }}
                  </td>
                  <td>{{ event.strategy || '--' }}</td>
                  <td>{{ statusText(event.status) }}</td>
                  <td>{{ formatPrice(event.price) }}</td>
                  <td>{{ formatVolume(event.volume) }}</td>
                  <td>{{ formatMoney(event.fee) }}</td>
                  <td class="message">{{ event.message || '--' }}</td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无匹配的交易事件" />
          </div>
        </n-tab-pane>

        <n-tab-pane name="positions" tab="持仓">
          <div class="table-scroll">
            <table v-if="positions.length" class="data-table">
              <thead><tr><th>代码</th><th>名称</th><th>持仓类型</th><th>均价</th><th>数量</th><th>可卖</th><th>成本市值</th></tr></thead>
              <tbody>
                <tr v-for="position in positions" :key="pickField(position, 'id', 'ID', 'stockCode', 'StockCode')">
                  <td class="mono">{{ pickField(position, 'stockCode', 'StockCode') }}</td>
                  <td>{{ pickField(position, 'stockName', 'StockName') }}</td>
                  <td><n-tag size="small" :type="position.positionType === 'short' ? 'warning' : position.positionType === 'margin_long' ? 'info' : 'default'">{{ positionTypeText(position.positionType) }}</n-tag></td>
                  <td>{{ formatPrice(pickField(position, 'avgCost', 'AvgCost')) }}</td>
                  <td>{{ formatVolume(pickField(position, 'volume', 'Volume')) }}</td>
                  <td>{{ formatVolume(pickField(position, 'sellable', 'Sellable')) }}</td>
                  <td>{{ formatMoney(toNumber(pickField(position, 'avgCost', 'AvgCost')) * toNumber(pickField(position, 'volume', 'Volume'))) }}</td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无持仓" />
          </div>
        </n-tab-pane>

        <n-tab-pane name="orders" tab="委托">
          <div class="table-scroll">
            <table v-if="orders.length" class="data-table">
              <thead>
                <tr><th>时间</th><th>股票</th><th>订单种类</th><th>状态</th><th>委托价</th><th>委托量</th><th>成交价</th><th>成交量</th><th>策略</th></tr>
              </thead>
              <tbody>
                <tr v-for="order in orders" :key="pickField(order, 'id', 'ID')">
                  <td class="mono">{{ formatEventTime(pickField(order, 'createdAt', 'CreatedAt')) }}</td>
                  <td>{{ pickField(order, 'stockCode', 'StockCode') }} <small>{{ pickField(order, 'stockName', 'StockName') }}</small></td>
                  <td>{{ orderTypeText(pickField(order, 'orderType', 'OrderType', 'businessType', 'BusinessType'), pickField(order, 'side', 'Side')) }}</td>
                  <td>{{ statusText(pickField(order, 'status', 'Status')) }}</td>
                  <td>{{ formatPrice(pickField(order, 'price', 'Price')) }}</td>
                  <td>{{ formatVolume(pickField(order, 'volume', 'Volume')) }}</td>
                  <td>{{ formatPrice(pickField(order, 'filledPrice', 'FilledPrice')) }}</td>
                  <td>{{ formatVolume(pickField(order, 'filledVol', 'FilledVol')) }}</td>
                  <td>{{ pickField(order, 'strategyTag', 'StrategyTag') || '--' }}</td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无委托" />
          </div>
        </n-tab-pane>

        <n-tab-pane name="margin-risk" :tab="`两融风险 (${riskEvents.length})`">
          <div class="table-scroll">
            <table v-if="riskEvents.length" class="data-table">
              <thead><tr><th>时间</th><th>级别</th><th>原因码</th><th>当前值</th><th>阈值</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="risk in riskEvents" :key="risk.id">
                  <td class="mono">{{ formatEventTime(risk.occurredAt) }}</td>
                  <td><n-tag size="small" :type="risk.level === 'critical' || risk.level === 'danger' ? 'error' : risk.level === 'warning' ? 'warning' : 'info'">{{ risk.level }}</n-tag></td>
                  <td class="mono">{{ risk.reasonCode }}</td>
                  <td>{{ formatRiskValue(risk.currentValue, risk.unit) }}</td>
                  <td>{{ formatRiskValue(risk.threshold, risk.unit) }}</td>
                  <td class="message">{{ risk.message || '--' }}</td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无两融风险事件" />
          </div>
        </n-tab-pane>

        <n-tab-pane name="equity" tab="权益">
          <div class="table-scroll">
            <table v-if="equityPoints.length" class="data-table">
              <thead><tr><th>日期</th><th>时间</th><th>权益</th><th>现金</th></tr></thead>
              <tbody>
                <tr v-for="point in [...equityPoints].reverse()" :key="pickField(point, 'id', 'ID')">
                  <td>{{ pickField(point, 'dayKey', 'DayKey') }}</td>
                  <td class="mono">{{ formatEventTime(pickField(point, 'createdAt', 'CreatedAt')) }}</td>
                  <td>{{ formatMoney(pickField(point, 'equity', 'Equity')) }}</td>
                  <td>{{ formatMoney(pickField(point, 'cash', 'Cash')) }}</td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无权益记录" />
          </div>
        </n-tab-pane>
      </n-tabs>
    </n-spin>
  </section>
</template>

<style scoped>
.dashboard { height: 100%; padding: 20px; overflow: auto; color: var(--n-text-color); }
.dashboard-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-line { display: flex; align-items: center; gap: 8px; }
.title-line h2 { margin: 0; font-size: 22px; }
.dashboard-header p { margin: 5px 0 0; color: #8a8f99; font-size: 13px; }
.metrics { display: grid; grid-template-columns: repeat(5, minmax(120px, 1fr)); gap: 10px; margin: 18px 0 8px; }
.metrics > * { padding: 14px 16px; border: 1px solid rgba(128,128,128,.16); border-radius: 8px; background: rgba(128,128,128,.04); }
.margin-metrics { margin-top: 8px; }
.fallback-tip { margin: 4px 0 0; color: #d89614; font-size: 12px; }
.toolbar { display: grid; grid-template-columns: minmax(180px, 1fr) 130px 150px 150px auto; gap: 10px; margin: 12px 0; }
.pause-control { display: flex; align-items: center; gap: 7px; white-space: nowrap; color: #777; }
.table-scroll { max-height: calc(100vh - 330px); min-height: 220px; overflow: auto; border: 1px solid rgba(128,128,128,.15); border-radius: 8px; }
.data-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.data-table thead { position: sticky; top: 0; z-index: 1; background: var(--n-color, #fff); }
.data-table th, .data-table td { padding: 10px 12px; text-align: right; border-bottom: 1px solid rgba(128,128,128,.12); white-space: nowrap; }
.data-table th { color: #8a8f99; font-weight: 500; }
.data-table th:first-child, .data-table td:first-child,
.data-table th:nth-child(2), .data-table td:nth-child(2) { text-align: left; }
.data-table tbody tr:hover { background: rgba(24,160,88,.05); }
.data-table small { display: block; color: #8a8f99; font-weight: normal; }
.mono { font-family: Consolas, monospace; }
.message { max-width: 240px; overflow: hidden; text-overflow: ellipsis; }
.profit-up { color: #d03050; }
.profit-down { color: #18a058; }
@media (max-width: 900px) {
  .metrics { grid-template-columns: repeat(2, 1fr); }
  .toolbar { grid-template-columns: 1fr 1fr; }
}
</style>
