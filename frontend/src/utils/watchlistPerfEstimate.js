/**
 * 自选加载优化耗时估算（基于架构路径，非真机采样）。
 * 场景假设：80 只自选，单次行情 HTTP ~400–800ms，SQLite GetFollowList ~40–80ms。
 */
export const WATCHLIST_PERF_BASELINE = {
  dbListMs: 60,
  quoteHttpMs: 600,
  renderFullDomMs: 180, // 80 卡片全量挂载
  renderVirtualDomMs: 45, // 仅可视行
}

/** 优化前：串行 await 列表 + 行情，再关 loading */
export function estimateBeforeMs(b = WATCHLIST_PERF_BASELINE) {
  return {
    timeToFirstListMs: b.dbListMs + b.quoteHttpMs, // 等行情才出列表
    timeToInteractiveMs: b.dbListMs + b.quoteHttpMs + b.renderFullDomMs,
    networkOnReenterMs: b.dbListMs + b.quoteHttpMs, // 每次重进都打满
    networkOnTabSwitchMs: b.dbListMs + b.quoteHttpMs,
    pollWhenHidden: true,
  }
}

/** ① SWR：先出 DB 列表 */
export function estimateAfterSWR(b = WATCHLIST_PERF_BASELINE) {
  return {
    timeToFirstListMs: b.dbListMs,
    timeToInteractiveMs: b.dbListMs + b.renderFullDomMs,
    savedFirstPaintMs: estimateBeforeMs(b).timeToFirstListMs - b.dbListMs,
  }
}

/** ② 行情进程缓存命中（5s TTL） */
export function estimateAfterQuoteCache(b = WATCHLIST_PERF_BASELINE) {
  return {
    reenterQuoteMs: 5, // 内存命中
    savedOnReenterMs: b.quoteHttpMs - 5,
  }
}

/** ③ 前端列表 30s 缓存 */
export function estimateAfterListCache(b = WATCHLIST_PERF_BASELINE) {
  return {
    tabSwitchListMs: 2,
    savedOnTabSwitchMs: b.dbListMs - 2,
  }
}

/** ④ 隐藏暂停轮询：隐藏期网络 ≈ 0 */
export function estimateAfterVisibilityPoll() {
  return {
    hiddenPollRequestsPerMinBefore: 20, // RefreshInterval≈3s
    hiddenPollRequestsPerMinAfter: 0,
  }
}

/** ⑤ 虚拟滚动 */
export function estimateAfterVirtual(b = WATCHLIST_PERF_BASELINE) {
  return {
    scrollFrameBudgetMsBefore: b.renderFullDomMs,
    scrollFrameBudgetMsAfter: b.renderVirtualDomMs,
    savedRenderMs: b.renderFullDomMs - b.renderVirtualDomMs,
  }
}

export function formatWatchlistPerfReport() {
  const b = WATCHLIST_PERF_BASELINE
  const before = estimateBeforeMs(b)
  const swr = estimateAfterSWR(b)
  const qc = estimateAfterQuoteCache(b)
  const lc = estimateAfterListCache(b)
  const vis = estimateAfterVisibilityPoll()
  const virt = estimateAfterVirtual(b)
  return {
    assumptions: b,
    before,
    items: [
      {
        name: 'SWR 模式',
        beforeMs: before.timeToFirstListMs,
        afterMs: swr.timeToFirstListMs,
        deltaMs: swr.savedFirstPaintMs,
        note: '首屏列表感知时间（不等行情）',
      },
      {
        name: '进程内行情缓存',
        beforeMs: b.quoteHttpMs,
        afterMs: qc.reenterQuoteMs,
        deltaMs: qc.savedOnReenterMs,
        note: '5s 内重复进入，行情请求',
      },
      {
        name: '前端列表缓存',
        beforeMs: b.dbListMs,
        afterMs: lc.tabSwitchListMs,
        deltaMs: lc.savedOnTabSwitchMs,
        note: '30s 内切 Tab 回来，GetFollowList',
      },
      {
        name: '行情轮询优化',
        beforeMs: vis.hiddenPollRequestsPerMinBefore,
        afterMs: vis.hiddenPollRequestsPerMinAfter,
        deltaMs: vis.hiddenPollRequestsPerMinBefore,
        note: '页面隐藏时每分钟行情请求次数',
        unit: 'req/min',
      },
      {
        name: '虚拟滚动',
        beforeMs: virt.scrollFrameBudgetMsBefore,
        afterMs: virt.scrollFrameBudgetMsAfter,
        deltaMs: virt.savedRenderMs,
        note: '80 只卡片初次布局/绘制量级',
      },
    ],
  }
}
