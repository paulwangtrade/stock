/**
 * OpportunityProjection display helpers (Phase16-C2.1).
 * Formats GET /api/opportunities/projections for Drawer UI; every field carries data-source attribution.
 */
import { ORIGIN_EMPTY } from './tradePlanOriginDisplay.js'

export const PROJECTION_LAYER_SOURCE = {
  signal: 'SignalSnapshot',
  opportunity: 'CandidatePool',
  decision: 'TradePlanItem',
  trade_plan: 'TradePlan',
  portfolio: 'PortfolioSnapshot',
  research: 'ResearchCandidate',
}

export const PROJECTION_PIPELINE = [
  { key: 'signal', label: '发现', source: PROJECTION_LAYER_SOURCE.signal },
  { key: 'opportunity', label: '关注', source: PROJECTION_LAYER_SOURCE.opportunity },
  { key: 'decision', label: '决策', source: PROJECTION_LAYER_SOURCE.decision },
  { key: 'trade_plan', label: '计划', source: PROJECTION_LAYER_SOURCE.trade_plan },
  { key: 'portfolio', label: '持仓', source: PROJECTION_LAYER_SOURCE.portfolio },
]

export const DECISION_STATUS_LABEL = {
  BUY_CANDIDATE: '买入候选',
  WATCH: '观察',
  REJECT: '拒绝',
  NOT_IN_PLAN: '未入计划',
  UNKNOWN: '未知',
}

export const CANDIDATE_STATUS_LABEL = {
  IN_POOL: '在候选池',
  NOT_IN_POOL: '不在候选池',
  PLAN_PENDING: '计划待执行',
  PLAN_SKIPPED: '计划已跳过',
  PLAN_FILLED: '计划已成交',
}

export const HOLDING_STATUS_LABEL = {
  HELD: '已持仓',
  NOT_HELD: '未持仓',
}

export const PROJECTION_DISCLAIMER =
  '本面板为四层投资决策只读解释（Signal → Opportunity → Decision → TradePlan → Portfolio）。' +
  '机会页表格中的「趋势分」为前端估算，与 CandidatePool Score 无关。'

function str(v) {
  if (v == null) return ''
  return String(v).trim()
}

