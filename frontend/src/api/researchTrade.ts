/** 研究交易 Intent HTTP API（Phase3-PR3） */

export type ResearchTradeIntentIDResponse = {
  id: number
  status: string
}

export type CreateResearchTradeIntentRequest = {
  symbol: string
  stock_name?: string
  price: number
  volume: number
  account_id?: number
  candidate_snapshot_id?: number
  signal_score?: number
  signal_tag?: string
  reason?: string
}

export type ExecuteResearchTradeIntentResponse = {
  status: string
  order_id?: number
  client_order_id?: string
  error_code?: string
  error_message?: string
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  })
  const text = await res.text()
  let data: any = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = null
  }
  if (!res.ok) {
    const msg = data?.error || data?.message || `请求失败: HTTP ${res.status}`
    throw new Error(msg)
  }
  return data as T
}

export function createResearchTradeIntent(
  req: CreateResearchTradeIntentRequest,
): Promise<ResearchTradeIntentIDResponse> {
  return postJSON('/api/research_trade/intent', req)
}

export function confirmResearchTradeIntent(intentId: number): Promise<ResearchTradeIntentIDResponse> {
  return postJSON('/api/research_trade/intent/confirm', { intent_id: intentId })
}

export function executeResearchTradeIntent(
  intentId: number,
): Promise<ExecuteResearchTradeIntentResponse> {
  return postJSON('/api/research_trade/intent/execute', { intent_id: intentId })
}
