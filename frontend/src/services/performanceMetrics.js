/**
 * 前端性能埋点：TTI / 自选 / 快照 / 筛选。
 * 关闭：构建时 VITE_PERF_METRICS=0，或运行时 localStorage GOSTOCK_PERF=0 / window.__GOSTOCK_PERF__=false
 */

const MAX_METRICS = 200
const metrics = []

function readEnvFlag() {
  try {
    if (typeof window !== 'undefined' && window.__GOSTOCK_PERF__ === false) return false
    if (typeof window !== 'undefined' && window.__GOSTOCK_PERF__ === true) return true
    if (typeof localStorage !== 'undefined') {
      const ls = localStorage.getItem('GOSTOCK_PERF')
      if (ls === '0' || ls === 'false') return false
      if (ls === '1' || ls === 'true') return true
    }
    const env = typeof import.meta !== 'undefined' ? import.meta.env?.VITE_PERF_METRICS : undefined
    if (env === '0' || env === 'false') return false
    if (env === '1' || env === 'true') return true
    // 默认：开发开、生产关
    return !!(typeof import.meta !== 'undefined' && import.meta.env?.DEV)
  } catch {
    return false
  }
}

let perfEnabled = readEnvFlag()

export function isPerfMetricsEnabled() {
  return perfEnabled
}

export function setPerfMetricsEnabled(on) {
  perfEnabled = !!on
  try {
    if (typeof window !== 'undefined') window.__GOSTOCK_PERF__ = perfEnabled
  } catch {
    /* ignore */
  }
}

function logMetric(metric) {
  if (!perfEnabled) return
  try {
    // eslint-disable-next-line no-console
    console.log(`[Perf] ${metric.name}: ${metric.durationMs}ms`, metric.detail || '')
  } catch {
    /* ignore */
  }
}

/**
 * 记录轻量前端性能指标。仅保存在内存中，调用方可按需读取或上报。
 */
export function recordPerformanceMetric(name, durationMs, detail = {}) {
  if (!perfEnabled) return null
  const duration = Number(durationMs)
  if (!name || !Number.isFinite(duration)) return null
  const metric = {
    name: String(name),
    durationMs: Math.max(0, Math.round(duration * 100) / 100),
    at: Date.now(),
    detail,
  }
  metrics.push(metric)
  if (metrics.length > MAX_METRICS) metrics.splice(0, metrics.length - MAX_METRICS)
  logMetric(metric)
  try {
    if (typeof performance !== 'undefined' && performance.mark) {
      performance.mark(`gostock:${metric.name}:end`)
      const startName = `gostock:${metric.name}:start`
      if (performance.getEntriesByName(startName).length) {
        performance.measure(`gostock:${metric.name}`, startName, `gostock:${metric.name}:end`)
      }
    }
  } catch {
    /* ignore */
  }
  return metric
}

export function markPerformanceStart(name) {
  if (!perfEnabled || !name) return
  try {
    performance.mark(`gostock:${name}:start`)
  } catch {
    /* ignore */
  }
}

export function measurePerformance(name, detail = {}) {
  const startedAt = performance.now()
  markPerformanceStart(name)
  let done = false
  return () => {
    if (done) return null
    done = true
    return recordPerformanceMetric(name, performance.now() - startedAt, detail)
  }
}

export function getPerformanceMetrics(name) {
  const snapshot = name ? metrics.filter((item) => item.name === name) : metrics
  return snapshot.map((item) => ({ ...item, detail: { ...item.detail } }))
}

export function clearPerformanceMetrics() {
  metrics.length = 0
}

/** 导出 JSON，便于粘贴到分析工具 */
export function exportPerformanceMetricsJSON() {
  return JSON.stringify(getPerformanceMetrics(), null, 2)
}

export function downloadPerformanceMetrics(filename = `gostock-perf-${Date.now()}.json`) {
  if (typeof document === 'undefined') return
  const blob = new Blob([exportPerformanceMetricsJSON()], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

/** 应用启动起点（main.js 调用） */
export function markAppBootStart() {
  if (!perfEnabled) return
  try {
    performance.mark('gostock:app-boot:start')
    if (typeof window !== 'undefined') {
      window.__GOSTOCK_BOOT_START__ = performance.now()
    }
  } catch {
    /* ignore */
  }
}

let ttiRecorded = false

/** loading 结束 → TTI（幂等，只记一次） */
export function recordAppTTI(detail = {}) {
  if (!perfEnabled || ttiRecorded) return null
  ttiRecorded = true
  let duration = 0
  try {
    const boot = typeof window !== 'undefined' ? window.__GOSTOCK_BOOT_START__ : null
    if (Number.isFinite(boot)) {
      duration = performance.now() - boot
    } else if (performance.getEntriesByName('gostock:app-boot:start').length) {
      performance.mark('gostock:app-boot:end')
      performance.measure('gostock:tti', 'gostock:app-boot:start', 'gostock:app-boot:end')
      const entries = performance.getEntriesByName('gostock:tti')
      duration = entries.length ? entries[entries.length - 1].duration : 0
    }
  } catch {
    duration = 0
  }
  return recordPerformanceMetric('tti.app_interactive', duration, detail)
}
