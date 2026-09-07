<script setup>
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NInput,
  NSpace,
  NSpin,
  NStatistic,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import {
  getPaperDailyReports,
  getPaperDashboardRuns,
  getPaperDashboardToday,
  getPaperHoldingsEvaluation,
  getPaperExitEvaluation,
  getPaperExecutionSummary,
  getPaperObservationMetrics,
  getPaperPositionAttribution,
} from '../api/paperObservation'
import { getPortfolioSnapshot } from '../api/portfolioSnapshot'
import PortfolioObservationPanel from './PortfolioObservationPanel.vue'
import PortfolioDecisionDashboard from './PortfolioDecisionDashboard.vue'
import ExitReviewDrawer from './ExitReviewDrawer.vue'
import InvestmentNarrativePanel from './InvestmentNarrativePanel.vue'
import {
  EXIT_EVAL_FILTER,
  exitEvalRowState,
  exitReasonLabel,
  exitReviewActionButtonProps,
  exitReviewDecisionLabel,
  filterExitEvalByMode,
  formatOutcomeReviewTime,
  hasLatestExitOutcome,
  shouldShowExitReviewAction,
  sortExitEvalHoldings,
} from '../utils/exitReviewDisplay.js'
import { getUpcomingTradePlan } from '../api/tradePlans'
import ProductCapabilityPanel from './ProductCapabilityPanel.vue'
import {
  getAdvancedRiskReport,
  getAssistantContext,
  getStrategyExplanation,
} from '../api/productCapabilities'
import {
  POSITION_STATE,
  positionStateTagType,
} from '../utils/positionStateDisplay.js'

const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const tradeDate = ref('')
const activeTab = ref('overview')
const today = ref(null)
/** P5-F: asset facts from Portfolio Snapshot (not dashboard/positions + PS join). */
const snapshot = ref(null)
const runs = ref(null)
const dailyReports = ref(null)
const metrics = ref(null)
const metricsUnavailable = ref(false)
const executionSummary = ref(null)
const attribution = ref(null)
const attributionUnavailable = ref(false)
const holdingsEval = ref(null)
const holdingsEvalUnavailable = ref(false)
const exitEval = ref(null)
const exitEvalUnavailable = ref(false)
const exitEvalFilter = ref(EXIT_EVAL_FILTER.ALL)
const exitReviewDrawerVisible = ref(false)
const exitReviewTargetRow = ref(null)
const narrativeDrawerVisible = ref(false)
const narrativeStockCode = ref('')
const expandedEvalKeys = ref([])
const expandedExitEvalKeys = ref([])
/** Phase13-D: advanced observation entries */
const advRiskLoading = ref(false)
const advRiskError = ref('')
const advRiskReport = ref(null)
const advExplainLoading = ref(false)
const advExplainError = ref('')
const advExplainResult = ref(null)
const advCtxLoading = ref(false)
const advCtxError = ref('')
const advCtxResult = ref(null)

async function loadObsAdvancedRisk() {
  advRiskLoading.value = true
  advRiskError.value = ''
  try {
    const resp = await getAdvancedRiskReport({ tradeDate: tradeDate.value || undefined })
    if (!resp?.ok) {
      advRiskError.value = resp?.message || '风险报告失败'
      advRiskReport.value = null
      return
    }
    advRiskReport.value = resp.report || null
  } catch (e) {
    advRiskError.value = e?.message || String(e)
    advRiskReport.value = null
  } finally {
    advRiskLoading.value = false
  }
}

async function loadObsStrategyExplain() {
  advExplainLoading.value = true
  advExplainError.value = ''
  try {
    const up = await getUpcomingTradePlan(tradeDate.value || undefined)
    const plan = up?.plan
    const item = (plan?.items || []).find((it) => Number(it?.id || 0) > 0)
    if (!plan?.id || !item?.id) {
      advExplainError.value = '暂无带 id 的 TradePlan 明细，无法生成策略解释'
      advExplainResult.value = null
      return
    }
    const resp = await getStrategyExplanation({
      planId: plan.id,
      planItemId: item.id,
    })
    if (!resp?.ok) {
      advExplainError.value = resp?.message || '策略解释失败'
      advExplainResult.value = null
      return
    }
    advExplainResult.value = resp.explanation || null
  } catch (e) {
    advExplainError.value = e?.message || String(e)
    advExplainResult.value = null
  } finally {
    advExplainLoading.value = false
  }
}

async function loadObsAssistantContext() {
  advCtxLoading.value = true
  advCtxError.value = ''
  try {
    const resp = await getAssistantContext({
      scene: 'risk_explain',
      tradeDate: tradeDate.value || undefined,
    })
    if (!resp?.ok) {
      advCtxError.value = resp?.message || '上下文构建失败'
      advCtxResult.value = null
      return
    }
    advCtxResult.value = resp.context || null
  } catch (e) {
    advCtxError.value = e?.message || String(e)
    advCtxResult.value = null
  } finally {
    advCtxLoading.value = false
  }
}

const dataSourceNote = computed(
  () =>
    snapshot.value?.dataSourceNote ||
    today.value?.dataSourceNote ||
    runs.value?.dataSourceNote ||
    'Phase6.6 Paper Trading MVP · paper_sim_* · 仅用于策略观察和模拟分析，不代表真实交易账户',
)

/** Banner: backend S1_NEW_LOCKED only — never available_qty==0. */
const hasNewLockedPosition = computed(() =>
  (positionDisplayRows.value || []).some(
    (row) => row.positionStatus === POSITION_STATE.S1 || row.isNewPosition === true,
  ),
)

/** Qty columns are Snapshot.total_qty / available_qty / locked_qty — never available+locked. */
const positionDisplayRows = computed(() => snapshot.value?.positions || [])

const snapshotRisk = computed(() => {
  const s = snapshot.value
  if (!s?.found) return null
  const equity = Number(s.equity) || 0
  const mv = Number(s.marketValue) || 0
  const ratio = equity > 0 ? mv / equity : null
  let concentration = null
  for (const p of s.positions || []) {
    if (mv > 0) {
      const w = (Number(p.marketValue) || 0) / mv
      if (concentration == null || w > concentration) concentration = w
    }
  }
  return {
    totalAsset: equity,
    cash: Number(s.cash) || 0,
    positionValue: mv,
    positionRatio: ratio,
    concentration,
  }
})

function formatMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatPct(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(2)}%`
}

function formatTime(v) {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return String(v)
  return d.toLocaleString()
}

function runStatusType(s) {
  const v = String(s || '')
  if (v === 'completed') return 'success'
  if (v === 'completed_with_rejects') return 'warning'
  if (v === 'failed') return 'error'
  if (v.startsWith('skipped_')) return 'default'
  return 'info'
}

function quoteSourceLabel(source) {
  const v = String(source || '').trim()
  if (v === 'tencent' || v === 'live') return 'Tencent'
  if (v === 'open_fallback') return 'Tencent'
  if (v === 'position_mark' || v === 'persisted') return 'position_mark'
  return v || 'position_mark'
}

function formatQuoteClock(v) {
  if (v === null || v === undefined || v === '') return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleTimeString('zh-CN', { hour12: false })
}

const positionColumns = [
  { title: '代码', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      return formatStockDisplayName(row?.stockName, row?.stockCode)
    },
  },
  {
    title: '持仓状态',
    key: 'positionStatus',
    width: 140,
    render(row) {
      if (!row.positionStatus) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(
        NTag,
        { size: 'tiny', type: positionStateTagType(row.positionStatus), bordered: false },
        { default: () => row.positionStatusLabel },
      )
    },
  },
  {
    title: '新建仓',
    key: 'isNewPosition',
    width: 80,
    render(row) {
      if (row.isNewPosition == null) return h(NText, { depth: 3 }, { default: () => '—' })
      return row.isNewPosition ? '是' : '否'
    },
  },
  {
    title: '持仓数量',
    key: 'totalQty',
    width: 90,
    render(row) {
      if (row.totalQty == null) return h(NText, { depth: 3 }, { default: () => '—' })
      return String(row.totalQty)
    },
  },
  {
    title: '可卖数量',
    key: 'availableQty',
    width: 90,
    render(row) {
      if (row.availableQty == null) return h(NText, { depth: 3 }, { default: () => '—' })
      return String(row.availableQty)
    },
  },
  {
    title: 'T+1锁定',
    key: 'lockedQty',
    width: 90,
    render(row) {
      if (row.lockedQty == null) return h(NText, { depth: 3 }, { default: () => '—' })
      return String(row.lockedQty)
    },
  },
  {
    title: '成本价',
    key: 'avgCost',
    width: 90,
    render(row) {
      return formatMoney(row.avgCost)
    },
  },
  {
    title: '当前价',
    key: 'displayPrice',
    width: 180,
    render(row) {
      const source = String(row?.displayQuoteSource || '').trim()
      const sourceLabel = quoteSourceLabel(source || 'persisted')
      const displayPrice =
        row?.displayPrice != null && Number.isFinite(Number(row.displayPrice))
          ? Number(row.displayPrice)
          : Number(row?.markPrice)
      const ledgerPrice =
        row?.markPrice != null && Number.isFinite(Number(row.markPrice)) ? Number(row.markPrice) : null
      return h(
        NSpace,
        { size: 4, align: 'center' },
        {
          default: () => [
            h(NText, null, { default: () => formatMoney(displayPrice) }),
            h(
              NTooltip,
              null,
              {
                trigger: () =>
                  h(
                    NTag,
                    { size: 'tiny', bordered: false, type: source === 'live' ? 'success' : 'default' },
                    { default: () => sourceLabel },
                  ),
                default: () =>
                  `展示价格（display_price）：${formatMoney(displayPrice)}\n账本价（mark_price）：${ledgerPrice == null ? '—' : formatMoney(ledgerPrice)}\n来源：${sourceLabel}\n不计入账户权益`,
              },
            ),
          ],
        },
      )
    },
  },
  {
    title: '市值',
    key: 'displayMarketValue',
    width: 110,
    render(row) {
      const v = row.displayMarketValue != null ? row.displayMarketValue : row.marketValue
      return formatMoney(v)
    },
  },
  {
    title: '浮盈亏',
    key: 'displayPnl',
    width: 140,
    render(row) {
      const pnl = Number(row.displayPnl != null ? row.displayPnl : row.pnl)
      const rate = Number(row.displayPnlPercent != null ? row.displayPnlPercent : row.pnlPercent)
      const money = formatMoney(pnl)
      if (!Number.isFinite(pnl)) return money
      const pct = Number.isFinite(rate) ? formatPct(rate) : '—'
      const sign = pnl >= 0 ? '+' : ''
      return h('span', { class: pnl >= 0 ? 'pnl-up' : pnl < 0 ? 'pnl-down' : '' }, [
        `${sign}${pct}`,
        h('small', { style: 'margin-left:6px;opacity:.85' }, `${money}`),
      ])
    },
  },
  {
    title: '收益率',
    key: 'displayPnlPercent',
    width: 90,
    render(row) {
      const rate = row.displayPnlPercent != null ? row.displayPnlPercent : row.pnlPercent
      return formatPct(rate)
    },
  },
  {
    title: '叙事',
    key: 'narrative',
    width: 72,
    fixed: 'right',
    render(row) {
      if (!row?.stockCode) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      return h(
        NButton,
        {
          size: 'small',
          quaternary: true,
          type: 'info',
          onClick: () => openInvestmentNarrative(row.stockCode),
        },
        { default: () => '叙事' },
      )
    },
  },
]

const runColumns = [
  {
    title: '时间',
    key: 'startedAt',
    width: 170,
    render(row) {
      return formatTime(row.startedAt)
    },
  },
  { title: '交易日', key: 'tradeDate', width: 110 },
  { title: 'trigger', key: 'trigger', width: 80 },
  { title: 'actor', key: 'actor', width: 90 },
  {
    title: 'result',
    key: 'status',
    width: 180,
    render(row) {
      return h(
        NTag,
        { size: 'small', type: runStatusType(row.status), bordered: false },
        { default: () => row.status || '—' },
      )
    },
  },
  {
    title: '订单/成交/拒绝',
    key: 'counts',
    width: 130,
    render(row) {
      return `${row.ordersTotal ?? 0} / ${row.filledCount ?? 0} / ${row.rejectCount ?? 0}`
    },
  },
  {
    title: 'message',
    key: 'message',
    ellipsis: { tooltip: true },
  },
]

const dailyReportColumns = [
  { title: '日期', key: 'reportDate', width: 110 },
  {
    title: '权益',
    key: 'equity',
    width: 110,
    render(row) {
      return formatMoney(row.equity)
    },
  },
  {
    title: '现金',
    key: 'cash',
    width: 110,
    render(row) {
      return formatMoney(row.cash)
    },
  },
  {
    title: '持仓市值',
    key: 'marketValue',
    width: 110,
    render(row) {
      return formatMoney(row.marketValue)
    },
  },
  {
    title: '浮盈亏',
    key: 'floatingPnl',
    width: 110,
    render(row) {
      return formatMoney(row.floatingPnl)
    },
  },
  { title: '成交数量', key: 'filledCount', width: 90 },
  { title: '拒绝数量', key: 'rejectedCount', width: 90 },
  {
    title: '风险暴露',
    key: 'exposure',
    width: 200,
    render(row) {
      const single = formatPct(row.maxSinglePositionPct)
      const gross = formatPct(row.maxGrossExposurePct)
      const code = row.maxSingleStockCode ? `（${row.maxSingleStockCode}）` : ''
      return `单票 ${single}${code}｜总敞口 ${gross}`
    },
  },
]

const attributionColumns = [
  { title: '代码', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 100,
    ellipsis: { tooltip: true },
    render(row) {
      return formatStockDisplayName(row?.stockName, row?.stockCode)
    },
  },
  { title: '持仓数量', key: 'totalVolume', width: 90 },
  {
    title: '来源',
    key: 'sourceSummary',
    minWidth: 180,
    ellipsis: { tooltip: true },
    render(row) {
      return String(row?.sourceSummary || '').trim() || '—'
    },
  },
  {
    title: '对账',
    key: 'reconcile',
    width: 110,
    render(row) {
      const st = String(row?.reconcile?.status || '')
      const type = st === 'matched' ? 'success' : 'warning'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => st || '—' })
    },
  },
  {
    title: '未归因',
    key: 'unattributed',
    width: 90,
    render(row) {
      const v = Number(row?.unattributed?.volume || row?.reconcile?.unattributedVolume || 0)
      if (v <= 0) return h(NText, { depth: 3 }, { default: () => '0' })
      return h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => String(v) })
    },
  },
  {
    title: 'Lots（多计划不覆盖）',
    key: 'lots',
    minWidth: 280,
    render(row) {
      const lots = Array.isArray(row?.lots) ? row.lots : []
      if (!lots.length) {
        return h(NText, { depth: 3 }, { default: () => '无 fill lot' })
      }
      return lots
        .map((lot) => `Plan#${lot.planId} ${lot.volume}股 @${Number(lot.fillPrice).toFixed(2)}`)
        .join(' · ')
    },
  },
]

