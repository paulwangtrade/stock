/**
 * 自选页「今日模型观察」只读数据装配。
 * 优先 TradePlan upcoming（AfterClose CandidatePool → Draft 投影）；
 * 若无计划则回退研究页 CandidatePool API（signal snapshot，非交易池）。
 * 不写入自选、不改 CandidatePool。
 */

import { getUpcomingTradePlan } from '../api/tradePlans'
import { getCandidatePool } from '../api/candidatePool'

/**
 * @typedef {object} ModelObservationRow
 * @property {number} rank
 * @property {string} stockCode
 * @property {string} stockName
 * @property {string} source
 * @property {string} status
 * @property {string} origin  trade_plan | research_pool
 */

/**
 * @returns {Promise<{
 *   rows: ModelObservationRow[],
 *   tradeDate: string,
 *   planId: number,
 *   sourceLabel: string,
 *   emptyMessage: string,
 * }>}
 */
export async function loadModelObservation() {
  // 1) TradePlan upcoming（含 next_trading_day Draft / 今日 Frozen）
  try {
    const res = await getUpcomingTradePlan()
    const plan = res?.plan
    const items = Array.isArray(plan?.items) ? plan.items : []
    if (items.length) {
      const rows = items.map((it, i) => ({
        rank: Number(it.priority) > 0 ? Number(it.priority) : i + 1,
        stockCode: String(it.stock_code || '').trim(),
        stockName: String(it.stock_name || '').trim(),
        source: String(it.strategy_name || it.reason || plan.source_session || 'trade_plan').trim() || 'trade_plan',
        status: String(it.intent_status || it.status || 'pending').trim() || 'pending',
        origin: 'trade_plan',
      })).filter((r) => r.stockCode)
      rows.sort((a, b) => a.rank - b.rank)
      return {
        rows,
        tradeDate: String(plan.trade_date || res.trade_date || ''),
        planId: Number(plan.id || res.plan_id || 0),
        sourceLabel: 'TradePlan / CandidatePool 投影',
        emptyMessage: '',
      }
    }
  } catch (_) {
    /* fall through */
  }

  // 2) 研究候选池（只读展示；与交易 AfterClose 池不同源）
  try {
    const res = await getCandidatePool()
    const data = Array.isArray(res?.data) ? res.data : []
    if (data.length) {
      const rows = data.slice(0, 10).map((it, i) => ({
        rank: i + 1,
        stockCode: String(it.stock_code || '').trim(),
        stockName: String(it.stock_name || '').trim(),
        source: String(it.signal_tag || it.reason || 'research_pool').trim() || 'research_pool',
        status: String(it.direction || '观察').trim() || '观察',
        origin: 'research_pool',
      })).filter((r) => r.stockCode)
      return {
        rows,
        tradeDate: String(res.snapshot_time || '').slice(0, 10),
        planId: 0,
        sourceLabel: '研究 CandidatePool（signal snapshot）',
        emptyMessage: '',
      }
    }
  } catch (_) {
    /* fall through */
  }

  return {
    rows: [],
    tradeDate: '',
    planId: 0,
    sourceLabel: '',
    emptyMessage: '暂无模型观察数据（无 Upcoming TradePlan / 研究候选池）',
  }
}

/** @param {ModelObservationRow[]} rows */
export function modelObservationCodeSet(rows) {
  return new Set((rows || []).map((r) => String(r.stockCode || '').trim().toLowerCase()).filter(Boolean))
}
