/** Broker Reconcile 只读 HTTP API（Phase6.5.9.2.1.3.3） */

import { mapBrokerReconcileView } from './brokerReconcileMap.js'

export {
  DIVERGENCE_KINDS,
  countDivergencesByKind,
  mapBrokerReconcileView,
  mapDivergence,
} from './brokerReconcileMap.js'

export type BrokerReconcileDivergence = {
  kind: string
  order_id: string
  detail: string
  severity: string
}

export type BrokerReconcileView = {
  status: string
  checked_at: string
  total_orders: number
  divergence_count: number
  divergences: BrokerReconcileDivergence[]
  by_kind: Record<string, number>
  observation_error: string
}

/**
 * GET /api/execution/broker-reconcile
 * 直接返回 BrokerReconcileView（无 code/ok 信封）。
 * 只读：不触发 repair / retry / cancel / 交易。
 */
export async function getBrokerReconcile(): Promise<BrokerReconcileView> {
  const res = await fetch('/api/execution/broker-reconcile')
  if (!res.ok) {
    throw new Error(`Broker Reconcile 请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const view = mapBrokerReconcileView(body)
  if (!view || !view.status) {
    throw new Error('Broker Reconcile 响应无效')
  }
  return view as BrokerReconcileView
}
