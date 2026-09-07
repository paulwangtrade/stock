<script setup>
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NEmpty,
  NSpin,
  NStatistic,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NSpace,
  useMessage,
} from 'naive-ui'
import {
  GetFollowRealtimeList,
  GetPaperAccountSnapshot,
  GetPaperMarginSnapshot,
  GetStockRealTimePrice,
} from '../../wailsjs/go/main/App'
import {
  accountModeText,
  formatMoney,
  formatPercent,
  formatPrice,
  formatVolume,
  pickField,
  positionTypeText,
  profitClass,
  toNumber,
} from '../utils/tradingFormat'
import { createMarginFallback, normalizeMarginSnapshot } from '../utils/marginRiskModel'
import { withTimeout } from '../utils/withTimeout'
import ProductCapabilityPanel from './ProductCapabilityPanel.vue'
import { getAdvancedRiskReport } from '../api/productCapabilities'
import {
  getPortfolioPositionState,
  indexPositionStatesBySymbol,
} from '../api/portfolioPositionState'
import { positionStateTagType } from '../utils/positionStateDisplay.js'
import { canShowSellButton } from '../utils/portfolioSellEntry.js'
import SellDraftDialog from './SellDraftDialog.vue'

const message = useMessage()
const activeTab = ref('self')
const loading = ref(false)
const loadError = ref('')
const selfPositions = ref([])
const paperSnapshot = ref(null)
const paperPrices = ref({})
const sellDialogVisible = ref(false)
const sellTargetRow = ref(null)
/** Phase11-K PositionState index (symbol → row); never invent 新仓 from qty. */
const positionStateBySymbol = ref(Object.create(null))

/** Phase13-D: advanced risk report entry */
const riskLoading = ref(false)
const riskError = ref('')
const riskReport = ref(null)

async function loadAdvancedRiskReport() {
  riskLoading.value = true
  riskError.value = ''
  try {
    const resp = await getAdvancedRiskReport({})
    if (!resp?.ok) {
      riskError.value = resp?.message || '风险报告请求失败'
      riskReport.value = null
      return
    }
    riskReport.value = resp.report || null
  } catch (e) {
    riskError.value = e?.message || String(e)
    riskReport.value = null
  } finally {
    riskLoading.value = false
  }
}

/** 持仓页整页刷新兜底，防止实时行情 Wails 调用长期不返回 */
const HOLDINGS_UI_TIMEOUT_MS = 20_000

const ownRows = computed(() => selfPositions.value.map((item) => {
  const price = toNumber(pickField(item, '当前价格', 'Price', 'price'))
  const cost = toNumber(pickField(item, 'costPrice', 'CostPrice'))
  const volume = toNumber(pickField(item, 'costVolume', 'CostVolume', 'Volume', 'volume'))
  const marketValue = price * volume
  const profit = marketValue - cost * volume
  return {
    key: `self-${pickField(item, '股票代码', 'StockCode', 'stockCode')}`,
    code: pickField(item, '股票代码', 'StockCode', 'stockCode') || '--',
    name: pickField(item, '股票名称', 'Name', 'stockName') || '--',
    price,
    cost,
    volume,
    marketValue,
    profit,
    profitRate: cost > 0 ? (price / cost - 1) * 100 : 0,
  }
}))

const quantRows = computed(() => (paperSnapshot.value?.positions || []).map((item) => {
  const code = pickField(item, 'stockCode', 'StockCode') || '--'
  const avgCost = toNumber(pickField(item, 'avgCost', 'AvgCost'))
  const volume = toNumber(pickField(item, 'volume', 'Volume'))
  const price = toNumber(paperPrices.value[code], avgCost)
  const ps = positionStateBySymbol.value[String(code).toLowerCase()] || null
  return {
    key: `paper-${pickField(item, 'id', 'ID', 'stockCode', 'StockCode')}`,
    code,
    name: pickField(item, 'stockName', 'StockName') || '--',
    avgCost,
    price,
    volume,
    sellable: toNumber(pickField(item, 'sellable', 'Sellable')),
    positionType: item.positionType || 'cash_long',
    marketValue: price * volume,
    profit: item.positionType === 'short'
      ? (avgCost - price) * volume
      : (price - avgCost) * volume,
    profitRate: avgCost > 0
      ? (item.positionType === 'short' ? (avgCost / price - 1) : (price / avgCost - 1)) * 100
      : 0,
    positionStatus: ps?.positionStatus || '',
    positionStatusLabel: ps?.positionStatusLabel || '—',
    isNewPosition: !!ps?.isNewPosition,
    availableQty: ps ? ps.availableQty : null,
    lockedQty: ps ? ps.lockedQty : null,
    // PositionState 持仓数量 = 可卖+锁定；无 PS 时回退纸面 volume（仅展示）
    totalQty: ps
      ? (Number(ps.availableQty) || 0) + (Number(ps.lockedQty) || 0)
      : volume,
  }
}))

