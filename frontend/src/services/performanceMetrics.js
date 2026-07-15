const MAX_METRICS = 100
const metrics = []

/**
 * 记录轻量前端性能指标。仅保存在内存中，调用方可按需读取或上报。
 */
export function recordPerformanceMetric(name, durationMs, detail = {}) {
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
  return metric
}

export function measurePerformance(name, detail = {}) {
  const startedAt = performance.now()
  return () => recordPerformanceMetric(name, performance.now() - startedAt, detail)
}

export function getPerformanceMetrics(name) {
  const snapshot = name ? metrics.filter((item) => item.name === name) : metrics
  return snapshot.map((item) => ({ ...item, detail: { ...item.detail } }))
}

export function clearPerformanceMetrics() {
  metrics.length = 0
}
