<script setup>
/**
 * Phase15-B-A 首页展示重构（只读）。
 * Phase17-A.1：我的计划双槽（今日执行 / 下一交易日准备）— 仅展示；不改 upcoming 语义。
 * Phase17-A.2：资产区展示「账户今日盈亏」语义（tooltip）；不改计算。
 * Phase17.1：流程横向紧凑；今日机会按 stock_code 聚合；盈亏色用 marketColor（A 股红涨绿跌）。
 * 主数据：GET /api/investment/home（资产来自后端 Snapshot 投影，前端不重算）。
 * 计划：GET /api/tradeplans/upcoming + same-day-candidates（只读）。
 * Track-B 开关：GET /api/papertrading/dashboard/today.enabled（只读，失败则按未开启）。
 * 旧运维区块保留在模板中，SHOW_LEGACY_HOME_BLOCKS=false 隐藏，不删数据映射。
 */
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NDataTable,
  NEmpty,
  NList,
  NListItem,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import {
  attentionLabel,
  autoExecuteSentence,
  fetchTrackBPaperTradingEnabled,
  getInvestmentHome,
  itemTypeLabel,
  planUserStatus,
  sanitizeInternalCopy,
  tradingStatusLabel,
  tradingStepLabel,
} from '../api/investmentHome'
import { getSameDayCandidates, getUpcomingTradePlan } from '../api/tradePlans'
import { positionStateTagType } from '../utils/positionStateDisplay.js'
import { applyStockClickAction } from '../utils/stockDisplay.js'
import { buildDashboardPlanContext, shanghaiDate } from '../utils/planContext.js'
import { buildHomePlanSlotModel } from '../utils/homePlanDisplay.js'
import { formatPriceWithContext, priceColumnTitle, PRICE_KIND } from '../utils/priceDisplay.js'
import { pnlColor } from '../utils/marketColor.js'
import { formatFieldTooltip } from '../utils/statusDisplay.js'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'
import OpportunityProjectionDrawer from './OpportunityProjectionDrawer.vue'

/** Hidden, not deleted. Flip to true only for developer inspection. */
const SHOW_LEGACY_HOME_BLOCKS = false

/** Phase16.19-B3: compact home opportunity table Top N (no pagination). */
const HOME_OPPORTUNITY_TOP_N = 10

const router = useRouter()
const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const view = ref(null)
const planContext = ref(null)
/** @type {import('vue').Ref<Record<string, any[]>>} */
const sameDayByDate = ref({})
const enablePaperTrading = ref(false)

const portfolio = computed(() => view.value?.portfolio || null)
const decision = computed(() => view.value?.decision || null)
const daily = computed(() => view.value?.daily || null)
const trading = computed(() => view.value?.trading || null)
const dailyAttention = computed(() => view.value?.dailyAttention || null)
const attentionRows = computed(() => dailyAttention.value?.items || [])
const positionStates = computed(() => view.value?.positionStates || [])

const opportunityCards = computed(() =>
  Array.isArray(view.value?.opportunityCards) ? view.value.opportunityCards : [],
)

/**
 * Phase17.1 display-only：按 candidate stock_code 聚合（一候选×多 highlight 不再重复行）。
 * 不改 opportunity_attention / CandidatePool。
 */
const opportunityTableRows = computed(() => {
  const cards = opportunityCards.value
  /** @type {Map<string, any>} */
  const byCode = new Map()
  for (const card of cards) {
    const cand = card?.candidate_stock
    if (!cand?.display && !cand?.code) continue
    const codeKey = String(cand.code || '')
      .trim()
      .toLowerCase()
    if (!codeKey) continue
    let row = byCode.get(codeKey)
    if (!row) {
      row = {
        key: codeKey,
        code: cand.code || '',
        name: cand.name || '',
        display: cand.display || null,
        signalLabel: card.user_label || '—',
        signalCount: 0,
        score: card.candidate_score,
        holdingScore: card.holding_score,
        scoreGap: card.score_gap,
        strategyStatus: card.user_label || '机会对比',
      }
      byCode.set(codeKey, row)
    }
    row.signalCount += 1
    if (card.user_label === '建议研究') {
      row.signalLabel = '建议研究'
      row.strategyStatus = '建议研究'
    }
    const gap = Number(card.score_gap)
    const prev = Number(row.scoreGap)
    if (Number.isFinite(gap) && (!Number.isFinite(prev) || gap > prev)) {
      row.scoreGap = gap
      row.holdingScore = card.holding_score
    }
  }
  return Array.from(byCode.values()).slice(0, HOME_OPPORTUNITY_TOP_N)
})

