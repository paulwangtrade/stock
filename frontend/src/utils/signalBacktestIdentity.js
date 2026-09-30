/**
 * 信号回测输入：代码 / 名称 → 唯一股票。
 * 只做本地候选消歧，不触发交易。
 */

function compact(value) {
  return String(value || '').replace(/\s+/g, '').trim()
}

function compactKey(value) {
  return compact(value).toLowerCase()
}

function sixDigits(value) {
  const matched = String(value || '').match(/(\d{6})/)
  return matched ? matched[1] : ''
}

/** 6 位代码、sh/sz/bj/hk 前缀，或 688137.SH 这类交易所后缀。 */
export function looksLikeStockCode(value) {
  return /^(?:(?:sh|sz|bj|hk)\d{6}|\d{6}(?:\.(?:sh|sz|bj|hk|ss))?)$/i.test(compact(value))
}

export function normalizeStockBasics(list) {
  const rows = []
  const seen = new Map()
  for (const item of list || []) {
    const name = String(item?.name ?? item?.Name ?? '').trim()
    const tsCode = String(item?.ts_code ?? item?.TsCode ?? item?.tsCode ?? '').trim()
    let symbol = String(item?.symbol ?? item?.Symbol ?? '').trim()
    if (!/^\d{6}$/.test(symbol)) symbol = sixDigits(tsCode) || (/^\d{6}$/.test(symbol) ? symbol : '')
    if (!name && !symbol && !tsCode) continue
    const klineCode = /^\d{6}$/.test(symbol) ? symbol : (symbol || tsCode)
    const key = symbol || tsCode || compactKey(name)
    const row = {
      name,
      symbol,
      tsCode,
      klineCode,
      label: [name, symbol || tsCode].filter(Boolean).join(' '),
    }
    const prev = seen.get(key)
    if (!prev) {
      seen.set(key, row)
      rows.push(row)
    } else if (!prev.name && row.name) {
      Object.assign(prev, row)
    }
  }
  return rows
}

function isExactCode(query, row) {
  const q = compactKey(query)
  const symbol = compactKey(row.symbol)
  const ts = compactKey(row.tsCode).replace(/\.ss$/, '.sh')
  if (symbol && (q === symbol || q === `sh${symbol}` || q === `sz${symbol}` || q === `bj${symbol}` || q === `hk${symbol}`)) {
    return true
  }
  if (ts && q === ts) return true
  const digits = q.match(/^(\d{6})(?:\.(?:sh|sz|bj|hk|ss))?$/)
  return !!(digits && symbol === digits[1])
}

function isExactName(query, row) {
  const q = compactKey(query)
  return !!q && compactKey(row.name) === q
}

function isFuzzyHit(query, row) {
  const q = compactKey(query)
  if (!q) return false
  if (row.name && compactKey(row.name).includes(q)) return true
  if (row.symbol && row.symbol.toLowerCase().startsWith(q)) return true
  if (row.tsCode && row.tsCode.toLowerCase().startsWith(q)) return true
  return false
}

/**
 * 「盛帮股份 301233」这类展示串。纯代码、纯名称返回 null。
 * @returns {{ name: string, code: string } | null}
 */
export function displayNameCode(raw) {
  const query = compact(raw)
  if (!query || looksLikeStockCode(query)) return null
  const matched = query.match(/^(.*?)(\d{6})(.*)$/)
  if (!matched) return null
  const name = `${matched[1]}${matched[3]}`
  if (!name) return null
  return { name, code: matched[2] }
}

function displayRowMatches(query, row) {
  const parsed = displayNameCode(query)
  if (!parsed || !isExactCode(parsed.code, row)) return false
  return compactKey(row.label) === compactKey(query) || isExactName(parsed.name, row)
}

/** 带名称的展示串用其中的代码去查本地库，避免整句被当成未知名称。 */
export function stockSearchKey(raw) {
  const query = String(raw || '').trim()
  return displayNameCode(query)?.code || query
}

function pack(status, hit, matches, extra = {}) {
  return {
    status,
    name: hit?.name || '',
    symbol: hit?.symbol || '',
    tsCode: hit?.tsCode || '',
    klineCode: hit?.klineCode || hit?.symbol || '',
    label: hit?.label || '',
    matches,
    matchCount: extra.matchCount ?? matches.length,
    message: extra.message || '',
  }
}