const attributionLotColumns = [
  { title: 'plan_id', key: 'planId', width: 80 },
  { title: 'item_id', key: 'planItemId', width: 80 },
  { title: 'order_id', key: 'orderId', width: 80 },
  { title: 'fill_id', key: 'fillId', width: 80 },
  { title: '数量', key: 'volume', width: 80 },
  {
    title: '成交价',
    key: 'fillPrice',
    width: 90,
    render(row) {
      return formatMoney(row.fillPrice)
    },
  },
  {
    title: '成本额',
    key: 'costAmount',
    width: 110,
    render(row) {
      return formatMoney(row.costAmount)
    },
  },
  { title: '交易日', key: 'tradeDate', width: 110 },
]

const expandedAttributionLots = computed(() => {
  const rows = attribution.value?.positions || []
  const out = []
  for (const p of rows) {
    for (const lot of p.lots || []) {
      out.push({
        ...lot,
        stockCode: p.stockCode,
        _key: `${p.stockCode}-${lot.fillId}`,
      })
    }
  }
  return out
})

function evalStateType(state) {
  const v = String(state || '')
  if (v === 'EXIT_CANDIDATE' || v === 'DANGER') return 'warning'
  if (v === 'WATCH') return 'info'
  return 'success'
}

function riskStateType(state) {
  const v = String(state || '')
  if (v === 'DANGER') return 'error'
  if (v === 'WATCH') return 'warning'
  return 'success'
}

function exitEvalStateType(state) {
  const v = String(state || '')
  if (v === 'REVIEW_REQUIRED') return 'error'
  if (v === 'WATCH') return 'warning'
  return 'success'
}

const exitEvalTableRows = computed(() => {
  const holdings = exitEval.value?.holdings || []
  const filtered = filterExitEvalByMode(holdings, exitEvalFilter.value)
  return sortExitEvalHoldings(filtered)
})

const exitReviewMeta = computed(() => {
  if (!exitEval.value) return null
  return {
    accountId: exitEval.value.accountId,
    asOf: exitEval.value.asOf,
    policy: exitEval.value.policy,
    dataSourceNote: exitEval.value.dataSourceNote,
  }
})

function openExitReview(row) {
  if (!row?.stockCode) return
  exitReviewTargetRow.value = row
  exitReviewDrawerVisible.value = true
}

function openInvestmentNarrative(stockCode) {
  const code = String(stockCode || '').trim()
  if (!code) return
  narrativeStockCode.value = code
  narrativeDrawerVisible.value = true
}

async function onExitOutcomeSaved() {
  try {
    exitEval.value = await getPaperExitEvaluation()
    exitEvalUnavailable.value = false
  } catch {
    /* keep prior list */
  }
}

function formatNullableMoney(v) {
  if (v === null || v === undefined || v === '') return '—'
  return formatMoney(v)
}

function formatNullablePct(v) {
  if (v === null || v === undefined || v === '') return '—'
  return formatPct(v)
}

const holdingsEvalLotColumns = [
  { title: 'plan_id', key: 'planId', width: 80 },
  { title: 'plan_item_id', key: 'planItemId', width: 100 },
  { title: 'buy fill_id', key: 'fillId', width: 100 },
  { title: '买入日期', key: 'buyDate', width: 110 },
  { title: '持有天数', key: 'holdingDays', width: 90 },
  { title: '数量', key: 'volume', width: 80 },
  {
    title: '成本',
    key: 'costPrice',
    width: 90,
    render(row) {
      return formatNullableMoney(row.costPrice)
    },
  },
  {
    title: '现价',
    key: 'currentPrice',
    width: 90,
    render(row) {
      return formatNullableMoney(row.currentPrice)
    },
  },
  {
    title: '浮盈亏',
    key: 'pnl',
    width: 100,
    render(row) {
      return formatNullableMoney(row.pnl)
    },
  },
]