function formatScore(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return String(Math.round(n))
}

function formatScoreGap(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  const r = Math.round(n)
  return r > 0 ? `+${r}` : String(r)
}

/** Guidance / 「当前状态」仍优先今日槽，否则下一交易日（不改 Beta 引导语义）。 */
const guidancePlan = computed(() => {
  const ctx = planContext.value
  return ctx?.current_plan?.plan || ctx?.next_plan?.plan || null
})
const planCard = computed(() => planUserStatus(guidancePlan.value))

const todayPlanSlot = computed(() => {
  const ctx = planContext.value
  const slot = ctx?.current_plan || null
  const td = String(slot?.trade_date || '').trim()
  const cands = td ? sameDayByDate.value[td] || [] : []
  return buildHomePlanSlotModel('today', slot, cands)
})

const nextPlanSlot = computed(() => {
  const ctx = planContext.value
  const slot = ctx?.next_plan || null
  const td = String(slot?.trade_date || '').trim()
  const cands = td ? sameDayByDate.value[td] || [] : []
  return buildHomePlanSlotModel('next', slot, cands)
})

const homePlanSlots = computed(() => [todayPlanSlot.value, nextPlanSlot.value])

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

function openStockKline(model) {
  applyStockClickAction(model, klineModal)
}

const projectionDrawerVisible = ref(false)
const projectionDrawerRow = ref(null)

function openOpportunityExplain(row) {
  const code = String(row?.code || '').trim()
  if (!code) {
    message.warning('无法解析股票代码')
    return
  }
  projectionDrawerRow.value = {
    stockCode: code,
    stockName: row?.name || '',
    tradeDate: String(view.value?.tradeDate || '').trim(),
  }
  projectionDrawerVisible.value = true
}

function opportunityTagType(label) {
  if (label === '建议研究') return 'warning'
  if (label === '等待确认') return 'info'
  return 'default'
}

const opportunityColumns = [
  {
    title: '股票',
    key: 'stock',
    width: 150,
    ellipsis: { tooltip: true },
    render(row) {
      if (row.display?.klineKey || row.display?.displayText) {
        return h(StockLink, {
          model: row.display,
          onOpen: openStockKline,
        })
      }
      return h(NText, { depth: 3 }, { default: () => row.name || row.code || '—' })
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '主要信号'),
          default: () => formatFieldTooltip('signal'),
        },
      ),
    key: 'signal',
    width: 96,
    render(row) {
      const label = row.signalLabel || '—'
      if (label === '—') return h(NText, { depth: 3 }, { default: () => '—' })
      return h(
        NTag,
        { size: 'tiny', type: opportunityTagType(label), bordered: false },
        { default: () => label },
      )
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '信号数量'),
          default: () => '相对持仓对比条数（同一候选展开聚合后的条数，非多只股票）',
        },
      ),
    key: 'signalCount',
    width: 88,
    render(row) {
      const n = Number(row.signalCount)
      return Number.isFinite(n) && n > 0 ? String(n) : '—'
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '评分'),
          default: () => formatFieldTooltip('score'),
        },
      ),
    key: 'score',
    width: 64,
    render(row) {
      return formatScore(row.score)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '最大分差'),
          default: () =>
            `${formatFieldTooltip('risk')}（聚合后取相对持仓最大分差；无独立风控码）`,
        },
      ),
    key: 'risk',
    width: 80,
    render(row) {
      const gap = formatScoreGap(row.scoreGap)
      if (gap === '—') return h(NText, { depth: 3 }, { default: () => '—' })
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h(NText, null, { default: () => gap }),
          default: () =>
            `相对持仓最大优势 ${gap}（持仓分 ${formatScore(row.holdingScore)}）`,
        },
      )
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, priceColumnTitle(PRICE_KIND.last)),
          default: () => formatFieldTooltip('last'),
        },
      ),
    key: 'quote',
    width: 100,
    render() {
      // Home payload has no live/ref price — show — with priceDisplay semantics (do not invent).
      const ctx = formatPriceWithContext(null, PRICE_KIND.last, {
        extraTooltip: '首页聚合接口未提供行情价；请到机会页或组合页查看',
      })
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h(NText, { depth: 3 }, { default: () => ctx.value }),
          default: () => ctx.tooltip,
        },
      )
    },
  },
  {
    title: '策略状态',
    key: 'strategyStatus',
    width: 100,
    ellipsis: { tooltip: true },
    render(row) {
      const status = row.strategyStatus || '—'
      const holdingHint = row.holdingCode
        ? `对比持仓 ${row.holdingCode}`
        : '机会对比'
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h(NText, null, { default: () => status }),
          default: () => holdingHint,
        },
      )
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    render(row) {
      return h(
        NSpace,
        { size: 4, wrap: false },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'tiny',
                tertiary: true,
                disabled: !(row.display?.klineKey),
                onClick: () => openStockKline(row.display),
              },
              { default: () => 'K线' },
            ),
            h(
              NButton,
              {
                size: 'tiny',
                tertiary: true,
                type: 'info',
                disabled: !row.code,
                onClick: () => openOpportunityExplain(row),
              },
              { default: () => '解释' },
            ),
          ],
        },
      )
    },
  },
]

