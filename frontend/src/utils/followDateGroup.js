/** 关注按日期分组 + 过期归并月组 + 空组清理 */

import {
  AddGroup,
  AddStockGroup,
  GetFollowList,
  GetGroupList,
  GetGroupStockList,
  RemoveGroup,
  RemoveStockGroup,
} from '../../wailsjs/go/main/App'
import { data } from '../../wailsjs/go/models'
import { getFollowDateGroupSettings } from './signalSettingsStore'
import { invalidateFollowListCache } from './watchlistFollowListCache.js'

export const DATE_GROUP_RE = /^\d{4}-\d{2}-\d{2}$/
export const MONTH_GROUP_RE = /^\d{4}-\d{2}$/
export const LEGACY_SIGNAL_GROUP_NAMES = new Set(['弹'])

let maintenancePromise = null

export function isDateGroupName(name) {
  return DATE_GROUP_RE.test(String(name || '').trim())
}

export function isMonthArchiveGroupName(name) {
  const n = String(name || '').trim()
  if (!MONTH_GROUP_RE.test(n)) return false
  const parts = n.split('-')
  const month = Number(parts[1])
  return parts.length === 2 && month >= 1 && month <= 12
}

export function isAutoManagedGroupName(name) {
  const n = String(name || '').trim()
  return isDateGroupName(n) || isMonthArchiveGroupName(n) || LEGACY_SIGNAL_GROUP_NAMES.has(n)
}

export function formatFollowDateGroupName(date = new Date()) {
  const d = date instanceof Date ? date : new Date(date)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function parseDateGroupName(name) {
  const m = String(name || '').trim().match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!m) return null
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  return Number.isNaN(d.getTime()) ? null : d
}

export function formatMonthArchiveGroupName(date) {
  const d = date instanceof Date ? date : parseDateGroupName(String(date))
  if (!d) return null
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

function startOfDay(d) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

function daysBetween(a, b) {
  return Math.round((startOfDay(b) - startOfDay(a)) / 86400000)
}

function groupStockCode(row) {
  return String(row?.stockCode || row?.StockCode || '').trim().toLowerCase()
}

export function sortWatchlistGroups(groups) {
  const list = [...(groups || [])]
  return list.sort((a, b) => {
    const aDate = parseDateGroupName(a.name)
    const bDate = parseDateGroupName(b.name)
    if (aDate && bDate) return bDate - aDate

    const aMonth = isMonthArchiveGroupName(a.name)
    const bMonth = isMonthArchiveGroupName(b.name)
    if (aMonth && bMonth) return String(b.name).localeCompare(String(a.name))
    if (aDate && !bDate && !bMonth) return -1
    if (bDate && !aDate && !aMonth) return 1
    if (aMonth && !bMonth && !isDateGroupName(b.name)) return -1
    if (bMonth && !aMonth && !isDateGroupName(a.name)) return 1

    const sa = Number(a.sort) || 0
    const sb = Number(b.sort) || 0
    if (sa !== sb) return sa - sb
    return String(a.name || '').localeCompare(String(b.name || ''), 'zh-CN')
  })
}

async function ensureGroup(groupName) {
  const list = await GetGroupList()
  const found = (list || []).find((g) => g.name === groupName)
  if (found?.ID) return found.ID

  const maxSort = (list || []).reduce((m, g) => Math.max(m, g.sort || 0), 0)
  const group = data.Group.createFrom({ name: groupName, sort: maxSort + 1 })
  const result = await AddGroup(group)
  if (result !== '添加成功') {
    throw new Error(result || '创建分组失败')
  }

  const refreshed = await GetGroupList()
  const created = (refreshed || []).find((g) => g.name === groupName)
  if (!created?.ID) throw new Error('创建分组后未找到')
  return created.ID
}

/** 关注成功后加入当天日期分组 */
export async function addStockToFollowDateGroup(stockCode, date = new Date()) {
  const settings = getFollowDateGroupSettings()
  if (!settings.enabled) return null

  const groupName = formatFollowDateGroupName(date)
  const groupId = await ensureGroup(groupName)
  const addResult = await AddStockGroup(groupId, stockCode)
  await runDateGroupMaintenance()
  return { groupName, groupId, addResult }
}

/** Follow + 日期分组（仅「关注成功」时进组） */
export async function followWithDateGroup(code, followFn) {
  const r = await followFn(code)
  let groupInfo = null
  if (r === '关注成功') {
    invalidateFollowListCache()
    try {
      groupInfo = await addStockToFollowDateGroup(code)
    } catch (err) {
      groupInfo = { error: err?.message || String(err) }
    }
  }
  return { followResult: r, groupInfo }
}

export function formatFollowGroupMessage(followResult, groupInfo) {
  if (followResult !== '关注成功') return followResult
  if (groupInfo?.error) return `关注成功，但加入日期分组失败：${groupInfo.error}`
  if (groupInfo?.groupName && groupInfo?.addResult === '添加成功') {
    return `关注成功，已加入「${groupInfo.groupName}」分组`
  }
  if (groupInfo?.groupName) return `关注成功，已在「${groupInfo.groupName}」分组`
  return '关注成功'
}

async function archiveExpiredDateGroups(retainDays) {
  const today = new Date()
  const groups = await GetGroupList()
  for (const g of groups || []) {
    if (!isDateGroupName(g.name)) continue
    const groupDate = parseDateGroupName(g.name)
    if (!groupDate) continue
    if (daysBetween(groupDate, today) <= retainDays) continue

    const monthName = formatMonthArchiveGroupName(groupDate)
    if (!monthName) continue
    const monthId = await ensureGroup(monthName)

    const stocks = await GetGroupStockList(g.ID)
    for (const row of stocks || []) {
      const code = groupStockCode(row)
      if (!code) continue
      await AddStockGroup(monthId, code)
      await RemoveStockGroup(code, '', g.ID)
    }
    await RemoveGroup(g.ID)
  }
}

async function cleanupEmptyAutoGroups() {
  const groups = await GetGroupList()
  for (const g of groups || []) {
    if (!isAutoManagedGroupName(g.name)) continue
    const active = await GetFollowList(g.ID)
    if (!active?.length) {
      await RemoveGroup(g.ID)
    }
  }
}

async function runDateGroupMaintenanceInternal() {
  const settings = getFollowDateGroupSettings()
  if (!settings.enabled) {
    await cleanupEmptyAutoGroups()
    return sortWatchlistGroups(await GetGroupList())
  }
  await archiveExpiredDateGroups(settings.retainDays)
  await cleanupEmptyAutoGroups()
  return sortWatchlistGroups(await GetGroupList())
}

/** 过期归并 + 空组清理 + 排序（并发去重） */
export async function runDateGroupMaintenance() {
  if (maintenancePromise) return maintenancePromise
  maintenancePromise = runDateGroupMaintenanceInternal()
    .catch((err) => {
      console.error('[runDateGroupMaintenance]', err)
      return GetGroupList()
    })
    .finally(() => {
      maintenancePromise = null
    })
  return maintenancePromise
}

export async function refreshWatchlistGroups() {
  const sorted = await runDateGroupMaintenance()
  return sorted || []
}
