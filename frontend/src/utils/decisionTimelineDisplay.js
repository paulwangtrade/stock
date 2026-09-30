/**
 * Decision timeline display helpers.
 * Observation only — labels describe past evidence, not trade actions.
 */

export const DECISION_TIMELINE_DISCLAIMER = '回顾用，不是买卖指令'

export const LANE_FILTERS = [
  { id: 'all', label: '全部' },
  { id: 'observe', label: '观察' },
  { id: 'entry', label: '入场记录' },
  { id: 'holding', label: '持有记录' },
  { id: 'exit', label: '离场记录' },
]

const LANE_LABEL = {
  observe: '观察',
  entry: '入场',
  holding: '持有',
  exit: '离场',
}

const KIND_LABEL = {
  signal: '信号',
  watch: '关注',
  follow: '自选',
  paper_sim: '模拟盘',
  external_mirror: '外部镜像',
  outcome: '结果备注',
}

const SOURCE_LABEL = {
  signal_snapshot: '信号快照',
  watch_follow: '关注/自选',
  paper_sim: '模拟盘',
  external_mirror: '外部镜像',
  outcome_note: '结果备注',
}

const STATUS_LABEL = {
  ok: '有记录',
  empty: '没有记录',
  unavailable: '不可用',
}

export function laneLabel(lane) {
  return LANE_LABEL[lane] || '记录'
}

export function kindLabel(kind) {
  return KIND_LABEL[kind] || '记录'
}

export function sourceLabel(source) {
  return SOURCE_LABEL[source] || source || '来源'
}

export function sourceStatusLabel(status) {
  return STATUS_LABEL[status] || status || ''
}

export function emptyTimelineMessage(timeline) {
  if (!timeline || !Array.isArray(timeline.events) || timeline.events.length === 0) {
    return '这条股票还没有可回顾的信号、关注或结果记录。缺历史时这里保持空白，不会补写买卖指令。'
  }
  return ''
}

export function filterTimelineEvents(events, lane) {
  const list = Array.isArray(events) ? events : []
  if (!lane || lane === 'all') return list
  return list.filter((ev) => ev && ev.lane === lane)
}

export function canOpenKline(event) {
  return /^\d{4}-\d{2}-\d{2}$/.test(String(event?.kline_date || ''))
}

export function sourceTagType(status) {
  if (status === 'ok') return 'success'
  if (status === 'unavailable') return 'warning'
  return 'default'
}
