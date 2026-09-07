<script setup>
/**
 * 我的组合 — Portfolio 只读页（P5-G-A + Phase15-B1 live price overlay）。
 * Phase14-A-R1-B：paper_sim 持仓人工卖出入口（可卖/锁定/操作列 + SellDraftDialog）。
 * Phase16.23 Holdings UX：成本价/估值价/操作记录摘要（展示层 only）。
 * Phase16.27-P0：单票今日浮盈 = (display_price − quote_pre_close)×qty；≠ 账户 daily_pnl。
 * Phase16.27-P1：市值/收益率/估值价与行情价分列（UI only；不改 mark/equity）。
 * Phase16.27-P2：来源 chip（Strategy/Watchlist/Manual）+ 复用 provenance 抽屉；不扩 provenance DTO。
 * Phase17-A.2：账户/持仓盈亏命名与 tooltip、as_of 展示（纯前端语义；不改交易链）。
 * Phase17-B.2：组合开 K 注入持仓上下文 + Modal 摘要/footer 卖出（复用 SellDraftDialog；不改图表核心）。
 * Phase17.1：盈亏色统一 A 股红涨绿跌（marketColor.pnlColor）；不改 daily_pnl 计算。
 * Phase17-C4：持仓健康等级/原因摘要（exit-evaluation HealthScore 只读展示；无自动卖出）。
 * Phase17.1：做 T 适宜性列 + HoldingTSuitabilityDrawer（只读；无买卖按钮）。
 * Phase17.2：行情参考价 + 行情时间/新鲜度（display projection；不改 mark/Settlement）。
 * Phase17.5：PositionOriginDrawer「为什么买入」（复用 Provenance API；不改来源模型）。
 * 资产事实：GET /api/portfolio/snapshot（include_display=1 仅用于行情展示 overlay）。
 * 账户今日盈亏 / 成交 / 风险标签：仍 GET /api/portfolio/dashboard。
 */
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NDataTable,
  NEmpty,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import { getPortfolioDashboard } from '../api/portfolioDashboard'
import { getPortfolioSnapshot } from '../api/portfolioSnapshot'
import { getPaperExitEvaluation } from '../api/paperObservation'
import {
  getPortfolioProvenance,
  PortfolioProvenanceNotFoundError,
} from '../api/portfolioProvenance.ts'
import { getTradePlanById, TRADE_PLAN_CODE_OK } from '../api/tradePlans.ts'
import SellDraftDialog from './SellDraftDialog.vue'
import PortfolioProvenanceDrawer from './PortfolioProvenanceDrawer.vue'
import PositionOriginDrawer from './PositionOriginDrawer.vue'
import HoldingHealthDrawer from './HoldingHealthDrawer.vue'
import HoldingTSuitabilityDrawer from './HoldingTSuitabilityDrawer.vue'
import { canShowSellButton, maxSellQuantity } from '../utils/portfolioSellEntry.js'
import {
  pickLatestPlanIdFromTrades,
  provenanceSourceChipMeta,
  resolveProvenanceSourceBucket,
} from '../utils/portfolioSourceChip.js'
import {
  buildHealthReasonSummary,
  healthGradeListLabel,
  healthGradeTagType,
} from '../utils/holdingHealthDisplay.js'
import {
  tSuitLevelEmoji,
  tSuitLevelLabel,
  tSuitLevelTagType,
} from '../utils/holdingTSuitabilityDisplay.js'
import {
  buildQuotePriceTooltip,
  formatPriceWithContext,
  formatQuoteTimestamp,
  isLiveQuoteSource,
  priceColumnTitle,
  priceFreshnessShortLabel,
  PRICE_KIND,
  resolveMarkPrice,
  resolvePriceFreshness,
  resolveQuotePriceOnly,
  resolveQuoteSource,
} from '../utils/portfolioQuoteDisplay.js'
import { formatFieldTooltip } from '../utils/statusDisplay.js'
import { pnlColor } from '../utils/marketColor.js'
import { adaptPortfolioPosition, applyStockClickAction } from '../utils/stockDisplayAdapters.js'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'

const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const snapshot = ref(null)
const dash = ref(null)
const sellDialogVisible = ref(false)
const sellTargetRow = ref(null)
const provenanceDrawerVisible = ref(false)
const provenanceTargetRow = ref(null)
const originDrawerVisible = ref(false)
const originTargetRow = ref(null)
const healthDrawerVisible = ref(false)
const healthTargetRow = ref(null)
const tSuitDrawerVisible = ref(false)
const tSuitTargetRow = ref(null)
/** @type {import('vue').Ref<Record<string, any>>} */
const healthByCode = ref({})
/** @type {import('vue').Ref<Record<string, any>>} */
const tSuitByCode = ref({})
/** @type {import('vue').Ref<Record<string, { bucket: string, planId: number }>>} */
const sourceChipByCode = ref({})
let sourceEnrichToken = 0
let healthEnrichToken = 0

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
  /** Snapshot position row for summary / sell; null when no position context. */
  positionRow: null,
  /** Chart cost line / position-aware signals (from positionRow). */
  costPrice: undefined,
  costVolume: undefined,
})