/**
 * @param {string} raw 用户输入的代码或名称
 * @param {Array<object>} basics GetStockList 一类的候选
 * @returns {{status:'empty'|'unique'|'code_only'|'ambiguous'|'unknown', klineCode:string, name:string, symbol:string, matches:object[], message:string}}
 */
export function resolveStockIdentity(raw, basics) {
  const query = compact(raw)
  if (!query) {
    return pack('empty', null, [], { message: '' })
  }
  const rows = normalizeStockBasics(basics)
  const exactCode = rows.filter((row) => isExactCode(query, row))
  if (exactCode.length === 1) return pack('unique', exactCode[0], exactCode)
  if (exactCode.length > 1) {
    return pack('ambiguous', null, exactCode.slice(0, 8), {
      matchCount: exactCode.length,
      message: '这个代码对应多只股票，请点选一只',
    })
  }

  if (looksLikeStockCode(query)) {
    const symbol = sixDigits(query)
    return {
      status: 'code_only',
      name: '',
      symbol,
      tsCode: '',
      klineCode: symbol || query,
      label: symbol || query,
      matches: [],
      matchCount: 0,
      message: '本地股票库没有这只代码的名称，仍可按代码回测',
    }
  }

  const exactName = rows.filter((row) => isExactName(query, row))
  if (exactName.length === 1) return pack('unique', exactName[0], exactName)
  if (exactName.length > 1) {
    return pack('ambiguous', null, exactName.slice(0, 8), {
      matchCount: exactName.length,
      message: exactName.length > 8
        ? `「${query}」对应 ${exactName.length} 只股票，请点选或改用代码`
        : `「${query}」对应多只股票，请点选一只`,
    })
  }

  const parsed = displayNameCode(query)
  if (parsed) {
    const picked = rows.filter((row) => displayRowMatches(query, row))
    if (picked.length === 1) return pack('unique', picked[0], picked)
    return pack('unknown', null, [], {
      message: `没有找到「${query}」，请输入 6 位代码，或换一个更完整的名称`,
    })
  }

  const fuzzy = rows.filter((row) => isFuzzyHit(query, row))
  if (fuzzy.length === 1 && compact(query).length >= 2) return pack('unique', fuzzy[0], fuzzy)
  if (fuzzy.length >= 1) {
    return pack('ambiguous', null, fuzzy.slice(0, 8), {
      matchCount: fuzzy.length,
      message: fuzzy.length > 8
        ? `找到 ${fuzzy.length} 只相关股票，请点选或改用代码`
        : '找到多只股票，请点选一只',
    })
  }

  return pack('unknown', null, [], {
    message: `没有找到「${query}」，请输入 6 位代码，或换一个更完整的名称`,
  })
}

function acceptedKeysFor(row) {
  const code = row?.symbol || row?.klineCode || ''
  return [compactKey(code), compactKey(row?.label), compactKey(`${row?.name || ''}${code}`)].filter(Boolean)
}

/**
 * 点选自动完成建议。输入框随后会变成 option.label（名称 + 代码），
 * 这里直接收下该条的代码，而不是把展示串再当成一次自由搜索。
 */
export function acceptStockSuggestion(resolved, selected) {
  const picked = compact(typeof selected === 'object' && selected
    ? (selected.value || selected.label || '')
    : selected)
  if (!picked) return null
  const row = (resolved?.matches || []).find((item) => {
    const code = item.symbol || item.klineCode
    return code === picked || compact(item.label) === picked
  })
  const code = row?.symbol || row?.klineCode || ''
  if (!row || !code) return null
  return {
    ...pack('unique', row, [row]),
    acceptedKeys: acceptedKeysFor(row),
  }
}

/** 当前输入仍是已解析代码，或仍是该条建议的展示串。 */
export function queryMatchesResolvedStock(resolved, raw) {
  const key = compactKey(raw)
  if (!key || !resolved) return false
  if (Array.isArray(resolved.acceptedKeys) && resolved.acceptedKeys.includes(key)) return true
  const code = compactKey(resolved.symbol || resolved.klineCode)
  const label = compactKey(resolved.label)
  if (resolved.status === 'unique' && (key === code || (label && key === label))) return true
  if (resolved.status === 'code_only' && code && key === code) return true
  return false
}

export function identityAllowsRun(resolved) {
  return resolved?.status === 'unique' || resolved?.status === 'code_only'
}

export function toStockSuggestions(resolved, limit = 8) {
  return (resolved?.matches || []).slice(0, limit).map((row) => ({
    label: row.label,
    value: row.symbol || row.klineCode,
  }))
}
