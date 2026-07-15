import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const read = (path) => readFile(new URL(`../${path}`, import.meta.url), 'utf8')

const [router, research, stock, market, service, scan, indexCache, statusBar, appWin, appVue, metrics] = await Promise.all([
  read('src/router/router.js'),
  read('src/components/researchIndex.vue'),
  read('src/components/stock.vue'),
  read('src/components/market.vue'),
  read('src/services/quantAutomationService.js'),
  read('src/utils/watchlistSignalScan.js'),
  read('src/utils/indexKlineCache.js'),
  read('src/utils/marketStatusBar.js'),
  read('../app_windows.go'),
  read('src/App.vue'),
  read('src/services/performanceMetrics.js'),
])

assert.doesNotMatch(router, /^import .*components\/.*\.vue/m, '路由不应静态导入页面组件')
assert.match(router, /component:\s*\(\)\s*=>\s*import\(/, '路由应使用动态 import')
assert.doesNotMatch(research, /display-directive="show"/, '研究页签不应预挂载隐藏面板')
assert.match(research, /display-directive="if"/, '研究页签应按需挂载')
assert.doesNotMatch(stock, /scanWatchlistSignals/, '自选页不应自行执行全量信号扫描')
assert.match(stock, /watchlistSignalsByCode/, '自选页应消费全局扫描快照')
assert.match(stock, /content-visibility:\s*auto/, '自选卡应启用屏外渲染虚拟化')
assert.match(stock, /document\.hidden \|\| route\.name !== 'stock'/, '自选页不可见时应跳过行情 UI 更新')
assert.match(service, /scanInFlight/, '全局扫描应具备 in-flight 去重')
assert.match(service, /visibilitychange/, '全局扫描应响应页面可见性')
assert.match(service, /measurePerformance\('quant\.scan'\)/, '应记录扫描耗时')
assert.match(scan, /DAILY_BARS_CACHE_TTL_MS/, '日 K 缓存应设置 TTL')
assert.match(scan, /getSharedIndexDailyKLine/, '信号扫描应共用指数日 K 缓存')
assert.match(statusBar, /getSharedIndexDailyKLine/, '市场状态栏应共用指数日 K 缓存')
assert.match(indexCache, /inflight/, '指数 K 线缓存应具备 in-flight 去重')
assert.match(market, /shouldPollMarket/, '市场页应在不可见时暂停昂贵轮询')
assert.match(market, /visibilitychange/, '市场页应监听页面可见性')
assert.match(appWin, /handleUpdateSettings/, '设置更新应走热更新/条件 reload')
assert.doesNotMatch(appWin, /runtime\.WindowReloadApp\(ctx\)\s*\n\s*\}/, 'Windows updateSettings 不应无条件 WindowReloadApp')
assert.match(appVue, /monitor_perf/, '壳层应接收 Monitor 耗时事件')
assert.match(metrics, /recordPerformanceMetric/, '应具备 performanceMetrics 记录入口')

console.log('前端性能与资源治理验证通过')
