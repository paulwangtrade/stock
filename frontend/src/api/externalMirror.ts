/** 实盘镜像观察 CRUD。只读写 source=external_mirror，不写 paper_sim。 */

import { EXTERNAL_MIRROR_DISCLAIMER, EXTERNAL_MIRROR_SOURCE } from '../utils/externalMirrorEntry.js'

export type ExternalMirrorHolding = {
  id: number
  source: string
  stockCode: string
  stockName: string
  quantity: number
  costPrice: number
  entryDate: string
  note: string
  feedsTradePlan: boolean
  tradable: boolean
  updatedAt: string
}

export type ExternalMirrorDraft = {
  stockCode: string
  stockName?: string
  quantity: number
  costPrice: number
  entryDate?: string
  note?: string
}

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

export function mapMirrorHolding(raw: any): ExternalMirrorHolding | null {
  if (!raw || typeof raw !== 'object') return null
  const id = num(raw.id, 0)
  const stockCode = str(raw.stockCode ?? raw.stock_code).trim()
  if (!id || !stockCode) return null
  const source = str(raw.source, EXTERNAL_MIRROR_SOURCE).trim().toLowerCase()
  if (source !== EXTERNAL_MIRROR_SOURCE) return null
  return {
    id,
    source: EXTERNAL_MIRROR_SOURCE,
    stockCode,
    stockName: str(raw.stockName ?? raw.stock_name).trim(),
    quantity: num(raw.quantity),
    costPrice: num(raw.costPrice ?? raw.cost_price),
    entryDate: str(raw.entryDate ?? raw.entry_date),
    note: str(raw.note),
    feedsTradePlan: false,
    tradable: false,
    updatedAt: str(raw.updatedAt ?? raw.updated_at),
  }
}

async function readBody(res: Response): Promise<any> {
  const text = await res.text()
  if (!text) return {}
  try {
    return JSON.parse(text)
  } catch {
    throw new Error(`实盘镜像响应无效: HTTP ${res.status}`)
  }
}

function failMessage(body: any, res: Response): string {
  return body?.message || `实盘镜像请求失败: HTTP ${res.status}`
}

export async function listExternalMirrorHoldings(): Promise<{
  holdings: ExternalMirrorHolding[]
  disclaimer: string
}> {
  const res = await fetch('/api/external-mirror/holdings')
  const body = await readBody(res)
  if (!res.ok || !body?.ok) throw new Error(failMessage(body, res))
  const holdings = (Array.isArray(body.holdings) ? body.holdings : [])
    .map(mapMirrorHolding)
    .filter((row: ExternalMirrorHolding | null): row is ExternalMirrorHolding => !!row)
  return {
    holdings,
    disclaimer: str(body.disclaimer, EXTERNAL_MIRROR_DISCLAIMER) || EXTERNAL_MIRROR_DISCLAIMER,
  }
}

export async function createExternalMirrorHolding(draft: ExternalMirrorDraft): Promise<ExternalMirrorHolding> {
  const res = await fetch('/api/external-mirror/holdings', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      stockCode: draft.stockCode,
      stockName: draft.stockName || '',
      quantity: draft.quantity,
      costPrice: draft.costPrice,
      entryDate: draft.entryDate || '',
      note: draft.note || '',
      source: EXTERNAL_MIRROR_SOURCE,
      feedsTradePlan: false,
      tradable: false,
    }),
  })
  const body = await readBody(res)
  if (!res.ok || !body?.ok) throw new Error(failMessage(body, res))
  const row = mapMirrorHolding(body.holding)
  if (!row) throw new Error('实盘镜像响应无效')
  return row
}

export async function updateExternalMirrorHolding(
  id: number,
  draft: ExternalMirrorDraft,
): Promise<ExternalMirrorHolding> {
  const res = await fetch(`/api/external-mirror/holdings/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      quantity: draft.quantity,
      costPrice: draft.costPrice,
      entryDate: draft.entryDate || '',
      stockName: draft.stockName || '',
      note: draft.note || '',
      source: EXTERNAL_MIRROR_SOURCE,
      feedsTradePlan: false,
      tradable: false,
    }),
  })
  const body = await readBody(res)
  if (!res.ok || !body?.ok) throw new Error(failMessage(body, res))
  const row = mapMirrorHolding(body.holding)
  if (!row) throw new Error('实盘镜像响应无效')
  return row
}

export async function deleteExternalMirrorHolding(id: number): Promise<void> {
  const res = await fetch(`/api/external-mirror/holdings/${id}`, { method: 'DELETE' })
  const body = await readBody(res)
  if (!res.ok || !body?.ok) throw new Error(failMessage(body, res))
}
