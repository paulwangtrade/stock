/** Trading Day Monitor — read-only (Phase11-C.1). GET /api/trading/day-monitor only. */

export type MonitorStepStatus = {
  status: string
  reason: string
  planId: number
  eventType: string
  source: string
}

export type TradingDayMonitorView = {
  tradeDate: string
  morning: {
    materialize: MonitorStepStatus
    approve: MonitorStepStatus
    freeze: MonitorStepStatus
  }
  execution: {
    session: string
    status: string
    planId: number
    reason: string
    eventType: string
    source: string
  }
  settlement: {
    status: string
    reason: string
    eventType: string
    source: string
  }
  dataSourceNote: string
  disclaimer: string
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function mapStep(raw: any): MonitorStepStatus {
  return {
    status: str(raw?.status, 'PENDING'),
    reason: str(raw?.reason),
    planId: num(raw?.plan_id),
    eventType: str(raw?.event_type),
    source: str(raw?.source),
  }
}

function mapMonitor(raw: any): TradingDayMonitorView {
  const morning = raw?.morning || {}
  const execution = raw?.execution || {}
  const settlement = raw?.settlement || {}
  return {
    tradeDate: str(raw?.trade_date),
    morning: {
      materialize: mapStep(morning.materialize),
      approve: mapStep(morning.approve),
      freeze: mapStep(morning.freeze),
    },
    execution: {
      session: str(execution.session),
      status: str(execution.status, 'PENDING'),
      planId: num(execution.plan_id),
      reason: str(execution.reason),
      eventType: str(execution.event_type),
      source: str(execution.source),
    },
    settlement: {
      status: str(settlement.status, 'PENDING'),
      reason: str(settlement.reason),
      eventType: str(settlement.event_type),
      source: str(settlement.source),
    },
    dataSourceNote: str(raw?.data_source_note),
    disclaimer: str(raw?.disclaimer),
  }
}

/** GET /api/trading/day-monitor */
export async function getTradingDayMonitor(tradeDate?: string): Promise<TradingDayMonitorView> {
  const q = tradeDate ? `?trade_date=${encodeURIComponent(tradeDate)}` : ''
  const res = await fetch(`/api/trading/day-monitor${q}`)
  if (!res.ok) {
    throw new Error(`trading day monitor HTTP ${res.status}`)
  }
  const body = await res.json()
  if (!body?.ok) {
    throw new Error(body?.message || 'trading day monitor failed')
  }
  return mapMonitor(body.monitor)
}