/** Phase17-B.2: open K-line with optional portfolio position context (no extra fetch). */
function openStockKline(model, row = null) {
  applyStockClickAction(model, klineModal)
  const pos = row && typeof row === 'object' ? row : null
  klineModal.positionRow = pos
  if (!pos) {
    klineModal.costPrice = undefined
    klineModal.costVolume = undefined
    return
  }
  const cost = Number(pos.avgCost ?? pos.avg_cost)
  const qty = Number(pos.totalQty ?? pos.total_qty)
  klineModal.costPrice = Number.isFinite(cost) && cost > 0 ? cost : undefined
  klineModal.costVolume = Number.isFinite(qty) && qty > 0 ? qty : undefined
}

const klinePositionAware = computed(() => {
  const q = Number(klineModal.costVolume)
  return Number.isFinite(q) && q > 0
})

const klineCanSell = computed(() => canShowSellButton(klineModal.positionRow))

function klineSummaryCurrentPrice(row) {
  if (!row) return null
  const d = Number(row.displayPrice ?? row.display_price)
  if (Number.isFinite(d) && d > 0) return d
  return null
}

function formatQty(v) {
  const n = Math.trunc(Number(v) || 0)
  return n.toLocaleString('zh-CN')
}

const found = computed(() => !!snapshot.value?.found)
const tradeDate = computed(() => snapshot.value?.tradeDate || dash.value?.tradeDate || '')
const disclaimer = computed(
  () =>
    snapshot.value?.disclaimer ||
    dash.value?.disclaimer ||
    'Paper 模拟账户观察视图。不代表真实交易账户。不生成买卖单。',
)
const dataSourceNote = computed(
  () => snapshot.value?.dataSourceNote || dash.value?.dataSourceNote || '',
)

/** Assets from Snapshot only. dailyPnl stays dashboard report-delta — never snapshot pnl. */
const summary = computed(() => {
  const s = snapshot.value
  if (!s) return null
  return {
    cash: s.cash,
    equity: s.equity,
    marketValue: s.marketValue,
    positionCount: s.positionCount,
    dailyPnl: dash.value?.summary?.dailyPnl ?? null,
    dailyPnlBasis: dash.value?.summary?.dailyPnlBasis || '',
  }
})
const positions = computed(() => {
  const rows = snapshot.value?.positions || []
  return rows.map((row) => {
    const key = normCode(row.stockCode)
    const health = key ? healthByCode.value[key] || null : null
    const tSuitability = key ? tSuitByCode.value[key] || null : null
    return { ...row, health, tSuitability }
  })
})
const quoteOverlayActive = computed(() => !!snapshot.value?.quoteOverlay)
const risk = computed(() => dash.value?.risk || null)
const fills = computed(() => dash.value?.trades?.fills || [])

/** Prefer snapshot as_of; fall back to dashboard as_of / updated_at. */
const dataAsOf = computed(() => {
  const raw =
    snapshot.value?.asOf ||
    snapshot.value?.updatedAt ||
    dash.value?.asOf ||
    ''
  return formatAsOfDisplay(raw)
})

function dailyPnlBasisLabel(basis) {
  const b = String(basis || '').trim()
  if (b === 'daily_report_delta') return '日报差'
  if (b === 'unavailable') return '暂无上一日报'
  return b || ''
}

function formatAsOfDisplay(raw) {
  const s = String(raw || '').trim()
  if (!s) return ''
  // RFC3339 / ISO → local-ish readable without inventing timezone conversion beyond string trim
  return s.replace('T', ' ').replace(/Z$/, ' UTC').slice(0, 19)
}

