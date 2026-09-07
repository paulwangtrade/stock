# PHASE17-B.1 K 线加载性能优化 — 实现完成

**日期：** 2026-09-06  
**依据：** [PHASE17_B1_KLINE_OPTIMIZATION_DESIGN.md](./PHASE17_B1_KLINE_OPTIMIZATION_DESIGN.md)  
**范围：** **P1 数据加载策略 only**（barCount 命中 · LIVE/IDLE TTL · 停 IDLE poll）  
**未做：** 第三层缓存 / schema / 行情接口 / K 线模型 / 指标 / 交易链 / Modal keep-alive（P2）

---

## 1. 修改文件

| 文件 | 原因 |
| --- | --- |
| `backend/data/kline_cache.go` | LIVE/IDLE TTL；`latest` 允许 `barCount < limit` 仍命中并 trim |
| `backend/data/kline_cache_test.go` | TTL/假 miss/LIVE·IDLE 单测 |
| `backend/data/eastmoney_kline_api.go` | IDLE 且有本地 stale → 直接返回，不主动 HTTP latest |
| `frontend/src/utils/aShareSessionClock.js` | **新增** 连续竞价时钟（无重依赖，供 Node selftest / klineCache） |
| `frontend/src/utils/tradingSession.js` | 复用并 re-export `isAShareMarketOpenNow` / `isKlineMarketLive` |
| `frontend/src/utils/klineCache.js` | LIVE 5min / IDLE 30min 内存 TTL |
| `frontend/src/utils/chartMarkers.klineCache.selftest.mjs` | TTL / LIVE·IDLE 断言 |
| `frontend/src/components/StockLightweightKlineChart.vue` | IDLE 停 60s poll；每分钟重评；指数日 K 走 `getOrFetch` |

**禁止项确认：** 未新建缓存介质；未改 DB schema；未改东财接口契约；未改 K 线结构 / 指标算法 / 交易执行链。

---

## 2. 行为摘要

### 2.1 缓存命中（假 miss）

对 `end_key=latest`：TTL 内且 `len(bars)>0` → **命中**，`trim` 到请求 limit（不足则返回全部）。  
历史 `endKey` 仍要求足够根数（分页语义不变）。

### 2.2 LIVE / IDLE

| | LIVE（连续竞价） | IDLE（午休/盘后/周末/节假日） |
| --- | --- | --- |
| 后端 latest TTL（日 101） | 60s | 盘后 6h / 非交易日 24h |
| 后端 latest TTL（分钟） | 30s | 2h / 周末 24h |
| 前端内存 TTL | 5min | 30min |
| 弹窗 60s poll | **开** | **关**（每分钟重评跨盘） |
| TTL miss 且有本地 bars | 增量/全量 latest（保持） | **直接返回 stale，不打 HTTP** |

市场状态：后端 `tradingcalendar.IsTradingDay` + 上海 09:30–11:30 / 13:00–15:00；前端 `aShareSessionClock`（与 `tradingSession` 同源语义）。

### 2.3 左拖补充

历史分页仍走 `GetStockEastMoneyKLinePage`（非 latest），不受本次 latest 放宽影响。

---

## 3. 单测结果

### 3.1 Go

```text
go test ./backend/data/ -count=1 -run "TestKLineCache|TestKlineCache|TestNormalizeKLine|TestMergeKLine"
→ ok  go-stock/backend/data
```

覆盖：

- `TestKLineCacheGetLatestAllowsFewerBarsThanLimit` — 缓存 5 根、limit 800 → 命中（场景 4）
- `TestKlineCacheTTLAtLiveVsIdle` — LIVE 60s / 周末 IDLE 24h（场景 1 TTL 档）
- `TestKLineCacheTTLExpired` — 超过 IDLE 长 TTL 后 miss + stale 可读

### 3.2 前端 selftest

```text
node src/utils/chartMarkers.klineCache.selftest.mjs
→ chartMarkers.klineCache.selftest: OK
```

含 LIVE/IDLE 内存 TTL 断言。

---

## 4. Build 结果

```text
Set-Location D:\stock\frontend; npm run build
→ vite build exit 0（client production）
```

说明：实现过程中 D: 盘曾满（`build/` 约 43GB 已清理）；`App.vue` 曾被截断，已 `git checkout` 恢复，未纳入本切片功能改动。完整 `wails build` 未强制执行（本切片前端 + Go 包测为主）；需要桌面包时请先关 `go-stock.exe` 再 `wails build`。

---

## 5. Runtime 验证（场景对照）

验收日为 **2026-09-06（周日）**，盘中 LIVE 需下一交易日人工点验；逻辑侧已由单测/代码路径覆盖：

| 场景 | 预期 | 验证方式 | 结果 |
| --- | --- | --- | --- |
| 1 交易日盘中 | K 线仍可刷新（短 TTL + poll） | `klineMarketSessionLive` / TTL 60s 单测；`setupPoll` LIVE 分支 | **逻辑 PASS**（盘中实机待下一交易日） |
| 2 收盘后打开 | 有缓存则不等待远端 | IDLE stale 短路 + 长 TTL + FE 30min | **逻辑 PASS** |
| 3 周末 | 不产生 60s poll | `isKlineMarketLive()===false` → 不 `setInterval(refreshLatestPoll)`；仅 60s gate 重评 | **逻辑 PASS**（当日周末可实机：打开 K 线不应周期性 IPC latest） |
| 4 缓存不足 limit | 不假 miss | `TestKLineCacheGetLatestAllowsFewerBarsThanLimit` | **PASS** |

### 建议实机 checklist（下一交易日 10:00 / 15:30）

1. 盘中打开 K 线 → DevTools/日志可见约 60s 一次 refresh（或网络 latest）。  
2. 收盘后二次打开同股日 K → 应秒开（SQLite/内存 hit），无东财全量等待。  
3. 周末打开 → 无周期性 GetStockEastMoneyKLine（gate timer 除外）。

---

## 6. 结论

Phase17-B.1 **P1 已落地**：假 miss 消除、IDLE 停 poll + 长 TTL、LIVE 保持短 TTL/轮询。  
**可进入**下一观察切片；P2 图表实例复用仍按设计暂缓。
