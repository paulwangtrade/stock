/**
 * Promise 超时包装：用于 UI loading 兜底，避免 Wails/网络长时间不返回。
 * @param {Promise<T>} promise
 * @param {number} ms
 * @param {string} [label]
 * @returns {Promise<T>}
 * @template T
 */
export function withTimeout(promise, ms, label = 'request') {
  const timeoutMs = Number(ms)
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
    return promise
  }
  let timer = null
  const timeoutPromise = new Promise((_, reject) => {
    timer = setTimeout(() => {
      reject(new Error(`${label} timeout after ${timeoutMs}ms`))
    }, timeoutMs)
  })
  return Promise.race([promise, timeoutPromise]).finally(() => {
    if (timer != null) clearTimeout(timer)
  })
}