function formatMoney(v) {
  if (v === null || v === undefined) return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function formatPct(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(1)}%`
}

function riskTagType(level) {
  const s = String(level || '').toUpperCase()
  if (s === 'HIGH') return 'error'
  if (s === 'MEDIUM') return 'warning'
  if (s === 'LOW') return 'success'
  return 'default'
}

function openSellDialog(row) {
  sellTargetRow.value = row
  sellDialogVisible.value = true
}

function openOriginDrawer(row) {
  originTargetRow.value = row
  originDrawerVisible.value = true
}

function openProvenanceDrawer(row) {
  provenanceTargetRow.value = row
  provenanceDrawerVisible.value = true
}

function openFullProvenanceFromOrigin(row) {
  originDrawerVisible.value = false
  openProvenanceDrawer(row || originTargetRow.value)
}

const originDrawerSourceHint = computed(() => {
  const row = originTargetRow.value
  if (!row) return null
  return sourceChipForRow(row)
})

function openHealthDrawer(row) {
  healthTargetRow.value = row
  healthDrawerVisible.value = true
}

function openTSuitDrawer(row) {
  tSuitTargetRow.value = row
  tSuitDrawerVisible.value = true
}

const healthDrawerHealth = computed(() => healthTargetRow.value?.health || null)
const tSuitDrawerSuitability = computed(() => tSuitTargetRow.value?.tSuitability || null)

function renderHealthGrade(row) {
  const health = row?.health
  const grade = health?.grade
  if (!grade) {
    return h(
      NTooltip,
      { trigger: 'hover' },
      {
        trigger: () => h(NText, { depth: 3 }, { default: () => '—' }),
        default: () => '暂无健康评分（评价未就绪或不适用）',
      },
    )
  }
  const label = healthGradeListLabel(grade)
  const tip = [
    health.gradeLabel || label,
    Number.isFinite(Number(health.score)) ? `评分 ${health.score}` : '',
    '点击查看持仓健康解释（只读，非卖出建议）',
  ]
    .filter(Boolean)
    .join('\n')
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h(
          NTag,
          {
            size: 'small',
            bordered: false,
            type: healthGradeTagType(grade),
            style: { cursor: 'pointer' },
            onClick: () => openHealthDrawer(row),
          },
          { default: () => label },
        ),
      default: () => tip,
    },
  )
}

function renderHealthReasons(row) {
  const lines = buildHealthReasonSummary(row?.health)
  if (!lines.length) {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }
  const shown = lines.slice(0, 3)
  const tip = lines.map((x) => x.text).join('\n')
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h(
          'div',
          {
            style: {
              fontSize: '12px',
              lineHeight: '1.35',
              cursor: 'pointer',
              maxWidth: '160px',
            },
            onClick: () => openHealthDrawer(row),
          },
          shown.map((line) =>
            h(
              'div',
              {
                style: {
                  color: line.kind === 'warn' ? 'var(--n-warning-color)' : 'var(--n-success-color)',
                },
              },
              line.text,
            ),
          ),
        ),
      default: () => tip,
    },
  )
}

function renderTSuitLevel(row) {
  const suit = row?.tSuitability
  const level = String(suit?.level || '').trim().toLowerCase()
  if (!level) {
    return h(
      NTooltip,
      { trigger: 'hover' },
      {
        trigger: () => h(NText, { depth: 3 }, { default: () => '—' }),
        default: () => '暂无做 T 适宜性评价（评价未就绪或不适用）',
      },
    )
  }
  const label = `${tSuitLevelEmoji(level)} ${tSuitLevelLabel(level)}`.trim()
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h(
          NTag,
          {
            size: 'small',
            bordered: false,
            type: tSuitLevelTagType(level),
            style: { cursor: 'pointer' },
            onClick: () => openTSuitDrawer(row),
          },
          { default: () => label },
        ),
      default: () => '持仓做 T 适宜性 · 非交易指令。点击查看原因。',
    },
  )
}

function sourceChipForRow(row) {
  const key = normCode(row?.stockCode)
  const hit = key ? sourceChipByCode.value[key] : null
  const bucket = hit?.bucket || 'unknown'
  const meta = provenanceSourceChipMeta(bucket)
  return {
    ...meta,
    bucket,
    planId: Math.trunc(Number(hit?.planId) || 0),
  }
}

function renderSourceChip(row) {
  const chip = sourceChipForRow(row)
  const tip =
    chip.planId > 0
      ? `${chip.label} · 计划 #${chip.planId}\n点击查看「为什么买入」`
      : `${chip.label}\n点击查看「为什么买入」`
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h(
          NTag,
          {
            size: 'tiny',
            bordered: false,
            type: chip.type,
            style: { cursor: 'pointer' },
            onClick: () => openOriginDrawer(row),
          },
          { default: () => chip.label },
        ),
      default: () => tip,
    },
  )
}

/**
 * Best-effort source chips: prefer today's fill plan_id; else provenance trades.
 * Bucket from existing TradePlan.source_session via getTradePlanById (no provenance DTO change).
 */
