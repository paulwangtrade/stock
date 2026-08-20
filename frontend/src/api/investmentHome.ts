/** Investment Home — read-only cockpit. GET /api/investment/home (display mapping only; no trading writes). */

import { collectOpportunityNameHints, toOpportunityCards } from '../utils/opportunityCard.js'
import { mapPositionStateBundle } from '../utils/positionStateDisplay.js'

export type HomePortfolioSummary = {
  found: boolean
  equity: number
  cash: number
  marketValue: number
  dailyPnl: number | null
  positionCount: number
  narrative: string
}

export type HomeTradingStatus = {
  materializeStatus: string
  approveStatus: string
  freezeStatus: string
  executionStatus: string
  settlementStatus: string
  executionReason: string
  narrative: string
  /** Backend trading_status.plan_id; display-only, not a lifecycle write. */
  planId: number
}

/** @deprecated Prefer DailyAttentionItem from daily_attention. */
export type HomeAttentionItem = {
  kind: string
  stockCode: string
  title: string
  detail: string
  level: string
}

export type DailyAttentionItem = {
  itemType: string
  stockCode: string
  priority: number
  title: string
  reason: string
  source: string
  severity: string
  suggestedAction: string
}

export type DailyAttentionView = {
  overallAction: string
  headline: string
  quality: string
  items: DailyAttentionItem[]
}

export type HomeDecisionSummary = {
  overallAttention: string
  healthStatus: string
  healthNotes: string[]
  explanation: string
  equity: number
  cash: number
  positionCount: number
}

export type HomeDailySummary = {
  tradingNarrative: string
  riskNarrative: string
  tomorrowFocus: string[]
  portfolioNarrative: string
}

export type HomePositionStateRow = {
  symbol: string
  positionStatus: string
  positionStatusLabel: string
  isNewPosition: boolean
  totalQty: number
  availableQty: number
  lockedQty: number
  canSell?: boolean
  holdingDays: number
  riskTag: string
  explanation: string
}

export type InvestmentHomeView = {
  tradeDate: string
  portfolio: HomePortfolioSummary
  decision: HomeDecisionSummary | null
  daily: HomeDailySummary | null
  trading: HomeTradingStatus
  dailyAttention: DailyAttentionView | null
  /** Phase11-K unified holding states (from home.position_states). */
  positionStates: HomePositionStateRow[]
  /** Phase12-D2-B compare cards; source opportunity_attention, not attention.stock_code. */
  opportunityCards: ReturnType<typeof toOpportunityCards>
  /** @deprecated Prefer dailyAttention */
  attentionItems: HomeAttentionItem[]
  quality: string
  disclaimer: string
}

const SAFE_ACTIONS = new Set(['HOLD', 'WATCH', 'REVIEW'])

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function nullableNum(v: unknown): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

/** Clamp to HOLD|WATCH|REVIEW — never surface BUY/SELL/AUTO*. */
export function clampSuggestedAction(raw: unknown): string {
  const s = String(raw || '').toUpperCase()
  if (SAFE_ACTIONS.has(s)) return s
  if (s === 'BUY' || s === 'SELL' || s.startsWith('AUTO')) return 'REVIEW'
  if (s === 'FAIL' || s === 'BLOCKED' || s === 'NEED_REVIEW' || s === 'POSSIBLE_REPLACE') return 'REVIEW'
  if (!s) return 'HOLD'
  return 'WATCH'
}

function mapDailyAttention(raw: any): DailyAttentionView | null {
  if (!raw || typeof raw !== 'object') return null
  const itemsRaw = Array.isArray(raw.items) ? raw.items : []
  const items: DailyAttentionItem[] = itemsRaw
    .map((it: any) => ({
      itemType: str(it?.item_type),
      stockCode: str(it?.stock_code),
      priority: num(it?.priority, 99),
      title: str(it?.title),
      reason: str(it?.reason),
      source: str(it?.source),
      severity: str(it?.severity),
      suggestedAction: clampSuggestedAction(it?.suggested_action),
    }))
    .filter((it: DailyAttentionItem) => SAFE_ACTIONS.has(it.suggestedAction))
    .sort((a: DailyAttentionItem, b: DailyAttentionItem) => a.priority - b.priority)

  return {
    overallAction: clampSuggestedAction(raw.overall_action),
    headline: str(raw.headline),
    quality: str(raw.quality, 'OK'),
    items,
  }
}