const holdingsEvalColumns = [
  {
    type: 'expand',
    expandable: (row) => Array.isArray(row?.lots) && row.lots.length > 0,
    renderExpand(row) {
      const lots = Array.isArray(row?.lots) ? row.lots : []
      return h(NDataTable, {
        size: 'small',
        bordered: false,
        singleLine: false,
        columns: holdingsEvalLotColumns,
        data: lots,
        rowKey: (lot) => `${row.stockCode}-${lot.fillId}`,
      })
    },
  },
  { title: '股票', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      const d = row?.displayName
      const raw = String(d?.displayName || row?.stockName || '').trim()
      const main = formatStockDisplayName(raw, row?.stockCode)
      const snap = String(d?.snapshotName || '').trim()
      if (d?.nameChanged && snap && snap !== main) {
        return h(
          NTooltip,
          { trigger: 'hover' },
          {
            trigger: () => main,
            default: () => `快照名：${snap}`,
          },
        )
      }
      return main
    },
  },
  { title: '当前数量', key: 'totalVolume', width: 90 },
  {
    title: '成本',
    key: 'avgCost',
    width: 90,
    render(row) {
      return formatNullableMoney(row.avgCost)
    },
  },
  {
    title: '现价',
    key: 'currentPrice',
    width: 90,
    render(row) {
      return formatNullableMoney(row.currentPrice ?? row.marketPrice)
    },
  },
  {
    title: '行情',
    key: 'quoteSource',
    width: 110,
    render(row) {
      return quoteSourceLabel(row?.quoteSource)
    },
  },
  {
    title: '行情时间',
    key: 'quoteTime',
    width: 100,
    render(row) {
      return formatQuoteClock(row?.quoteTime)
    },
  },
  {
    title: '市值',
    key: 'marketValue',
    width: 110,
    render(row) {
      return formatNullableMoney(row.marketValue)
    },
  },
  {
    title: '浮盈亏',
    key: 'unrealizedPnl',
    width: 110,
    render(row) {
      return formatNullableMoney(row.unrealizedPnl)
    },
  },
  {
    title: '收益率',
    key: 'unrealizedReturn',
    width: 90,
    render(row) {
      return formatNullablePct(row.unrealizedReturn)
    },
  },
  {
    title: '持有天数',
    key: 'holdingDays',
    width: 90,
  },
  {
    title: '周期',
    key: 'holdingPeriodState',
    width: 100,
    render(row) {
      return String(row?.holdingPeriodState || '—')
    },
  },
  {
    title: '盈亏',
    key: 'profitState',
    width: 100,
    render(row) {
      const st = String(row?.profitState || 'UNKNOWN')
      const type = st === 'PROFIT' ? 'success' : st === 'LOSS' ? 'error' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => st })
    },
  },
  {
    title: '趋势',
    key: 'trendState',
    width: 90,
    render(row) {
      return String(row?.trendState || 'UNKNOWN')
    },
  },
  {
    title: '风险',
    key: 'riskState',
    width: 100,
    render(row) {
      const st = String(row?.riskState || 'NORMAL')
      return h(
        NTag,
        { size: 'small', type: riskStateType(st), bordered: false },
        { default: () => st },
      )
    },
  },
  {
    title: '评价状态',
    key: 'evalState',
    width: 110,
    render(row) {
      const st = String(row?.evalState || 'NORMAL')
      return h(
        NTag,
        { size: 'small', type: evalStateType(st), bordered: false },
        { default: () => st },
      )
    },
  },
]

const exitEvalLotColumns = [
  { title: 'plan_id', key: 'planId', width: 80 },
  { title: 'plan_item_id', key: 'planItemId', width: 100 },
  { title: 'fill_id', key: 'fillId', width: 80 },
  {
    title: '策略',
    key: 'strategyName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return String(row?.context?.entry?.strategyName || '').trim() || '—'
    },
  },
  {
    title: '入场原因',
    key: 'entryReason',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      return String(row?.context?.entry?.entryReason || '').trim() || '—'
    },
  },
  {
    title: 'plan状态',
    key: 'planStatus',
    width: 100,
    render(row) {
      return String(row?.context?.plan?.planStatus || '').trim() || '—'
    },
  },
  { title: '持有天数', key: 'holdingDays', width: 90 },
  {
    title: '收益率',
    key: 'unrealizedReturn',
    width: 90,
    render(row) {
      return formatNullablePct(row.unrealizedReturn)
    },
  },
  {
    title: 'Lot状态',
    key: 'evaluation',
    width: 130,
    render(row) {
      const st = String(row?.evaluation?.state || 'NORMAL')
      return h(NTag, { size: 'small', type: exitEvalStateType(st), bordered: false }, { default: () => st })
    },
  },
]

const exitEvalColumns = [
  {
    type: 'expand',
    expandable: (row) => Array.isArray(row?.lots) && row.lots.length > 0,
    renderExpand(row) {
      const lots = Array.isArray(row?.lots) ? row.lots : []
      return h(NDataTable, {
        size: 'small',
        bordered: false,
        singleLine: false,
        columns: exitEvalLotColumns,
        data: lots,
        rowKey: (lot) => `${row.stockCode}-${lot.fillId}`,
      })
    },
  },
  { title: '股票', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 100,
    ellipsis: { tooltip: true },
    render(row) {
      return formatStockDisplayName(row?.stockName, row?.stockCode)
    },
  },
  {
    title: '当前状态',
    key: 'evaluation',
    width: 140,
    render(row) {
      const st = String(row?.evaluation?.state || 'NORMAL')
      return h(NTag, { size: 'small', type: exitEvalStateType(st), bordered: false }, { default: () => st })
    },
  },
  {
    title: '原因',
    key: 'reasonCodes',
    minWidth: 180,
    render(row) {
      const codes = Array.isArray(row?.evaluation?.reasonCodes) ? row.evaluation.reasonCodes : []
      if (!codes.length) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      return h(
        NSpace,
        { size: 4, wrap: true },
        {
          default: () =>
            codes.map((c) =>
              h(NTag, { size: 'small', bordered: false }, { default: () => exitReasonLabel(c) }),
            ),
        },
      )
    },
  },
  {
    title: '复评说明',
    key: 'summary',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row) {
      return String(row?.evaluation?.summary || '').trim() || '—'
    },
  },
  {
    title: '已复评',
    key: 'latestOutcome',
    width: 168,
    render(row) {
      if (!hasLatestExitOutcome(row)) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      const o = row.latestOutcome
      const label = exitReviewDecisionLabel(o.decision)
      const when = formatOutcomeReviewTime(o.reviewTime)
      return h(
        NTooltip,
        {},
        {
          trigger: () =>
            h(
              NTag,
              { size: 'small', type: 'success', bordered: false },
              { default: () => label },
            ),
          default: () => `最近决定：${label} · ${when}`,
        },
      )
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 96,
    fixed: 'right',
    render(row) {
      if (!shouldShowExitReviewAction(row)) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      const st = exitEvalRowState(row)
      const btnProps = exitReviewActionButtonProps(st)
      return h(
        NButton,
        {
          size: 'small',
          type: btnProps.type,
          quaternary: btnProps.quaternary,
          text: btnProps.text,
          onClick: () => openExitReview(row),
        },
        { default: () => '查看复评' },
      )
    },
  },
]