function formatMoney(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return ORIGIN_EMPTY
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatScore(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return ORIGIN_EMPTY
  return n.toFixed(4)
}

function formatRank(value) {
  const n = Math.trunc(Number(value))
  if (!Number.isFinite(n) || n <= 0) return ORIGIN_EMPTY
  return String(n)
}

function formatTime(value) {
  const s = str(value)
  if (!s) return ORIGIN_EMPTY
  return s.includes('T') ? s.replace('T', ' ').slice(0, 19) : s
}

function field(label, value, source, missing = false) {
  return {
    label,
    value: missing ? ORIGIN_EMPTY : value || ORIGIN_EMPTY,
    source,
    missing,
  }
}

function enumLabel(map, key, fallback = ORIGIN_EMPTY) {
  const k = str(key).toUpperCase()
  return map[k] || key || fallback
}

/** Whether a pipeline layer has meaningful data. */
export function isProjectionLayerActive(projection, key) {
  if (!projection || typeof projection !== 'object') return false
  switch (key) {
    case 'signal':
      return !!projection.signal?.present
    case 'opportunity':
      return !!projection.opportunity?.present
    case 'decision':
      return !!str(projection.decision?.decision_status)
    case 'trade_plan':
      return !!projection.trade_plan?.present
    case 'portfolio':
      return !!str(projection.portfolio?.holding_status)
    default:
      return false
  }
}

export function buildProjectionPipeline(projection) {
  return PROJECTION_PIPELINE.map((step) => ({
    ...step,
    active: isProjectionLayerActive(projection, step.key),
  }))
}

function buildSignalSection(projection) {
  const src = PROJECTION_LAYER_SOURCE.signal
  const sig = projection?.signal || {}
  const present = !!sig.present
  return {
    id: 'signal',
    title: '一、Signal（发现）',
    sourceBanner: `来源：${src}`,
    present,
    fields: [
      field('signal_tag', present ? str(sig.signal_tag) || ORIGIN_EMPTY : ORIGIN_EMPTY, src, !present || !str(sig.signal_tag)),
      field('signal_price', present ? formatMoney(sig.signal_price) : ORIGIN_EMPTY, src, !present || !Number.isFinite(Number(sig.signal_price))),
      field('signal_time', present ? formatTime(sig.signal_time) : ORIGIN_EMPTY, src, !present || !str(sig.signal_time)),
      field(
        'trigger_reason',
        present ? str(sig.trigger_reason) || ORIGIN_EMPTY : ORIGIN_EMPTY,
        src,
        !present || !str(sig.trigger_reason),
      ),
    ],
  }
}

function buildOpportunitySection(projection) {
  const src = PROJECTION_LAYER_SOURCE.opportunity
  const opp = projection?.opportunity || {}
  const present = !!opp.present
  const strategyName = str(opp.strategy_name) || str(opp.strategy_source)
  return {
    id: 'opportunity',
    title: '二、Opportunity（关注）',
    sourceBanner: `来源：${src}`,
    present,
    fields: [
      field(
        'pool_source',
        present ? str(opp.pool_source) || ORIGIN_EMPTY : ORIGIN_EMPTY,
        src,
        !present || !str(opp.pool_source),
      ),
      field(
        'strategy_name',
        present ? strategyName || ORIGIN_EMPTY : ORIGIN_EMPTY,
        src,
        !present || !strategyName,
      ),
      field('score', present ? formatScore(opp.score) : ORIGIN_EMPTY, src, !present || !Number.isFinite(Number(opp.score))),
      field('rank', present ? formatRank(opp.rank) : ORIGIN_EMPTY, src, !present || !(Math.trunc(Number(opp.rank)) > 0)),
      field(
        'strategy_source',
        present ? str(opp.strategy_source) || ORIGIN_EMPTY : ORIGIN_EMPTY,
        src,
        !present || !str(opp.strategy_source),
      ),
    ],
  }
}

function buildDecisionSection(projection) {
  const src = PROJECTION_LAYER_SOURCE.decision
  const dec = projection?.decision || {}
  return {
    id: 'decision',
    title: '三、Decision（决策）',
    sourceBanner: `来源：${src}`,
    present: !!str(dec.decision_status),
    fields: [
      field(
        'decision_status',
        enumLabel(DECISION_STATUS_LABEL, dec.decision_status),
        src,
        !str(dec.decision_status),
      ),
      field(
        'candidate_status',
        enumLabel(CANDIDATE_STATUS_LABEL, dec.candidate_status),
        src,
        !str(dec.candidate_status),
      ),
    ],
  }
}

function buildTradePlanSection(projection) {
  const src = PROJECTION_LAYER_SOURCE.trade_plan
  const plan = projection?.trade_plan || {}
  const present = !!plan.present
  return {
    id: 'trade_plan',
    title: '四、TradePlan（计划）',
    sourceBanner: `来源：${src}`,
    present,
    fields: [
      field(
        'plan_id',
        present && plan.plan_id ? String(plan.plan_id) : ORIGIN_EMPTY,
        src,
        !present || !(Math.trunc(Number(plan.plan_id)) > 0),
      ),
      field('plan_status', present ? str(plan.plan_status) || ORIGIN_EMPTY : ORIGIN_EMPTY, src, !present || !str(plan.plan_status)),
      field('trade_date', present ? str(plan.trade_date) || ORIGIN_EMPTY : ORIGIN_EMPTY, src, !present || !str(plan.trade_date)),
    ],
  }
}

function buildPortfolioSection(projection) {
  const src = PROJECTION_LAYER_SOURCE.portfolio
  const port = projection?.portfolio || {}
  return {
    id: 'portfolio',
    title: '五、Portfolio（持仓）',
    sourceBanner: `来源：${src}`,
    present: !!str(port.holding_status),
    fields: [
      field(
        'holding_status',
        enumLabel(HOLDING_STATUS_LABEL, port.holding_status),
        src,
        !str(port.holding_status),
      ),
    ],
  }
}

function buildResearchSection(projection) {
  const src = PROJECTION_LAYER_SOURCE.research
  const research = projection?.research
  const present = !!research && !!str(research.research_id)
  const tags = present && Array.isArray(research.tags) ? research.tags.filter(Boolean) : []
  return {
    id: 'research',
    title: '六、Research（研究标注）',
    sourceBanner: `来源：${src}`,
    present,
    optional: true,
    fields: [
      field(
        'research_tags',
        tags.length ? tags.join('、') : ORIGIN_EMPTY,
        src,
        !tags.length,
      ),
    ],
  }
}

/** Build drawer-ready view model from OpportunityProjection API payload. */
export function buildOpportunityProjectionView(projection) {
  if (!projection || typeof projection !== 'object') return null
  return {
    stockCode: str(projection.stock_code),
    stockName: str(projection.stock_name),
    tradeDate: str(projection.trade_date),
    quality: str(projection.metadata?.quality) || 'partial',
    pipeline: buildProjectionPipeline(projection),
    sections: [
      buildSignalSection(projection),
      buildOpportunitySection(projection),
      buildDecisionSection(projection),
      buildTradePlanSection(projection),
      buildPortfolioSection(projection),
      buildResearchSection(projection),
    ],
  }
}

export function projectionQualityTagType(quality) {
  return quality === 'complete' ? 'success' : 'warning'
}

export function projectionQualityLabel(quality) {
  return quality === 'complete' ? '解释完整' : '部分层缺失'
}

/** Human label for table field keys shown in drawer. */
export const PROJECTION_FIELD_LABEL = {
  signal_tag: '信号类型',
  signal_price: '信号价格',
  signal_time: '信号时间',
  trigger_reason: '发现依据 / 入选说明',
  pool_source: '发现来源',
  strategy_name: '策略名称',
  score: '评分',
  rank: '排名',
  strategy_source: '策略来源标识',
  decision_status: '决策状态',
  candidate_status: '候选状态',
  plan_id: '计划 ID',
  plan_status: '计划状态',
  trade_date: '计划交易日',
  holding_status: '持仓状态',
  research_tags: '研究标签',
}

export function projectionFieldLabel(key) {
  return PROJECTION_FIELD_LABEL[key] || key
}