/** Home-only: read Snapshot qty fields. Never available+locked. Do not change global mapPositionStateRow. */
function mapHomePositionStates(raw: any): HomePositionStateRow[] {
  const bundle = mapPositionStateBundle(raw)
  const list = Array.isArray(raw?.positions) ? raw.positions : Array.isArray(raw) ? raw : []
  const qtyBySymbol: Record<string, { total: number; available: number; locked: number }> = Object.create(null)
  for (const it of list) {
    const sym = String(it?.symbol || it?.stock_code || it?.stockCode || '').trim().toLowerCase()
    if (!sym) continue
    const total = Number(it?.total_qty ?? it?.totalQty)
    const available = Number(it?.available_qty ?? it?.availableQty)
    const locked = Number(it?.locked_qty ?? it?.lockedQty)
    qtyBySymbol[sym] = {
      total: Number.isFinite(total) ? total : 0,
      available: Number.isFinite(available) ? available : 0,
      locked: Number.isFinite(locked) ? locked : 0,
    }
  }
  return bundle.positions.map((p) => {
    const q = qtyBySymbol[String(p.symbol || '').toLowerCase()]
    if (!q) {
      return { ...p, totalQty: Number.isFinite(Number(p.totalQty)) ? Number(p.totalQty) : 0 }
    }
    return { ...p, totalQty: q.total, availableQty: q.available, lockedQty: q.locked }
  }) as HomePositionStateRow[]
}

function mapHome(raw: any): InvestmentHomeView {
  const ps = raw?.portfolio_summary || {}
  const ds = raw?.decision_summary
  const daily = raw?.daily_summary
  const ts = raw?.trading_status || {}
  const items = Array.isArray(raw?.attention_items) ? raw.attention_items : []

  let decision: HomeDecisionSummary | null = null
  if (ds && typeof ds === 'object') {
    const health = ds.portfolio_health || {}
    decision = {
      overallAttention: clampSuggestedAction(ds.overall_attention),
      healthStatus: clampSuggestedAction(health.status || 'HOLD'),
      healthNotes: Array.isArray(health.notes) ? health.notes.map((x: unknown) => str(x)) : [],
      explanation: str(ds.explanation),
      equity: num(health.equity),
      cash: num(health.cash),
      positionCount: num(health.position_count),
    }
  }

  let dailyMapped: HomeDailySummary | null = null
  if (daily && typeof daily === 'object') {
    const trading = daily.trading_summary || {}
    const risk = daily.risk_summary || {}
    const port = daily.portfolio_summary || {}
    const focus = Array.isArray(daily.tomorrow_focus) ? daily.tomorrow_focus : []
    dailyMapped = {
      tradingNarrative: str(trading.narrative),
      riskNarrative: str(risk.narrative),
      tomorrowFocus: focus.map((x: unknown) => str(x)).filter(Boolean),
      portfolioNarrative: str(port.narrative),
    }
  }

  return {
    tradeDate: str(raw?.trade_date),
    portfolio: {
      found: !!ps.found,
      equity: num(ps.equity),
      cash: num(ps.cash),
      marketValue: num(ps.market_value),
      dailyPnl: nullableNum(ps.daily_pnl),
      positionCount: num(ps.position_count),
      narrative: str(ps.narrative),
    },
    decision,
    daily: dailyMapped,
    trading: {
      materializeStatus: str(ts.materialize_status, 'PENDING'),
      approveStatus: str(ts.approve_status, 'PENDING'),
      freezeStatus: str(ts.freeze_status, 'PENDING'),
      executionStatus: str(ts.execution_status, 'PENDING'),
      settlementStatus: str(ts.settlement_status, 'PENDING'),
      executionReason: str(ts.execution_reason),
      narrative: str(ts.narrative),
      planId: num(ts.plan_id, 0),
    },
    dailyAttention: mapDailyAttention(raw?.daily_attention),
    positionStates: mapHomePositionStates(raw?.position_states),
    opportunityCards: toOpportunityCards(
      ds && typeof ds === 'object' ? ds.opportunity_attention : null,
      collectOpportunityNameHints(raw),
      3,
    ),
    attentionItems: items.map((it: any) => ({
      kind: str(it?.kind),
      stockCode: str(it?.stock_code),
      title: str(it?.title),
      detail: str(it?.detail),
      level: str(it?.level),
    })),
    quality: str(raw?.quality, 'OK'),
    disclaimer: str(raw?.disclaimer),
  }
}

/** GET /api/investment/home */
export async function getInvestmentHome(tradeDate?: string): Promise<InvestmentHomeView> {
  const q = tradeDate ? `?trade_date=${encodeURIComponent(tradeDate)}` : ''
  const res = await fetch(`/api/investment/home${q}`)
  if (!res.ok) {
    throw new Error(`investment home HTTP ${res.status}`)
  }
  const body = await res.json()
  if (!body?.ok) {
    throw new Error(body?.message || 'investment home failed')
  }
  return mapHome(body.home)
}

