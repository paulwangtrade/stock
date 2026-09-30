/** Shared read-only ExitWatch. Observation is not a sell order. */

import { EXIT_WATCH_DISCLAIMER } from '../utils/exitWatchDisplay.js'

export type ExitWatchItem = {
  source: string
  positionId: string
  stockCode: string
  stockName: string
  class: string
  label: string
  reasonCodes: string[]
  rule: string
  dedupKey: string
  bar: string
  policyRef: string
  asOf: string
  summary: string
  sellIntentAllowed: boolean
  requiresManualConfirm: boolean
  manualSellDraftRef: string
  notAnOrder: boolean
  disclaimer: string
}

export type ExitWatchSourceStatus = {
  source: string
  ok: boolean
  message: string
}

export type ExitWatchView = {
  items: ExitWatchItem[]
  sources: ExitWatchSourceStatus[]
  asOf: string
  policyRef: string
  disclaimer: string
  autoSell: boolean
  persistTrailing: boolean
  persistHwm: boolean
  newLedger: boolean
  writesTradePlan: boolean
  writesPaperSim: boolean
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function mapItem(raw: any): ExitWatchItem | null {
  if (!raw || typeof raw !== 'object') return null
  const source = str(raw.source).trim().toLowerCase()
  const klass = str(raw.class).trim()
  if (!source || !klass) return null
  const reasonCodes = Array.isArray(raw.reason_codes)
    ? raw.reason_codes.map((c: unknown) => str(c).trim()).filter(Boolean)
    : []
  return {
    source,
    positionId: str(raw.position_id).trim(),
    stockCode: str(raw.stock_code).trim(),
    stockName: str(raw.stock_name).trim(),
    class: klass,
    label: str(raw.label).trim(),
    reasonCodes,
    rule: str(raw.rule).trim(),
    dedupKey: str(raw.dedup_key).trim(),
    bar: str(raw.bar).trim(),
    policyRef: str(raw.policy_ref).trim(),
    asOf: str(raw.as_of).trim(),
    summary: str(raw.summary).trim(),
    sellIntentAllowed: raw.sell_intent_allowed === true && source === 'paper_sim',
    requiresManualConfirm: true,
    manualSellDraftRef: source === 'paper_sim' ? str(raw.manual_sell_draft_ref).trim() : '',
    notAnOrder: true,
    disclaimer: EXIT_WATCH_DISCLAIMER,
  }
}

export async function getExitWatch(source?: string): Promise<ExitWatchView> {
  const q = source && source !== 'all' ? `?source=${encodeURIComponent(source)}` : ''
  const res = await fetch(`/api/exit-watch${q}`)
  if (!res.ok) throw new Error(`退出观察请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '退出观察响应无效')
  const raw = body.exit_watch || body.exitWatch || {}
  const items = Array.isArray(raw.items) ? raw.items.map(mapItem).filter(Boolean) : []
  const sources = Array.isArray(raw.sources)
    ? raw.sources.map((s: any) => ({
        source: str(s?.source).trim(),
        ok: s?.ok === true,
        message: str(s?.message).trim(),
      }))
    : []
  return {
    items: items as ExitWatchItem[],
    sources,
    asOf: str(raw.as_of ?? raw.asOf).trim(),
    policyRef: str(raw.policy_ref ?? raw.policyRef).trim(),
    disclaimer: EXIT_WATCH_DISCLAIMER,
    autoSell: false,
    persistTrailing: false,
    persistHwm: false,
    newLedger: false,
    writesTradePlan: false,
    writesPaperSim: false,
  }
}