async function enrichSourceChips(positions, fillIndex) {
  const token = ++sourceEnrichToken
  const rows = Array.isArray(positions) ? positions : []
  if (!rows.length) {
    sourceChipByCode.value = {}
    return
  }

  const planIdByCode = new Map()
  for (const row of rows) {
    const key = normCode(row.stockCode)
    if (!key) continue
    const todayFill = fillIndex?.get(key)?.[0]
    const fromFill = Math.trunc(Number(todayFill?.planId) || 0)
    if (fromFill > 0) planIdByCode.set(key, fromFill)
  }

  const needProv = rows.filter((row) => {
    const key = normCode(row.stockCode)
    return key && !planIdByCode.has(key)
  })

  await Promise.all(
    needProv.map(async (row) => {
      const key = normCode(row.stockCode)
      try {
        const prov = await getPortfolioProvenance(row.stockCode)
        const pid = pickLatestPlanIdFromTrades(prov.trades)
        if (pid > 0) planIdByCode.set(key, pid)
      } catch (e) {
        if (!(e instanceof PortfolioProvenanceNotFoundError)) {
          /* best-effort; keep unknown */
        }
      }
    }),
  )

  if (token !== sourceEnrichToken) return

  const uniquePlanIds = [...new Set([...planIdByCode.values()].filter((id) => id > 0))]
  const sessionByPlan = new Map()
  await Promise.all(
    uniquePlanIds.map(async (planId) => {
      try {
        const res = await getTradePlanById(planId)
        if (res.code !== TRADE_PLAN_CODE_OK || !res.plan) return
        sessionByPlan.set(planId, String(res.plan.source_session || '').trim())
      } catch (_) {
        /* unknown bucket */
      }
    }),
  )

  if (token !== sourceEnrichToken) return

  const next = {}
  for (const row of rows) {
    const key = normCode(row.stockCode)
    if (!key) continue
    const planId = planIdByCode.get(key) || 0
    const bucket = planId > 0
      ? resolveProvenanceSourceBucket(sessionByPlan.get(planId))
      : 'unknown'
    next[key] = { bucket, planId }
  }
  sourceChipByCode.value = next
}

function renderStockNameKlineLink(row) {
  const model = adaptPortfolioPosition(row)
  if (!model.klineKey) {
    return model.displayText || model.name || model.code || '—'
  }
  return h(StockLink, {
    model,
    onOpen: (m) => openStockKline(m, row),
  })
}

/** Normalize code for fill matching (sh600519 / 600519). */
function normCode(code) {
  return String(code || '')
    .trim()
    .toLowerCase()
    .replace(/^(sh|sz|bj)/, '')
}

/**
 * Index today's dashboard fills by stock (already loaded — no extra request).
 * Full history stays in provenance drawer (lazy).
 */
const fillsByNormCode = computed(() => {
  const map = new Map()
  for (const f of fills.value) {
    const key = normCode(f.stockCode)
    if (!key) continue
    const list = map.get(key) || []
    list.push(f)
    map.set(key, list)
  }
  for (const [, list] of map) {
    list.sort((a, b) => String(b.filledAt || '').localeCompare(String(a.filledAt || '')))
  }
  return map
})

function lastFillForRow(row) {
  const key = normCode(row?.stockCode)
  if (!key) return null
  const list = fillsByNormCode.value.get(key)
  return list && list.length ? list[0] : null
}

function formatFillTime(raw) {
  const s = String(raw || '').trim()
  if (!s) return '—'
  return s.replace('T', ' ').slice(0, 19)
}

/** Compact activity line from today's fills only (no invented history). */
function activitySummary(row) {
  const f = lastFillForRow(row)
  if (!f) {
    return { text: '暂无记录', tip: '今日 dashboard 无成交摘要。点击「详情」懒加载持仓来源与历史成交（provenance）' }
  }
  const side = String(f.side || '').toLowerCase()
  const sideLabel = side.includes('sell') || side === 's' ? '卖出' : '买入'
  const price = formatMoney(f.price)
  const when = formatFillTime(f.filledAt)
  const plan = Number(f.planId) > 0 ? `计划 #${f.planId}` : '来源计划 —'
  return {
    text: `${sideLabel} ${price} · ${when}`,
    tip: `${sideLabel}价（成交价）${price}\n时间 ${when}\n${plan}\n完整记录请点「详情」`,
  }
}