/** User-facing labels for pipeline steps (no internal jargon). */
export function tradingStepLabel(key: string): string {
  switch (key) {
    case 'materialize':
      return '晨间准备'
    case 'approve':
      return '计划确认'
    case 'freeze':
      return '计划锁定'
    case 'execution':
      return '今日执行'
    case 'settlement':
      return '日终结算'
    default:
      return key
  }
}

export function tradingStatusLabel(status: string): string {
  switch (String(status || '').toUpperCase()) {
    case 'PASS':
      return '已完成'
    case 'FAIL':
      return '未就绪'
    case 'SKIP':
      return '已跳过'
    case 'PENDING':
      return '未开始'
    case 'UNKNOWN':
      return '暂无'
    default:
      return status || '—'
  }
}

export function attentionLabel(level: string): string {
  switch (clampSuggestedAction(level)) {
    case 'HOLD':
      return '暂无强制关注'
    case 'WATCH':
      return '建议留意'
    case 'REVIEW':
      return '建议复盘'
    default:
      return '—'
  }
}

export function itemTypeLabel(itemType: string): string {
  switch (String(itemType || '').toUpperCase()) {
    case 'PORTFOLIO':
      return '组合'
    case 'RISK':
      return '风险'
    case 'EFFICIENCY':
      return '效率'
    case 'OPPORTUNITY':
      return '机会'
    case 'POSITION':
      return '持仓'
    case 'TRADING':
      return '交易'
    case 'TOMORROW':
      return '明日'
    default:
      return '关注'
  }
}

const INTERNAL_COPY_REPLACEMENTS: Array<[RegExp, string]> = [
  [/MATERIALIZATION_PENDING/gi, '等待生成买入参数'],
  [/GROSS_EXPOSURE_EXCEEDED/gi, '组合仓位已达上限'],
  [/PositionState/gi, '持仓'],
  [/\bGateway\b/gi, ''],
  [/\bPreTrade\b/gi, ''],
  [/\bSettlement\b/gi, ''],
  [/\bMaterialize\b/gi, '已准备'],
  [/\bFreeze\b/gi, '已锁定计划'],
  [/\bDraft\b/gi, '待确认'],
]

/** Strip pipeline / ledger jargon from user-visible strings. Comparisons still use raw API values. */
export function sanitizeInternalCopy(raw: unknown): string {
  let s = String(raw || '')
  for (const [re, to] of INTERNAL_COPY_REPLACEMENTS) {
    s = s.replace(re, to)
  }
  return s.replace(/\s{2,}/g, ' ').trim()
}

const OPPORTUNITY_TYPES = new Set(['OPPORTUNITY', 'TOMORROW'])

/** Home “今日机会”: opportunity / tomorrow only, max 3. */
export function pickOpportunityItems(
  items: DailyAttentionItem[] | null | undefined,
  limit = 3,
): DailyAttentionItem[] {
  const list = Array.isArray(items) ? items : []
  return list
    .filter((it) => OPPORTUNITY_TYPES.has(String(it?.itemType || '').toUpperCase()))
    .slice(0, Math.max(0, limit))
}

export function opportunityUserLabel(item: DailyAttentionItem | null | undefined): string {
  if (String(item?.itemType || '').toUpperCase() === 'TOMORROW') {
    return '等待确认'
  }
  switch (clampSuggestedAction(item?.suggestedAction)) {
    case 'REVIEW':
      return '建议研究'
    case 'WATCH':
    case 'HOLD':
    default:
      return '关注'
  }
}

export type PlanUserStatusKey = 'none' | 'pending_confirm' | 'prepared' | 'locked'

export type PlanUserStatusInput = {
  status?: string
  trade_date?: string
  freeze?: { is_frozen?: boolean }
  morning?: { materialization_status?: string }
} | null | undefined

/** Map TradePlan fields to user labels. Never treat Monitor freeze FAIL as “锁定失败”. */
export function planUserStatus(plan: PlanUserStatusInput): { key: PlanUserStatusKey; label: string } {
  if (!plan) {
    return { key: 'none', label: '暂无交易计划' }
  }
  const frozen = !!plan.freeze?.is_frozen || String(plan.status || '').toLowerCase() === 'ready'
  if (frozen) {
    return { key: 'locked', label: '已锁定' }
  }
  if (String(plan.morning?.materialization_status || '').toUpperCase() === 'PASS') {
    return { key: 'prepared', label: '已准备' }
  }
  return { key: 'pending_confirm', label: '待确认' }
}

export function autoExecuteSentence(opts: {
  enableOpenBuy: boolean
  hasPlan: boolean
  isFrozen: boolean
  isTodayPlan: boolean
}): string {
  if (!opts.enableOpenBuy || !opts.hasPlan || !opts.isTodayPlan || !opts.isFrozen) {
    return '今日不会自动买入，等待确认'
  }
  return '计划已锁定，等待执行窗口'
}
