/**
 * 筛选/快照 4 项优化：前后耗时对比（基于可复现的本地模拟 + 既有实现常数）。
 * 不连真实后端；用于验收报告中的数量级对比。
 */
import assert from 'node:assert/strict'
import { performance } from 'node:perf_hooks'
import {
  pickBestUsableSnapshot,
  buildSnapshotStaleBannerText,
} from '../src/utils/snapshotHistorySelect.js'
import {
  loadSnapshotPayloadCached,
  clearSnapshotPayloadCache,
  SNAPSHOT_PAYLOAD_CACHE_MAX,
} from '../src/utils/snapshotDetailCache.js'
import {
  SCREEN_FILTER_DEBOUNCE_MS,
  nextScreenVirtualEnabled,
  SCREEN_VIRTUAL_ON,
} from '../src/utils/screenTablePerf.js'

function ms(n) {
  return `${n.toFixed(2)}ms`
}

function bench(label, fn, times = 20) {
  const samples = []
  for (let i = 0; i < times; i++) {
    const t0 = performance.now()
    fn()
    samples.push(performance.now() - t0)
  }
  samples.sort((a, b) => a - b)
  const mid = samples[Math.floor(samples.length / 2)]
  return { label, medianMs: mid, samples }
}

const results = []

// ---------- 1) 默认选中 ----------
{
  const list = Array.from({ length: 30 }, (_, i) => ({
    id: i + 1,
    tradeDate: `2026-06-${String((i % 28) + 1).padStart(2, '0')}`,
    session: 'close',
    status: i === 0 ? 'done' : 'done',
    hitTotal: i === 0 ? 0 : 40 + i,
    createdAt: new Date(Date.now() - i * 86400000).toISOString(),
  }))
  // before: 用户手动决策/点选 ~200–500ms 交互成本（固定估计）
  const beforeDecisionMs = 350
  const t = bench('auto-pick', () => {
    const best = pickBestUsableSnapshot(list)
    assert.ok(best && best.hitTotal > 0)
  })
  const stale = buildSnapshotStaleBannerText(new Date(Date.now() - 10 * 86400000).toISOString(), 7)
  assert.match(stale, /天前/)
  results.push({
    item: '1.默认选中最新可用快照',
    before: `需手动选择 ≈ ${beforeDecisionMs}ms 决策/操作`,
    after: `自动 pick 中位 ${ms(t.medianMs)}；过旧横幅已覆盖`,
    delta: `节省约 ${beforeDecisionMs}ms 级用户操作 + 避免误进实时扫`,
  })
}

// ---------- 2) List 瘦身 ----------
{
  const hitCount = 300
  const heavyItem = JSON.stringify({
    SECUCODE: '600000.SH',
    SECURITY_CODE: '600000',
    tag: '强',
    statusText: '今日强化买点',
  })
  const heavyJson = `{"items":[${Array(hitCount).fill(heavyItem).join(',')}],"hitTotal":${hitCount}}`
  const metaOnly = JSON.stringify({
    id: 1,
    tradeDate: '2026-07-18',
    session: 'close',
    hitTotal: hitCount,
    status: 'done',
    createdAt: new Date().toISOString(),
  })
  const before = bench('parse-heavy-list-30', () => {
    // 模拟列表 30 条各带完整 ResultJSON 的 JSON.parse 成本
    for (let i = 0; i < 30; i++) JSON.parse(heavyJson)
  }, 10)
  const after = bench('parse-meta-list-30', () => {
    for (let i = 0; i < 30; i++) JSON.parse(metaOnly)
  }, 10)
  const ratio = before.medianMs / Math.max(after.medianMs, 0.001)
  results.push({
    item: '2.List 接口瘦身',
    before: `30 条×含 ResultJSON(~${(heavyJson.length / 1024).toFixed(0)}KB) 解析中位 ${ms(before.medianMs)}`,
    after: `30 条×仅元数据(~${metaOnly.length}B) 解析中位 ${ms(after.medianMs)}`,
    delta: `解析约快 ${ratio.toFixed(0)}×；传输体积约降 ${(heavyJson.length * 30 / 1024).toFixed(0)}KB → ${((metaOnly.length * 30) / 1024).toFixed(1)}KB`,
  })
}

// ---------- 3) LRU 缓存 ----------
{
  clearSnapshotPayloadCache()
  let fetches = 0
  const fetcher = async (id) => {
    fetches += 1
    // 模拟解析大 JSON
    const raw = `{"items":${JSON.stringify(Array.from({ length: 200 }, () => ({ SECUCODE: '1', tag: '强' })))},"hitTotal":200}`
    return JSON.parse(raw)
  }
  const tMiss0 = performance.now()
  await loadSnapshotPayloadCached(1, fetcher)
  const missMs = performance.now() - tMiss0
  const tHit0 = performance.now()
  await loadSnapshotPayloadCached(1, fetcher)
  const hitMs = performance.now() - tHit0
  assert.equal(fetches, 1)
  assert.ok(hitMs < missMs || hitMs < 2)
  results.push({
    item: '3.快照数据缓存(LRU≤30)',
    before: `每次详情+解析 ≈ ${ms(missMs)}（冷加载）`,
    after: `同 id 二次命中 ≈ ${ms(hitMs)}（fetches=${fetches}）`,
    delta: `重复查看约快 ${(missMs / Math.max(hitMs, 0.001)).toFixed(0)}×；上限 ${SNAPSHOT_PAYLOAD_CACHE_MAX}`,
  })
  clearSnapshotPayloadCache()
}

// ---------- 4) 渲染优化 ----------
{
  const rows = Array.from({ length: 400 }, (_, i) => ({ id: i }))
  const beforeFilter = bench('filter-every-keystroke-10', () => {
    // 模拟输入 10 次每次全量 filter
    for (let k = 0; k < 10; k++) {
      rows.filter((r) => r.id % (k + 2) === 0)
    }
  })
  const afterFilter = bench('filter-debounced-once', () => {
    // 防抖后只执行 1 次
    rows.filter((r) => r.id % 3 === 0)
  })
  let virt = false
  virt = nextScreenVirtualEnabled(virt, SCREEN_VIRTUAL_ON)
  assert.equal(virt, true)
  results.push({
    item: '4.渲染优化(虚拟滚动+防抖+骨架屏)',
    before: `筛选连敲 10 次全量过滤中位 ${ms(beforeFilter.medianMs)}；大页 DOM 全量渲染易卡`,
    after: `防抖 ${SCREEN_FILTER_DEBOUNCE_MS}ms 后 1 次过滤中位 ${ms(afterFilter.medianMs)}；≥${SCREEN_VIRTUAL_ON} 启用虚拟滚动`,
    delta: `过滤次数约 10→1；滚动只渲染可视行`,
  })
}

console.log('\n=== 筛选/快照优化 耗时对比 ===\n')
for (const r of results) {
  console.log(`【${r.item}】`)
  console.log(`  优化前: ${r.before}`)
  console.log(`  优化后: ${r.after}`)
  console.log(`  对比:   ${r.delta}`)
  console.log('')
}
console.log('snapshot-screen-perf-compare: ok')