const ownStats = computed(() => ownRows.value.reduce((stats, row) => {
  stats.marketValue += row.marketValue
  stats.costValue += row.cost * row.volume
  stats.profit += row.profit
  return stats
}, { marketValue: 0, costValue: 0, profit: 0 }))

const quantStats = computed(() => {
  const account = paperSnapshot.value?.account || {}
  const positionCost = quantRows.value.reduce((sum, row) => sum + row.avgCost * row.volume, 0)
  const marketValue = quantRows.value.reduce((sum, row) => sum + row.marketValue, 0)
  return {
    cash: toNumber(pickField(account, 'cash', 'Cash')),
    equity: toNumber(pickField(account, 'equity', 'Equity')),
    positionCost,
    marketValue,
    profit: quantRows.value.reduce((sum, row) => sum + row.profit, 0),
    margin: paperSnapshot.value?.margin || {},
  }
})

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    await withTimeout((async () => {
      const [followed, snapshot, psBundle] = await Promise.all([
        GetFollowRealtimeList(0),
        GetPaperAccountSnapshot(0),
        getPortfolioPositionState().catch(() => null),
      ])
      positionStateBySymbol.value = indexPositionStatesBySymbol(psBundle?.positions || [])
      selfPositions.value = (Array.isArray(followed) ? followed : [])
        .filter(item => toNumber(pickField(item, 'costVolume', 'CostVolume', 'Volume', 'volume')) > 0)
      const seedMarks = (snapshot?.positions || []).map((item) => ({
        stockCode: String(pickField(item, 'stockCode', 'StockCode') || ''),
        price: toNumber(pickField(item, 'marketPrice', 'MarketPrice', 'avgCost', 'AvgCost')),
      })).filter((item) => item.stockCode && item.price > 0)
      try {
        // Metrics.maintenanceRatio 为比值(1.5)；FinanceLiabilities 汇总融资余额；normalizeMarginSnapshot 负责口径统一
        paperSnapshot.value = normalizeMarginSnapshot(
          await GetPaperMarginSnapshot(0, seedMarks),
          snapshot,
        )
      } catch (error) {
        paperSnapshot.value = createMarginFallback(snapshot, error)
      }
      const quotePairs = await Promise.all((paperSnapshot.value?.positions || []).map(async (position) => {
        const code = pickField(position, 'stockCode', 'StockCode')
        if (!code) return null
        try {
          const quote = await withTimeout(GetStockRealTimePrice(code), 8_000, `GetStockRealTimePrice:${code}`)
          return [code, toNumber(pickField(quote, 'price', 'Price'))]
        } catch {
          return null
        }
      }))
      paperPrices.value = Object.fromEntries(quotePairs.filter(pair => pair?.[1] > 0))
    })(), HOLDINGS_UI_TIMEOUT_MS, 'holdingsRefresh')
  } catch (error) {
    loadError.value = error?.message || String(error)
    message.error(loadError.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

function openQuantSell(row) {
  sellTargetRow.value = row
  sellDialogVisible.value = true
}
</script>

<template>
  <section class="positions-page">
    <header class="page-header">
      <div>
        <h2>持仓中心</h2>
        <p>自主交易与量化模拟仓位独立核算</p>
      </div>
      <n-button :loading="loading" secondary @click="refresh">刷新</n-button>
    </header>

    <n-tag v-if="loadError" type="warning" :bordered="false" style="margin-bottom: 10px">
      {{ loadError }}
      <n-button text type="primary" size="tiny" style="margin-left: 8px" @click="refresh">重试</n-button>
    </n-tag>

    <ProductCapabilityPanel
      title="高级风险报告"
      feature="AdvancedRisk"
      scene="advanced_risk_report"
      usage-opened-key="risk_report_opened"
      usage-viewed-key="risk_report_viewed"
      @open="loadAdvancedRiskReport"
    >
      <n-spin :show="riskLoading">
        <n-tag v-if="riskError" type="warning" :bordered="false" style="margin-bottom: 8px">{{ riskError }}</n-tag>
        <template v-if="riskReport">
          <n-space align="center" :wrap="true" style="margin-bottom: 8px">
            <n-tag size="small" :bordered="false">{{ riskReport.status || '—' }}</n-tag>
            <n-tag size="small" type="info" :bordered="false">
              band {{ riskReport.score?.band || '—' }}
            </n-tag>
            <n-text>综合分 {{ riskReport.score?.overall ?? '—' }}</n-text>
          </n-space>
          <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 6px">
            {{ (riskReport.warnings || []).slice(0, 3).map((w) => w.message || w.code || w).join('；') || '无告警摘要' }}
          </n-text>
          <n-text
            v-for="(d, i) in riskReport.disclaimers || []"
            :key="`rd-${i}`"
            depth="3"
            style="display: block; font-size: 11px"
          >
            {{ d }}
          </n-text>
        </template>
        <n-empty v-else-if="!riskError && !riskLoading" size="small" description="点击「打开高级分析」查看风险报告" />
      </n-spin>
    </ProductCapabilityPanel>

    <n-spin :show="loading">
      <n-tabs v-model:value="activeTab" type="line" animated>
        <n-tab-pane name="self" tab="自持仓">
          <div class="stats-grid">
            <n-statistic label="持仓市值" :value="formatMoney(ownStats.marketValue)" />
            <n-statistic label="持仓成本" :value="formatMoney(ownStats.costValue)" />
            <n-statistic label="持仓盈亏">
              <span :class="profitClass(ownStats.profit)">{{ formatMoney(ownStats.profit) }}</span>
            </n-statistic>
            <n-statistic label="持仓股票" :value="ownRows.length" suffix="只" />
          </div>
          <div class="table-wrap">
            <table v-if="ownRows.length" class="position-table">
              <thead>
                <tr>
                  <th>代码</th><th>名称</th><th>现价</th><th>成本</th>
                  <th>数量</th><th>市值</th><th>盈亏</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in ownRows" :key="row.key">
                  <td class="code">{{ row.code }}</td>
                  <td>{{ row.name }}</td>
                  <td>{{ formatPrice(row.price) }}</td>
                  <td>{{ formatPrice(row.cost) }}</td>
                  <td>{{ formatVolume(row.volume) }}</td>
                  <td>{{ formatMoney(row.marketValue) }}</td>
                  <td :class="profitClass(row.profit)">
                    {{ formatMoney(row.profit) }}
                    <small>{{ row.profitRate >= 0 ? '+' : '' }}{{ row.profitRate.toFixed(2) }}%</small>
                  </td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无自持仓" />
          </div>
        </n-tab-pane>

        <n-tab-pane name="quant" tab="量化仓">
          <div class="stats-grid">
            <n-statistic label="账户模式" :value="accountModeText(quantStats.margin.accountMode)" />
            <n-statistic label="融资余额" :value="formatMoney(quantStats.margin.financingBalance)" />
            <n-statistic label="融券负债" :value="formatMoney(quantStats.margin.securitiesLiability)" />
            <n-statistic label="维持担保比例" :value="formatPercent(quantStats.margin.maintenanceRatio)" />
          </div>
          <div class="stats-grid secondary-stats">
            <n-statistic label="可用保证金" :value="formatMoney(quantStats.margin.availableMargin)" />
            <n-statistic label="净暴露" :value="formatMoney(quantStats.margin.netExposure)" />
            <n-statistic label="总暴露" :value="formatMoney(quantStats.margin.grossExposure)" />
            <n-statistic label="浮动盈亏"><span :class="profitClass(quantStats.profit)">{{ formatMoney(quantStats.profit) }}</span></n-statistic>
          </div>
          <div class="table-wrap">
            <table v-if="quantRows.length" class="position-table">
              <thead>
                <tr>
                  <th>账户</th><th>持仓类型</th><th>代码</th><th>名称</th>
                  <th>成本价</th><th>现价</th>
                  <th>持仓状态</th><th>新建仓</th><th>持仓数量</th><th>可卖数量</th><th>T+1锁定</th>
                  <th>市值</th><th>浮盈</th><th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in quantRows" :key="row.key">
                  <td><n-tag size="small" type="info" :bordered="false">{{ accountModeText(quantStats.margin.accountMode) }}</n-tag></td>
                  <td><n-tag size="small" :type="row.positionType === 'short' ? 'warning' : row.positionType === 'margin_long' ? 'info' : 'default'">{{ positionTypeText(row.positionType) }}</n-tag></td>
                  <td class="code">{{ row.code }}</td>
                  <td>{{ row.name }}</td>
                  <td>{{ formatPrice(row.avgCost) }}</td>
                  <td>{{ formatPrice(row.price) }}</td>
                  <td>
                    <n-tag
                      v-if="row.positionStatus"
                      size="tiny"
                      :type="positionStateTagType(row.positionStatus)"
                      :bordered="false"
                    >
                      {{ row.positionStatusLabel }}
                    </n-tag>
                    <n-text v-else depth="3">—</n-text>
                  </td>
                  <td>{{ row.positionStatus ? (row.isNewPosition ? '是' : '否') : '—' }}</td>
                  <td>{{ formatVolume(row.totalQty) }}</td>
                  <td>{{ row.availableQty == null ? '—' : formatVolume(row.availableQty) }}</td>
                  <td>{{ row.lockedQty == null ? '—' : formatVolume(row.lockedQty) }}</td>
                  <td>{{ formatMoney(row.marketValue) }}</td>
                  <td :class="profitClass(row.profit)">
                    {{ formatMoney(row.profit) }}
                    <small>{{ row.profitRate >= 0 ? '+' : '' }}{{ row.profitRate.toFixed(2) }}%</small>
                  </td>
                  <td>
                    <n-button
                      v-if="canShowSellButton(row)"
                      size="tiny"
                      type="warning"
                      secondary
                      @click="openQuantSell(row)"
                    >
                      卖出
                    </n-button>
                    <n-text v-else depth="3">—</n-text>
                  </td>
                </tr>
              </tbody>
            </table>
            <n-empty v-else description="暂无量化模拟持仓" />
          </div>
        </n-tab-pane>
      </n-tabs>
    </n-spin>

    <SellDraftDialog
      v-model:show="sellDialogVisible"
      :row="sellTargetRow"
      actor="ui:holdings-sell"
      @created="refresh"
    />
  </section>
</template>

<style scoped>
.positions-page { height: 100%; padding: 20px; overflow: auto; color: var(--n-text-color); }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.page-header h2 { margin: 0; font-size: 22px; }
.page-header p { margin: 5px 0 0; color: #8a8f99; font-size: 13px; }
.stats-grid { display: grid; grid-template-columns: repeat(4, minmax(130px, 1fr)); gap: 12px; margin: 14px 0; }
.stats-grid > * { padding: 14px 16px; border: 1px solid rgba(128,128,128,.16); border-radius: 8px; background: rgba(128,128,128,.04); }
.secondary-stats { margin-top: -4px; }
.table-wrap { overflow-x: auto; min-height: 180px; }
.position-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.position-table th { padding: 11px 12px; text-align: right; color: #8a8f99; border-bottom: 1px solid rgba(128,128,128,.2); white-space: nowrap; }
.position-table td { padding: 12px; text-align: right; border-bottom: 1px solid rgba(128,128,128,.12); white-space: nowrap; }
.position-table th:nth-child(-n+2), .position-table td:nth-child(-n+2) { text-align: left; }
.position-table tbody tr:hover { background: rgba(24,160,88,.05); }
.code { font-family: Consolas, monospace; }
.profit-up { color: #d03050; }
.profit-down { color: #18a058; }
small { margin-left: 5px; opacity: .8; }
@media (max-width: 760px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