const isTodayPlan = computed(() => {
  const ctx = planContext.value
  const planDate = String(ctx?.current_plan?.plan?.trade_date || '').trim()
  const today = String(ctx?.current_plan?.trade_date || ctx?.today || '').trim()
  return !!planDate && !!today && planDate === today
})
const currentStatusText = computed(() =>
  autoExecuteSentence({
    enablePaperTrading: !!enablePaperTrading.value,
    hasPlan: !!guidancePlan.value,
    isFrozen: planCard.value.key === 'locked',
    isTodayPlan: !!isTodayPlan.value,
  }),
)

const pipelineSteps = computed(() => {
  const t = trading.value
  if (!t) return []
  return [
    { key: 'materialize', status: t.materializeStatus },
    { key: 'approve', status: t.approveStatus },
    { key: 'freeze', status: t.freezeStatus },
    { key: 'execution', status: t.executionStatus },
    { key: 'settlement', status: t.settlementStatus },
  ]
})

function formatMoney(v) {
  if (v === null || v === undefined) return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function attentionTagType(level) {
  const s = String(level || '').toUpperCase()
  if (s === 'REVIEW') return 'error'
  if (s === 'WATCH') return 'warning'
  if (s === 'HOLD') return 'success'
  return 'default'
}

function statusTagType(status) {
  const s = String(status || '').toUpperCase()
  if (s === 'PASS') return 'success'
  if (s === 'FAIL') return 'error'
  if (s === 'SKIP') return 'warning'
  if (s === 'PENDING') return 'info'
  return 'default'
}

function planTagType(key) {
  if (key === 'locked') return 'success'
  if (key === 'prepared') return 'info'
  if (key === 'pending_confirm') return 'warning'
  return 'default'
}

const BETA_TRADING_FLOW = [
  {
    key: 'discover',
    label: '发现机会',
    hint: '选股 / 机会列表',
    route: 'stockScreen',
  },
  {
    key: 'generate',
    label: '生成计划',
    hint: '生成下一交易日草稿',
    route: 'tradePlanUpcoming',
  },
  {
    key: 'materialize',
    label: '准备价格',
    hint: '写入限价与数量',
    route: 'tradePlanUpcoming',
  },
  {
    key: 'approve_freeze',
    label: '批准冻结',
    hint: '人工批准并锁定',
    route: 'tradePlanUpcoming',
  },
  {
    key: 'execute',
    label: '模拟成交',
    hint: '在组合查看结果',
    route: 'portfolioDashboard',
  },
]

/** UI-only: which step the user should focus on next (does not trigger trading). */
const currentFlowStepKey = computed(() => {
  const cardKey = planCard.value.key
  if (!guidancePlan.value || cardKey === 'none') return 'generate'
  if (cardKey === 'pending_confirm') return 'materialize'
  if (cardKey === 'prepared') return 'approve_freeze'
  if (cardKey === 'locked') return 'execute'
  return 'generate'
})

const currentFlowStepLabel = computed(() => {
  const hit = BETA_TRADING_FLOW.find((s) => s.key === currentFlowStepKey.value)
  return hit?.label || '生成计划'
})

const betaTradingFlowSteps = computed(() => {
  const order = BETA_TRADING_FLOW.map((s) => s.key)
  const currentIdx = Math.max(0, order.indexOf(currentFlowStepKey.value))
  return BETA_TRADING_FLOW.map((step, idx) => {
    let state = 'upcoming'
    if (idx < currentIdx) state = 'done'
    else if (idx === currentIdx) state = 'current'
    return { ...step, state }
  })
})

function goFlowStep(step) {
  if (!step?.route) return
  if (step.key === 'materialize') {
    router.push({ name: step.route, query: { focus: 'materialize' } })
    return
  }
  router.push({ name: step.route })
}

function goDiscover() {
  goFlowStep(BETA_TRADING_FLOW[0])
}

function goPortfolio() {
  goFlowStep(BETA_TRADING_FLOW[4])
}

function goPlan(opts = {}) {
  const query = {}
  const tradeDate = String(opts.tradeDate || '').trim()
  const planId = Math.trunc(Number(opts.planId) || 0)
  if (tradeDate) query.trade_date = tradeDate
  if (planId > 0) query.plan_id = String(planId)
  router.push({
    name: 'tradePlanUpcoming',
    query: Object.keys(query).length ? query : undefined,
  })
}

function goPlanRow(row) {
  goPlan({ tradeDate: row?.tradeDate, planId: row?.planId })
}

function goPlanSlot(slot) {
  const row = slot?.primary || slot?.rows?.[0]
  if (row) {
    goPlanRow(row)
    return
  }
  goPlan({ tradeDate: slot?.tradeDate })
}

async function loadPaperTradingSwitch() {
  enablePaperTrading.value = await fetchTrackBPaperTradingEnabled()
}

async function loadSameDayForDates(dates) {
  const uniq = [...new Set((dates || []).map((d) => String(d || '').trim()).filter(Boolean))]
  const next = { ...sameDayByDate.value }
  await Promise.all(
    uniq.map(async (td) => {
      try {
        const res = await getSameDayCandidates(td)
        next[td] = Array.isArray(res.candidates) ? res.candidates.filter((c) => c.id > 0) : []
      } catch (_) {
        next[td] = []
      }
    }),
  )
  sameDayByDate.value = next
}

async function loadPlanContext() {
  try {
    const now = new Date()
    const resCurrent = await getUpcomingTradePlan(shanghaiDate(now))
    const nextDay = buildDashboardPlanContext({
      now,
      resCurrent,
      resNext: { ok: false, plan: null },
    }).next_plan.trade_date
    const resNext = nextDay
      ? await getUpcomingTradePlan(nextDay)
      : { ok: false, plan: null }
    planContext.value = buildDashboardPlanContext({ now, resCurrent, resNext })
    const ctx = planContext.value
    await loadSameDayForDates([
      ctx?.current_plan?.trade_date,
      ctx?.next_plan?.trade_date,
      ctx?.current_plan?.plan?.trade_date,
      ctx?.next_plan?.plan?.trade_date,
    ])
  } catch {
    planContext.value = buildDashboardPlanContext({ now: new Date() })
    sameDayByDate.value = {}
  }
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  try {
    const homePromise = getInvestmentHome()
    const switchPromise = loadPaperTradingSwitch()
    view.value = await homePromise
    await Promise.all([switchPromise, loadPlanContext()])
  } catch (e) {
    view.value = null
    planContext.value = null
    sameDayByDate.value = {}
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="investment-home">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center" :wrap="true">
        <n-text strong style="font-size: 18px">首页</n-text>
        <n-tag size="small" type="info" :bordered="false">模拟账户 · 只读</n-tag>
        <n-text v-if="view?.tradeDate" depth="3">{{ view.tradeDate }}</n-text>
      </n-space>
      <n-button :loading="loading" @click="refresh">刷新</n-button>
    </n-space>

    <n-alert type="info" :bordered="false" style="margin-bottom: 14px">
      模拟账户，不是券商资金。非投资建议，不生成买卖指令。
    </n-alert>

    <section class="block beta-flow-card" data-phase="PHASE14A-R0-D">
      <n-space justify="space-between" align="center" :wrap="true" style="margin-bottom: 10px">
        <n-text strong class="block-title" style="margin-bottom: 0">今日交易流程</n-text>
        <n-tag size="small" type="info" :bordered="false">Beta 引导</n-tag>
      </n-space>
      <n-text depth="3" class="block-sub">
        点击进入对应页面。下一步：<n-text strong>{{ currentFlowStepLabel }}</n-text>
      </n-text>
      <div class="beta-flow-steps" role="list" aria-label="今日交易流程">
        <template v-for="(step, idx) in betaTradingFlowSteps" :key="step.key">
          <button
            type="button"
            class="beta-flow-step"
            :class="[`is-${step.state}`]"
            role="listitem"
            :title="step.hint"
            @click="goFlowStep(step)"
          >
            <span class="beta-flow-step-index">{{ idx + 1 }}</span>
            <span class="beta-flow-step-label">{{ step.label }}</span>
            <span v-if="step.state === 'current'" class="beta-flow-step-badge is-current">下一步</span>
            <span v-else-if="step.state === 'done'" class="beta-flow-step-badge is-done">完成</span>
          </button>
          <div
            v-if="idx < betaTradingFlowSteps.length - 1"
            class="beta-flow-arrow"
            aria-hidden="true"
          >
            →
          </div>
        </template>
      </div>
    </section>

    <n-space :wrap="true" style="margin-bottom: 16px">
      <n-button @click="goDiscover">发现机会</n-button>
      <n-button @click="goPlan">查看计划</n-button>
      <n-button type="primary" @click="goPortfolio">查看组合</n-button>
    </n-space>

    <n-spin :show="loading">
      <template v-if="errorMessage && !view">
        <n-empty :description="errorMessage" />
      </template>

      <template v-else-if="view">
        <section class="block">
          <n-text strong class="block-title">我的资产</n-text>
          <n-text depth="3" class="block-sub">
            模拟账户摘要。账户今日盈亏为结算权益差，不是盘中持仓浮盈。明细请到我的组合。
          </n-text>
          <n-space :wrap="true" :size="28" style="margin-top: 10px">
            <n-statistic label="总资产">
              <template #default>{{ formatMoney(portfolio?.equity) }}</template>
            </n-statistic>
            <n-statistic label="现金">
              <template #default>{{ formatMoney(portfolio?.cash) }}</template>
            </n-statistic>
            <n-statistic label="股票市值">
              <template #default>{{ formatMoney(portfolio?.marketValue) }}</template>
            </n-statistic>
            <n-statistic>
              <template #label>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span style="cursor: help; border-bottom: 1px dashed rgba(128, 128, 128, 0.45)">
                      账户今日盈亏
                    </span>
                  </template>
                  相对上一交易日结算权益变化。不是盘中持仓浮盈合计。
                </n-tooltip>
              </template>
              <template #default>
                <span :style="{ color: pnlColor(portfolio?.dailyPnl) }">
                  {{
                    portfolio?.dailyPnl === null || portfolio?.dailyPnl === undefined
                      ? '—'
                      : formatMoney(portfolio.dailyPnl)
                  }}
                </span>
              </template>
            </n-statistic>
            <n-statistic label="持仓数量">
              <template #default>{{ portfolio?.positionCount ?? '—' }}</template>
            </n-statistic>
          </n-space>
          <n-button text type="primary" style="margin-top: 8px" @click="goPortfolio">
            查看组合
          </n-button>
        </section>

        <section class="block">
          <n-space justify="space-between" align="center" :wrap="true" style="margin-bottom: 6px">
            <n-text strong class="block-title" style="margin-bottom: 0">今日机会</n-text>
            <n-tag size="small" :bordered="false">Top {{ opportunityTableRows.length }}</n-tag>
          </n-space>
          <n-text depth="3" class="block-sub">
            按股票聚合（最多 {{ HOME_OPPORTUNITY_TOP_N }} 只）。信号数量为相对持仓对比条数；行情价请到机会页查看。
          </n-text>
          <n-data-table
            v-if="opportunityTableRows.length"
            size="small"
            :columns="opportunityColumns"
            :data="opportunityTableRows"
            :row-key="(row) => row.key"
            :pagination="false"
            :bordered="false"
            class="home-opp-table"
          ></n-data-table>
          <n-empty v-else description="今天没有单独机会提示" size="small">
            <template #extra>
              <n-text depth="3" style="font-size: 12px">
                无对比机会时属正常，可点下方发现更多
              </n-text>
            </template>
          </n-empty>
          <n-button text type="primary" style="margin-top: 8px" @click="goDiscover">
            发现更多机会
          </n-button>
        </section>

        <section class="block">
          <n-text strong class="block-title">我的计划</n-text>
          <n-text depth="3" class="block-sub">
            今日执行与下一交易日准备分开展示。来源为 Strategy / Watchlist / Manual 浅标签；详情进入交易计划页。
          </n-text>
          <div
            v-for="slot in homePlanSlots"
            :key="slot.kind"
            class="plan-slot"
          >
            <n-space justify="space-between" align="center" :wrap="true" style="margin-bottom: 6px">
              <n-text strong>{{ slot.title }}</n-text>
              <n-text v-if="slot.tradeDate" depth="3">{{ slot.tradeDate }}</n-text>
            </n-space>
            <template v-if="slot.hasPlan">
              <div
                v-for="row in slot.rows"
                :key="row.key"
                class="plan-row"
                :class="{ 'is-extra': !row.isPrimary }"
              >
                <n-space align="center" :wrap="true" :size="[8, 6]">
                  <n-tag size="small" :type="planTagType(row.statusKey)" :bordered="false">
                    {{ row.statusLabel }}
                  </n-tag>
                  <n-tag
                    size="small"
                    :type="row.sourceChip.type"
                    :bordered="false"
                  >
                    {{ row.sourceChip.label }}
                  </n-tag>
                  <n-text v-if="row.planId > 0" depth="3">#{{ row.planId }}</n-text>
                  <n-text v-if="row.itemCount != null" depth="3">共 {{ row.itemCount }} 只</n-text>
                  <n-text v-else-if="!row.isPrimary" depth="3">同日其他计划</n-text>
                  <n-button text type="primary" size="tiny" @click="goPlanRow(row)">
                    查看详情
                  </n-button>
                </n-space>
                <n-space
                  v-if="row.stocks?.length"
                  :wrap="true"
                  :size="[12, 8]"
                  style="margin-top: 6px"
                >
                  <stock-link
                    v-for="stock in row.stocks"
                    :key="stock.stock_code"
                    :model="stock"
                    @open="openStockKline"
                  />
                </n-space>
              </div>
            </template>
            <n-empty v-else :description="slot.emptyText" size="small" />
            <n-button text type="primary" style="margin-top: 6px" @click="goPlanSlot(slot)">
              {{ slot.kind === 'today' ? '查看今日计划' : '查看下一交易日' }}
            </n-button>
          </div>
        </section>

        <section class="block">
          <n-text strong class="block-title">当前状态</n-text>
          <n-text style="display: block; margin-top: 8px; line-height: 1.6">
            {{ currentStatusText }}
          </n-text>
          <n-text
            v-if="!enablePaperTrading"
            depth="3"
            style="display: block; margin-top: 6px; font-size: 12px; line-height: 1.5"
          >
            模拟账户模式下可浏览计划与研究，不会自动下单。这是 Beta 安全设计。
          </n-text>
        </section>

        <!-- Legacy ops blocks: hidden, not deleted. Data still loaded via getInvestmentHome. -->
        <template v-if="SHOW_LEGACY_HOME_BLOCKS">
          <section class="block">
            <n-text strong class="block-title">账户总览</n-text>
            <n-text v-if="portfolio?.narrative" depth="3" class="block-sub">
              {{ sanitizeInternalCopy(portfolio.narrative) }}
            </n-text>
            <n-space :wrap="true" :size="28" style="margin-top: 10px">
              <n-statistic label="总资产">
                <template #default>{{ formatMoney(portfolio?.equity) }}</template>
              </n-statistic>
              <n-statistic>
                <template #label>
                  <n-tooltip trigger="hover">
                    <template #trigger>
                      <span style="cursor: help; border-bottom: 1px dashed rgba(128, 128, 128, 0.45)">
                        账户今日盈亏
                      </span>
                    </template>
                    相对上一交易日结算权益变化。不是盘中持仓浮盈合计。
                  </n-tooltip>
                </template>
                <template #default>
                  <span :style="{ color: pnlColor(portfolio?.dailyPnl) }">
                    {{
                      portfolio?.dailyPnl === null || portfolio?.dailyPnl === undefined
                        ? '—'
                        : formatMoney(portfolio.dailyPnl)
                    }}
                  </span>
                </template>
              </n-statistic>
              <n-statistic label="持仓数量">
                <template #default>{{ portfolio?.positionCount ?? '—' }}</template>
              </n-statistic>
              <n-statistic label="股票市值">
                <template #default>{{ formatMoney(portfolio?.marketValue) }}</template>
              </n-statistic>
              <n-statistic label="现金">
                <template #default>{{ formatMoney(portfolio?.cash) }}</template>
              </n-statistic>
            </n-space>
          </section>

          <section class="block">
            <n-text strong class="block-title">今日关注建议</n-text>
            <template v-if="decision">
              <n-space align="center" :wrap="true" style="margin: 8px 0 6px">
                <n-tag
                  size="medium"
                  :type="attentionTagType(decision.overallAttention)"
                  :bordered="false"
                >
                  {{ attentionLabel(decision.overallAttention) }}
                </n-tag>
                <n-text depth="3">组合状态：{{ attentionLabel(decision.healthStatus) }}</n-text>
              </n-space>
              <n-text v-if="decision.explanation" style="display: block; line-height: 1.6">
                {{ sanitizeInternalCopy(decision.explanation) }}
              </n-text>
              <n-space v-if="decision.healthNotes?.length" :wrap="true" style="margin-top: 8px">
                <n-tag
                  v-for="(note, i) in decision.healthNotes"
                  :key="i"
                  size="small"
                  :bordered="false"
                >
                  {{ sanitizeInternalCopy(note) }}
                </n-tag>
              </n-space>
            </template>
            <n-empty v-else description="关注建议暂不可用" size="small" />
          </section>

          <section class="block">
            <n-text strong class="block-title">今日摘要</n-text>
            <template v-if="daily">
              <div class="summary-row">
                <n-text depth="3" class="row-label">今日交易</n-text>
                <n-text>{{ sanitizeInternalCopy(daily.tradingNarrative) || '—' }}</n-text>
              </div>
              <div class="summary-row">
                <n-text depth="3" class="row-label">风险摘要</n-text>
                <n-text>{{ sanitizeInternalCopy(daily.riskNarrative) || '—' }}</n-text>
              </div>
              <div class="summary-row">
                <n-text depth="3" class="row-label">明日关注</n-text>
                <n-space v-if="daily.tomorrowFocus?.length" :wrap="true">
                  <n-tag
                    v-for="(f, i) in daily.tomorrowFocus"
                    :key="i"
                    size="small"
                    type="info"
                    :bordered="false"
                  >
                    {{ sanitizeInternalCopy(f) }}
                  </n-tag>
                </n-space>
                <n-text v-else>—</n-text>
              </div>
            </template>
            <n-empty v-else description="今日摘要暂不可用" size="small" />
          </section>

          <section class="block">
            <n-text strong class="block-title">今日流程</n-text>
            <n-text v-if="trading?.narrative" depth="3" class="block-sub">
              {{ sanitizeInternalCopy(trading.narrative) }}
            </n-text>
            <div class="pipeline">
              <div v-for="step in pipelineSteps" :key="step.key" class="pipeline-item">
                <n-text class="pipeline-label">{{ tradingStepLabel(step.key) }}</n-text>
                <n-tag size="small" :type="statusTagType(step.status)" :bordered="false">
                  {{ tradingStatusLabel(step.status) }}
                </n-tag>
              </div>
            </div>
            <n-text
              v-if="trading?.executionStatus === 'FAIL' && trading?.executionReason"
              type="error"
              style="display: block; margin-top: 8px"
            >
              说明：{{ sanitizeInternalCopy(trading.executionReason) }}
            </n-text>
          </section>

          <section class="block">
            <n-space align="center" :wrap="true" style="margin-bottom: 6px">
              <n-text strong class="block-title" style="margin-bottom: 0">今日关注</n-text>
              <n-tag
                v-if="dailyAttention"
                size="small"
                :type="attentionTagType(dailyAttention.overallAction)"
                :bordered="false"
              >
                {{ attentionLabel(dailyAttention.overallAction) }}
              </n-tag>
            </n-space>
            <n-text v-if="dailyAttention?.headline" depth="3" class="block-sub">
              {{ sanitizeInternalCopy(dailyAttention.headline) }}
            </n-text>
            <n-list v-if="attentionRows.length" bordered>
              <n-list-item v-for="(item, idx) in attentionRows" :key="idx">
                <n-space vertical :size="4" style="width: 100%">
                  <n-space align="center" :wrap="true">
                    <n-tag
                      size="tiny"
                      :type="attentionTagType(item.suggestedAction)"
                      :bordered="false"
                    >
                      {{ attentionLabel(item.suggestedAction) }}
                    </n-tag>
                    <n-text strong>{{ sanitizeInternalCopy(item.title) }}</n-text>
                    <n-text v-if="item.stockCode" depth="3">{{ item.stockCode }}</n-text>
                  </n-space>
                  <n-text v-if="item.reason" depth="3">{{ sanitizeInternalCopy(item.reason) }}</n-text>
                </n-space>
              </n-list-item>
            </n-list>
            <n-empty v-else description="暂无今日关注事项" size="small" />
          </section>

          <section class="block">
            <n-text strong class="block-title">持仓状态</n-text>
            <n-text depth="3" class="block-sub">
              持仓数量来自组合快照；可卖与锁定分列。
            </n-text>
            <div v-if="positionStates.length" class="ps-table-wrap">
              <table class="ps-table">
                <thead>
                  <tr>
                    <th>代码</th>
                    <th>状态</th>
                    <th>新建仓</th>
                    <th>持仓数量</th>
                    <th>可卖数量</th>
                    <th>T+1锁定</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in positionStates" :key="row.symbol">
                    <td class="code">{{ row.symbol }}</td>
                    <td>
                      <n-tag
                        size="tiny"
                        :type="positionStateTagType(row.positionStatus)"
                        :bordered="false"
                      >
                        {{ row.positionStatusLabel }}
                      </n-tag>
                    </td>
                    <td>{{ row.isNewPosition ? '是' : '否' }}</td>
                    <td>{{ row.totalQty }}</td>
                    <td>{{ row.availableQty }}</td>
                    <td>{{ row.lockedQty }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <n-empty v-else description="暂无持仓状态" size="small" />
          </section>
        </template>
      </template>
    </n-spin>

    <stock-kline-modal
      v-model:show="klineModal.visible"
      :title="klineModal.title"
      :chart-key="'home-plan-kline-' + klineModal.chartCode"
      :code="klineModal.chartCode"
      :stock-name="klineModal.stockName"
    />

    <OpportunityProjectionDrawer
      v-model:show="projectionDrawerVisible"
      :row="projectionDrawerRow"
    />
  </div>
</template>

<style scoped>
.investment-home {
  padding: 8px 4px 24px;
  max-width: 960px;
}
.block {
  margin-bottom: 22px;
  padding: 14px 16px;
  border: 1px solid rgba(128, 128, 128, 0.18);
  border-radius: 10px;
  background: rgba(128, 128, 128, 0.04);
}
.block-title {
  display: block;
  font-size: 15px;
  margin-bottom: 4px;
}
.block-sub {
  display: block;
  margin-bottom: 6px;
  line-height: 1.5;
}
.plan-slot {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(128, 128, 128, 0.14);
}
.plan-slot:first-of-type {
  margin-top: 8px;
  padding-top: 0;
  border-top: none;
}
.plan-row {
  margin-bottom: 10px;
}
.plan-row.is-extra {
  opacity: 0.92;
  padding-left: 4px;
  border-left: 2px solid rgba(128, 128, 128, 0.25);
}
.home-opp-table {
  margin-top: 4px;
}
.summary-row {
  display: flex;
  gap: 12px;
  margin-top: 10px;
  align-items: flex-start;
}
.row-label {
  flex: 0 0 72px;
  padding-top: 2px;
}
.pipeline {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 18px;
  margin-top: 10px;
}
.pipeline-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 96px;
}
.pipeline-label {
  font-size: 13px;
  opacity: 0.85;
}
.ps-table-wrap {
  overflow-x: auto;
  margin-top: 8px;
}
.ps-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.ps-table th,
.ps-table td {
  padding: 8px 10px;
  text-align: right;
  border-bottom: 1px solid rgba(128, 128, 128, 0.14);
  white-space: nowrap;
}
.ps-table th:nth-child(-n + 2),
.ps-table td:nth-child(-n + 2) {
  text-align: left;
}
.ps-table .code {
  font-family: Consolas, monospace;
}
.beta-flow-card {
  border-color: rgba(32, 128, 240, 0.28);
  background: rgba(32, 128, 240, 0.04);
  padding-bottom: 12px;
}
.beta-flow-steps {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 4px;
  margin-top: 8px;
}
.beta-flow-step {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: 0 1 auto;
  min-width: 0;
  max-width: 100%;
  padding: 6px 10px;
  border: 1px solid rgba(128, 128, 128, 0.22);
  border-radius: 999px;
  background: var(--n-color, #fff);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s, background 0.15s;
}
.beta-flow-step:hover {
  border-color: rgba(32, 128, 240, 0.45);
}
.beta-flow-step.is-current {
  border-color: rgba(240, 160, 32, 0.75);
  background: rgba(240, 160, 32, 0.12);
}
.beta-flow-step.is-done {
  border-color: rgba(24, 160, 88, 0.35);
  background: rgba(24, 160, 88, 0.06);
  opacity: 0.92;
}
.beta-flow-step.is-upcoming {
  opacity: 0.85;
}
.beta-flow-step-index {
  flex: 0 0 20px;
  width: 20px;
  height: 20px;
  line-height: 20px;
  text-align: center;
  border-radius: 50%;
  font-size: 11px;
  font-weight: 600;
  background: rgba(128, 128, 128, 0.12);
}
.beta-flow-step.is-current .beta-flow-step-index {
  background: rgba(240, 160, 32, 0.35);
}
.beta-flow-step.is-done .beta-flow-step-index {
  background: rgba(24, 160, 88, 0.22);
  color: #18a058;
}
.beta-flow-step-label {
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
}
.beta-flow-step-badge {
  font-size: 11px;
  line-height: 1;
  padding: 2px 6px;
  border-radius: 999px;
  white-space: nowrap;
}
.beta-flow-step-badge.is-current {
  background: rgba(240, 160, 32, 0.22);
  color: #ad6800;
}
.beta-flow-step-badge.is-done {
  background: rgba(24, 160, 88, 0.16);
  color: #18a058;
}
.beta-flow-arrow {
  flex: 0 0 auto;
  color: rgba(128, 128, 128, 0.5);
  font-size: 13px;
  line-height: 1;
  user-select: none;
  padding: 0 2px;
}
</style>
