/** Decision performance observation — read-only (Phase10-E.4) */

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

export type DecisionStateMetrics = {
  decisionState: string
  samples: number
  evaluated: number
  pending: number
  insufficient: number
  winRate: number | null
  avgReturn: number | null
  avgAlpha: number | null
  avgMaxDrawdown: number | null
  riskCaptureRate: number | null
}

export type ObservationPerformanceView = {
  disclaimer: string
  horizon: number
  benchmark: string
  samples: number
  evaluated: number
  decisionAccuracy: number | null
  winRate: number | null
  avgReturn: number | null
  avgAlpha: number | null
  byState: Record<string, DecisionStateMetrics>
  industryAvailable: boolean
}

const fallbackDisclaimer = '历史判断统计，不代表未来收益。不是交易建议。不生成买卖单。'

function parseState(raw: any, fallbackState: string): DecisionStateMetrics {
  const o = raw || {}
  return {
    decisionState: str(o.decision_state ?? o.decisionState, fallbackState),
    samples: num(o.samples),
    evaluated: num(o.evaluated),
    pending: num(o.pending),
    insufficient: num(o.insufficient),
    winRate: nullableNum(o.win_rate ?? o.winRate),
    avgReturn: nullableNum(o.avg_return ?? o.avgReturn),
    avgAlpha: nullableNum(o.avg_alpha ?? o.avgAlpha),
    avgMaxDrawdown: nullableNum(o.avg_max_drawdown ?? o.avgMaxDrawdown),
    riskCaptureRate: nullableNum(o.risk_capture_rate ?? o.riskCaptureRate),
  }
}

/** GET /api/papertrading/observation/performance */
export async function getObservationPerformance(query?: {
  horizon?: number
  benchmark?: string
}): Promise<ObservationPerformanceView> {
  const params = new URLSearchParams()
  if (query?.horizon) params.set('horizon', String(query.horizon))
  if (query?.benchmark?.trim()) params.set('benchmark', query.benchmark.trim())
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/papertrading/observation/performance${q}`)
  if (!res.ok) throw new Error(`判断效果请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '判断效果响应无效')
  const raw = body.performance || {}
  const by = raw.by_state || raw.byState || {}
  const industry = raw.industry_benchmark || raw.industryBenchmark || {}
  return {
    disclaimer: str(body.disclaimer || raw.disclaimer, fallbackDisclaimer),
    horizon: num(raw.horizon, 5),
    benchmark: str(raw.benchmark, 'csi300'),
    samples: num(raw.samples),
    evaluated: num(raw.evaluated),
    decisionAccuracy: nullableNum(raw.decision_accuracy ?? raw.decisionAccuracy),
    winRate: nullableNum(raw.win_rate ?? raw.winRate),
    avgReturn: nullableNum(raw.avg_return ?? raw.avgReturn),
    avgAlpha: nullableNum(raw.avg_alpha ?? raw.avgAlpha),
    byState: {
      HOLD_NORMAL: parseState(by.HOLD_NORMAL, 'HOLD_NORMAL'),
      HOLD_WATCH: parseState(by.HOLD_WATCH, 'HOLD_WATCH'),
      HOLD_REVIEW: parseState(by.HOLD_REVIEW, 'HOLD_REVIEW'),
    },
    industryAvailable: !!industry.available,
  }
}
