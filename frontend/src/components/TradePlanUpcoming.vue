<script setup>
import { computed, h, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NDataTable,
  NDatePicker,
  NEmpty,
  NSpace,
  NSpin,
  NTag,
  NText,
  NTooltip,
  useDialog,
  useMessage,
} from 'naive-ui'
import {
  TRADE_PLAN_CODE_NO_PLAN,
  TRADE_PLAN_CODE_NO_UPCOMING,
  TRADE_PLAN_CODE_OK,
  approveTradePlan,
  canShowMorningMaterializeButton,
  formatMaterializeMorningDisplay,
  freezeTradePlan,
  generateNextTradePlan,
  getTradePlanById,
  getTradePlanExecutionReadiness,
  getTradePlanLifecycle,
  getTradePlanReadiness,
  getUpcomingTradePlan,
  getSameDayCandidates,
  inferPricingStageForMaterializeUI,
  materializeMorningTradePlan,
} from '../api/tradePlans'
import {
  paperTradingRun,
  PAPER_RUN_ERROR,
  mapPaperRunUserMessage,
  parsePaperRunErrorCode,
} from '../api/paperTradingRun'
import { getPortfolioDashboard } from '../api/portfolioDashboard'
import { getPortfolioSnapshot } from '../api/portfolioSnapshot'
import { buildTradePlanPresentation, emptyTradePlanPresentation } from '../viewmodels/tradePlanPresentation.js'
import { buildIntentMaterializePreflight } from '../viewmodels/tradePlan/intentPreflight.js'
import {
  T_SELL_GUIDANCE_PHASES,
  canExecuteTSell,
  resolveIsTSellFlow,
  resolveTradePlanSourceLabel,
  resolveTradePlanSourceBucketLabel,
  resolveCandidateStatusLabel,
  resolveTSellGuidancePhase,
  resolveTSellStickyCtaKind,
  tSellExecuteDisabledReason,
  tSellStickyCtaHint,
  tSellStickyCtaLabel,
} from '../viewmodels/tradePlan/tSellFlow.js'
import {
  buildSellExecutionFeedback,
  resolveTSellExecutionStatus,
  tSellExecutionStatusLabel,
  tSellExecutionStatusTagType,
} from '../viewmodels/tradePlan/sellExecutionFeedback.js'
import {
  buildSellPlanDetailCard,
  resolvePlanLifecycleTagType,
} from '../viewmodels/tradePlan/sellPlanDetail.js'
import { describeReadinessFinding } from '../utils/readinessExplain'
import { buildUpcomingDisplayContext } from '../utils/planContext.js'
import {
  adaptTradePlanCamel,
  adaptTradePlanItem,
  applyStockClickAction,
} from '../utils/stockDisplayAdapters.js'
import {
  formatPriceValue,
  formatPriceWithContext,
  priceColumnTitle,
  priceKindTooltip,
  PRICE_KIND,
} from '../utils/priceDisplay.js'
import {
  formatStatus,
  formatFieldTooltip,
  pendingMorningPrepLabel,
  pendingMorningPrepTooltip,
} from '../utils/statusDisplay.js'
import { BETA_SIM_MODE_BANNER, BETA_SIM_MODE_HINT, betaConfirmContent } from '../utils/betaFriendSafety.js'
import ProductCapabilityPanel from './ProductCapabilityPanel.vue'
import TradePlanOriginPanel from './TradePlanOriginPanel.vue'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'
import TradePlanExplanationSummaryTable from './tradeplan/TradePlanExplanationSummaryTable.vue'
import TradePlanExplanationDrawer from './tradeplan/TradePlanExplanationDrawer.vue'
import { getStrategyExplanation } from '../api/productCapabilities'

const UI_ACTOR = 'ui:trade-plan-upcoming'
const UI_SOURCE = 'ui'
const T_SELL_EXECUTE_ACTOR = 'ui:tradeplan-sell'

const route = useRoute()
const router = useRouter()

const materializeCtaRowRef = ref(null)
const materializeCtaHighlight = ref(false)
let materializeFocusHighlightTimer = null

const REF_AMOUNT_TIP =
  '该金额为观察阶段参考配置，不代表真实仓位计算结果。真实仓位需要结合账户资金、风险预算和价格模型。'

const message = useMessage()
const dialog = useDialog()
const loading = ref(false)
const sameDayCandidates = ref([])
const sameDayCandidatesLoading = ref(false)
const selectingCandidate = ref(false)

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

function openStockKline(model) {
  applyStockClickAction(model, klineModal)
}

function renderStockNameKlineLink(model) {
  if (!model?.displayText && !model?.code) {
    return '—'
  }
  if (!model.klineKey) {
    return model.displayText || model.code || '—'
  }
  return h(StockLink, {
    model,
    onOpen: openStockKline,
  })
}
const generating = ref(false)
const approving = ref(false)
const freezing = ref(false)
const materializing = ref(false)
const executingSell = ref(false)
const lastSellRunResult = ref(null)
const sellDashboardFills = ref([])
const sellPortfolioPositions = ref([])
const portfolioRefreshNote = ref('')
const tradeDateInput = ref(null)
const requestTradeDate = ref('')
const nextTradingDay = ref('')
const emptyMessage = ref('')
const plan = ref(null)
/** Last known pricing_stage from materialize response (UI-only until upcoming exposes it). */
const knownPricingStage = ref('')
const materializeResult = ref(null)
const materializeDisplay = ref(null)

/** Dual-slot summaries (Phase10 UI semantics). Detail pane still uses `plan`. */
const focusSlot = ref('today') // today | next | custom
const todaySlot = ref({ tradeDate: '', plan: null, empty: '' })
const nextSlot = ref({ tradeDate: '', plan: null, empty: '', note: '' })
const showTodaySlot = ref(true)
const samePlanMessage = ref('')
const displayDefaultFocus = ref('today')

/** Phase13-D: strategy explanation panel state */
const explainLoading = ref(false)
const explainError = ref('')
/** Map<itemId(number), { itemId, code, name, explanation }> */
const explainCache = ref(new Map())
/** Drawer state */
const explainDrawerShow = ref(false)
const explainDrawerEntry = ref(null)

/** Ordered list of cached entries (insertion order) for Level 1 summary table */
const explainCacheItems = computed(() => Array.from(explainCache.value.values()))

async function loadStrategyExplanation(row, { openDrawer = true } = {}) {
  const planId = Number(plan.value?.id || 0)
  const itemId = Number(row?.id || 0)
  if (!planId || !itemId) {
    explainError.value = '缺少计划信息，无法拉取策略解释'
    return
  }
  const packEntry = (explanation) => ({
    itemId,
    code: row.stock_code,
    name: row.stock_name,
    planItemStatus: row.status,
    limitPrice: row.limit_price,
    explanation: explanation || null,
  })

  // Cache hit: reuse, optionally open drawer
  if (explainCache.value.has(itemId)) {
    const cached = explainCache.value.get(itemId)
    if (openDrawer) openExplainDetail(cached)
    return
  }

  explainLoading.value = true
  explainError.value = ''
  try {
    const resp = await getStrategyExplanation({
      planId,
      planItemId: itemId,
      includeExit: false,
    })
    if (!resp?.ok) {
      explainError.value = resp?.message || '策略解释请求失败'
      return
    }
    const entry = packEntry(resp.explanation || null)
    const newCache = new Map(explainCache.value)
    newCache.set(itemId, entry)
    explainCache.value = newCache
    if (openDrawer) openExplainDetail(entry)
  } catch (e) {
    explainError.value = e?.message || String(e)
  } finally {
    explainLoading.value = false
  }
}

function onExplainPanelOpen() {
  // Phase16.24: do NOT batch-load all items — only warm first candidate without forcing drawer.
  const items = presentation.value?.executionCandidates || []
  const first = items.find((it) => Number(it?.id || 0) > 0) || items[0]
  if (first) loadStrategyExplanation(first, { openDrawer: false })
  else {
    explainError.value = '当前无执行候选，无法打开策略解释'
  }
}

function openExplainDetail(entry) {
  explainDrawerEntry.value = entry || null
  explainDrawerShow.value = true
}

const tradeDatePlaceholder = computed(() =>
  nextTradingDay.value
    ? `交易日期（下一交易日 ${nextTradingDay.value}）`
    : '交易日期（未选则查今日）',
)

const readinessLoading = ref(false)
const readinessError = ref('')
const readinessData = ref(null)
const executionReadinessLoading = ref(false)
const executionReadinessError = ref('')
const executionReadinessData = ref(null)
const lifecycleView = ref(null)
const lifecycleError = ref('')

const hasPlan = computed(() => !!plan.value)
const hasReadiness = computed(() => !!readinessData.value)
const hasExecutionReadiness = computed(() => !!executionReadinessData.value)

/** Phase13 P0: read-only presentation buckets (does not alter plan / trading). */
const presentation = computed(() => {
  if (!plan.value) return emptyTradePlanPresentation()
  return buildTradePlanPresentation(plan.value)
})
const executionCandidates = computed(() => presentation.value.executionCandidates || [])
const blockedCandidates = computed(() => presentation.value.blockedCandidates || [])
const researchCandidates = computed(() => presentation.value.researchCandidates || [])
const presentationSummary = computed(() => presentation.value.summary || {})

const executionStatusTagType = computed(() => {
  const st = String(executionReadinessData.value?.status || '').toUpperCase()
  if (st === 'READY') return 'success'
  if (st === 'WARNING') return 'warning'
  if (st === 'BLOCKED') return 'error'
  return 'default'
})

const concentrationLabel = computed(() => {
  const c = String(executionReadinessData.value?.concentration || '').toUpperCase()
  if (c === 'HIGH') return '偏高（高）'
  if (c === 'ELEVATED') return '偏高'
  return '正常'
})

const executionConflictCount = computed(() => (executionReadinessData.value?.conflicts || []).length)

/** Risk observation default (matches backend allocation.DefaultMaxGrossExposurePct). Display only. */
const DEFAULT_MAX_GROSS_EXPOSURE_PCT = 85

const executionCashAfterPlan = computed(() => {
  const d = executionReadinessData.value
  if (!d) return null
  return Math.max(0, Number(d.availableCash || 0) - Number(d.requiredCash || 0))
})

const executionCurrentStockPct = computed(() => {
  const d = executionReadinessData.value
  const equity = Number(d?.totalEquity || 0)
  if (!equity) return null
  const cash = Number(d?.availableCash || 0)
  return ((equity - cash) / equity) * 100
})

const executionAfterStockPct = computed(() => {
  const d = executionReadinessData.value
  const equity = Number(d?.totalEquity || 0)
  if (!equity) return null
  const cash = Number(d?.availableCash || 0)
  const required = Number(d?.requiredCash || 0)
  const stockMv = equity - cash + required
  return (stockMv / equity) * 100
})

const GAP_SKIP_TOOLTIP =
  '开盘价格偏离参考价格超过允许范围，本次不生成买入订单'

function isGapSkipIntent(row) {
  return String(row?.intent_status || '').trim().toLowerCase() === 'gap_skip'
}

function renderGapSkipLabel() {
  return h(
    NTooltip,
    { placement: 'top' },
    {
      trigger: () =>
        h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => '跳空跳过' }),
      default: () => GAP_SKIP_TOOLTIP,
    },
  )
}

const sourceLabel = computed(() => resolveTradePlanSourceLabel(plan.value?.source_session))

const showSameDaySelector = computed(() => (sameDayCandidates.value || []).length > 1)

const currentViewingPlanId = computed(() => Math.trunc(Number(plan.value?.id) || 0))

function candidateChipLabel(c) {
  const bucket = resolveTradePlanSourceBucketLabel(c?.source)
  const session = resolveTradePlanSourceLabel(c?.source_session)
  const st = resolveCandidateStatusLabel(c?.status, c?.is_frozen)
  const ver = Number(c?.plan_version) > 0 ? `v${c.plan_version}` : 'v—'
  return `${bucket} · ${session} · ${st} · ${ver}`
}

function candidateTitle(c) {
  return `计划 #${c?.id || '—'} · ${c?.source_session || c?.source || ''} · ${c?.status || ''}`
}

const triggerLabel = computed(() => {
  const s = String(plan.value?.source_session || '').trim()
  if (s === 't_sell') return 't_sell'
  if (s === 'exit_review') return 'exit_review'
  if (s === 'after_close') return 'after_close'
  if (s === 'morning_rebuild') return 'morning_rebuild'
  return 'manual'
})

const isTSellFlow = computed(() =>
  resolveIsTSellFlow({
    routeFlow: route.query.flow,
    sourceSession: plan.value?.source_session,
  }),
)

/** 选股宇宙：复用 items[].strategy_name（follow = fallback；其它 = 策略结果）。不改 API/引擎。 */
function inferSelectionSource(p) {
  const items = Array.isArray(p?.items) ? p.items : []
  if (!items.length) {
    return { kind: 'unknown', label: '未知', strategyLabel: '' }
  }
  const names = items.map((it) => String(it?.strategy_name || '').trim())
  const followCount = names.filter((n) => n.toLowerCase() === 'follow').length
  if (followCount > 0 && followCount === names.length) {
    return { kind: 'follow_fallback', label: '备用执行方案', strategyLabel: 'follow' }
  }
  const strategyLabel = names.find((n) => n && n.toLowerCase() !== 'follow') || names.find(Boolean) || ''
  if (strategyLabel) {
    return { kind: 'strategy_result', label: 'Strategy Result', strategyLabel }
  }
  return { kind: 'unknown', label: '未知', strategyLabel: '' }
}

const selectionSource = computed(() => inferSelectionSource(plan.value))
const isFollowFallback = computed(() => selectionSource.value.kind === 'follow_fallback')
const selectionSourceMetaLabel = computed(() => {
  const s = selectionSource.value
  if (s.kind === 'follow_fallback') return '⚠ 备用执行方案'
  if (s.kind === 'strategy_result') return `策略结果（${s.strategyLabel}）`
  return s.label || '—'
})