function renderMarkPrice(row) {
  const mark = resolveMarkPrice(row)
  const ctx = formatPriceWithContext(mark, PRICE_KIND.mark, {
    extraTooltip: '账本估值价；市值/累计浮盈/总权益按此口径，不计入行情 overlay',
  })
  if (ctx.value === '—' || mark == null || !(Number(mark) > 0)) {
    return h(
      NTooltip,
      { trigger: 'hover' },
      {
        trigger: () => h(NText, { depth: 3 }, { default: () => '—' }),
        default: () => '估值数据不可用',
      },
    )
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

function renderCostPrice(row) {
  const ctx = formatPriceWithContext(row.avgCost, PRICE_KIND.cost, {
    extraTooltip: formatFieldTooltip('cost') || '持仓成本价，仅供展示',
  })
  if (ctx.value === '—') {
    return h(NText, { depth: 3 }, { default: () => '—' })
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

/** Live/open quote only — never fall back to mark (would fake 行情价). */
function resolveQuotePriceForColumn(row) {
  return resolveQuotePriceOnly(row)
}

function renderQuotePrice(row) {
  const quote = resolveQuotePriceForColumn(row)
  if (quote == null) {
    return h(
      NTooltip,
      { trigger: 'hover' },
      {
        trigger: () => h(NText, { depth: 3 }, { default: () => '—' }),
        default: () => '暂无实时行情；不显示 0，不用估值价冒充行情价',
      },
    )
  }
  const ctx = formatPriceWithContext(quote, PRICE_KIND.last, {
    extraTooltip: buildQuotePriceTooltip(row, formatMoney),
  })
  const ts = formatQuoteTimestamp(row)
  const freshLabel = priceFreshnessShortLabel(resolvePriceFreshness(row))
  const sub =
    ts || freshLabel
      ? h(
          NText,
          { depth: 3, style: { display: 'block', fontSize: '11px', lineHeight: '1.2' } },
          {
            default: () => [ts, freshLabel && ts ? ` · ${freshLabel}` : freshLabel].filter(Boolean).join(''),
          },
        )
      : null
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h('div', null, [h(NText, null, { default: () => ctx.value }), sub].filter(Boolean)),
      default: () => ctx.tooltip,
    },
  )
}

function renderMarketValue(row) {
  const v = row.marketValue
  if (v == null || !Number.isFinite(Number(v))) {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }
  const dm = row.displayMarketValue
  const tip =
    dm != null &&
    Number.isFinite(Number(dm)) &&
    isLiveQuoteSource(resolveQuoteSource(row))
      ? `账本市值（mark×数量）= ${formatMoney(v)}\n行情市值（仅参考）= ${formatMoney(dm)}`
      : '账本市值 = 估值价 × 数量；与总览市值同口径'
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () => h(NText, null, { default: () => formatMoney(v) }),
      default: () => tip,
    },
  )
}

function renderPnlPercent(row) {
  const v = row.pnlPercent
  if (v == null || !Number.isFinite(Number(v))) {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h('span', { style: { color: pnlColor(v) } }, formatPct(v)),
      default: () => '累计收益率（相对成本 · 账本估值价口径）',
    },
  )
}

const positionColumns = computed(() => [
  {
    title: '股票',
    key: 'stock',
    width: 150,
    fixed: 'left',
    ellipsis: { tooltip: true },
    render(row) {
      return renderStockNameKlineLink(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '数量'),
          default: () => '总数量；可卖/锁定见 tooltip',
        },
      ),
    key: 'totalQty',
    width: 80,
    render(row) {
      const total = Number(row.totalQty || 0).toLocaleString('zh-CN')
      const tip = `可卖 ${maxSellQuantity(row).toLocaleString('zh-CN')} · 锁定 ${Number(row.lockedQty || 0).toLocaleString('zh-CN')}`
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h(NText, null, { default: () => total }),
          default: () => tip,
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
          trigger: () => h('span', null, priceColumnTitle(PRICE_KIND.cost)),
          default: () => formatFieldTooltip('cost') || '持仓成本价',
        },
      ),
    key: 'avgCost',
    width: 96,
    render(row) {
      return renderCostPrice(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, priceColumnTitle(PRICE_KIND.mark)),
          default: () => '账面估值价（mark）。驱动市值/累计盈亏/总权益；无有效值显示 —。',
        },
      ),
    key: 'markPrice',
    width: 100,
    render(row) {
      return renderMarkPrice(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, priceColumnTitle(PRICE_KIND.last)),
          default: () =>
            '行情参考价（quote_price）。仅 live/open 时显示；下方为行情时间/新鲜度。缺失为 —，不用估值价冒充。Settlement 估值见「估值价」列。',
        },
      ),
    key: 'displayPrice',
    width: 112,
    render(row) {
      return renderQuotePrice(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '市值'),
          default: () => '账本市值 = snapshot.market_value（mark×数量）；不在前端重算',
        },
      ),
    key: 'marketValue',
    width: 110,
    render(row) {
      return renderMarketValue(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '累计浮盈'),
          default: () =>
            '相对成本 · 账本估值：(mark_price − 成本价) × 持仓数量。≠ 持仓今日浮盈；≠ 账户今日盈亏。',
        },
      ),
    key: 'pnl',
    width: 100,
    render(row) {
      const v = row.pnl
      return h('span', { style: { color: pnlColor(v) } }, formatMoney(v))
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '收益率'),
          default: () => '累计收益率 pnl_percent（相对成本 · 账本口径）',
        },
      ),
    key: 'pnlPercent',
    width: 88,
    render(row) {
      return renderPnlPercent(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '持仓今日浮盈'),
          default: () =>
            '相对昨收 · 行情口径：(当前行情 − 昨收) × 持仓数量。≠ 账户今日盈亏（日报权益差）。',
        },
      ),
    key: 'todayPnl',
    width: 118,
    render(row) {
      const v = row.todayPnl
      if (v == null || !Number.isFinite(Number(v))) {
        return h(
          NTooltip,
          { trigger: 'hover' },
          {
            trigger: () => h(NText, { depth: 3 }, { default: () => '—' }),
            default: () =>
              '暂无持仓今日浮盈（缺行情价或昨收）。账户今日盈亏见上方总览。',
          },
        )
      }
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h('span', { style: { color: pnlColor(v) } }, formatMoney(v)),
          default: () =>
            '(行情 − 昨收) × 数量；≠ 账户今日盈亏；≠ 累计浮盈（相对成本 · mark）',
        },
      )
    },
  },
  {
    title: '持仓状态',
    key: 'positionStatus',
    width: 96,
    render(row) {
      const label = row.positionStatusLabel || '—'
      return h(
        NTag,
        {
          size: 'small',
          bordered: false,
          type: row.isNewPosition ? 'warning' : 'default',
        },
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
          trigger: () => h('span', null, '健康'),
          default: () =>
            '持仓健康等级（A–D）。质量评价，非卖出建议。点击查看解释。',
        },
      ),
    key: 'healthGrade',
    width: 88,
    render(row) {
      return renderHealthGrade(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '健康摘要'),
          default: () => '支持/风险因素摘要（来自 Holding Health Score）。只读展示。',
        },
      ),
    key: 'healthReasons',
    width: 148,
    render(row) {
      return renderHealthReasons(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '做 T'),
          default: () =>
            '持仓做 T 适宜性（可考虑 / 观察 / 条件不足）。非交易指令。点击查看原因。',
        },
      ),
    key: 'tSuitability',
    width: 108,
    render(row) {
      return renderTSuitLevel(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '来源'),
          default: () =>
            '持仓来源浅标签（Strategy / Watchlist / Manual）。点击打开「为什么买入」解释；完整成交溯源见抽屉内入口。',
        },
      ),
    key: 'sourceChip',
    width: 88,
    render(row) {
      return renderSourceChip(row)
    },
  },
  {
    title: '操作记录',
    key: 'activity',
    minWidth: 168,
    ellipsis: { tooltip: true },
    render(row) {
      const act = activitySummary(row)
      return h(
        NSpace,
        { size: 6, align: 'center', wrap: false },
        {
          default: () => [
            h(
              NTooltip,
              { trigger: 'hover' },
              {
                trigger: () => h(NText, { depth: 3 }, { default: () => act.text }),
                default: () => act.tip,
              },
            ),
            h(
              NButton,
              {
                size: 'tiny',
                tertiary: true,
                onClick: () => openOriginDrawer(row),
              },
              { default: () => '为什么买入' },
            ),
          ],
        },
      )
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 80,
    fixed: 'right',
    render(row) {
      if (!canShowSellButton(row)) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      return h(
        NButton,
        {
          size: 'tiny',
          type: 'warning',
          secondary: true,
          onClick: () => openSellDialog(row),
        },
        { default: () => '卖出' },
      )
    },
  },
])

