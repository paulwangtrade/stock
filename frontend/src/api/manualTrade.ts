/** 模拟盘人工交易 Intent HTTP API（Phase3-PR4） */

export type ManualTradeIntentIDData = {
  id: number
  status: string
}

export type CreateManualTradeIntentRequest = {
  account_id?: number
  symbol: string
  stock_name?: string
  side: 'buy' | 'sell' | string
  price: number
  volume: number
  reason?: string
  order_kind?: string
}

export type ExecuteManualTradeIntentData = {
  status: string
  order_id?: number
  client_order_id?: string
  error_code?: string
  error_message?: string
}

type ManualTradeEnvelope<T> = {
  code: number
  data?: T
  error?: string
}

async function postManualTrade<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  })
  const text = await res.text()
  let env: ManualTradeEnvelope<T> | null = null
  try {
    env = text ? JSON.parse(text) : null
  } catch {
    env = null
  }
  if (!res.ok) {
    const msg = env?.error || (env as any)?.message || `请求失败: HTTP ${res.status}`
    throw new Error(msg)
  }
  if (env && typeof env.code === 'number' && env.code !== 0) {
    throw new Error(env.error || `请求失败: code ${env.code}`)
  }
  if (env && 'data' in env) {
    return env.data as T
  }
  // 兼容无包装
  return env as unknown as T
}

export function createManualTradeIntent(
  req: CreateManualTradeIntentRequest,
): Promise<ManualTradeIntentIDData> {
  return postManualTrade('/api/manual_trade/intent', {
    ...req,
    order_kind: req.order_kind || 'normal',
  })
}

export function confirmManualTradeIntent(intentId: number): Promise<ManualTradeIntentIDData> {
  return postManualTrade('/api/manual_trade/intent/confirm', { intent_id: intentId })
}

export function executeManualTradeIntent(intentId: number): Promise<ExecuteManualTradeIntentData> {
  return postManualTrade('/api/manual_trade/intent/execute', { intent_id: intentId })
}
