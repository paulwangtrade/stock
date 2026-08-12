/** Portfolio Management Observation — read-only (Phase10-E.2 / E.3) */

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function nullableNum(v: unknown): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

export type PortfolioObsAccount = {
  totalEquity: number
  cash: number
  marketValue: number
  exposure: number
  positionCount: number
  found: boolean
}

export type PortfolioObsDecision = {
  normalCount: number
  watchCount: number
  reviewCount: number
  exitCandidateCount: number
  normalWeight: number
  watchWeight: number
  reviewWeight: number
  exitCandidateWeight: number
}

export type PortfolioObsRebalance = {
  keepCount: number
  addCount: number
  increaseCount: number
  decreaseCount: number
  removeCount: number
  available: boolean
}

export type PortfolioObsHealth = {
  portfolioHealth: string
  healthyPositions: number
  watchPositions: number
  reviewPositions: number
  agingPositions: number
  riskPositions: number
  unknownHealthCount: number
}

export type PortfolioObsOpportunityCost = {
  available: boolean
  level: string
  note: string
}

export type PortfolioObsHistoryPoint = {
  asOf: string
  state: string
  reason: string
  source: string
}

export type PortfolioObsPosition = {
  symbol: string
  stockName: string
  currentWeight: number
  currentAmount: number
  targetWeight: number
  targetAmount: number
  deltaWeight: number
  cost: number | null
  currentPrice: number | null
  pnl: number | null
  returnRate: number | null
  holdingDays: number
  firstBuyDate: string
  holdingPeriodBucket: string
  isAging: boolean
  riskState: string
  profitState: string
  healthScore: number | null
  healthLevel: string
  decisionState: string
  decisionReason: string
  decisionHistory: PortfolioObsHistoryPoint[]
  decisionHistoryNote: string
  opportunityCostLevel: string
  rebalanceAction: string
  rebalanceReason: string
}

export type PortfolioObservationView = {
  disclaimer: string
  observationTime?: string
  account: PortfolioObsAccount
  decision: PortfolioObsDecision
  rebalance: PortfolioObsRebalance
  health: PortfolioObsHealth
  opportunityCost: PortfolioObsOpportunityCost
  positions: PortfolioObsPosition[]
  warnings: string[]
}

const fallbackDisclaimer = '观察结果不是交易建议。不生成买卖单，不执行调仓，不会自动卖出。'