const fillColumns = [
  { title: '代码', key: 'stockCode', width: 100 },
  {
    title: '名称',
    key: 'stockName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return adaptPortfolioPosition(row).name || '—'
    },
  },
  { title: '方向', key: 'side', width: 70 },
  {
    title: priceColumnTitle(PRICE_KIND.filled),
    key: 'price',
    width: 90,
    render(row) {
      return formatMoney(row.price)
    },
  },
  {
    title: '数量',
    key: 'volume',
    width: 90,
    render(row) {
      return Number(row.volume || 0).toLocaleString('zh-CN')
    },
  },
  {
    title: '时间',
    key: 'filledAt',
    width: 170,
    render(row) {
      return formatFillTime(row.filledAt)
    },
  },
]

async function enrichHoldingHealth(positions) {
  const token = ++healthEnrichToken
  const rows = Array.isArray(positions) ? positions : []
  if (!rows.length) {
    healthByCode.value = {}
    tSuitByCode.value = {}
    return
  }
  try {
    const view = await getPaperExitEvaluation()
    if (token !== healthEnrichToken) return
    const nextHealth = {}
    const nextSuit = {}
    for (const h of view?.holdings || []) {
      const key = normCode(h.stockCode)
      if (!key) continue
      if (h.healthScore) nextHealth[key] = h.healthScore
      if (h.tSuitability) nextSuit[key] = h.tSuitability
    }
    healthByCode.value = nextHealth
    tSuitByCode.value = nextSuit
  } catch (_) {
    if (token === healthEnrichToken) {
      healthByCode.value = {}
      tSuitByCode.value = {}
    }
  }
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [snapRes, dashRes] = await Promise.allSettled([
      getPortfolioSnapshot({ includeDisplay: true }),
      getPortfolioDashboard(),
    ])
    if (snapRes.status !== 'fulfilled') {
      throw snapRes.reason instanceof Error ? snapRes.reason : new Error(String(snapRes.reason))
    }
    snapshot.value = snapRes.value
    dash.value = dashRes.status === 'fulfilled' ? dashRes.value : null
    // Non-blocking: chips default to 未知来源 until enrichment finishes.
    sourceChipByCode.value = {}
    healthByCode.value = {}
    tSuitByCode.value = {}
    const pos = snapRes.value?.positions || []
    void enrichSourceChips(pos, fillsByNormCode.value).catch(() => {
      /* ignore enrichment errors */
    })
    void enrichHoldingHealth(pos).catch(() => {
      /* ignore health enrichment errors */
    })
  } catch (e) {
    snapshot.value = null
    dash.value = null
    sourceChipByCode.value = {}
    healthByCode.value = {}
    tSuitByCode.value = {}
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="portfolio-dashboard">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center" :wrap="true">
        <n-text strong style="font-size: 16px">我的组合</n-text>
        <n-tag size="small" type="info" :bordered="false">模拟账户 · 只读</n-tag>
        <n-tag size="small" :bordered="false">持仓快照</n-tag>
        <n-text v-if="tradeDate" depth="3">业务日 {{ tradeDate }}</n-text>
        <n-text v-if="dataAsOf" depth="3">数据截至 {{ dataAsOf }}</n-text>
      </n-space>
      <n-button :loading="loading" @click="refresh">刷新</n-button>
    </n-space>

    <n-alert type="warning" :bordered="false" style="margin-bottom: 14px">
      {{ disclaimer }}
    </n-alert>

    <n-spin :show="loading">
      <template v-if="errorMessage && !snapshot">
        <n-empty :description="errorMessage" />
      </template>

      <template v-else-if="snapshot && !found">
        <n-empty description="暂无 Paper 账户（尚未成交建仓）" />
      </template>

      <template v-else-if="snapshot">
        <!-- A. Summary -->
        <n-text strong style="display: block; margin-bottom: 8px">账户总览</n-text>
        <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
          权益/市值按账本估值价（mark）。账户今日盈亏为相对上一结算日报的权益差，不是盘中持仓浮盈合计。
        </n-text>
        <n-space :wrap="true" :size="24" style="margin-bottom: 20px">
          <n-statistic label="总权益">
            <template #default>{{ formatMoney(summary?.equity) }}</template>
          </n-statistic>
          <n-statistic label="现金">
            <template #default>{{ formatMoney(summary?.cash) }}</template>
          </n-statistic>
          <n-statistic label="市值">
            <template #default>{{ formatMoney(summary?.marketValue) }}</template>
          </n-statistic>
          <n-statistic>
            <template #label>
              <n-tooltip trigger="hover">
                <template #trigger>
                  <span style="cursor: help; border-bottom: 1px dashed rgba(128, 128, 128, 0.45)">
                    账户今日盈亏
                  </span>
                </template>
                相对上一交易日结算权益变化（日报差）。含成交与日终估值写入的权益变动；不含盘中行情
                overlay，也不是持仓表「持仓今日浮盈」之和。
              </n-tooltip>
            </template>
            <template #default>
              <span :style="{ color: pnlColor(summary?.dailyPnl) }">
                {{ summary?.dailyPnl == null ? '—' : formatMoney(summary.dailyPnl) }}
              </span>
              <n-tag
                v-if="dailyPnlBasisLabel(summary?.dailyPnlBasis)"
                size="tiny"
                :bordered="false"
                style="margin-left: 6px; vertical-align: middle"
              >
                {{ dailyPnlBasisLabel(summary?.dailyPnlBasis) }}
              </n-tag>
            </template>
          </n-statistic>
          <n-statistic label="持仓数" :value="summary?.positionCount ?? 0" />
        </n-space>

        <!-- C. Risk (before long tables for viewport scan) -->
        <n-text strong style="display: block; margin-bottom: 8px">风险</n-text>
        <n-space align="center" :wrap="true" style="margin-bottom: 20px">
          <n-text>风险等级</n-text>
          <n-tag size="medium" :type="riskTagType(risk?.riskLevel)" :bordered="false">
            {{ risk?.riskLevel || '—' }}
          </n-tag>
          <n-text depth="3">最大仓位比例 {{ formatPct(risk?.maxPositionRatio) }}</n-text>
        </n-space>

        <!-- B. Positions -->
        <n-space align="center" :wrap="true" style="margin-bottom: 8px">
          <n-text strong>持仓列表</n-text>
          <n-tag
            size="small"
            :type="quoteOverlayActive ? 'success' : 'default'"
            :bordered="false"
          >
            {{ quoteOverlayActive ? '含行情 overlay' : '账本估值' }}
          </n-tag>
          <n-text v-if="dataAsOf" depth="3" style="font-size: 12px">截至 {{ dataAsOf }}</n-text>
        </n-space>
        <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
          估值价/市值/累计浮盈为账本口径（mark）。持仓今日浮盈 = (行情 − 昨收)×数量（仅展示）。行情价仅参考并带行情时间；不改总权益。健康 / 做 T 列可点击查看解释（非买卖建议）。来源列可打开「为什么买入」。
        </n-text>
        <div v-if="positions.length" class="holdings-table-wrap">
          <n-data-table
            class="holdings-table"
            :columns="positionColumns"
            :data="positions"
            :bordered="false"
            size="small"
            :single-line="true"
            :scroll-x="1820"
            flex-height
            :style="{ height: '100%' }"
          />
        </div>
        <n-empty v-else description="无持仓" style="margin-bottom: 20px" />

        <!-- D. Today Trades -->
        <n-text strong style="display: block; margin-bottom: 8px">今日成交</n-text>
        <n-data-table
          v-if="fills.length"
          :columns="fillColumns"
          :data="fills"
          :bordered="false"
          size="small"
        />
        <n-empty v-else description="今日无成交" />

        <SellDraftDialog
          v-model:show="sellDialogVisible"
          :row="sellTargetRow"
          :trade-date="tradeDate"
          actor="ui:portfolio-sell"
          @created="refresh"
        />

        <PositionOriginDrawer
          v-model:show="originDrawerVisible"
          :row="originTargetRow"
          :source-hint="originDrawerSourceHint"
          @open-full-provenance="openFullProvenanceFromOrigin"
        />
        <PortfolioProvenanceDrawer
          v-model:show="provenanceDrawerVisible"
          :row="provenanceTargetRow"
        />
        <HoldingHealthDrawer
          v-model:show="healthDrawerVisible"
          :row="healthTargetRow"
          :health="healthDrawerHealth"
        />
        <HoldingTSuitabilityDrawer
          v-model:show="tSuitDrawerVisible"
          :row="tSuitTargetRow"
          :suitability="tSuitDrawerSuitability"
        />

        <stock-kline-modal
          v-model:show="klineModal.visible"
          :title="klineModal.title"
          :chart-key="'portfolio-kline-' + klineModal.chartCode"
          :code="klineModal.chartCode"
          :stock-name="klineModal.stockName"
          :cost-price="klineModal.costPrice"
          :cost-volume="klineModal.costVolume"
          :position-aware-signals="klinePositionAware"
        >
          <template v-if="klineModal.positionRow" #prepend>
            <div class="kline-pos-summary">
              <n-text strong style="display: block; margin-bottom: 6px">持仓摘要</n-text>
              <div class="kline-pos-summary__row">
                <span class="kline-pos-summary__item">
                  股票
                  <strong>
                    {{ klineModal.positionRow.stockName || klineModal.stockName || '—' }}
                    {{ klineModal.positionRow.stockCode || klineModal.chartCode }}
                  </strong>
                </span>
                <span class="kline-pos-summary__item">
                  数量
                  <strong>{{ formatQty(klineModal.positionRow.totalQty) }}</strong>
                </span>
                <span class="kline-pos-summary__item">
                  成本
                  <strong>{{ formatMoney(klineModal.positionRow.avgCost) }}</strong>
                </span>
                <span class="kline-pos-summary__item">
                  现价
                  <strong>
                    {{
                      klineSummaryCurrentPrice(klineModal.positionRow) == null
                        ? '—'
                        : formatMoney(klineSummaryCurrentPrice(klineModal.positionRow))
                    }}
                  </strong>
                </span>
                <span class="kline-pos-summary__item">
                  浮盈
                  <strong :style="{ color: pnlColor(klineModal.positionRow.pnl) }">
                    {{ formatMoney(klineModal.positionRow.pnl) }}
                  </strong>
                </span>
              </div>
              <n-text depth="3" style="display: block; margin-top: 4px; font-size: 11px">
                浮盈为相对成本·账本估值（累计浮盈）。成本线按持仓成本绘制；现价为行情 overlay（无则 —）。
              </n-text>
            </div>
          </template>
          <template v-if="klineCanSell" #footer>
            <n-space justify="end">
              <n-button type="warning" secondary @click="openSellDialog(klineModal.positionRow)">
                卖出计划
              </n-button>
            </n-space>
          </template>
        </stock-kline-modal>

        <n-text depth="3" style="display: block; margin-top: 16px; font-size: 12px">
          {{ dataSourceNote }}
        </n-text>
      </template>
    </n-spin>
  </div>
</template>

<style scoped>
.portfolio-dashboard {
  padding: 12px 16px 24px;
  max-width: 1200px;
  box-sizing: border-box;
}
.holdings-table-wrap {
  height: min(520px, calc(100vh - 360px));
  min-height: 280px;
  margin-bottom: 20px;
  overflow: hidden;
}
.holdings-table :deep(.n-data-table-thead) {
  position: sticky;
  top: 0;
  z-index: 2;
}
.kline-pos-summary {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(128, 128, 128, 0.08);
  font-size: 12px;
  line-height: 1.5;
}
.kline-pos-summary__row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
}
.kline-pos-summary__item strong {
  margin-left: 4px;
  font-weight: 600;
}
</style>