/** Phase16.28-B: Origin 表补全股票名称（Origin API 仅有 stock_code）. */
const originNameByCode = computed(() => {
  const map = Object.create(null)
  const items = Array.isArray(plan.value?.items) ? plan.value.items : []
  for (const it of items) {
    const code = String(it?.stock_code || '').trim()
    const name = String(it?.stock_name || '').trim()
    if (!code || !name) continue
    map[code] = name
    map[code.toLowerCase()] = name
  }
  return map
})

function pad2(n) {
  return String(n).padStart(2, '0')
}

const generatedAtLabel = computed(() => {
  const raw = String(plan.value?.generated_at || '').trim()
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return (
    `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ` +
    `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
  )
})

const planVersionLabel = computed(() => {
  const v = Number(plan.value?.plan_version)
  if (!Number.isFinite(v) || v <= 0) return '—'
  return `v${v}`
})

const readinessReady = computed(() => {
  const blockers = readinessData.value?.blockers || []
  return blockers.length === 0
})

const blockerCount = computed(() => (readinessData.value?.blockers || []).length)
const warningCount = computed(() => (readinessData.value?.warnings || []).length)

const isDraft = computed(() => String(plan.value?.status || '') === 'draft')
const isReady = computed(() => String(plan.value?.status || '') === 'ready')
const isApproved = computed(() => !!String(plan.value?.freeze?.approved_at || '').trim())
const isFrozen = computed(() => !!plan.value?.freeze?.is_frozen)

const lifecycleLabel = computed(() => {
  if (isFrozen.value || isReady.value) return formatStatus('ready', 'plan').label
  if (isDraft.value) return formatStatus('draft', 'plan').label
  return formatStatus(plan.value?.status, 'plan').label
})

const lifecycleTagType = computed(() =>
  resolvePlanLifecycleTagType({
    lifecycleLabel: lifecycleLabel.value,
    isReady: isReady.value,
    isFrozen: isFrozen.value,
  }),
)

const sellPlanDetailCard = computed(() => {
  if (!isTSellFlow.value || !plan.value) return null
  return buildSellPlanDetailCard({
    plan: plan.value,
    snapshotPositions: sellPortfolioPositions.value,
  })
})

const approvalLabel = computed(() => (isApproved.value ? '已审批' : '未审批'))
const freezeLabel = computed(() => (isFrozen.value ? '已冻结' : '未冻结'))

const windowStatus = computed(() => String(plan.value?.window?.status || '').trim())
const windowReasonLabel = computed(() => {
  const label = String(plan.value?.window?.reason_label || '').trim()
  if (label) return label
  const reason = String(plan.value?.window?.reason || '').trim()
  if (reason === 'PLAN_FROZEN_AFTER_DEADLINE') return 'Plan frozen after deadline'
  if (reason === 'PLAN_NOT_FROZEN_BEFORE_OPEN') return 'Plan not frozen before open deadline'
  if (reason === 'NO_EXECUTION_WINDOW') return 'No execution window available'
  return reason || '—'
})
const isMissedOpenWindow = computed(() => windowStatus.value === 'MISSED_OPEN_WINDOW')
const openWindowLabel = computed(() => {
  const start = String(plan.value?.window?.open_window_start || '09:30:00').slice(0, 5)
  const end = String(plan.value?.window?.open_window_end || '11:30:00').slice(0, 5)
  return `${start} - ${end}`
})
const freezeDeadlineLabel = computed(() =>
  String(plan.value?.window?.freeze_deadline || '09:29:30').slice(0, 8),
)

function morningCheckTagType(v) {
  const s = String(v || '').toUpperCase()
  if (s === 'PASS' || s === 'READY') return 'success'
  if (s === 'FAIL' || s === 'MISSED_DEADLINE') return 'error'
  if (s === 'PENDING' || s === 'NOT_READY') return 'warning'
  return 'default'
}

const morningStatus = computed(() => String(plan.value?.morning?.status || '').trim())
const morningReasonLabel = computed(() => {
  const label = String(plan.value?.morning?.reason_label || '').trim()
  if (label) return label
  return String(plan.value?.morning?.reason || '').trim() || '—'
})

const automationMode = computed(() => String(plan.value?.automation?.mode || 'MANUAL').trim())
const automationMaterialization = computed(() => String(plan.value?.automation?.materialization || '—'))
const automationApproval = computed(() => String(plan.value?.automation?.approval || '—'))
const automationFreeze = computed(() => String(plan.value?.automation?.freeze || '—'))

const approveDisabledReason = computed(() => {
  if (!hasPlan.value) return '当前无交易计划'
  if (isFrozen.value) return '计划已冻结，无法再次审批'
  if (!isDraft.value) {
    return `当前状态为 ${formatStatus(plan.value?.status, 'plan').label}，仅草稿可审批`
  }
  if (isApproved.value) return '计划已审批'
  if (readinessLoading.value) return 'Readiness 评估中'
  if (!hasReadiness.value) return readinessError.value || '暂无 Readiness，无法审批'
  if (!readinessReady.value) return 'Readiness 存在阻断，暂不可审批'
  return ''
})

const freezeDisabledReason = computed(() => {
  if (!hasPlan.value) return '当前无交易计划'
  if (isFrozen.value) return '计划已冻结'
  if (!isDraft.value) {
    return `当前状态为 ${formatStatus(plan.value?.status, 'plan').label}，仅草稿可冻结`
  }
  if (!isApproved.value) return '请先完成计划审批'
  if (readinessLoading.value) return 'Readiness 评估中'
  if (!hasReadiness.value) return readinessError.value || '暂无 Readiness，无法冻结'
  if (!readinessReady.value) return 'Risk / Readiness 存在阻断，暂不可冻结'
  return ''
})

const canApprove = computed(() => !approveDisabledReason.value)
const canFreeze = computed(() => !freezeDisabledReason.value)

const inferredPricingStage = computed(() =>
  inferPricingStageForMaterializeUI({
    explicitPricingStage: knownPricingStage.value,
    lifecycleStage: readinessData.value?.lifecycle_stage,
    blockers: readinessData.value?.blockers || [],
  }),
)

const showMorningMaterialize = computed(() => {
  if (isTSellFlow.value) return false
  return canShowMorningMaterializeButton({
    status: plan.value?.status,
    isFrozen: isFrozen.value,
    pricingStage: inferredPricingStage.value,
  })
})

const intentPreflight = computed(() =>
  buildIntentMaterializePreflight(plan.value, { isFrozen: isFrozen.value }),
)

const canMaterializeByPreflight = computed(
  () => showMorningMaterialize.value && intentPreflight.value.canMaterialize,
)

const materializeResultTagType = computed(() => {
  const level = String(materializeDisplay.value?.level || '').toLowerCase()
  if (level === 'success') return 'success'
  if (level === 'warning') return 'warning'
  if (materializeDisplay.value?.ok) return 'success'
  return 'error'
})

/** Phase14-A-R0-B / 16.23: user-facing draft→locked flow (UI-only; does not change state machine). */
const GUIDANCE_PHASES = [
  { id: 1, mark: '①', label: '计划生成', note: '生成下一交易日草稿；不会自动进入模拟执行。' },
  { id: 2, mark: '②', label: '价格准备', note: '准备价格只写入限价与数量，不是下单或执行。' },
  { id: 3, mark: '③', label: '等待批准', note: '人工批准后仍为草稿；需再次确认后锁定。' },
  {
    id: 4,
    mark: '④',
    label: '锁定并等待模拟执行',
    note: '锁定后可供本地模拟执行读取；仍不会真实下单。',
  },
]

const guidancePhases = computed(() =>
  isTSellFlow.value ? T_SELL_GUIDANCE_PHASES : GUIDANCE_PHASES,
)

const isMorningMaterialized = computed(() => {
  if (isTSellFlow.value) return true
  const stage = String(inferredPricingStage.value || '').trim()
  if (stage === 'morning_materialized') return true
  if (stage === 'after_close_intent') return false
  // Materialize button hidden + draft + readiness ready → treat as prepared
  return !showMorningMaterialize.value && isDraft.value && readinessReady.value
})

/** Active step among ①–④ (plan exists ⇒ step 1 done). */
const currentGuidancePhase = computed(() => {
  if (!hasPlan.value) return 1
  if (isTSellFlow.value) {
    return resolveTSellGuidancePhase({
      hasPlan: true,
      isApproved: isApproved.value,
      isFrozen: isFrozen.value,
      isReady: isReady.value,
    })
  }
  if (isFrozen.value || isReady.value) return 4
  if (isApproved.value && !isFrozen.value) return 4
  if (isMorningMaterialized.value && !isApproved.value) return 3
  return 2
})

const currentGuidanceMeta = computed(() => {
  const phase = guidancePhases.value.find((p) => p.id === currentGuidancePhase.value)
  return phase || guidancePhases.value[0]
})

const sellExecutionFeedback = computed(() =>
  buildSellExecutionFeedback({
    plan: plan.value,
    lifecycle: lifecycleView.value,
    dashboardFills: sellDashboardFills.value,
    lastRun: lastSellRunResult.value,
  }),
)

const executeSellDisabledReason = computed(() =>
  tSellExecuteDisabledReason({
    hasPlan: hasPlan.value,
    isFrozen: isFrozen.value,
    isReady: isReady.value,
    executing: executingSell.value,
    planId: plan.value?.id,
    hasFill: sellExecutionFeedback.value.hasFill,
  }),
)

const tSellExecutionStatus = computed(() =>
  resolveTSellExecutionStatus({
    executing: executingSell.value,
    isApproved: isApproved.value,
    isFrozen: isFrozen.value,
    isReady: isReady.value,
    hasFill: sellExecutionFeedback.value.hasFill,
  }),
)

const tSellExecutionStatusText = computed(() => tSellExecutionStatusLabel(tSellExecutionStatus.value))
const tSellExecutionStatusType = computed(() => tSellExecutionStatusTagType(tSellExecutionStatus.value))

const stickyCtaKind = computed(() => {
  if (!hasPlan.value) return 'none'
  if (isTSellFlow.value) {
    return resolveTSellStickyCtaKind({
      hasPlan: true,
      isApproved: isApproved.value,
      isFrozen: isFrozen.value,
      isReady: isReady.value,
    })
  }
  if (isFrozen.value || isReady.value) return 'wait_exec'
  if (isApproved.value && !isFrozen.value) return 'freeze'
  if (isMorningMaterialized.value && !isApproved.value) return 'approve'
  return 'materialize'
})

const stickyCtaLabel = computed(() => {
  if (isTSellFlow.value) return tSellStickyCtaLabel(stickyCtaKind.value)
  switch (stickyCtaKind.value) {
    case 'materialize':
      return '准备价格'
    case 'approve':
      return '批准计划'
    case 'freeze':
      return '冻结计划'
    case 'wait_exec':
      return '等待模拟执行'
    default:
      return ''
  }
})

const stickyCtaLoading = computed(() => {
  switch (stickyCtaKind.value) {
    case 'materialize':
      return materializing.value
    case 'approve':
      return approving.value
    case 'freeze':
      return freezing.value
    case 'execute_sell':
      return executingSell.value
    default:
      return false
  }
})

const stickyCtaDisabled = computed(() => {
  if (loading.value || generating.value || readinessLoading.value) return true
  if (isTSellFlow.value) {
    switch (stickyCtaKind.value) {
      case 'approve':
        return !canApprove.value
      case 'freeze':
        return !canFreeze.value
      case 'execute_sell':
        return !canExecuteTSell({
          hasPlan: hasPlan.value,
          isFrozen: isFrozen.value,
          isReady: isReady.value,
          executing: executingSell.value,
          planId: plan.value?.id,
          hasFill: sellExecutionFeedback.value.hasFill,
        })
      default:
        return true
    }
  }
  switch (stickyCtaKind.value) {
    case 'materialize':
      return !canMaterializeByPreflight.value
    case 'approve':
      return !canApprove.value
    case 'freeze':
      return !canFreeze.value
    case 'wait_exec':
      return true
    default:
      return true
  }
})

const stickyCtaHint = computed(() => {
  if (isTSellFlow.value) {
    return tSellStickyCtaHint(stickyCtaKind.value, {
      approveDisabledReason: approveDisabledReason.value,
      freezeDisabledReason: freezeDisabledReason.value,
      executeDisabledReason: executeSellDisabledReason.value,
    })
  }
  switch (stickyCtaKind.value) {
    case 'materialize':
      if (!showMorningMaterialize.value) {
        if (isMorningMaterialized.value) return '价格已准备'
        return '当前不满足价格准备条件（需草稿、未锁定，且为盘后计划阶段）'
      }
      if (!intentPreflight.value.canMaterialize) {
        return intentPreflight.value.blockReason || intentPreflight.value.warnReason || '暂不可准备价格'
      }
      if (intentPreflight.value.warnReason) return intentPreflight.value.warnReason
      return `根据开盘价写入限价与数量（${intentPreflight.value.summaryLine}）`
    case 'approve':
      return approveDisabledReason.value || '确认计划内容后批准'
    case 'freeze':
      return freezeDisabledReason.value || '冻结后状态变为 Ready，可供模拟盘读取'
    case 'wait_exec':
      return '计划已冻结；模拟执行由 Paper Trading Job 在开盘窗口读取'
    default:
      return ''
  }
})

const stickyCtaButtonType = computed(() => {
  switch (stickyCtaKind.value) {
    case 'materialize':
      return 'info'
    case 'approve':
      return 'primary'
    case 'freeze':
      return 'warning'
    case 'execute_sell':
      return 'error'
    default:
      return 'default'
  }
})

function onStickyCtaClick() {
  switch (stickyCtaKind.value) {
    case 'materialize':
      confirmMaterializeMorning()
      break
    case 'approve':
      confirmApprove()
      break
    case 'freeze':
      confirmFreeze()
      break
    case 'execute_sell':
      confirmExecuteSell()
      break
    default:
      break
  }
}

const sellFillColumns = [
  { title: '代码', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      return renderStockNameKlineLink(adaptTradePlanCamel(row))
    },
  },
  {
    title: '成交数量',
    key: 'quantity',
    width: 100,
    render(row) {
      return Number(row.quantity || 0).toLocaleString('zh-CN')
    },
  },
  {
    title: '成交价',
    key: 'price',
    width: 90,
    render(row) {
      const n = Number(row.price)
      return Number.isFinite(n) && n > 0 ? n.toFixed(2) : '—'
    },
  },
  {
    title: '成交时间',
    key: 'filledAt',
    width: 170,
    render(row) {
      return formatSellFillTime(row.filledAt)
    },
  },
]

/** Phase16.19-B1 decision main table: identity → score → risk → strategy → prices → status → explain. */
const itemColumns = [
  {
    title: '股票',
    key: 'stock',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      return renderStockNameKlineLink(adaptTradePlanItem(row))
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, '评分'),
          default: () => formatFieldTooltip('score'),
        },
      ),
    key: 'score',
    width: 80,
    render(row) {
      const n = Number(row.score)
      if (!Number.isFinite(n)) return '—'
      return n.toFixed(2)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, '风险'),
          default: () => formatFieldTooltip('risk'),
        },
      ),
    key: 'risk',
    ellipsis: { tooltip: true },
    minWidth: 100,
    render(row) {
      const code = String(row.risk_code || '').trim()
      const msg = String(row.risk_message || '').trim()
      if (!code && !msg) return '—'
      return code && msg ? `${code}: ${msg}` : code || msg
    },
  },
  {
    title: '策略',
    key: 'strategy_name',
    ellipsis: { tooltip: true },
    minWidth: 100,
    render(row) {
      return row.strategy_name || '—'
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, priceColumnTitle(PRICE_KIND.ref)),
          default: () => priceKindTooltip(PRICE_KIND.ref),
        },
      ),
    key: 'ref_price',
    width: 110,
    render(row) {
      return renderRefPriceCell(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, priceColumnTitle(PRICE_KIND.limit)),
          default: () => `${priceKindTooltip(PRICE_KIND.limit)}\n${pendingMorningPrepTooltip()}`,
        },
      ),
    key: 'limit_price',
    width: 120,
    render(row) {
      if (isGapSkipIntent(row)) return renderGapSkipLabel()
      const text = formatPreviewLimitOrVolume(row.limit_price)
      if (text === pendingMorningPrepLabel()) {
        return h(
          NTooltip,
          { trigger: 'hover' },
          {
            trigger: () => h(NText, { depth: 3 }, { default: () => text }),
            default: () => pendingMorningPrepTooltip(),
          },
        )
      }
      return text
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render(row) {
      const st = formatStatus(row.status, 'plan_item')
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(NTag, { size: 'small', type: st.type, bordered: false }, { default: () => st.label }),
          default: () => st.tooltip || st.label,
        },
      )
    },
  },
  {
    title: '操作',
    key: 'strategy_explain',
    width: 100,
    render(row) {
      const itemId = Number(row?.id || 0)
      const cached = itemId && explainCache.value.has(itemId)
      return h(
        NButton,
        {
          size: 'tiny',
          secondary: true,
          disabled: !itemId || !plan.value?.id,
          loading: explainLoading.value && !cached,
          onClick: () => loadStrategyExplanation(row, { openDrawer: true }),
        },
        { default: () => '查看解释' },
      )
    },
  },
]

const blockedColumns = [
  {
    title: '股票',
    key: 'stock',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      return renderStockNameKlineLink(adaptTradePlanItem(row))
    },
  },
  { title: '状态', key: 'status', width: 90 },
  {
    title: '阻断原因',
    key: 'block_reason',
    ellipsis: { tooltip: true },
    render(row) {
      return row.block_reason || '—'
    },
  },
  {
    title: '得分',
    key: 'score',
    width: 80,
    render(row) {
      return Number(row.score || 0).toFixed(2)
    },
  },
]

function formatPreviewPrice(v) {
  return formatPriceValue(v, { digits: 2, empty: '—' })
}

function renderRefPriceCell(row) {
  const ctx = formatPriceWithContext(row.ref_price, PRICE_KIND.ref, {
    asOf: row.ref_as_of || row.refAsOf || '',
    source: row.ref_source || row.refSource || '',
    digits: 2,
  })
  if (ctx.value === '—') return '—'
  if (!ctx.tooltip || ctx.tooltip === `${ctx.label}：—`) {
    return h(NText, null, { default: () => ctx.value })
  }
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () => h(NText, null, { default: () => ctx.value }),
      default: () => ctx.tooltip,
    },
  )
}

/** AfterClose: limit/volume = 0 is expected — show as pending morning prep, not error. */
function formatPreviewLimitOrVolume(v, { isVolume = false } = {}) {
  const n = Number(v)
  if (!Number.isFinite(n) || n <= 0) return pendingMorningPrepLabel()
  if (isVolume) return String(Math.trunc(n))
  return formatPriceValue(n, { digits: 2 })
}

function formatIntentStatus(status, row) {
  if (row && isGapSkipIntent(row)) {
    return renderGapSkipLabel()
  }
  const s = String(status || '').trim()
  if (!s) return pendingMorningPrepLabel()
  if (s.toLowerCase() === 'gap_skip') return renderGapSkipLabel()
  return formatStatus(s, 'intent').label
}

/** UI-only execution display status (does not mutate plan/intent state machine). */
function mapExecutionDisplayStatus(row) {
  const itemStatus = String(row?.status || '').trim().toLowerCase()
  const intent = String(row?.intent_status || '').trim().toLowerCase()
  if (itemStatus === 'filled' || intent === 'executed' || intent === 'done') {
    return 'EXECUTED'
  }
  if (intent === 'priced' || intent === 'ready') {
    return 'READY'
  }
  const limit = Number(row?.limit_price)
  const vol = Number(row?.target_volume)
  if (Number.isFinite(limit) && limit > 0 && Number.isFinite(vol) && vol > 0 && intent) {
    return 'READY'
  }
  return 'WAITING_INTENT'
}

function formatEstimatedFillAmount(row) {
  const limit = Number(row?.limit_price)
  const vol = Number(row?.target_volume)
  if (!Number.isFinite(limit) || limit <= 0 || !Number.isFinite(vol) || vol <= 0) return '—'
  return (limit * vol).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

const previewColumns = [
  {
    title: '股票',
    key: 'stock',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      return renderStockNameKlineLink(adaptTradePlanItem(row))
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, '预算金额'),
          default: () => REF_AMOUNT_TIP,
        },
      ),
    key: 'target_amount',
    width: 110,
    render(row) {
      const n = Number(row.target_amount) || 0
      return n.toLocaleString()
    },
  },
  {
    title: '目标数量',
    key: 'target_volume',
    width: 100,
    render(row) {
      if (isGapSkipIntent(row)) return renderGapSkipLabel()
      return formatPreviewLimitOrVolume(row.target_volume, { isVolume: true })
    },
  },
  {
    title: '估算成交金额',
    key: 'estimated_fill_amount',
    width: 120,
    render(row) {
      return formatEstimatedFillAmount(row)
    },
  },
  {
    title: '资金占用',
    key: 'capital_ratio',
    width: 90,
    render() {
      return 'N/A'
    },
  },
  {
    title: '订单规则',
    key: 'entry_rule',
    width: 140,
    ellipsis: { tooltip: true },
    render(row) {
      return String(row.entry_rule || '').trim() || '—'
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, '准备状态'),
          default: () => '早盘准备进度（内部技术状态的中文展示，不是真实下单）',
        },
      ),
    key: 'intent_status',
    minWidth: 140,
    ellipsis: { tooltip: true },
    render(row) {
      if (isGapSkipIntent(row)) return renderGapSkipLabel()
      const raw = String(row.intent_status || '').trim()
      const empty = !raw
      const label = formatIntentStatus(row.intent_status, row)
      const tip = empty
        ? pendingMorningPrepTooltip()
        : formatStatus(raw, 'intent').tooltip || label
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(
              NTag,
              { size: 'small', type: empty ? 'warning' : 'info', bordered: false },
              { default: () => label },
            ),
          default: () => tip,
        },
      )
    },
  },
  {
    title: '执行状态',
    key: 'execution_display_status',
    width: 120,
    render(row) {
      const st = mapExecutionDisplayStatus(row)
      const disp = formatStatus(st, 'execution')
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(
              NTag,
              { size: 'small', type: disp.type, bordered: false },
              { default: () => disp.label },
            ),
          default: () => disp.tooltip || disp.label,
        },
      )
    },
  },
]

function formatMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 0 })
}

function clearReadiness() {
  readinessData.value = null
  readinessError.value = ''
  readinessLoading.value = false
  executionReadinessData.value = null
  executionReadinessError.value = ''
  executionReadinessLoading.value = false
  lifecycleView.value = null
  lifecycleError.value = ''
}

function clearMaterializeResult() {
  materializeResult.value = null
  materializeDisplay.value = null
}

async function loadExecutionReadiness(planId) {
  executionReadinessLoading.value = true
  executionReadinessError.value = ''
  executionReadinessData.value = null
  try {
    const res = await getTradePlanExecutionReadiness(planId)
    if (res.code === TRADE_PLAN_CODE_NO_UPCOMING || res.code === TRADE_PLAN_CODE_NO_PLAN) {
      executionReadinessError.value = res.message || '无执行准备观察'
      return
    }
    if (res.code !== TRADE_PLAN_CODE_OK || !res.ok) {
      executionReadinessError.value = res.message || `执行准备观察加载失败 (code=${res.code})`
      return
    }
    executionReadinessData.value = res
  } catch (e) {
    executionReadinessError.value = e?.message || String(e)
  } finally {
    executionReadinessLoading.value = false
  }
}

async function loadReadiness(planId) {
  readinessLoading.value = true
  readinessError.value = ''
  readinessData.value = null
  try {
    const res = await getTradePlanReadiness({ planId })
    if (res.code === TRADE_PLAN_CODE_NO_PLAN) {
      readinessError.value = res.message || '无 Readiness（40401）'
      return
    }
    if (res.code !== TRADE_PLAN_CODE_OK || !res.readiness) {
      readinessError.value = res.message || `Readiness 加载失败 (code=${res.code})`
      message.warning(readinessError.value)
      return
    }
    readinessData.value = res.readiness
  } catch (e) {
    readinessError.value = e?.message || String(e)
    message.warning(readinessError.value)
  } finally {
    readinessLoading.value = false
  }
}

async function loadLifecycle(planId) {
  lifecycleView.value = null
  lifecycleError.value = ''
  const id = Math.trunc(Number(planId) || 0)
  if (!id) return
  try {
    const res = await getTradePlanLifecycle(id)
    if (!res.ok || !res.lifecycle) {
      lifecycleError.value = res.message || '生命周期不可用'
      return
    }
    lifecycleView.value = res.lifecycle
  } catch (e) {
    lifecycleError.value = e?.message || String(e)
  }
}

function formatLifecycleTs(v) {
  if (v == null || v === '') return 'null'
  return String(v)
}

function formatSellFillTime(raw) {
  const s = String(raw || '').trim()
  if (!s || s === 'null') return '—'
  return s.replace('T', ' ').slice(0, 19)
}

function slotStatusLabel(p) {
  if (!p) return '无计划'
  if (p.freeze?.is_frozen || String(p.status || '') === 'ready') return formatStatus('frozen', 'plan').label
  if (String(p.status || '') === 'draft') return formatStatus('draft', 'plan').label
  return formatStatus(p.status, 'plan').label
}

function slotRoleHint(kind, p) {
  if (!p) return kind === 'today' ? '今日暂无待执行计划' : '下一交易日暂无草稿/计划'
  if (kind === 'today') {
    return p.freeze?.is_frozen || String(p.status) === 'ready'
      ? '已进入执行周期'
      : '今日视图（非 Frozen）'
  }
  return String(p.status) === 'draft' ? '等待审核' : '下一交易日视图'
}

async function loadSlotPair() {
  const now = new Date()
  const resToday = await getUpcomingTradePlan()
  const prelim = buildUpcomingDisplayContext({ resToday, now })
  const nextDay = prelim.nextTradingDay
  nextTradingDay.value = nextDay
  let resNext = null
  if (nextDay) {
    resNext = await getUpcomingTradePlan(nextDay)
  }
  const ctx = buildUpcomingDisplayContext({ resToday, resNext, now })
  showTodaySlot.value = ctx.showTodaySlot
  samePlanMessage.value = ctx.samePlanMessage
  displayDefaultFocus.value = ctx.defaultFocus
  todaySlot.value = {
    tradeDate: ctx.todaySlot.tradeDate,
    plan: ctx.todaySlot.plan,
    empty: ctx.todaySlot.empty,
  }
  nextSlot.value = {
    tradeDate: ctx.nextSlot.tradeDate,
    plan: ctx.nextSlot.plan,
    empty: ctx.nextSlot.empty,
    note: ctx.nextSlot.note || '',
  }
  return { resToday, resNext, nextDay, ctx }
}

async function focusTodaySlot() {
  if (!showTodaySlot.value) {
    await focusNextSlot()
    return
  }
  focusSlot.value = 'today'
  const td = String(todaySlot.value.tradeDate || '').trim()
  if (td) tradeDateInput.value = td
  else tradeDateInput.value = null
  // Slot navigation uses upcoming for that day; sync URL after load (prefer today).
  await refresh({ skipSlotReload: true, prefer: 'today', ignoreRoutePlanId: true })
}

async function focusNextSlot() {
  focusSlot.value = 'next'
  const td = String(nextSlot.value.tradeDate || nextTradingDay.value || '').trim()
  if (td) tradeDateInput.value = td
  if (samePlanMessage.value && todaySlot.value.plan) {
    loading.value = true
    emptyMessage.value = ''
    clearReadiness()
    try {
      const loaded = await applyPlanResponse(
        {
          code: TRADE_PLAN_CODE_OK,
          ok: true,
          trade_date: todaySlot.value.tradeDate,
          next_trading_day: nextTradingDay.value,
          plan_id: todaySlot.value.plan.id,
          plan: todaySlot.value.plan,
          message: '',
        },
        { queryDate: todaySlot.value.tradeDate },
      )
      if (!loaded) return
      await bindActivePlanExtras(loaded)
    } catch (e) {
      emptyMessage.value = e?.message || String(e)
      message.error(emptyMessage.value)
    } finally {
      loading.value = false
    }
    return
  }
  await refresh({ skipSlotReload: true, prefer: 'next', ignoreRoutePlanId: true })
}

function readRoutePlanId() {
  const raw = route.query.plan_id ?? route.query.planId
  return Math.trunc(Number(raw) || 0)
}

/** Keep URL plan_id aligned with the active detail plan (display-only). */
async function syncRoutePlanId(planId) {
  const id = Math.trunc(Number(planId) || 0)
  const cur = readRoutePlanId()
  if (id > 0) {
    if (cur === id) return
    const nextQuery = { ...route.query, plan_id: String(id) }
    delete nextQuery.planId
    try {
      await router.replace({ name: route.name, query: nextQuery })
    } catch (_) {
      /* ignore navigation duplicates */
    }
    return
  }
  if (cur <= 0) return
  const nextQuery = { ...route.query }
  delete nextQuery.plan_id
  delete nextQuery.planId
  try {
    await router.replace({ name: route.name, query: nextQuery })
  } catch (_) {
    /* ignore */
  }
}

async function applyPlanResponse(res, { queryDate } = {}) {
  requestTradeDate.value = res.trade_date || queryDate || String(tradeDateInput.value || '').trim() || ''
  nextTradingDay.value = res.next_trading_day || ''
  if (res.code === TRADE_PLAN_CODE_NO_UPCOMING) {
    plan.value = null
    knownPricingStage.value = ''
    clearMaterializeResult()
    sameDayCandidates.value = []
    emptyMessage.value = res.message || '暂无即将交易的计划'
    return null
  }
  if (res.code !== TRADE_PLAN_CODE_OK) {
    plan.value = null
    knownPricingStage.value = ''
    clearMaterializeResult()
    sameDayCandidates.value = []
    emptyMessage.value = res.message || `加载失败 (code=${res.code})`
    message.error(emptyMessage.value)
    return null
  }
  const prevId = Number(plan.value?.id) || 0
  plan.value = res.plan
  if (!res.plan) {
    knownPricingStage.value = ''
    clearMaterializeResult()
    sameDayCandidates.value = []
    emptyMessage.value = res.message || '暂无即将交易的计划'
    return null
  }
  if (prevId && prevId !== Number(res.plan.id)) {
    knownPricingStage.value = ''
    clearMaterializeResult()
  }
  if (res.plan.trade_date) {
    tradeDateInput.value = res.plan.trade_date
  }
  emptyMessage.value = ''
  await loadSameDayCandidates(res.plan.trade_date)
  return res.plan
}

async function loadSameDayCandidates(tradeDate) {
  const td = String(tradeDate || '').trim()
  if (!td) {
    sameDayCandidates.value = []
    return
  }
  sameDayCandidatesLoading.value = true
  try {
    const res = await getSameDayCandidates(td)
    sameDayCandidates.value = Array.isArray(res.candidates) ? res.candidates.filter((c) => c.id > 0) : []
  } catch (_) {
    sameDayCandidates.value = []
  } finally {
    sameDayCandidatesLoading.value = false
  }
}

async function selectSameDayCandidate(cand) {
  const id = Math.trunc(Number(cand?.id) || 0)
  if (id <= 0 || selectingCandidate.value) return
  if (id === currentViewingPlanId.value) return
  selectingCandidate.value = true
  try {
    const nextQuery = { ...route.query, plan_id: String(id) }
    await router.replace({ name: route.name, query: nextQuery })
    await refreshFromPlanId(id)
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    selectingCandidate.value = false
  }
}

async function refreshPortfolioCaches() {
  const td = String(plan.value?.trade_date || '').trim()
  const pid = Math.trunc(Number(plan.value?.id) || 0)
  try {
    const snap = await getPortfolioSnapshot(td ? { tradeDate: td } : undefined)
    sellPortfolioPositions.value = Array.isArray(snap?.positions) ? snap.positions : []
    const dash = await getPortfolioDashboard(td || undefined)
    sellDashboardFills.value = (dash.trades?.fills || []).filter(
      (f) => f.planId === pid && String(f.side || '').toLowerCase() === 'sell',
    )
    portfolioRefreshNote.value = new Date().toLocaleString('zh-CN')
  } catch {
    /* best-effort; plan/lifecycle refresh still primary */
  }
}

async function refreshAfterSellExecute() {
  await refreshCurrentPlan()
  await refreshPortfolioCaches()
}

async function refreshCurrentPlan() {
  const id = Math.trunc(Number(plan.value?.id) || 0)
  if (id > 0) await refreshFromPlanId(id)
  else await refresh({ ignoreRoutePlanId: true })
}

/** After active plan is set: sync URL + load satellite panels from the same plan id. */
async function bindActivePlanExtras(loaded) {
  if (!loaded?.id) return
  await syncRoutePlanId(loaded.id)
  await loadReadiness(loaded.id)
  await loadExecutionReadiness(loaded.id)
  await loadLifecycle(loaded.id)
}

async function refreshFromPlanId(planId) {
  const id = Math.trunc(Number(planId) || 0)
  if (id <= 0) {
    await refresh({ ignoreRoutePlanId: true })
    return
  }
  loading.value = true
  emptyMessage.value = ''
  clearReadiness()
  let fallbackUpcoming = false
  try {
    const res = await getTradePlanById(id)
    const loaded = await applyPlanResponse(res)
    if (!loaded) {
      fallbackUpcoming = true
      return
    }
    try {
      await loadSlotPair()
    } catch (_) {
      /* slot pair is best-effort; detail already loaded */
    }
    const td = String(loaded.trade_date || '').trim()
    if (td && td === String(nextSlot.value.tradeDate || nextTradingDay.value || '').trim()) {
      focusSlot.value = 'next'
    } else if (td && td === String(todaySlot.value.tradeDate || '').trim()) {
      focusSlot.value = 'today'
    } else {
      focusSlot.value = 'custom'
    }
    await bindActivePlanExtras(loaded)
    if (resolveIsTSellFlow({
      routeFlow: route.query.flow,
      sourceSession: loaded?.source_session,
    })) {
      await refreshPortfolioCaches()
    }
  } catch (e) {
    plan.value = null
    knownPricingStage.value = ''
    clearMaterializeResult()
    emptyMessage.value = e?.message || String(e)
    fallbackUpcoming = true
  } finally {
    loading.value = false
  }
  if (fallbackUpcoming) {
    message.warning('无法加载指定计划，已回退到即将执行计划')
    await refresh({ ignoreRoutePlanId: true })
  }
}

async function refresh(opts = {}) {
  // Soft refresh / init: restore URL plan_id before upcoming slot selection.
  // Slot navigation passes ignoreRoutePlanId so today/next can change the active plan.
  if (!opts.ignoreRoutePlanId) {
    const routeId = readRoutePlanId()
    if (routeId > 0) {
      await refreshFromPlanId(routeId)
      return
    }
  }

  const skipSlotReload = !!opts.skipSlotReload
  const prefer = opts.prefer || ''
  loading.value = true
  emptyMessage.value = ''
  clearReadiness()
  try {
    let resToday = null
    let resNext = null
    let nextDay = nextTradingDay.value
    if (!skipSlotReload) {
      const pair = await loadSlotPair()
      resToday = pair.resToday
      resNext = pair.resNext
      nextDay = pair.nextDay || ''
    }

    const queryDate = tradeDateInput.value ? String(tradeDateInput.value).trim() : undefined

    // Date picker override → custom detail (upcoming semantics unchanged for that date).
    if (queryDate && prefer !== 'today' && prefer !== 'next') {
      // If query matches next-day slot date, treat as next focus for card highlight.
      if (nextDay && queryDate === nextDay) focusSlot.value = 'next'
      else if (todaySlot.value.tradeDate && queryDate === todaySlot.value.tradeDate) focusSlot.value = 'today'
      else focusSlot.value = 'custom'
      const res = await getUpcomingTradePlan(queryDate)
      const loaded = await applyPlanResponse(res, { queryDate })
      if (!loaded) return
      await bindActivePlanExtras(loaded)
      return
    }

    // Default / today focus: keep Frozen-first today upcoming (do NOT auto-switch to next Draft).
    if (prefer === 'next' || focusSlot.value === 'next') {
      focusSlot.value = 'next'
      if (nextSlot.value.plan) {
        const loaded = await applyPlanResponse(
          {
            code: TRADE_PLAN_CODE_OK,
            ok: true,
            trade_date: nextSlot.value.tradeDate,
            next_trading_day: nextDay,
            plan_id: nextSlot.value.plan.id,
            plan: nextSlot.value.plan,
            message: '',
          },
          { queryDate: nextSlot.value.tradeDate },
        )
        if (!loaded) return
        await bindActivePlanExtras(loaded)
        return
      }
      if (samePlanMessage.value && todaySlot.value.plan) {
        const loaded = await applyPlanResponse(
          {
            code: TRADE_PLAN_CODE_OK,
            ok: true,
            trade_date: todaySlot.value.tradeDate,
            next_trading_day: nextDay,
            plan_id: todaySlot.value.plan.id,
            plan: todaySlot.value.plan,
            message: '',
          },
          { queryDate: todaySlot.value.tradeDate },
        )
        if (!loaded) return
        await bindActivePlanExtras(loaded)
        return
      }
      if (nextDay) {
        const res = resNext || (await getUpcomingTradePlan(nextDay))
        const loaded = await applyPlanResponse(res, { queryDate: nextDay })
        if (!loaded) return
        await bindActivePlanExtras(loaded)
        return
      }
    }

    // Default: presentation context picks today (trading day + plan) or next.
    if (!prefer || prefer === 'today' || focusSlot.value === 'today') {
      if (showTodaySlot.value && todaySlot.value.plan) {
        focusSlot.value = 'today'
        const loaded = await applyPlanResponse(
          {
            code: TRADE_PLAN_CODE_OK,
            ok: true,
            trade_date: todaySlot.value.tradeDate,
            next_trading_day: nextDay,
            plan_id: todaySlot.value.plan.id,
            plan: todaySlot.value.plan,
            message: '',
          },
          { queryDate: todaySlot.value.tradeDate },
        )
        if (!loaded) return
        await bindActivePlanExtras(loaded)
        return
      }
    }

    focusSlot.value = displayDefaultFocus.value || 'next'
    if (nextSlot.value.plan) {
      const loaded = await applyPlanResponse(
        {
          code: TRADE_PLAN_CODE_OK,
          ok: true,
          trade_date: nextSlot.value.tradeDate,
          next_trading_day: nextDay,
          plan_id: nextSlot.value.plan.id,
          plan: nextSlot.value.plan,
          message: '',
        },
        { queryDate: nextSlot.value.tradeDate },
      )
      if (!loaded) return
      await bindActivePlanExtras(loaded)
      return
    }
    if (nextDay) {
      const res = resNext || (await getUpcomingTradePlan(nextDay))
      const loaded = await applyPlanResponse(res, { queryDate: nextDay })
      if (!loaded) return
      await bindActivePlanExtras(loaded)
      return
    }

    focusSlot.value = displayDefaultFocus.value || 'next'
    emptyMessage.value = '无法解析下一交易日'
  } catch (e) {
    plan.value = null
    knownPricingStage.value = ''
    clearMaterializeResult()
    emptyMessage.value = e?.message || String(e)
    message.error(emptyMessage.value)
  } finally {
    loading.value = false
  }
}

function confirmGenerateNext() {
  if (generating.value) return
  const selectedTradeDate = tradeDateInput.value ? String(tradeDateInput.value).trim() : ''
  const targetDay = selectedTradeDate || nextTradingDay.value || '下一交易日'
  dialog.warning({
    title: '生成交易计划',
    content: betaConfirmContent([
      `将为 ${targetDay} 生成下一交易日模拟计划草稿。`,
      '不会自动批准、锁定或执行，也不会真实下单。',
      '重复生成会创建新的计划版本（仅本地模拟）。',
    ]),
    positiveText: '确认生成模拟计划',
    negativeText: '取消',
    maskClosable: false,
    closeOnEsc: false,
    onPositiveClick: async () => {
      if (generating.value) return false
      generating.value = true
      try {
        const res = await generateNextTradePlan({
          actor: UI_ACTOR,
          ...(selectedTradeDate ? { tradeDate: selectedTradeDate } : {}),
        })
        if (res.code !== TRADE_PLAN_CODE_OK) {
          message.error(res.message || `生成失败 (code=${res.code})`)
          return
        }
        if (!res.ok && res.failed_step === 'risk') {
          message.warning(
            `计划 v${res.plan_version || '—'} 已生成，但风险检查未通过；未批准、未锁定、未执行`,
          )
        } else if (!res.ok) {
          message.error(res.message || `生成失败（${res.failed_step || 'unknown'}）`)
          return
        } else {
          message.success(`已生成 ${res.trade_date} 交易计划 v${res.plan_version}`)
        }
        const newPlanId = Number(res.plan_id) || 0
        if (res.trade_date) {
          tradeDateInput.value = String(res.trade_date).trim()
        }
        if (newPlanId > 0) {
          await refreshFromPlanId(newPlanId)
        } else {
          await refresh({ ignoreRoutePlanId: true })
        }
      } catch (e) {
        message.error(e?.message || String(e))
      } finally {
        generating.value = false
      }
    },
  })
}

function confirmApprove() {
  if (!canApprove.value || !plan.value) {
    message.warning(approveDisabledReason.value || '当前不可批准')
    return
  }
  const p = plan.value
  const execCount = presentation.value.summary?.executionCount ?? 0
  dialog.warning({
    title: isTSellFlow.value ? '批准卖出（模拟确认）' : '批准计划（模拟确认）',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', { style: 'font-weight: 600; margin-bottom: 6px' }, BETA_SIM_MODE_BANNER),
        h('div', { style: 'margin-bottom: 8px; color: #666' }, BETA_SIM_MODE_HINT),
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', `执行候选：${execCount}`),
        h(
          'div',
          `计划状态：${formatStatus(p.status, 'plan').label}`,
        ),
        h('div', { style: 'margin-top: 8px' }, isTSellFlow.value ? '批准卖出后：' : '批准后：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', '仍保持草稿（仅记录批准时间与操作者）'),
          h('li', isTSellFlow.value ? '不会立即模拟卖出，也不会真实下单' : '不会锁定、不会真实下单'),
          h('li', '确认生成/批准的是模拟计划，不是券商委托'),
        ]),
      ]),
    positiveText: isTSellFlow.value ? '确认批准模拟卖出' : '确认批准模拟计划',
    negativeText: '取消',
    onPositiveClick: async () => {
      approving.value = true
      try {
        const res = await approveTradePlan({
          planId: p.id,
          actor: UI_ACTOR,
          source: UI_SOURCE,
        })
        if (res.code !== TRADE_PLAN_CODE_OK || !res.ok) {
          message.error(res.message || `批准失败 (code=${res.code})`)
          return
        }
        message.success(isTSellFlow.value ? '卖出计划已批准（仍为草稿）' : '计划已批准（仍为草稿）')
        await refreshCurrentPlan()
      } catch (e) {
        message.error(e?.message || String(e))
      } finally {
        approving.value = false
      }
    },
  })
}

function confirmFreeze() {
  if (!canFreeze.value || !plan.value) {
    message.warning(freezeDisabledReason.value || '当前不可冻结')
    return
  }
  const p = plan.value
  const execCount = presentation.value.summary?.executionCount ?? 0
  dialog.warning({
    title: isTSellFlow.value ? '锁定卖出计划（模拟）' : '锁定计划（模拟）',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', { style: 'font-weight: 600; margin-bottom: 6px' }, BETA_SIM_MODE_BANNER),
        h('div', { style: 'margin-bottom: 8px; color: #666' }, BETA_SIM_MODE_HINT),
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', `执行候选：${execCount}`),
        h('div', `计划状态：${formatStatus(p.status, 'plan').label}`),
        h('div', { style: 'margin-top: 8px' }, '锁定后说明：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', '状态将变为「已准备 / 已锁定」'),
          h('li', '可供本地模拟执行读取'),
          h('li', '后续修改需要生成新版本'),
          h(
            'li',
            isTSellFlow.value
              ? '本操作不会立即卖出；需再确认「执行卖出」才会写入模拟成交'
              : '本操作不会立即下单；模拟成交仍由本地流程处理',
          ),
        ]),
      ]),
    positiveText: '确认锁定模拟计划',
    negativeText: '取消',
    onPositiveClick: async () => {
      freezing.value = true
      try {
        const res = await freezeTradePlan({
          planId: p.id,
          actor: UI_ACTOR,
          reason: 'ui freeze',
          source: UI_SOURCE,
        })
        if (res.code !== TRADE_PLAN_CODE_OK || !res.ok) {
          message.error(res.message || `冻结失败 (code=${res.code})`)
          return
        }
        message.success(
          res.already_frozen
            ? (isTSellFlow.value ? '卖出计划已处于锁定状态' : '计划已处于锁定状态')
            : (isTSellFlow.value ? '卖出计划已锁定（模拟执行准备）' : '计划已锁定（模拟执行准备）'),
        )
        await refreshCurrentPlan()
      } catch (e) {
        message.error(e?.message || String(e))
      } finally {
        freezing.value = false
      }
    },
  })
}

function confirmExecuteSell() {
  if (!isTSellFlow.value || !plan.value) return
  if (!canExecuteTSell({
    hasPlan: true,
    isFrozen: isFrozen.value,
    isReady: isReady.value,
    executing: executingSell.value,
    planId: plan.value.id,
    hasFill: sellExecutionFeedback.value.hasFill,
  })) {
    message.warning(executeSellDisabledReason.value || '当前不可执行卖出')
    return
  }
  const p = plan.value
  dialog.warning({
    title: '执行卖出（本地模拟）',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', { style: 'font-weight: 600; margin-bottom: 6px' }, BETA_SIM_MODE_BANNER),
        h('div', { style: 'margin-bottom: 8px; color: #666' }, BETA_SIM_MODE_HINT),
        h('div', `计划：${p.id}`),
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', { style: 'margin-top: 8px' }, '确认后将写入本地模拟卖出记录：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', '仅模拟成交，不会连接券商'),
          h('li', '可在组合/执行记录中观察结果'),
        ]),
      ]),
    positiveText: '确认模拟卖出',
    negativeText: '取消',
    onPositiveClick: async () => {
      executingSell.value = true
      lastSellRunResult.value = null
      try {
        const res = await paperTradingRun(p.id, {
          actor: T_SELL_EXECUTE_ACTOR,
          tradeDate: p.trade_date,
        })
        lastSellRunResult.value = res.result || { status: res.message, message: res.message }
        if (res.alreadyExecuted || res.errorCode === PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED) {
          message.warning(mapPaperRunUserMessage(PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED))
          await refreshAfterSellExecute()
          return
        }
        const filled = Math.trunc(Number(res.result?.filledCount) || 0)
        const detail = res.message || res.result?.message || ''
        if (filled > 0) {
          message.success(`卖出执行完成：成交 ${filled} 笔${detail ? `（${detail}）` : ''}`)
        } else {
          message.warning(detail || '执行完成，但未产生成交（请查看 Readiness / 生命周期）')
        }
        await refreshAfterSellExecute()
      } catch (e) {
        const code = e?.errorCode || parsePaperRunErrorCode(e?.message)
        if (code === PAPER_RUN_ERROR.PLAN_NOT_FROZEN) {
          message.error(mapPaperRunUserMessage(PAPER_RUN_ERROR.PLAN_NOT_FROZEN, e?.httpStatus, e?.message))
        } else if (code === PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED) {
          message.warning(mapPaperRunUserMessage(PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED))
          await refreshAfterSellExecute()
        } else if (code === PAPER_RUN_ERROR.MISSING_PLAN_ID) {
          message.error(mapPaperRunUserMessage(PAPER_RUN_ERROR.MISSING_PLAN_ID))
        } else {
          message.error(e?.userMessage || e?.message || String(e))
        }
      } finally {
        executingSell.value = false
      }
    },
  })
}

function confirmMaterializeMorning() {
  if (!showMorningMaterialize.value || !plan.value) {
    message.warning('当前计划不满足早盘准备条件（需草稿、盘后计划阶段、未锁定）')
    return
  }
  if (!intentPreflight.value.canMaterialize) {
    message.warning(intentPreflight.value.blockReason || '暂无可执行计划，请检查候选选择')
    return
  }
  const p = plan.value
  const preflightWarn = intentPreflight.value.warnReason
  dialog.warning({
    title: '准备价格（早盘准备）',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', { style: 'font-weight: 600; margin-bottom: 6px' }, BETA_SIM_MODE_BANNER),
        h('div', { style: 'margin-bottom: 8px; color: #666' }, '这是参数准备，不是真实下单，也不会自动买卖。'),
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', `执行准备预检：${intentPreflight.value.summaryLine}`),
        preflightWarn
          ? h('div', { style: 'margin-top: 6px; color: #d03050' }, preflightWarn)
          : null,
        h('div', { style: 'margin-top: 8px' }, '将写入：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', '参考限价'),
          h('li', '目标数量'),
          h('li', '刷新执行准备状态'),
          h('li', '不会自动批准、锁定或真实下单'),
        ]),
      ]),
    positiveText: '确认早盘准备（非下单）',
    negativeText: '取消',
    onPositiveClick: async () => {
      materializing.value = true
      clearMaterializeResult()
      try {
        const res = await materializeMorningTradePlan({ planId: p.id })
        materializeResult.value = res
        materializeDisplay.value = formatMaterializeMorningDisplay(res)
        if (res.pricing_stage) {
          knownPricingStage.value = res.pricing_stage
        }
        if (!res.success) {
          message.error(materializeDisplay.value.toastMessage || materializeDisplay.value.title)
          return
        }
        const toast = materializeDisplay.value.toastMessage || materializeDisplay.value.title
        if (materializeDisplay.value.level === 'warning') {
          message.warning(toast)
        } else {
          message.success(toast)
        }
        await refreshFromPlanId(p.id)
      } catch (e) {
        const errMsg = e?.message || String(e)
        materializeDisplay.value = {
          ok: false,
          title: '早盘准备失败',
          detailLines: [errMsg],
        }
        message.error(errMsg)
      } finally {
        materializing.value = false
      }
    },
  })
}

function isMaterializeFocusQuery() {
  return String(route.query.focus || '').trim() === 'materialize'
}

async function applyMaterializeFocusFromRoute() {
  if (!isMaterializeFocusQuery()) return
  await nextTick()
  const el = materializeCtaRowRef.value
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  materializeCtaHighlight.value = true
  if (materializeFocusHighlightTimer) {
    clearTimeout(materializeFocusHighlightTimer)
  }
  materializeFocusHighlightTimer = setTimeout(() => {
    materializeCtaHighlight.value = false
    materializeFocusHighlightTimer = null
  }, 2600)
}

onMounted(async () => {
  const planId = readRoutePlanId()
  if (planId > 0) {
    await refreshFromPlanId(planId)
  } else {
    await refresh({ ignoreRoutePlanId: true })
  }
  await applyMaterializeFocusFromRoute()
})

watch(
  () => route.query.focus,
  async (focus) => {
    if (String(focus || '').trim() !== 'materialize') return
    if (loading.value) return
    await applyMaterializeFocusFromRoute()
  },
)
</script>

<template>
  <div class="trade-plan-upcoming">
    <n-space justify="space-between" align="center" style="margin-bottom: 8px">
      <n-space align="center" :wrap="true">
        <n-text strong>交易计划</n-text>
        <n-tag size="small" type="info" :bordered="false">观察阶段</n-tag>
        <n-text depth="3">展示上下文按日历交易日切槽；非交易日默认下一交易日计划</n-text>
      </n-space>
      <n-space align="center">
        <n-date-picker
          v-model:formatted-value="tradeDateInput"
          value-format="yyyy-MM-dd"
          type="date"
          clearable
          :disabled="generating"
          :placeholder="tradeDatePlaceholder"
          style="width: 240px"
        />
        <n-button
          type="primary"
          :loading="generating"
          :disabled="loading || readinessLoading || approving || freezing || materializing || generating || executingSell"
          @click="confirmGenerateNext"
        >
          {{ generating ? '生成中，请等待。' : '生成交易计划' }}
        </n-button>
        <n-button
          :loading="loading || readinessLoading"
          :disabled="generating || materializing"
          @click="() => { focusSlot = tradeDateInput ? 'custom' : (showTodaySlot ? 'today' : 'next'); refresh() }"
        >
          刷新
        </n-button>
      </n-space>
    </n-space>

    <div class="plan-slot-row" :class="{ 'plan-slot-row--single': !showTodaySlot }">
      <button
        v-if="showTodaySlot"
        type="button"
        class="plan-slot"
        :class="{ active: focusSlot === 'today' }"
        :disabled="loading || generating"
        @click="focusTodaySlot"
      >
        <div class="plan-slot-k">今日执行计划</div>
        <div class="plan-slot-date">{{ todaySlot.tradeDate || '—' }}</div>
        <div class="plan-slot-meta">
          <n-tag size="small" :type="todaySlot.plan?.freeze?.is_frozen ? 'success' : 'default'" :bordered="false">
            {{ slotStatusLabel(todaySlot.plan) }}
          </n-tag>
          <span v-if="todaySlot.plan">#{{ todaySlot.plan.id }}</span>
        </div>
        <div class="plan-slot-hint">{{ slotRoleHint('today', todaySlot.plan) }}</div>
        <div v-if="todaySlot.empty && !todaySlot.plan" class="plan-slot-empty">{{ todaySlot.empty }}</div>
      </button>
      <button
        type="button"
        class="plan-slot"
        :class="{ active: focusSlot === 'next' }"
        :disabled="loading || generating"
        @click="focusNextSlot"
      >
        <div class="plan-slot-k">下一交易日计划</div>
        <div class="plan-slot-date">{{ nextSlot.tradeDate || nextTradingDay || '—' }}</div>
        <div v-if="!samePlanMessage" class="plan-slot-meta">
          <n-tag size="small" :type="String(nextSlot.plan?.status) === 'draft' ? 'warning' : 'default'" :bordered="false">
            {{ slotStatusLabel(nextSlot.plan) }}
          </n-tag>
          <span v-if="nextSlot.plan">#{{ nextSlot.plan.id }}</span>
        </div>
        <div class="plan-slot-hint">{{ samePlanMessage || slotRoleHint('next', nextSlot.plan) }}</div>
        <div v-if="samePlanMessage" class="plan-slot-same">{{ samePlanMessage }}</div>
        <div v-else-if="nextSlot.empty && !nextSlot.plan" class="plan-slot-empty">{{ nextSlot.empty }}</div>
      </button>
    </div>

    <n-text v-if="generating" depth="3" class="generating-hint">
      生成中，请等待。
    </n-text>

    <n-text depth="3" class="observation-note">
      下方详情对应当前选中卡片。今日槽仅匹配日历当日 trade_date；非交易日隐藏今日槽并默认展示下一交易日计划。不会自动把 upcoming 默认结果当作「今日计划」。
    </n-text>

    <n-spin :show="loading || generating">
      <template v-if="hasPlan">
        <n-alert
          v-if="isTSellFlow"
          type="warning"
          :bordered="false"
          style="margin-bottom: 10px"
          title="人工卖出计划"
        >
          T-sell 卖出流程：批准 → 锁定 → 执行卖出。无需早盘准备或买入价格准备。
        </n-alert>

        <div v-if="isTSellFlow" class="sell-exec-feedback-panel">
          <n-space align="center" :wrap="true" style="margin-bottom: 8px">
            <n-text strong>执行状态</n-text>
            <n-tag size="medium" :type="tSellExecutionStatusType" :bordered="false">
              {{ tSellExecutionStatusText }}
            </n-tag>
            <n-text v-if="portfolioRefreshNote" depth="3" style="font-size: 12px">
              组合数据已刷新 · {{ portfolioRefreshNote }}
            </n-text>
          </n-space>

          <template v-if="sellExecutionFeedback.hasFill || sellExecutionFeedback.executedQuantity > 0">
            <n-space align="center" :wrap="true" style="margin-bottom: 6px">
              <n-text>成交数量</n-text>
              <n-text strong>{{ sellExecutionFeedback.executedQuantity.toLocaleString('zh-CN') }} 股</n-text>
              <n-text depth="3">|</n-text>
              <n-text>成交时间</n-text>
              <n-text>{{ formatSellFillTime(sellExecutionFeedback.fillTime) }}</n-text>
            </n-space>
            <n-text depth="3" class="sell-cash-notice">{{ sellExecutionFeedback.cashNotice }}</n-text>
            <n-data-table
              v-if="sellExecutionFeedback.fills.length"
              size="small"
              :bordered="false"
              style="margin-top: 8px"
              :columns="sellFillColumns"
              :data="sellExecutionFeedback.fills"
            />
          </template>
        </div>

        <n-space vertical :size="10" style="margin-bottom: 14px">
          <div class="meta-grid">
            <div class="meta-item">
              <span class="meta-k">交易计划日期</span>
              <span class="meta-v">{{ plan.trade_date || '—' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">计划版本</span>
              <span class="meta-v">{{ planVersionLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">生成时间</span>
              <span class="meta-v">{{ generatedAtLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">生成来源</span>
              <span class="meta-v">{{ sourceLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">触发方式</span>
              <span class="meta-v">{{ triggerLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">选股来源</span>
              <span class="meta-v" :class="{ 'meta-v-warn': isFollowFallback }">{{ selectionSourceMetaLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">当前状态</span>
              <n-tag size="small" :type="lifecycleTagType" :bordered="false">
                {{ lifecycleLabel }}
              </n-tag>
            </div>
          </div>

          <div
            v-if="showSameDaySelector"
            class="same-day-selector"
            data-phase="PHASE16-26-B1-2-2"
          >
            <div class="same-day-selector-head">
              <n-text strong>同日计划</n-text>
              <n-tag size="small" type="info" :bordered="false">
                {{ plan.trade_date || '—' }} · {{ sameDayCandidates.length }} 个
              </n-tag>
              <n-text depth="3" style="font-size: 12px">
                仅切换查看，不改变 upcoming / 执行 / 物化
              </n-text>
              <n-spin v-if="sameDayCandidatesLoading || selectingCandidate" size="small" />
            </div>
            <div class="same-day-chips">
              <button
                v-for="c in sameDayCandidates"
                :key="'cand-' + c.id"
                type="button"
                class="same-day-chip"
                :class="{ active: c.id === currentViewingPlanId }"
                :title="candidateTitle(c)"
                :disabled="loading || selectingCandidate || generating"
                @click="selectSameDayCandidate(c)"
              >
                <span class="same-day-chip-main">{{ candidateChipLabel(c) }}</span>
                <n-tag
                  v-if="c.id === currentViewingPlanId"
                  size="tiny"
                  type="success"
                  :bordered="false"
                >
                  当前
                </n-tag>
                <n-tag size="tiny" :bordered="false" :type="c.source === 'watchlist' ? 'warning' : 'default'">
                  {{ c.source || '—' }}
                </n-tag>
              </button>
            </div>
          </div>

          <TradePlanOriginPanel
            v-if="hasPlan"
            :plan-id="Number(plan.id) || 0"
            :source-session="String(plan.source_session || '')"
            :name-by-code="originNameByCode"
            @open-stock="openStockKline"
          />

          <div
            v-if="hasPlan"
            class="user-flow-bar"
            data-phase="PHASE14A-R0-B"
            role="region"
            aria-label="草稿至锁定操作进度"
          >
            <div class="user-flow-bar-head">
              <n-text strong>{{ isTSellFlow ? '卖出操作进度' : '操作进度' }}</n-text>
              <n-tag size="small" type="info" :bordered="false">
                当前：{{ currentGuidanceMeta.mark }} {{ currentGuidanceMeta.label }}
              </n-tag>
            </div>
            <div class="guidance-steps" role="list" aria-label="四步进度">
              <span
                v-for="p in guidancePhases"
                :key="'g-' + p.id"
                role="listitem"
                :class="[
                  'guidance-step',
                  {
                    'is-active': currentGuidancePhase === p.id,
                    'is-done': currentGuidancePhase > p.id,
                  },
                ]"
              >
                {{ p.mark }} {{ p.label }}
              </span>
            </div>
            <n-text depth="3" class="guidance-note">{{ currentGuidanceMeta.note }}</n-text>
            <div
              ref="materializeCtaRowRef"
              class="user-flow-cta-row"
              :class="{ 'is-materialize-focus': materializeCtaHighlight }"
            >
              <n-button
                v-if="stickyCtaKind !== 'none'"
                :type="stickyCtaButtonType"
                size="large"
                class="user-flow-cta-btn"
                :loading="stickyCtaLoading"
                :disabled="stickyCtaDisabled"
                @click="onStickyCtaClick"
              >
                {{ stickyCtaLabel }}
              </n-button>
              <n-text depth="3" class="user-flow-cta-hint">{{ stickyCtaHint }}</n-text>
            </div>
          </div>

          <div
            v-if="isTSellFlow && sellPlanDetailCard"
            class="execution-window-panel sell-plan-detail-card"
            role="region"
            aria-label="卖出计划详情"
          >
            <n-space align="center" style="margin-bottom: 8px">
              <n-text strong>卖出计划详情</n-text>
              <n-tag size="small" type="info" :bordered="false">T-sell</n-tag>
            </n-space>
            <div class="meta-grid">
              <div class="meta-item">
                <span class="meta-k">股票代码</span>
                <span class="meta-v">{{ adaptTradePlanCamel(sellPlanDetailCard).code || sellPlanDetailCard.stockCode }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">股票名称</span>
                <span class="meta-v">{{ adaptTradePlanCamel(sellPlanDetailCard).name || '—' }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">卖出数量</span>
                <span class="meta-v">{{ sellPlanDetailCard.sellQuantityLabel }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">当前可卖数量</span>
                <span class="meta-v">{{ sellPlanDetailCard.availableQtyLabel }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">卖出原因</span>
                <span class="meta-v">{{ sellPlanDetailCard.sellReason }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">创建时间</span>
                <span class="meta-v">{{ sellPlanDetailCard.createdAtLabel }}</span>
              </div>
            </div>
            <n-text
              v-if="sellPlanDetailCard.availableQtyLabel === '—'"
              depth="3"
              style="display: block; margin-top: 6px; font-size: 12px"
            >
              可卖数量来自组合快照；刷新计划或执行后将自动更新。
            </n-text>
          </div>

          <n-alert
            v-if="showMorningMaterialize && intentPreflight.selectedCount === 0"
            type="warning"
            :bordered="false"
            class="intent-preflight-alert"
            title="执行准备预检"
          >
            {{ intentPreflight.blockReason }}
          </n-alert>
          <n-alert
            v-else-if="showMorningMaterialize && intentPreflight.warnReason"
            type="info"
            :bordered="false"
            class="intent-preflight-alert"
            title="执行准备预检"
          >
            {{ intentPreflight.warnReason }}
          </n-alert>
          <n-text
            v-else-if="showMorningMaterialize && intentPreflight.selectedCount > 0"
            depth="3"
            class="intent-preflight-summary"
          >
            执行准备预检：{{ intentPreflight.summaryLine }}
          </n-text>

          <div v-if="isTSellFlow" class="execution-window-panel sell-manual-window-panel">
            <n-space align="center" style="margin-bottom: 8px">
              <n-text strong>人工执行窗口</n-text>
              <n-tag size="small" type="info" :bordered="false">T-sell</n-tag>
            </n-space>
            <n-text depth="3" style="display: block; font-size: 13px">
              计划冻结后可手动执行
            </n-text>
          </div>

          <div v-if="!isTSellFlow" class="execution-window-panel">
            <n-space align="center" style="margin-bottom: 8px">
              <n-text strong>Execution Window</n-text>
              <n-tag size="small" type="info" :bordered="false">derived</n-tag>
              <n-tag
                v-if="windowStatus"
                size="small"
                :type="isMissedOpenWindow ? 'warning' : windowStatus === 'OPEN_EXECUTABLE' ? 'success' : 'default'"
                :bordered="false"
              >
                {{ windowStatus }}
              </n-tag>
            </n-space>
            <div class="meta-grid">
              <div class="meta-item">
                <span class="meta-k">Open Window</span>
                <span class="meta-v">{{ openWindowLabel }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">Freeze Deadline</span>
                <span class="meta-v">{{ freezeDeadlineLabel }}</span>
              </div>
            </div>
            <div v-if="isMissedOpenWindow" class="missed-window-banner" role="status">
              <n-text strong>Missed Open Window</n-text>
              <n-text depth="3" style="display: block; margin-top: 4px">
                Reason: {{ windowReasonLabel }}
              </n-text>
              <n-text depth="3" style="display: block; margin-top: 4px; font-size: 12px">
                自动 Open cron 已错过；Manual RunExecution 仍可用（Phase A 不阻断人工执行）。
              </n-text>
            </div>
          </div>

          <div v-if="!isTSellFlow" class="execution-window-panel morning-readiness-panel">
            <n-space align="center" style="margin-bottom: 8px">
              <n-text strong>Morning Readiness</n-text>
              <n-tag size="small" type="info" :bordered="false">derived</n-tag>
              <n-tag
                v-if="morningStatus"
                size="small"
                :type="morningCheckTagType(morningStatus)"
                :bordered="false"
              >
                {{ morningStatus }}
              </n-tag>
            </n-space>
            <div class="meta-grid">
              <div class="meta-item">
                <span class="meta-k">Materialization</span>
                <n-tag
                  size="small"
                  :type="morningCheckTagType(plan.morning?.materialization_status)"
                  :bordered="false"
                >
                  {{ plan.morning?.materialization_status || '—' }}
                </n-tag>
              </div>
              <div class="meta-item">
                <span class="meta-k">Freeze</span>
                <n-tag
                  size="small"
                  :type="morningCheckTagType(plan.morning?.freeze_status)"
                  :bordered="false"
                >
                  {{ plan.morning?.freeze_status || '—' }}
                </n-tag>
              </div>
              <div class="meta-item">
                <span class="meta-k">Deadline</span>
                <n-tag
                  size="small"
                  :type="morningCheckTagType(plan.morning?.deadline_status)"
                  :bordered="false"
                >
                  {{ plan.morning?.deadline_status || '—' }}
                </n-tag>
              </div>
            </div>
            <n-text
              v-if="plan.morning?.reason"
              depth="3"
              style="display: block; margin-top: 6px; font-size: 12px"
            >
              Reason: {{ morningReasonLabel }}
            </n-text>
          </div>

          <div class="execution-window-panel automation-panel">
            <n-space align="center" style="margin-bottom: 8px">
              <n-text strong>Automation Mode</n-text>
              <n-tag size="small" type="info" :bordered="false">{{ automationMode }}</n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
              Today's Morning Automation
            </n-text>
            <div class="meta-grid">
              <div class="meta-item">
                <span class="meta-k">Materialization</span>
                <n-tag size="small" :type="morningCheckTagType(automationMaterialization)" :bordered="false">
                  {{ automationMaterialization }}
                </n-tag>
              </div>
              <div class="meta-item">
                <span class="meta-k">Approval</span>
                <n-tag size="small" :type="morningCheckTagType(automationApproval)" :bordered="false">
                  {{ automationApproval }}
                </n-tag>
              </div>
              <div class="meta-item">
                <span class="meta-k">Freeze</span>
                <n-tag size="small" :type="morningCheckTagType(automationFreeze)" :bordered="false">
                  {{ automationFreeze }}
                </n-tag>
              </div>
            </div>
            <n-text
              v-if="plan.automation?.materialization_reason"
              depth="3"
              style="display: block; margin-top: 6px; font-size: 12px"
            >
              Materialization: {{ plan.automation.materialization_reason }}
            </n-text>
            <n-text
              v-if="plan.automation?.approval_reason"
              depth="3"
              style="display: block; margin-top: 4px; font-size: 12px"
            >
              Approval: {{ plan.automation.approval_reason }}
            </n-text>
            <n-text
              v-if="plan.automation?.freeze_reason"
              depth="3"
              style="display: block; margin-top: 4px; font-size: 12px"
            >
              Freeze: {{ plan.automation.freeze_reason }}
            </n-text>
          </div>

          <div v-if="isFollowFallback" class="selection-source-banner" role="status">
            <div class="selection-source-title">
              选股来源：⚠ 备用执行方案
              <n-tooltip trigger="hover">
                <template #trigger>
                  <n-text depth="3" style="margin-left: 6px; font-size: 12px; cursor: help">说明</n-text>
                </template>
                内部标识 follow fallback：无主策略结果时的备用方案，不代表真实下单。
              </n-tooltip>
            </div>
            <ul class="selection-source-list">
              <li>原因：Strategy candidate result empty（首个启用策略无可用候选，系统回退自选）</li>
              <li>当前股票来源：用户自选列表排序（sort ASC）</li>
              <li>说明：该计划不是策略评分生成；Execution 仍可按状态机推进，Strategy 归因未验证</li>
            </ul>
          </div>
          <div v-else-if="selectionSource.kind === 'strategy_result'" class="selection-source-banner selection-source-ok" role="status">
            <div class="selection-source-title">选股来源：策略结果</div>
            <ul class="selection-source-list">
              <li>策略：{{ selectionSource.strategyLabel }}</li>
              <li>说明：明细来自策略运行结果投影（本地 Rank 仍可能为列表位次分）</li>
            </ul>
          </div>

          <div class="presentation-summary" role="status">
            <n-space align="center" :wrap="true" :size="8">
              <n-tag size="small" type="success" :bordered="false">
                执行候选 {{ presentationSummary.executionCount ?? 0 }} 条
              </n-tag>
              <n-tag size="small" type="warning" :bordered="false">
                阻断 {{ presentationSummary.blockedCount ?? 0 }} 条
              </n-tag>
              <n-tag size="small" type="default" :bordered="false">
                研究池 {{ presentationSummary.researchCount ?? 0 }} 条
              </n-tag>
            </n-space>
            <n-text depth="3" class="section-note" style="display: block; margin-top: 6px">
              {{ presentationSummary.disclaimer }}
            </n-text>
          </div>

          <div class="lifecycle-panel">
            <n-space align="center" style="margin-bottom: 8px">
              <n-text strong>生命周期</n-text>
              <n-tag size="small" type="info" :bordered="false">只读显示态</n-tag>
              <n-tag
                v-if="lifecycleView"
                size="small"
                :type="lifecycleView.displayStatus === 'FROZEN' || lifecycleView.displayStatus === 'COMPLETED' ? 'success' : 'warning'"
                :bordered="false"
              >
                {{ lifecycleView.displayStatus }}
              </n-tag>
            </n-space>
            <n-text v-if="lifecycleError" depth="3" type="warning" style="display: block; margin-bottom: 6px">
              {{ lifecycleError }}
            </n-text>
            <div v-if="lifecycleView" class="lifecycle-grid">
              <div class="meta-item">
                <span class="meta-k">plan_created_at</span>
                <span class="meta-v">{{ formatLifecycleTs(lifecycleView.planCreatedAt) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">approved_at</span>
                <span class="meta-v">{{ formatLifecycleTs(lifecycleView.approvedAt) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">freeze_at</span>
                <span class="meta-v">{{ formatLifecycleTs(lifecycleView.freezeAt) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">execution_started_at</span>
                <span class="meta-v">{{ formatLifecycleTs(lifecycleView.executionStartedAt) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">first_fill_at</span>
                <span class="meta-v">{{ formatLifecycleTs(lifecycleView.firstFillAt) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-k">db_status</span>
                <span class="meta-v">{{ lifecycleView.dbStatus || '—' }}</span>
              </div>
            </div>
            <n-text v-if="lifecycleView?.dataSourceNote" depth="3" style="display: block; margin-top: 6px; font-size: 12px">
              {{ lifecycleView.dataSourceNote }}
            </n-text>
          </div>

          <div v-if="isDraft" class="draft-status-hint">
            <n-text strong>草稿</n-text>
            <n-text depth="3">· {{ isApproved ? '已批准（仍为草稿）' : '未批准' }}</n-text>
            <n-text depth="3">· 不会自动买卖</n-text>
            <n-text depth="3" class="draft-status-detail">
              生成结果不会自动写入模拟观察；需人工批准并冻结后，才会进入模拟执行观察。这是 Beta 安全设计。
            </n-text>
          </div>

          <n-space align="center" :wrap="true">
            <n-text>审批状态：</n-text>
            <n-tag
              size="small"
              :type="isApproved ? 'success' : 'warning'"
              :bordered="false"
            >
              {{ approvalLabel }}
            </n-tag>
            <n-text depth="3">|</n-text>
            <n-text>冻结状态：</n-text>
            <n-tag
              size="small"
              :type="isFrozen ? 'success' : 'default'"
              :bordered="false"
            >
              {{ freezeLabel }}
            </n-tag>
            <template v-if="plan.freeze?.approved_at">
              <n-text depth="3">|</n-text>
              <n-text depth="3">ApprovedAt：{{ plan.freeze.approved_at }}</n-text>
            </template>
            <template v-if="plan.freeze?.freeze_at">
              <n-text depth="3">|</n-text>
              <n-text>FreezeAt：{{ plan.freeze.freeze_at }}</n-text>
            </template>
          </n-space>

          <n-text v-if="!isTSellFlow && isDraft && isMorningMaterialized && stickyCtaKind === 'approve'" depth="3" class="step-done-hint">
            ② 价格已准备 · 请使用上方按钮批准计划
          </n-text>

          <div v-if="!isTSellFlow && materializeDisplay" class="materialize-result-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 4px">
              <n-text strong>准备价格结果</n-text>
              <n-tag
                size="small"
                :type="materializeResultTagType"
                :bordered="false"
              >
                {{ materializeDisplay.title }}
              </n-tag>
            </n-space>
            <ul class="materialize-result-list">
              <li v-for="(line, i) in materializeDisplay.detailLines" :key="'m-' + i">
                {{ line }}
              </li>
            </ul>
            <template v-if="materializeResult?.success">
              <n-text depth="3" class="section-note">
                materialized_items={{ materializeResult.materialized_items }}
                · readiness_ready={{ materializeResult.readiness_ready ? 'true' : 'false' }}
                · blockers={{ (materializeResult.blockers || []).length }}
              </n-text>
            </template>
          </div>

          <div class="risk-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 4px">
              <n-text strong>Risk结果：</n-text>
              <n-tag
                size="small"
                :type="plan.risk?.passed ? 'success' : 'warning'"
                :bordered="false"
              >
                {{ plan.risk?.passed ? '通过' : '未通过' }}
              </n-tag>
            </n-space>
            <n-text depth="3" class="section-note">
              Risk通过表示风险规则检查通过，不代表计划一定盈利。
            </n-text>
            <div v-if="(plan.risk?.reasons || []).length" class="risk-reasons">
              <n-text depth="3">明细：</n-text>
              <ul>
                <li v-for="(r, i) in plan.risk.reasons" :key="i">{{ r }}</li>
              </ul>
            </div>
          </div>

          <div class="readiness-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 6px">
              <n-tooltip trigger="hover">
                <template #trigger>
                  <n-text strong style="cursor: help">执行准备状态</n-text>
                </template>
                内部对应 Intent Readiness：检查限价/数量是否齐备，不是真实下单审批。
              </n-tooltip>
              <n-tag size="small" type="info" :bordered="false">门禁观测</n-tag>
              <n-tag v-if="readinessLoading" size="small" :bordered="false">评估中</n-tag>
            </n-space>

            <n-spin :show="readinessLoading" size="small">
              <template v-if="hasReadiness">
                <n-space align="center" :wrap="true">
                  <n-text>Status：</n-text>
                  <n-tag
                    size="small"
                    :type="readinessReady ? 'success' : 'error'"
                    :bordered="false"
                  >
                    {{ readinessReady ? 'Ready' : 'Blocked' }}
                  </n-tag>
                  <n-text depth="3">|</n-text>
                  <n-text>Stage：{{ readinessData.lifecycle_stage || '—' }}</n-text>
                </n-space>

                <div v-if="!readinessReady" class="why-blocked">
                  <n-text strong>为什么 Blocked</n-text>
                  <div class="why-line">阻断数量：{{ blockerCount }}</div>
                  <div class="why-line">影响：当前计划不可Approve / Freeze</div>
                  <n-text depth="3" class="section-note">
                    该计划可以用于策略观察，但尚未满足执行准备条件。
                  </n-text>
                </div>
                <n-text v-else depth="3" class="section-note" style="display: block; margin-top: 6px">
                  Readiness Ready：无阻断项；WARN 不阻断 Ready。
                </n-text>

                <div v-if="blockerCount" class="readiness-list">
                  <n-text depth="3">Blockers（{{ blockerCount }}）：</n-text>
                  <ul>
                    <li v-for="(b, i) in readinessData.blockers" :key="'b-' + i">
                      <div class="finding-card">
                        <div class="finding-row">
                          <span class="finding-k">问题：</span>
                          <span class="finding-v">{{ describeReadinessFinding(b).title }}</span>
                        </div>
                        <div class="finding-row finding-code">
                          <span class="finding-k">代码：</span>
                          <span class="finding-v">[{{ b.rule_code || '—' }}] {{ b.code || '—' }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">影响：</span>
                          <span class="finding-v">{{ describeReadinessFinding(b).impact }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">原因：</span>
                          <span class="finding-v">{{ b.message || describeReadinessFinding(b).reason }}</span>
                        </div>
                      </div>
                    </li>
                  </ul>
                </div>
                <n-text v-else depth="3" style="display: block; margin-top: 4px">Blockers：无</n-text>

                <div v-if="warningCount" class="readiness-list">
                  <n-text depth="3">Warnings（{{ warningCount }}，不影响 Ready）：</n-text>
                  <ul>
                    <li v-for="(w, i) in readinessData.warnings" :key="'w-' + i">
                      <div class="finding-card">
                        <div class="finding-row">
                          <span class="finding-k">问题：</span>
                          <span class="finding-v">{{ describeReadinessFinding(w).title }}</span>
                        </div>
                        <div class="finding-row finding-code">
                          <span class="finding-k">代码：</span>
                          <span class="finding-v">[{{ w.rule_code || '—' }}] {{ w.code || '—' }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">影响：</span>
                          <span class="finding-v">{{ describeReadinessFinding(w).impact }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">原因：</span>
                          <span class="finding-v">{{ w.message || describeReadinessFinding(w).reason }}</span>
                        </div>
                      </div>
                    </li>
                  </ul>
                </div>
                <n-text v-else depth="3" style="display: block; margin-top: 4px">Warnings：无</n-text>

                <n-text depth="3" style="display: block; margin-top: 6px">
                  WARN 不阻断 Ready；Approve / Freeze 需两步确认；本页不提供下单。
                </n-text>
              </template>
              <n-empty
                v-else
                size="small"
                :description="readinessError || '暂无 Readiness'"
              />
            </n-spin>
          </div>

          <div class="readiness-block execution-readiness-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 6px">
              <n-text strong>执行准备观察</n-text>
              <n-tag size="small" type="info" :bordered="false">执行前观察 · 非审批</n-tag>
              <n-tag v-if="executionReadinessLoading" size="small" :bordered="false">评估中</n-tag>
            </n-space>
            <n-text depth="3" class="section-note" style="display: block; margin-bottom: 8px">
              用于评估当前模拟账户是否支持计划执行；不会自动阻止交易计划，也不替代「执行准备状态」检查。
            </n-text>

            <n-spin :show="executionReadinessLoading" size="small">
              <template v-if="hasExecutionReadiness">
                <n-space vertical :size="8">
                  <n-text strong depth="2">资金</n-text>
                  <n-space align="center" :wrap="true">
                    <n-text depth="3">执行前现金：</n-text>
                    <n-text>{{ formatMoney(executionReadinessData.availableCash) }}</n-text>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">计划占用：</n-text>
                    <n-text>{{ formatMoney(executionReadinessData.requiredCash) }}</n-text>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">执行后预计现金：</n-text>
                    <n-text>{{ formatMoney(executionCashAfterPlan) }}</n-text>
                  </n-space>
                  <n-space align="center" :wrap="true">
                    <n-text depth="3">总资产：</n-text>
                    <n-text>{{ formatMoney(executionReadinessData.totalEquity) }}</n-text>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">资金状态：</n-text>
                    <n-tag
                      size="small"
                      :type="executionReadinessData.cashEnough ? 'success' : 'error'"
                      :bordered="false"
                    >
                      {{ executionReadinessData.cashEnough ? '充足' : '不足' }}
                    </n-tag>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">综合：</n-text>
                    <n-tag size="small" :type="executionStatusTagType" :bordered="false">
                      {{ formatStatus(executionReadinessData.status, 'readiness').label }}
                    </n-tag>
                  </n-space>

                  <n-text strong depth="2">仓位</n-text>
                  <n-space align="center" :wrap="true">
                    <n-text depth="3">当前股票仓位：</n-text>
                    <n-text>
                      {{
                        executionCurrentStockPct != null
                          ? `${executionCurrentStockPct.toFixed(1)}%`
                          : '—'
                      }}
                    </n-text>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">执行后预计：</n-text>
                    <n-text>
                      {{
                        executionAfterStockPct != null
                          ? `${executionAfterStockPct.toFixed(1)}%`
                          : '—'
                      }}
                    </n-text>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">风险上限(maxGross)：</n-text>
                    <n-text>{{ DEFAULT_MAX_GROSS_EXPOSURE_PCT }}%</n-text>
                    <n-text depth="3">|</n-text>
                    <n-text depth="3">集中度：</n-text>
                    <n-text>{{ concentrationLabel }}</n-text>
                  </n-space>

                  <n-space align="center" :wrap="true">
                    <n-text depth="3">持仓冲突：</n-text>
                    <n-text v-if="!executionConflictCount">无</n-text>
                    <n-tag v-else size="small" type="warning" :bordered="false">
                      ⚠ 当前已有持仓（{{ executionConflictCount }}）
                    </n-tag>
                  </n-space>
                  <ul v-if="executionConflictCount" class="readiness-list">
                    <li v-for="(c, i) in executionReadinessData.conflicts" :key="'ec-' + i">
                      <n-text>{{ adaptTradePlanCamel(c).displayText }} · 持仓 {{ c.quantity }} · {{ c.message }}</n-text>
                    </li>
                  </ul>
                  <div v-if="(executionReadinessData.afterExecution || []).length" class="readiness-list">
                    <n-text depth="3">执行后单项权重（模拟）：</n-text>
                    <ul>
                      <li
                        v-for="(w, i) in executionReadinessData.afterExecution"
                        :key="'ew-' + i"
                      >
                        {{ adaptTradePlanCamel(w).displayText }}：{{ w.weightPct.toFixed(1) }}%
                      </li>
                    </ul>
                  </div>
                </n-space>
                <n-text depth="3" style="display: block; margin-top: 8px">
                  只读观察；不自动阻止执行、不下单、不改策略。
                </n-text>
              </template>
              <n-empty
                v-else
                size="small"
                :description="executionReadinessError || '暂无执行准备观察'"
              />
            </n-spin>
          </div>

          <n-text v-if="!isFrozen" depth="3" style="display: block">
            当前计划未锁定；批准后仍为草稿，锁定后才会变为「已准备」供本地模拟执行读取。不会真实下单。
          </n-text>
        </n-space>

        <ProductCapabilityPanel
          title="策略解释"
          feature="AdvancedObservation"
          scene="strategy_explanation"
          usage-opened-key="strategy_explanation_opened"
          usage-viewed-key="strategy_explanation_viewed"
          @open="onExplainPanelOpen"
        >
          <n-spin :show="explainLoading">
            <n-tag v-if="explainError" type="warning" :bordered="false" style="margin-bottom: 8px">
              {{ explainError }}
            </n-tag>
            <TradePlanExplanationSummaryTable
              :items="explainCacheItems"
              @open-detail="openExplainDetail"
              @open-stock="openStockKline"
            />
          </n-spin>
        </ProductCapabilityPanel>

        <TradePlanExplanationDrawer
          v-model:show="explainDrawerShow"
          :entry="explainDrawerEntry"
        />

        <n-space align="center" :wrap="true" style="margin-bottom: 6px">
          <n-text strong>执行候选</n-text>
          <n-tag size="small" type="success" :bordered="false">拟买入清单 · 只读分桶</n-tag>
        </n-space>
        <n-data-table
          v-if="executionCandidates.length"
          size="small"
          :columns="itemColumns"
          :data="executionCandidates"
          :row-key="(row) => `${row.stock_code}-${row.priority}`"
        />
        <n-empty v-else description="无执行候选（pending / 可执行）" />

        <div class="blocked-block">
          <n-space align="center" :wrap="true" style="margin-bottom: 6px">
            <n-text strong>阻断</n-text>
            <n-tag size="small" type="warning" :bordered="false">
              阻断 {{ presentationSummary.blockedCount ?? 0 }} 条
            </n-tag>
          </n-space>
          <n-text depth="3" class="section-note" style="display: block; margin-bottom: 8px">
            skipped / 风控拒绝 / 跳空跳过等扫描结果。不是拟买入清单；金额仅为扫描参考。
          </n-text>
          <n-data-table
            v-if="blockedCandidates.length"
            size="small"
            :columns="blockedColumns"
            :data="blockedCandidates"
            :row-key="(row) => `blocked-${row.stock_code}-${row.priority}`"
          />
          <n-empty v-else description="无阻断项" size="small" />
        </div>

        <div class="research-block">
          <n-space align="center" :wrap="true" style="margin-bottom: 6px">
            <n-text strong>研究池</n-text>
            <n-tag size="small" type="default" :bordered="false">
              研究池 {{ presentationSummary.researchCount ?? 0 }} 条
            </n-tag>
          </n-space>
          <n-data-table
            v-if="researchCandidates.length"
            size="small"
            :columns="itemColumns"
            :data="researchCandidates"
            :row-key="(row) => `research-${row.stock_code}-${row.rank || row.priority || 0}`"
          />
          <n-empty
            v-else
            size="small"
            :description="
              presentationSummary.researchEmptyReason === 'pool_not_loaded'
                ? '研究池尚未加载 CandidatePool（P0 空态；不影响执行候选）'
                : '研究池为空'
            "
          />
        </div>

        <div class="exec-preview-block">
          <n-space align="center" :wrap="true" style="margin-bottom: 6px">
            <n-text strong>执行预览</n-text>
            <n-tag size="small" type="info" :bordered="false">Execution Preview · 仅执行候选</n-tag>
          </n-space>
          <n-text depth="3" class="section-note" style="display: block; margin-bottom: 8px">
            执行候选的数量、订单规则与准备/执行状态。策略参考价与限价（参考）已上移至上方决策主表；本区不触发下单。盘后
            草稿阶段限价与数量为 0 属正常，显示为「待早盘准备」，不是异常。
          </n-text>
          <n-data-table
            v-if="executionCandidates.length"
            size="small"
            :columns="previewColumns"
            :data="executionCandidates"
            :row-key="(row) => `preview-${row.stock_code}-${row.priority}`"
          />
          <n-empty v-else description="无执行预览明细" size="small" />
        </div>

        <div class="review-block">
          <n-text strong>交易后复盘</n-text>
          <n-text depth="3" class="section-note" style="display: block; margin: 4px 0 6px">
            用于后续评估策略有效性。本次仅展示待记录项，不自动统计。
          </n-text>
          <ul class="review-list">
            <li>待记录：开盘价</li>
            <li>待记录：最高价</li>
            <li>待记录：最低价</li>
            <li>待记录：收盘价</li>
            <li>待记录：次日收益</li>
          </ul>
        </div>
      </template>

      <n-empty
        v-else
        :description="emptyMessage || (requestTradeDate ? `无 upcoming（${requestTradeDate}）` : '暂无即将交易的计划')"
      />
    </n-spin>

    <stock-kline-modal
      v-model:show="klineModal.visible"
      :title="klineModal.title"
      :chart-key="'tradeplan-kline-' + klineModal.chartCode"
      :code="klineModal.chartCode"
      :stock-name="klineModal.stockName"
    />
  </div>
</template>

<style scoped>
.trade-plan-upcoming {
  height: 100%;
  overflow: auto;
  padding: 4px 2px 12px;
  box-sizing: border-box;
}
.plan-slot-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 8px 0 12px;
}
.plan-slot-row--single {
  grid-template-columns: minmax(0, 1fr);
  max-width: 420px;
}
.plan-slot-same {
  margin-top: 6px;
  font-size: 12px;
  color: rgba(32, 128, 240, 0.95);
  line-height: 1.35;
}
.plan-slot {
  text-align: left;
  border: 1px solid rgba(128, 128, 128, 0.28);
  border-radius: 8px;
  background: rgba(128, 128, 128, 0.04);
  padding: 10px 12px;
  cursor: pointer;
  color: inherit;
  font: inherit;
}
.plan-slot.active {
  border-color: rgba(32, 128, 240, 0.55);
  background: rgba(32, 128, 240, 0.08);
  box-shadow: inset 0 0 0 1px rgba(32, 128, 240, 0.2);
}
.plan-slot:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}
.plan-slot-k {
  font-size: 12px;
  opacity: 0.7;
  margin-bottom: 2px;
}
.plan-slot-date {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 0.02em;
}
.plan-slot-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  font-size: 13px;
}
.plan-slot-hint {
  margin-top: 4px;
  font-size: 12px;
  opacity: 0.75;
}
.plan-slot-empty {
  margin-top: 4px;
  font-size: 12px;
  opacity: 0.65;
}
.meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 8px 16px;
  padding: 8px 0;
}
.same-day-selector {
  border: 1px solid rgba(128, 128, 128, 0.22);
  border-radius: 8px;
  padding: 10px 12px;
  background: rgba(128, 128, 128, 0.04);
  margin-bottom: 4px;
}
.same-day-selector-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.same-day-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.same-day-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  text-align: left;
  border: 1px solid rgba(128, 128, 128, 0.28);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.55);
  padding: 8px 10px;
  cursor: pointer;
  color: inherit;
  font: inherit;
  font-size: 12px;
  line-height: 1.35;
}
.same-day-chip:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.same-day-chip.active {
  border-color: rgba(32, 128, 240, 0.55);
  background: rgba(32, 128, 240, 0.1);
  box-shadow: inset 0 0 0 1px rgba(32, 128, 240, 0.18);
}
.same-day-chip-main {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lifecycle-panel {
  margin: 12px 0 16px;
  padding: 10px 12px;
  border: 1px solid rgba(32, 128, 240, 0.25);
  border-radius: 6px;
  background: rgba(32, 128, 240, 0.04);
}
.sell-exec-feedback-panel {
  margin: 0 0 12px;
  padding: 10px 12px;
  border: 1px solid rgba(208, 48, 80, 0.22);
  border-radius: 6px;
  background: rgba(208, 48, 80, 0.04);
}
.sell-cash-notice {
  display: block;
  margin-top: 4px;
  font-size: 12px;
}
.execution-window-panel {
  margin: 8px 0 12px;
  padding: 10px 12px;
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 6px;
  background: rgba(128, 128, 128, 0.04);
}
.missed-window-banner {
  margin-top: 8px;
  padding: 8px 10px;
  border-radius: 4px;
  border-left: 3px solid rgba(240, 160, 32, 0.75);
  background: rgba(240, 160, 32, 0.08);
}
.morning-readiness-panel {
  border-color: rgba(32, 160, 96, 0.28);
  background: rgba(32, 160, 96, 0.04);
}
.automation-panel {
  border-color: rgba(96, 64, 192, 0.28);
  background: rgba(96, 64, 192, 0.04);
}
.sell-plan-detail-card {
  border-color: rgba(240, 160, 32, 0.35);
  background: rgba(240, 160, 32, 0.05);
}
.sell-manual-window-panel {
  border-color: rgba(32, 128, 240, 0.28);
  background: rgba(32, 128, 240, 0.04);
}
.lifecycle-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 6px 14px;
}
.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.meta-k {
  font-size: 12px;
  opacity: 0.65;
}
.meta-v {
  font-size: 14px;
}
.meta-v-warn {
  color: #c27803;
  font-weight: 600;
}
.selection-source-banner {
  margin: 4px 0 8px;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid rgba(194, 120, 3, 0.45);
  background: rgba(240, 180, 41, 0.1);
}
.selection-source-ok {
  border-color: rgba(32, 128, 240, 0.35);
  background: rgba(32, 128, 240, 0.06);
}
.selection-source-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 4px;
}
.selection-source-list {
  margin: 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.55;
  opacity: 0.92;
}
.risk-reasons ul,
.readiness-list ul,
.review-list {
  margin: 4px 0 0;
  padding-left: 18px;
}
.readiness-list li {
  margin-bottom: 10px;
}
.finding-card {
  line-height: 1.55;
}
.finding-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.finding-k {
  opacity: 0.65;
  font-size: 12px;
  min-width: 36px;
}
.finding-v {
  font-size: 13px;
}
.finding-code .finding-v {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
.risk-block,
.readiness-block,
.review-block,
.blocked-block,
.research-block,
.materialize-result-block,
.exec-preview-block {
  padding: 8px 0 2px;
  border-top: 1px solid rgba(128, 128, 128, 0.2);
}
.presentation-summary {
  margin: 10px 0 12px;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.03);
}
.materialize-result-list {
  margin: 4px 0 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.55;
}
.why-blocked {
  margin-top: 8px;
  padding: 8px 10px;
  background: rgba(208, 48, 80, 0.06);
  border-radius: 4px;
}
.why-line {
  margin-top: 4px;
  font-size: 13px;
}
.section-note {
  display: block;
  font-size: 12px;
  margin-top: 2px;
}
.observation-note {
  display: block;
  margin-bottom: 12px;
  font-size: 12px;
}
.generating-hint {
  display: block;
  margin: -4px 0 10px;
  font-size: 13px;
  color: var(--n-primary-color, #2080f0);
}
.intent-preflight-alert {
  margin-bottom: 10px;
}
.intent-preflight-summary {
  display: block;
  margin-bottom: 10px;
  font-size: 12px;
}
.user-flow-bar {
  position: sticky;
  top: 0;
  z-index: 12;
  margin: 0 0 12px;
  padding: 12px 14px;
  border-radius: 6px;
  border: 1px solid rgba(32, 128, 240, 0.35);
  background: var(--n-color, #fff);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}
.user-flow-bar-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.user-flow-cta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 14px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid rgba(128, 128, 128, 0.18);
}
.user-flow-cta-row.is-materialize-focus {
  margin-top: 10px;
  padding: 10px 12px 10px;
  border-radius: 6px;
  background: rgba(32, 128, 240, 0.08);
  outline: 2px solid rgba(32, 128, 240, 0.45);
  outline-offset: 2px;
  animation: materialize-cta-pulse 1.2s ease-in-out 2;
}
@keyframes materialize-cta-pulse {
  0%,
  100% {
    outline-color: rgba(32, 128, 240, 0.45);
  }
  50% {
    outline-color: rgba(32, 128, 240, 0.12);
  }
}
.user-flow-cta-btn {
  min-width: 148px;
}
.user-flow-cta-hint {
  flex: 1 1 200px;
  font-size: 12px;
  line-height: 1.5;
}
.guidance-steps {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-bottom: 6px;
}
.guidance-step {
  font-size: 13px;
  opacity: 0.55;
  padding: 2px 0;
}
.guidance-step.is-done {
  opacity: 0.8;
}
.guidance-step.is-active {
  opacity: 1;
  font-weight: 600;
  color: var(--n-primary-color, #2080f0);
}
.guidance-note {
  display: block;
  font-size: 12px;
  line-height: 1.5;
  margin-top: 2px;
}
.step-done-hint {
  font-size: 13px;
  display: block;
  margin: 4px 0 8px;
}
.draft-status-hint {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 6px 10px;
  padding: 8px 10px;
  margin: 2px 0 4px;
  background: rgba(240, 160, 32, 0.08);
  border-radius: 4px;
  border-left: 3px solid rgba(240, 160, 32, 0.65);
  font-size: 13px;
}
.draft-status-detail {
  flex-basis: 100%;
  font-size: 12px;
  margin-top: 2px;
}
.col-tip {
  border-bottom: 1px dashed rgba(128, 128, 128, 0.55);
  cursor: help;
}
.review-list li {
  margin-bottom: 2px;
  font-size: 13px;
}
</style>