/** GET /api/papertrading/observation/portfolio */
export async function getPaperPortfolioObservation(query?: {
  target?: string
  enter?: string
  drop?: string
}): Promise<PortfolioObservationView> {
  const params = new URLSearchParams()
  if (query?.target?.trim()) params.set('target', query.target.trim())
  if (query?.enter?.trim()) params.set('enter', query.enter.trim())
  if (query?.drop?.trim()) params.set('drop', query.drop.trim())
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/papertrading/observation/portfolio${q}`)
  if (!res.ok) throw new Error(`组合观察请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '组合观察响应无效')
  const raw = body.portfolio || {}
  const acct = raw.account || {}
  const dec = raw.decision || {}
  const reb = raw.rebalance || {}
  const health = raw.health || {}
  const oc = raw.opportunity_cost || raw.opportunityCost || {}
  return {
    disclaimer: str(body.disclaimer || raw.disclaimer, fallbackDisclaimer),
    observationTime: raw.observation_time || raw.observationTime ? str(raw.observation_time ?? raw.observationTime) : undefined,
    account: {
      totalEquity: num(acct.total_equity ?? acct.totalEquity),
      cash: num(acct.cash),
      marketValue: num(acct.market_value ?? acct.marketValue),
      exposure: num(acct.exposure),
      positionCount: num(acct.position_count ?? acct.positionCount),
      found: !!acct.found,
    },
    decision: {
      normalCount: num(dec.normal_count ?? dec.normalCount),
      watchCount: num(dec.watch_count ?? dec.watchCount),
      reviewCount: num(dec.review_count ?? dec.reviewCount),
      exitCandidateCount: num(dec.exit_candidate_count ?? dec.exitCandidateCount),
      normalWeight: num(dec.normal_weight ?? dec.normalWeight),
      watchWeight: num(dec.watch_weight ?? dec.watchWeight),
      reviewWeight: num(dec.review_weight ?? dec.reviewWeight),
      exitCandidateWeight: num(dec.exit_candidate_weight ?? dec.exitCandidateWeight),
    },
    rebalance: {
      keepCount: num(reb.keep_count ?? reb.keepCount),
      addCount: num(reb.add_count ?? reb.addCount),
      increaseCount: num(reb.increase_count ?? reb.increaseCount),
      decreaseCount: num(reb.decrease_count ?? reb.decreaseCount),
      removeCount: num(reb.remove_count ?? reb.removeCount),
      available: !!reb.available,
    },
    health: {
      portfolioHealth: str(health.portfolio_health ?? health.portfolioHealth, 'UNKNOWN'),
      healthyPositions: num(health.healthy_positions ?? health.healthyPositions),
      watchPositions: num(health.watch_positions ?? health.watchPositions),
      reviewPositions: num(health.review_positions ?? health.reviewPositions),
      agingPositions: num(health.aging_positions ?? health.agingPositions),
      riskPositions: num(health.risk_positions ?? health.riskPositions),
      unknownHealthCount: num(health.unknown_health_count ?? health.unknownHealthCount),
    },
    opportunityCost: {
      available: !!oc.available,
      level: str(oc.level, 'UNKNOWN'),
      note: str(oc.note),
    },
    positions: Array.isArray(raw.positions)
      ? raw.positions.map((row: any) => ({
          symbol: str(row?.symbol),
          stockName: str(row?.stock_name ?? row?.stockName),
          currentWeight: num(row?.current_weight ?? row?.currentWeight),
          currentAmount: num(row?.current_amount ?? row?.currentAmount),
          targetWeight: num(row?.target_weight ?? row?.targetWeight),
          targetAmount: num(row?.target_amount ?? row?.targetAmount),
          deltaWeight: num(row?.delta_weight ?? row?.deltaWeight),
          cost: nullableNum(row?.cost),
          currentPrice: nullableNum(row?.current_price ?? row?.currentPrice),
          pnl: nullableNum(row?.pnl),
          returnRate: nullableNum(row?.return ?? row?.returnRate),
          holdingDays: num(row?.holding_days ?? row?.holdingDays),
          firstBuyDate: str(row?.first_buy_date ?? row?.firstBuyDate),
          holdingPeriodBucket: str(row?.holding_period_bucket ?? row?.holdingPeriodBucket),
          isAging: !!(row?.is_aging ?? row?.isAging),
          riskState: str(row?.risk_state ?? row?.riskState),
          profitState: str(row?.profit_state ?? row?.profitState),
          healthScore: nullableNum(row?.health_score ?? row?.healthScore),
          healthLevel: str(row?.health_level ?? row?.healthLevel),
          decisionState: str(row?.decision_state ?? row?.decisionState),
          decisionReason: str(row?.decision_reason ?? row?.decisionReason),
          decisionHistory: Array.isArray(row?.decision_history ?? row?.decisionHistory)
            ? (row.decision_history ?? row.decisionHistory).map((pt: any) => ({
                asOf: str(pt?.as_of ?? pt?.asOf),
                state: str(pt?.state),
                reason: str(pt?.reason),
                source: str(pt?.source),
              }))
            : [],
          decisionHistoryNote: str(row?.decision_history_note ?? row?.decisionHistoryNote),
          opportunityCostLevel: str(row?.opportunity_cost_level ?? row?.opportunityCostLevel, 'UNKNOWN'),
          rebalanceAction: str(row?.rebalance_action ?? row?.rebalanceAction),
          rebalanceReason: str(row?.rebalance_reason ?? row?.rebalanceReason),
        }))
      : [],
    warnings: Array.isArray(raw.warnings) ? raw.warnings.map((w: any) => String(w)) : [],
  }
}
