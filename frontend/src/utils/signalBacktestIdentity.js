/**
 * 单票对照 / 信号回测共用的本地身份解析。
 * 代码能唯一对应名称时带上名称；名称必须唯一，否则要求挑选；
 * 本地库没有这条代码时仍允许按代码继续（不编造名称）。
 */
import { toEastMoneyCode } from './stockCode.js'

const A_SHARE_CODE = /^(?:(?:sh|sz|bj)\d{6}|\d{6}(?:\.(?:sh|sz|bj))?)$/i

export function normalizeIdentityQuery(raw) {
  return String(raw || '').replace(/\s+/g, '').trim()
}

export function isStockCodeQuery(raw) {
  return A_SHARE_CODE.test(normalizeIdentityQuery(raw))
}

function field(row, keys) {
  if (!row || typeof row !== 'object') return ''
  for (const key of keys) {
    const value = row[key]
    if (value != null && String(value).trim()) return String(value).trim()
  }
  return ''
}

/** 东财展示代码；解析不出 6 位代码时返回空串，避免把名称当成代码。 */
export function canonicalStockCode(row) {
  const ts = field(row, ['ts_code', 'TsCode', 'SECUCODE', 'secucode', 'code'])
  const symbol = field(row, ['symbol', 'Symbol', 'SECURITY_CODE', 'symbolCode'])
  const candidates = []
  if (ts) candidates.push(ts)
  if (/^\d{6}$/.test(symbol)) candidates.push(symbol)
  for (const item of candidates) {
    if (!isStockCodeQuery(item) && !/^\d{6}/.test(item)) continue
    const em = toEastMoneyCode(item)
    if (/^\d{6}\.(SH|SZ|BJ)$/.test(em)) return em
  }
  return ''
}

export function stockDisplayName(row) {
  const name = field(row, ['name', 'Name', 'SECURITY_NAME_ABBR', 'fullname'])
  if (!name) return ''
  if (isStockCodeQuery(name)) return ''
  if (/^\d{6}\.(SH|SZ|BJ|HK)$/i.test(name)) return ''
  return name
}

function codeKeys(code) {
  const raw = String(code || '').trim()
  const keys = new Set()
  if (!raw) return keys
  keys.add(raw.toLowerCase())
  const em = toEastMoneyCode(raw)
  if (/^\d{6}\.(SH|SZ|BJ)$/.test(em)) {
    keys.add(em.toLowerCase())
    keys.add(em.slice(0, 6))
    const prefix = em.endsWith('.SH') ? 'sh' : em.endsWith('.SZ') ? 'sz' : 'bj'
    keys.add(prefix + em.slice(0, 6))
  }
  return keys
}

function sameCode(a, b) {
  const left = codeKeys(a)
  const right = codeKeys(b)
  if (!left.size || !right.size) return false
  for (const key of left) {
    if (right.has(key)) return true
  }
  return false
}

function toChoice(row) {
  const code = canonicalStockCode(row)
  const name = stockDisplayName(row)
  if (!code && !name) return null
  return {
    code,
    name,
    label: name && code ? `${name}（${code}）` : name || code,
  }
}

function uniqueByCode(choices) {
  const out = []
  for (const choice of choices) {
    if (!choice) continue
    if (out.some((item) => (choice.code && item.code && sameCode(item.code, choice.code)) || (!choice.code && item.name === choice.name))) {
      continue
    }
    out.push(choice)
  }
  return out
}

/**
 * @returns {{ status: 'resolved'|'ambiguous'|'code_only'|'missing', code: string, name: string, candidates: Array<{code:string,name:string,label:string}> }}
 */
export function resolveStockIdentity(query, candidates) {
  const q = normalizeIdentityQuery(query)
  const list = uniqueByCode((Array.isArray(candidates) ? candidates : []).map(toChoice).filter(Boolean))
  if (!q) {
    return { status: 'missing', code: '', name: '', candidates: [] }
  }

  if (isStockCodeQuery(q)) {
    const exact = list.filter((item) => item.code && sameCode(item.code, q))
    if (exact.length === 1) {
      return { status: 'resolved', code: exact[0].code, name: exact[0].name || '', candidates: exact }
    }
    if (exact.length > 1) {
      return { status: 'ambiguous', code: '', name: '', candidates: exact }
    }
    const em = toEastMoneyCode(q)
    const code = /^\d{6}\.(SH|SZ|BJ)$/.test(em) ? em : q.toUpperCase()
    return { status: 'code_only', code, name: '', candidates: [] }
  }

  const exactName = list.filter((item) => item.name === q)
  if (exactName.length === 1) {
    return {
      status: 'resolved',
      code: exactName[0].code,
      name: exactName[0].name,
      candidates: exactName,
    }
  }
  if (exactName.length > 1) {
    return { status: 'ambiguous', code: '', name: '', candidates: exactName }
  }
  if (list.length === 1 && list[0].code) {
    return { status: 'resolved', code: list[0].code, name: list[0].name || '', candidates: list }
  }
  if (list.length > 1) {
    return { status: 'ambiguous', code: '', name: '', candidates: list }
  }
  return { status: 'missing', code: '', name: '', candidates: [] }
}
