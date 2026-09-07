/**
 * Phase16.17-A runtime acceptance probe (not product code).
 * Simulates chart loadData path: getOrFetch + IPC-like fetcher.
 */
import {
  clearKlineCache,
  getOrFetch,
  getKlineCacheStats,
  klineCacheKey,
  getCachedKline,
} from '../frontend/src/utils/klineCache.js'

function now() {
  return performance.now()
}

async function main() {
  clearKlineCache()
  const symbol = '600519.sh'
  const results = []

  async function timedFetch(label, klt, fetcherDelayMs = 80) {
    let ipc = 0
    const key = klineCacheKey(symbol, klt)
    const t0 = now()
    const data = await getOrFetch(
      key,
      async () => {
        ipc += 1
        await new Promise((r) => setTimeout(r, fetcherDelayMs))
        return [{ day: '2026-09-03', close: 100, klt }]
      },
      5 * 60 * 1000,
      { shouldCache: (d) => Array.isArray(d) && d.length > 0 },
    )
    const ms = now() - t0
    const kind = ipc === 0 ? (getCachedKline(key) ? 'hit' : 'unknown') : 'miss'
    // refine: if ipc>0 miss; else hit (or pending shared)
    results.push({
      label,
      klt,
      ms: Number(ms.toFixed(1)),
      ipc,
      kind: ipc > 0 ? 'miss' : 'hit',
      bars: data.length,
    })
    return data
  }

  // Test1 first open daily
  await timedFetch('T1_first_daily', '101', 120)
  // Test2 reopen daily
  await timedFetch('T2_reopen_daily', '101', 120)
  // Test3 multi TF first
  await timedFetch('T3_first_weekly', '102', 100)
  await timedFetch('T3_first_monthly', '103', 100)
  // Test3 second pass
  await timedFetch('T3_second_daily', '101', 100)
  await timedFetch('T3_second_weekly', '102', 100)
  await timedFetch('T3_second_monthly', '103', 100)

  // Test4 in-flight
  clearKlineCache()
  let ipcParallel = 0
  const keyP = klineCacheKey(symbol, '101')
  const slow = async () => {
    ipcParallel += 1
    await new Promise((r) => setTimeout(r, 150))
    return [{ close: 1 }]
  }
  const tP0 = now()
  const [a, b] = await Promise.all([
    getOrFetch(keyP, slow),
    getOrFetch(keyP, slow),
  ])
  const parallelMs = now() - tP0

  // Exception: error not cached
  clearKlineCache()
  const errKey = klineCacheKey('fail.sz', '101')
  let threw = false
  try {
    await getOrFetch(errKey, async () => {
      throw new Error('ipc_fail')
    })
  } catch {
    threw = true
  }
  const errCached = getCachedKline(errKey)
  const pendingAfterErr = getKlineCacheStats().pending

  // empty not cached
  clearKlineCache()
  const emptyKey = klineCacheKey('empty.sz', '101')
  await getOrFetch(emptyKey, async () => [], undefined, {
    shouldCache: (d) => Array.isArray(d) && d.length > 0,
  })
  const emptyCached = getCachedKline(emptyKey)

  const out = {
    results,
    parallel: { ipc: ipcParallel, ms: Number(parallelMs.toFixed(1)), same: a === b || JSON.stringify(a) === JSON.stringify(b) },
    exception: { threw, errCached, pendingAfterErr, emptyCached },
  }
  console.log(JSON.stringify(out, null, 2))

  const t1 = results.find((r) => r.label === 'T1_first_daily')
  const t2 = results.find((r) => r.label === 'T2_reopen_daily')
  const pass =
    t1?.kind === 'miss' &&
    t1.ipc === 1 &&
    t2?.kind === 'hit' &&
    t2.ipc === 0 &&
    t2.ms < t1.ms &&
    results.filter((r) => r.label.startsWith('T3_second_')).every((r) => r.kind === 'hit') &&
    ipcParallel === 1 &&
    threw &&
    errCached == null &&
    pendingAfterErr === 0 &&
    emptyCached == null

  if (!pass) {
    console.error('ACCEPT_FAIL')
    process.exit(1)
  }
  console.log('ACCEPT_PASS')
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