function formatCompliance(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(1)}%`
}

/** Empty or sentinel names must not be disguised as the stock code. */
function formatStockDisplayName(name, code) {
  const n = String(name || '').trim()
  const c = String(code || '').trim()
  if (n && n !== '未知名称' && n !== c) return n
  if (c) return `未知名称(${c})`
  return '未知名称'
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  const date = tradeDate.value.trim() || undefined

  // Metrics must not block / clear the existing observation panels on failure.
  const metricsPromise = getPaperObservationMetrics(date)
    .then((m) => {
      metrics.value = m
      metricsUnavailable.value = false
    })
    .catch(() => {
      metrics.value = null
      metricsUnavailable.value = true
    })

  const attributionPromise = getPaperPositionAttribution()
    .then((a) => {
      attribution.value = a
      attributionUnavailable.value = false
    })
    .catch(() => {
      attribution.value = null
      attributionUnavailable.value = true
    })

  const holdingsEvalPromise = getPaperHoldingsEvaluation()
    .then((v) => {
      holdingsEval.value = v
      holdingsEvalUnavailable.value = false
    })
    .catch(() => {
      holdingsEval.value = null
      holdingsEvalUnavailable.value = true
    })

  const exitEvalPromise = getPaperExitEvaluation()
    .then((v) => {
      exitEval.value = v
      exitEvalUnavailable.value = false
    })
    .catch(() => {
      exitEval.value = null
      exitEvalUnavailable.value = true
    })

  const execSummaryPromise = getPaperExecutionSummary(date)
    .then((v) => {
      executionSummary.value = v
    })
    .catch(() => {
      executionSummary.value = null
    })

  try {
    const [t, snap, r, d] = await Promise.all([
      getPaperDashboardToday(date),
      getPortfolioSnapshot({ tradeDate: date, includeDisplay: true }),
      getPaperDashboardRuns({ tradeDate: date, limit: 50 }),
      getPaperDailyReports({ limit: 60 }),
    ])
    today.value = t
    snapshot.value = snap
    runs.value = r
    dailyReports.value = d
    if (!tradeDate.value && (t.tradeDate || snap.tradeDate)) {
      tradeDate.value = t.tradeDate || snap.tradeDate
    }
  } catch (e) {
    today.value = null
    snapshot.value = null
    runs.value = null
    dailyReports.value = null
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    await Promise.all([
      metricsPromise,
      attributionPromise,
      holdingsEvalPromise,
      exitEvalPromise,
      execSummaryPromise,
    ])
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="paper-observation">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>模拟盘观察</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      </n-space>
      <n-space>
        <n-input
          v-model:value="tradeDate"
          placeholder="交易日 YYYY-MM-DD"
          style="width: 160px"
          clearable
          @keyup.enter="refresh"
        />
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
    </n-space>

    <div class="source-banner">
      <n-text strong>数据来源：</n-text>
      <n-text>Phase6.6 Paper Trading MVP</n-text>
      <n-text depth="3"> · 数据表：</n-text>
      <n-text code>paper_sim_*</n-text>
      <div class="source-desc">
        仅用于策略观察和模拟分析。不代表真实交易账户。与「量化交易 / 研究中心模拟盘」的生产
        <n-text code>paper_*</n-text>
        账本相互独立。
      </div>
      <n-text depth="3" style="font-size: 12px">{{ dataSourceNote }}</n-text>
    </div>

    <n-spin :show="loading">
      <n-tabs v-model:value="activeTab" type="line" style="margin-top: 8px">
        <n-tab-pane name="overview" tab="今日观察">
<div class="obs-section paper-snapshot">
            <n-space align="center" style="margin: 4px 0 10px">
              <n-text strong>A. 账户快照</n-text>
              <n-tag size="small" type="success" :bordered="false">Account Snapshot</n-tag>
              <n-tag size="small" :bordered="false">Portfolio Snapshot · 只读</n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
              账户权益 / 现金 / 持仓来自 GET /api/portfolio/snapshot（GET 重算，不用账户 equity 列）。今日计划/成交计数仍来自 Observation today，与 B 窗 Close 成交笔数勿混读。
            </n-text>

            <template v-if="today">
              <n-text strong style="display: block; margin: 4px 0 8px">今日模拟交易</n-text>
              <n-space align="center" :wrap="true" style="margin-bottom: 10px">
                <n-text>交易日期：{{ today.tradeDate || '—' }}</n-text>
                <n-tag size="small" :type="today.enabled ? 'success' : 'default'" :bordered="false">
                  {{ today.enabled ? 'enabled' : 'disabled' }}
                </n-tag>
                <n-tag size="small" :type="runStatusType(today.runStatus)" :bordered="false">
                  {{ today.runStatus || '—' }}
                </n-tag>
                <n-text v-if="today.message" depth="3">{{ today.message }}</n-text>
              </n-space>

              <n-space :wrap="true" :size="24" style="margin-bottom: 18px">
                <n-statistic label="计划数量" :value="today.planCount" />
                <n-statistic label="订单数量" :value="today.ordersTotal" />
                <n-statistic label="成交数量" :value="today.filledCount" />
                <n-statistic label="拒绝数量" :value="today.rejectCount" />
                <n-statistic label="成交金额" :value="formatMoney(today.filledAmount)" />
              </n-space>
            </template>

            <template v-if="snapshot">
              <n-space :wrap="true" :size="24" style="margin-bottom: 18px">
                <n-statistic label="现金余额" :value="formatMoney(snapshot.cash)" />
                <n-statistic label="浮盈亏" :value="formatMoney(snapshot.ledgerPnl)" />
                <n-statistic label="权益" :value="formatMoney(snapshot.equity)" />
                <n-statistic label="股票市值" :value="formatMoney(snapshot.marketValue)" />
                <n-statistic label="持仓数量" :value="snapshot.positionCount ?? '—'" />
              </n-space>
            </template>

            <template v-if="snapshotRisk">
              <n-space align="center" style="margin: 4px 0 8px">
                <n-text strong>账户风险（只读）</n-text>
                <n-tag size="small" type="success" :bordered="false">Snapshot</n-tag>
              </n-space>
              <n-space :wrap="true" :size="24" style="margin-bottom: 12px">
                <n-statistic label="总资产" :value="formatNullableMoney(snapshotRisk.totalAsset)" />
                <n-statistic label="现金" :value="formatNullableMoney(snapshotRisk.cash)" />
                <n-statistic label="持仓市值" :value="formatNullableMoney(snapshotRisk.positionValue)" />
                <n-statistic
                  label="仓位占比"
                  :value="
                    snapshotRisk.positionRatio == null
                      ? '—'
                      : `${(Number(snapshotRisk.positionRatio) * 100).toFixed(1)}%`
                  "
                />
                <n-statistic
                  label="集中度(最大单票)"
                  :value="
                    snapshotRisk.concentration == null
                      ? '—'
                      : `${(Number(snapshotRisk.concentration) * 100).toFixed(1)}%`
                  "
                />
              </n-space>
            </template>
            <n-text v-else-if="snapshot && !snapshot.found" depth="3" style="display: block; margin-bottom: 10px">
              暂无模拟账户（found=false，未建户）
            </n-text>

            <template v-if="snapshot">
              <n-space align="center" style="margin: 8px 0">
                <n-text strong>当前模拟持仓</n-text>
                <n-tag
                  size="small"
                  :type="snapshot.quoteOverlay ? 'success' : 'default'"
                  :bordered="false"
                >
                  {{ snapshot.quoteOverlay ? '实时行情覆盖中（display only）' : '使用账本成交价' }}
                </n-tag>
              </n-space>
              <n-text depth="3" style="display: block; margin-bottom: 8px">
                持仓数量来自 Portfolio Snapshot（total_qty），可卖 / T+1 锁定分列；当前价与行盈亏为
                display_*，不计入账户权益。新仓标记来自嵌套 PositionState。
              </n-text>
              <n-tag
                v-if="hasNewLockedPosition"
                type="warning"
                size="small"
                :bordered="false"
                style="margin-bottom: 8px"
              >
                新建仓(T+1锁定)
              </n-tag>
              <n-data-table
                v-if="positionDisplayRows.length"
                size="small"
                :columns="positionColumns"
                :data="positionDisplayRows"
                :bordered="false"
                :single-line="false"
              />
              <n-empty v-else description="暂无模拟持仓" style="margin: 12px 0" />
            </template>

            

            <template v-if="runs">
              <n-text strong style="display: block; margin: 18px 0 8px">运行台账</n-text>
              <n-data-table
                v-if="(runs.runs || []).length"
                size="small"
                :columns="runColumns"
                :data="runs.runs"
                :bordered="false"
                :single-line="false"
              />
              <n-empty v-else description="暂无运行记录" style="margin: 12px 0" />
            </template>
          </div>

          <div class="obs-section exec-observation">
            <n-space align="center" style="margin: 18px 0 10px">
              <n-text strong>B. 执行观察</n-text>
              <n-tag size="small" type="info" :bordered="false">执行链健康观察</n-tag>
              <n-tag size="small" :bordered="false">只读 · metrics API</n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
              Session / 价格策略 / Legacy / Compliance。计数来自 Observation Metrics，不是上方账本「今日成交数量」。
            </n-text>

            <template v-if="executionSummary">
              <n-text strong style="display: block; margin-bottom: 6px">Execution Summary</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="total_orders" :value="executionSummary.totalOrders" />
                <n-statistic label="filled_orders" :value="executionSummary.filledOrders" />
                <n-statistic label="failed_orders" :value="executionSummary.failedOrders" />
                <n-statistic
                  label="fill_rate"
                  :value="`${(Number(executionSummary.fillRate || 0) * 100).toFixed(1)}%`"
                />
                <n-statistic
                  label="avg_slippage"
                  :value="executionSummary.avgSlippage == null ? 'null' : executionSummary.avgSlippage"
                />
              </n-space>
              <n-text depth="3" style="display: block; margin-bottom: 12px; font-size: 12px">
                {{ executionSummary.dataSourceNote }}
              </n-text>
            </template>

            <template v-if="metrics">
              <n-text strong style="display: block; margin-bottom: 6px">Session 执行次数（runs）</n-text>
              <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
                按 run.started_at 派生 Session；此处为执行次数（runs），不是成交数量（fills）。
              </n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="Session A 执行次数（runs）" :value="metrics.sessionDistribution.sessionA" />
                <n-statistic label="Session B 执行次数（runs）" :value="metrics.sessionDistribution.sessionB" />
                <n-statistic label="Session C 执行次数（runs）" :value="metrics.sessionDistribution.sessionC" />
                <n-statistic
                  label="After Close 执行次数（runs）"
                  :value="metrics.sessionDistribution.sessionClosed"
                />
                <n-statistic label="总执行次数（runs）" :value="metrics.totalRuns" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">B窗口成交价格策略</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 8px">
                <n-statistic label="符合：B窗口 Close 成交" :value="metrics.fillPolicy.bWindowCloseFills" />
                <n-statistic label="异常：B窗口 Open 成交" :value="metrics.fillPolicy.bWindowOpenFills" />
              </n-space>
              <n-text
                v-if="Number(metrics.fillPolicy.bWindowCloseFills) === 0"
                depth="3"
                style="display: block; margin-bottom: 16px; font-size: 12px"
              >
                默认 A 模式下暂无 Session B market_close 样本（不是异常）。15:05 Settlement 不计入 Close 成交；需
                fillMode=B 采数后才会上升。
              </n-text>
              <div v-else style="margin-bottom: 16px" />

              <n-text strong style="display: block; margin-bottom: 6px">历史基线（已排除）</n-text>
              <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
                旧执行路径数据，仅用于迁移观察，不参与当前合规统计。
              </n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="历史基线笔数" :value="metrics.legacy.legacyBaselineCount" />
                <n-statistic label="已排除笔数" :value="metrics.legacy.excludedFillCount" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">Quality</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="OK" :value="metrics.quality.okCount" />
                <n-statistic label="ANOMALY" :value="metrics.quality.anomalyCount" />
                <n-statistic label="Legacy Baseline（历史基线）" :value="metrics.quality.legacyCount" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">Compliance</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 8px">
                <n-statistic
                  label="执行价格策略合规率"
                  :value="formatCompliance(metrics.pricePolicyCompliance)"
                />
              </n-space>
            </template>

            <n-empty
              v-else-if="metricsUnavailable"
              description="执行观察数据暂不可用"
              style="margin: 8px 0 12px"
            />
            <n-text v-else depth="3" style="display: block; margin-bottom: 8px">
              执行观察加载中…
            </n-text>
          </div>

          <div class="obs-section holdings-analysis">
            <n-space align="center" style="margin: 18px 0 10px">
              <n-text strong>C. 持仓分析</n-text>
              <n-tag size="small" type="info" :bordered="false">Attribution + Holding Evaluation</n-tag>
              <n-tag size="small" :bordered="false">只读 · 非交易动作</n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
              归因拆解持仓来源；评价给出盈亏/周期/风险标签。不产生买入或卖出指令。
            </n-text>
<div style="margin-top: 16px" class="obs-subsection">
              <n-space align="center" style="margin-bottom: 6px">
                <n-text strong>持仓归因（Position Attribution）</n-text>
                <n-tag size="small" type="info" :bordered="false">只读 · fills→plans</n-tag>
                <n-tag
                  v-if="attribution && !attribution.reconcileAllMatched"
                  size="small"
                  type="warning"
                  :bordered="false"
                >
                  存在未完全归因
                </n-tag>
              </n-space>
              <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
                净持仓一行；多计划买入同一股票时 lots 全部保留（禁止覆盖）。未归因数量单独标注，不伪造
                fill。
              </n-text>
              <n-text v-if="attributionUnavailable" depth="3" type="warning" style="display: block; margin-bottom: 8px">
                归因 API 暂不可用（不影响上方持仓表）。
              </n-text>
              <template v-else-if="attribution">
                <n-data-table
                  v-if="(attribution.positions || []).length"
                  size="small"
                  :columns="attributionColumns"
                  :data="attribution.positions"
                  :bordered="false"
                  :single-line="false"
                  style="margin-bottom: 10px"
                />
                <n-empty v-else description="暂无归因持仓" style="margin: 8px 0" />
                <n-text strong style="display: block; margin: 8px 0 6px">Lot 明细</n-text>
                <n-data-table
                  v-if="expandedAttributionLots.length"
                  size="small"
                  :columns="[
                    { title: '代码', key: 'stockCode', width: 100 },
                    ...attributionLotColumns,
                  ]"
                  :data="expandedAttributionLots"
                  :row-key="(row) => row._key"
                  :bordered="false"
                  :single-line="false"
                />
                <n-empty v-else description="无 buy fill lots" size="small" />
                <n-text depth="3" style="display: block; margin-top: 6px; font-size: 12px">
                  {{ attribution.dataSourceNote }}
                </n-text>
              </template>
            </div>
<div class="holdings-evaluation">
            <n-space align="center" style="margin: 4px 0 10px">
              <n-text strong>C. 持仓分析 · 持仓评价</n-text>
              <n-tag size="small" type="info" :bordered="false">Holding Evaluation · 只读</n-tag>
              <n-tag size="small" :bordered="false">Attribution → Builder</n-tag>
              <n-tag
                v-if="holdingsEval && !holdingsEval.reconcileAllMatched"
                size="small"
                type="warning"
                :bordered="false"
              >
                存在未完全归因
              </n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
              回答「现在持有什么、表现如何」。持有天数取最早 attributed buy；风险标签（NORMAL / WATCH /
              DANGER）仅由浮亏收益率派生；趋势暂为 UNKNOWN（不接行情）。风险不是卖出建议、Exit Signal
              或交易动作。展开行可查看 Lot 来源（plan_id / plan_item_id / buy fill_id）。
            </n-text>
            <n-text
              v-if="holdingsEvalUnavailable"
              depth="3"
              type="warning"
              style="display: block; margin-bottom: 8px"
            >
              持仓评价 API 暂不可用（不影响上方 Snapshot）。
            </n-text>
            <template v-else-if="holdingsEval">
              <n-data-table
                v-if="(holdingsEval.holdings || []).length"
                size="small"
                :columns="holdingsEvalColumns"
                :data="holdingsEval.holdings"
                :row-key="(row) => row.stockCode"
                v-model:expanded-row-keys="expandedEvalKeys"
                :bordered="false"
                :single-line="false"
                style="margin-bottom: 8px"
              />
              <n-empty v-else description="暂无持仓评价（无归因 lots）" style="margin: 8px 0" />
              <n-text
                v-if="(holdingsEval.unattributable || []).length"
                depth="3"
                type="warning"
                style="display: block; margin-bottom: 6px; font-size: 12px"
              >
                未归因数量：
                {{
                  holdingsEval.unattributable
                    .map((u) => `${u.stockCode}×${u.volume}`)
                    .join(' · ')
                }}
                （不伪造 fill）
              </n-text>
              <n-text depth="3" style="display: block; font-size: 12px">
                {{ holdingsEval.dataSourceNote }}
              </n-text>
            </template>
          </div>
          </div>
<div class="exit-evaluation">
            <n-space align="center" style="margin: 4px 0 10px">
              <n-text strong>D. 退出复评</n-text>
              <n-tag size="small" type="warning" :bordered="false">Exit Evaluation · 复评观察</n-tag>
              <n-tag size="small" :bordered="false">非卖出建议</n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
              系统根据持有时间、浮亏与关联计划状态，标记是否需要
              <n-text strong>重新检查</n-text>
              持仓假设。对 WATCH / REVIEW_REQUIRED 标的点击
              <n-text strong>「查看复评」</n-text>
              了解原因。
              <n-text strong>不会自动卖出。</n-text>
            </n-text>
            <n-text
              v-if="exitEvalUnavailable"
              depth="3"
              type="warning"
              style="display: block; margin-bottom: 8px"
            >
              退出评估 API 暂不可用。
            </n-text>
            <template v-else-if="exitEval">
              <n-space v-if="(exitEval.holdings || []).length" :wrap="true" style="margin-bottom: 8px">
                <n-button
                  size="tiny"
                  :type="exitEvalFilter === EXIT_EVAL_FILTER.ALL ? 'primary' : 'default'"
                  :secondary="exitEvalFilter !== EXIT_EVAL_FILTER.ALL"
                  @click="exitEvalFilter = EXIT_EVAL_FILTER.ALL"
                >
                  全部
                </n-button>
                <n-button
                  size="tiny"
                  :type="exitEvalFilter === EXIT_EVAL_FILTER.WATCH_PLUS ? 'primary' : 'default'"
                  :secondary="exitEvalFilter !== EXIT_EVAL_FILTER.WATCH_PLUS"
                  @click="exitEvalFilter = EXIT_EVAL_FILTER.WATCH_PLUS"
                >
                  待关注
                </n-button>
                <n-button
                  size="tiny"
                  :type="exitEvalFilter === EXIT_EVAL_FILTER.REVIEW_REQUIRED ? 'primary' : 'default'"
                  :secondary="exitEvalFilter !== EXIT_EVAL_FILTER.REVIEW_REQUIRED"
                  @click="exitEvalFilter = EXIT_EVAL_FILTER.REVIEW_REQUIRED"
                >
                  需复评
                </n-button>
              </n-space>
              <n-data-table
                v-if="exitEvalTableRows.length"
                size="small"
                :columns="exitEvalColumns"
                :data="exitEvalTableRows"
                :row-key="(row) => row.stockCode"
                v-model:expanded-row-keys="expandedExitEvalKeys"
                :bordered="false"
                :single-line="false"
                style="margin-bottom: 8px"
              />
              <n-empty
                v-else-if="(exitEval.holdings || []).length"
                description="当前筛选下无退出复评记录"
                style="margin: 8px 0"
              />
              <n-empty v-else description="暂无退出评估（无归因 lots）" style="margin: 8px 0" />
              <n-text depth="3" style="display: block; font-size: 12px">
                {{ exitEval.dataSourceNote }}
              </n-text>
              <ExitReviewDrawer
                v-model:show="exitReviewDrawerVisible"
                :exit-row="exitReviewTargetRow"
                :exit-meta="exitReviewMeta"
                :cached-holding-eval="holdingsEval"
                @outcome-saved="onExitOutcomeSaved"
              />
              <n-drawer
                v-model:show="narrativeDrawerVisible"
                :width="480"
                placement="right"
              >
                <n-drawer-content title="投资叙事" closable>
                  <InvestmentNarrativePanel
                    v-if="narrativeStockCode"
                    :stock-code="narrativeStockCode"
                    :account-id="exitEval?.accountId || snapshot?.accountId || 0"
                    embedded
                  />
                </n-drawer-content>
              </n-drawer>
            </template>
          </div>
</n-tab-pane>

        <n-tab-pane name="portfolio" tab="组合观察">
          <PortfolioObservationPanel />
        </n-tab-pane>

        <n-tab-pane name="decision-dashboard" tab="决策差异分析">
          <PortfolioDecisionDashboard />
        </n-tab-pane>

        <n-tab-pane name="advanced" tab="高级分析">
          <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
            Phase13-D：风险报告 / 策略解释 / AI 上下文（仅本地 Context Builder，无外部 AI）。受 FeatureGate 控制。
          </n-text>

          <ProductCapabilityPanel
            title="高级风险报告"
            feature="AdvancedRisk"
            scene="advanced_risk_report"
            usage-opened-key="risk_report_opened"
            usage-viewed-key="risk_report_viewed"
            @open="loadObsAdvancedRisk"
          >
            <n-spin :show="advRiskLoading">
              <n-tag v-if="advRiskError" type="warning" :bordered="false">{{ advRiskError }}</n-tag>
              <template v-else-if="advRiskReport">
                <n-space align="center" :wrap="true" style="margin-bottom: 6px">
                  <n-tag size="small" :bordered="false">{{ advRiskReport.status }}</n-tag>
                  <n-tag size="small" type="info" :bordered="false">band {{ advRiskReport.score?.band }}</n-tag>
                  <n-text>综合分 {{ advRiskReport.score?.overall ?? '—' }}</n-text>
                </n-space>
                <n-text depth="3" style="font-size: 12px">
                  {{ (advRiskReport.warnings || []).slice(0, 3).map((w) => w.message || w.code).join('；') || '无告警' }}
                </n-text>
              </template>
              <n-empty v-else size="small" description="打开后展示高级风险报告" />
            </n-spin>
          </ProductCapabilityPanel>

          <ProductCapabilityPanel
            title="策略解释"
            feature="AdvancedObservation"
            scene="strategy_explanation"
            usage-opened-key="strategy_explanation_opened"
            usage-viewed-key="strategy_explanation_viewed"
            :show-tier-switch="false"
            @open="loadObsStrategyExplain"
          >
            <n-spin :show="advExplainLoading">
              <n-tag v-if="advExplainError" type="warning" :bordered="false">{{ advExplainError }}</n-tag>
              <template v-else-if="advExplainResult">
                <n-space align="center" :wrap="true" style="margin-bottom: 6px">
                  <n-tag size="small" :bordered="false">{{ advExplainResult.status }}</n-tag>
                  <n-text strong>{{ advExplainResult.headline || '—' }}</n-text>
                </n-space>
                <n-text depth="3" style="font-size: 12px; white-space: pre-wrap">
                  {{
                    [
                      advExplainResult.sections?.signal?.narrative,
                      advExplainResult.sections?.risk?.narrative,
                      advExplainResult.sections?.entry?.narrative,
                    ]
                      .filter(Boolean)
                      .join('\n') || '（无摘要）'
                  }}
                </n-text>
              </template>
              <n-empty v-else size="small" description="打开后展示策略解释（取 upcoming 首条明细）" />
            </n-spin>
          </ProductCapabilityPanel>

          <ProductCapabilityPanel
            title="AI 分析上下文"
            feature="AIAnalysis"
            scene="risk_explain"
            usage-opened-key="assistant_context_opened"
            usage-viewed-key="assistant_context_viewed"
            :show-tier-switch="false"
            @open="loadObsAssistantContext"
          >
            <n-spin :show="advCtxLoading">
              <n-tag v-if="advCtxError" type="warning" :bordered="false">{{ advCtxError }}</n-tag>
              <template v-else-if="advCtxResult">
                <n-space align="center" :wrap="true" style="margin-bottom: 6px">
                  <n-tag size="small" :bordered="false">{{ advCtxResult.status }}</n-tag>
                  <n-tag size="small" type="info" :bordered="false">{{ advCtxResult.scene }}</n-tag>
                </n-space>
                <n-text depth="3" style="display: block; font-size: 12px; white-space: pre-wrap; max-height: 180px; overflow: auto">
                  {{ advCtxResult.prompt_skeleton || '（无 prompt skeleton）' }}
                </n-text>
                <n-text depth="3" style="display: block; font-size: 11px; margin-top: 6px">
                  本地 Context Builder · 未调用外部 AI · 无买卖建议
                </n-text>
              </template>
              <n-empty v-else size="small" description="打开后展示本地 Assistant Context" />
            </n-spin>
          </ProductCapabilityPanel>
        </n-tab-pane>

        <n-tab-pane name="daily" tab="历史日报">
          <n-text depth="3" style="display: block; margin-bottom: 10px">
            日终不可变快照（paper_sim_daily_reports）。15:05 Settlement 后生成；非实时持仓。
          </n-text>
          <n-data-table
            v-if="dailyReports && (dailyReports.reports || []).length"
            size="small"
            :columns="dailyReportColumns"
            :data="dailyReports.reports"
            :bordered="false"
            :single-line="false"
          />
          <n-empty v-else description="暂无历史日报（需 enablePaperTrading 且当日 Settlement 成功后生成）" />
        </n-tab-pane>
      </n-tabs>

      <n-empty
        v-if="errorMessage && !today && !snapshot && !runs && !dailyReports"
        :description="errorMessage"
        style="margin-top: 24px"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.paper-observation {
  padding: 12px 16px;
}
.source-banner {
  padding: 10px 12px;
  margin-bottom: 8px;
  border-left: 3px solid #2080f0;
  background: rgba(32, 128, 240, 0.06);
}
.source-desc {
  margin: 6px 0 4px;
  font-size: 13px;
  line-height: 1.5;
}
.paper-snapshot {
  margin: 8px 0 18px;
  padding: 12px 14px;
  border: 1px solid rgba(24, 160, 88, 0.28);
  border-radius: 6px;
  background: rgba(24, 160, 88, 0.04);
}
.obs-section {
  margin: 8px 0 18px;
  padding: 12px 14px;
  border-radius: 6px;
}
.exec-observation {
  border: 1px solid rgba(32, 128, 240, 0.28);
  background: rgba(32, 128, 240, 0.04);
}
.holdings-analysis {
  border: 1px solid rgba(240, 160, 32, 0.3);
  background: rgba(240, 160, 32, 0.05);
}
.holdings-evaluation {
  margin: 8px 0 12px;
  padding: 0;
  border: none;
  background: transparent;
}
.exit-evaluation {
  margin: 8px 0 18px;
  padding: 12px 14px;
  border: 1px solid rgba(208, 48, 80, 0.28);
  border-radius: 6px;
  background: rgba(208, 48, 80, 0.04);
}
.portfolio-observation {
  border: 1px solid rgba(64, 128, 192, 0.28);
  background: rgba(64, 128, 192, 0.04);
}
.pnl-up { color: #d03050; }
.pnl-down { color: #18a058; }
</style>
