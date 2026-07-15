/** 新浪/应用内代码 → 东方财富格式（如 600519.SH） */
export function toEastMoneyCode(code) {
  if (!code) return ''
  let c = String(code).trim()
  if (!c || c.toLowerCase().startsWith('gb_')) return ''

  // 东财内部 secid：1.601101 / 0.000001
  const secidMatch = c.match(/^(\d+)\.(\d{6,})$/)
  if (secidMatch) {
    const marketNo = secidMatch[1]
    const num = secidMatch[2]
    if (marketNo === '1') return `${num}.SH`
    if (marketNo === '0') return `${num}.SZ`
    if (marketNo === '128') return `${num}.HK`
  }

  if (/\.(SH|SZ|BJ|HK|SS)$/i.test(c)) {
    return c.replace(/\.SS$/i, '.SH').toUpperCase()
  }

  const lower = c.toLowerCase()
  if (/^(sh|sz|bj|hk)\d{6}$/.test(lower)) {
    const prefix = lower.slice(0, 2)
    const num = lower.slice(2)
    if (prefix === 'sh') return `${num}.SH`
    if (prefix === 'sz') return `${num}.SZ`
    if (prefix === 'bj') return `${num}.BJ`
    if (prefix === 'hk') return `${num.toUpperCase()}.HK`
  }

  if (/^\d{6}$/.test(c)) {
    const d = c[0]
    if (d === '6') return `${c}.SH`
    if (d === '0' || d === '3') return `${c}.SZ`
    if (d === '4' || d === '8' || d === '9') return `${c}.BJ`
    return `${c}.SZ`
  }

  return c.toUpperCase()
}

/** 从交易所名称推断后缀 */
function marketSuffixFromRow(row) {
  const ms = String(row.MARKET_SHORT_NAME || row.MARKET_CODE || '').trim().toUpperCase()
  if (ms === 'SH' || ms === 'SS') return 'SH'
  if (ms === 'SZ') return 'SZ'
  if (ms === 'BJ') return 'BJ'
  const m = String(row.MARKET || row['交易所'] || '').trim()
  if (/上|沪/.test(m)) return 'SH'
  if (/深/.test(m)) return 'SZ'
  if (/北/.test(m)) return 'BJ'
  return ''
}

/** 从行内扫描 6 位数字代码 */
function pickSixDigitCode(row) {
  if (!row || typeof row !== 'object') return ''
  for (const v of Object.values(row)) {
    if (typeof v === 'string' && /^\d{6}$/.test(v.trim())) return v.trim()
  }
  return ''
}

/** 策略结果行 → 东财 K 线代码 */
export function resolveStrategyRowCode(row) {
  if (!row) return ''

  const sc = String(row.SECURITY_CODE || row.SECURITYCODE || row['股票代码'] || '').trim()

  // 自然语言选股：MARKET_SHORT_NAME 多为 sh / sz
  const msRaw = String(row.MARKET_SHORT_NAME || '').trim().toLowerCase()
  if (sc && /^(sh|sz|bj|hk)$/.test(msRaw)) {
    return toEastMoneyCode(msRaw + sc)
  }

  if (row.SECUCODE) {
    const em = toEastMoneyCode(String(row.SECUCODE))
    if (em) return em
  }

  if (row.DERIVE_SECURITY_CODE) {
    const em = toEastMoneyCode(String(row.DERIVE_SECURITY_CODE))
    if (em) return em
  }

  const suffix = marketSuffixFromRow(row)
  if (sc && suffix) {
    return `${sc}.${suffix}`
  }

  if (sc) {
    return toEastMoneyCode(sc)
  }

  const six = pickSixDigitCode(row)
  if (six) return toEastMoneyCode(six)

  const raw = row.code || row['代码']
  return raw ? toEastMoneyCode(String(raw)) : ''
}

/** 生成 K 线请求可尝试的代码变体（提高命中率） */
export function eastMoneyCodeVariants(code) {
  const out = []
  const add = (c) => {
    const n = toEastMoneyCode(c)
    if (n && !out.includes(n)) out.push(n)
  }
  add(code)
  const c = String(code || '').trim().toUpperCase()
  if (/^\d{6}\.(SH|SZ|BJ)$/.test(c)) {
    add(c.split('.')[0])
  }
  if (/^(SH|SZ|BJ)\d{6}$/.test(c)) {
    add(c.toLowerCase())
  }
  return out
}

export function resolveStrategyRowName(row) {
  return (
    row.SECURITY_SHORT_NAME ||
    row.SECURITY_NAME_ABBR ||
    row['股票名称'] ||
    row.SECURITY_NAME ||
    row.name ||
    ''
  )
}

export function toFollowCodeFromRow(row) {
  const em = resolveStrategyRowCode(row)
  if (!em) return ''
  const c = em.toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3)
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3)
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3)
  if (c.endsWith('.HK')) return 'hk' + c.slice(0, -3).toLowerCase()
  return em.toLowerCase()
}
