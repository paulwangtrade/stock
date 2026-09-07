# PHASE17.1 全系统多周期 K 线架构与数据链审计（只读）

**日期：** 2026-09-07  
**阶段：** Phase17.1 K 线可靠性专项  
**性质：** 只读架构审计（**未改代码 / 未清缓存 / 未删库 / 未调 TTL / 未改 API / 未修复**）  
**现象对齐：**  
- 入口 A 自选股 → 多周期 K 最新停 **2026-09-04**  
- 入口 B 我的跟踪机会 → 银之杰 → 多周期 K 最新停 **2026-09-04**  
- 「生成快照」后再开 → 出现 **2026-09-07**  
**关联：** [PHASE17_1_KLINE_INTRADAY_FRESHNESS_AUDIT.md](./PHASE17_1_KLINE_INTRADAY_FRESHNESS_AUDIT.md) · [PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md](./PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md)

---

## 0. 总判（先读）

| 问题 | 结论 |
| --- | --- |
| 个股「多周期K线」主产品入口是否共用一套 Modal/Chart？ | **是** — 几乎全部经 **`StockKlineModal` → `StockLightweightKlineChart`** |
| 是否同一条后端数据链？ | **是** — Wails `GetStockEastMoneyKLine*` → `MarketDataService.GetBars` → `EastMoneyKlineAdapter` → `EastMoneyKLineApi.GetKLineDataBefore` → SQLite **`kline_cache`**（东财主源，腾讯/新浪为失败 fallback） |
| A/B 入口差异是否造成本 bug？ | **否（E 排除）** — 同组件、同 API、同缓存；两边同时停在 09-04 符合「公共链新鲜度」而非入口接线错误 |
| 快照后出现 09-07 | **B. 隐藏副作用**（扫描强拉 K 并 `saveKLineCacheResult`），**不是**用户可见的正规「刷新 K 线」产品机制；若依赖快照才更新 → 接近 **C. 错误依赖** |
| 根因 | **B 后端缓存（主）+ C 交易日判断缺失（主）+ A 前端缓存（次）+ D 刷新机制在 IDLE/FE 被短路（次）** |

---

## 一、全系统 K 线入口盘点

### 1.1 个股「多周期K线」主入口（`StockKlineModal`）

| 入口 | 页面 / 组件 | 点击位置 | 调用组件 | 主要参数 |
| --- | --- | --- | --- | --- |
| **1 自选股列表** | `stock.vue` | 股票行 / 打开多周期（`showLightweightKline` → `openLightweightKlineModal`） | **`StockKlineModal`** | `code=lwKlineCode`（东财码）、`stockName`、可选策略信号 props |
| **2 我的跟踪机会** | `WatchedOpportunities.vue` | 「查看详情」→ `applyStockClickAction` | **`StockKlineModal`** | `code=klineModal.chartCode`、`stockName` |
| **3 我的组合持仓** | `PortfolioDashboard.vue` | 持仓行 StockLink / 开 K | **`StockKlineModal`** | `chartCode` + B.2：`costPrice` / `costVolume` / footer 卖出插槽 |
| **4 机会 / 选股** | `allStockList.vue` | 行「K线」/ StockLink / 信号点（`showKline`） | **`StockKlineModal`** | `chartCode` / `stockCode`、可选 `focusSignal` |
| **5 投资驾驶舱** | `InvestmentHome.vue` | 今日机会「K线」等 | **`StockKlineModal`** | `chartCode`、`stockName` |
| **6 研究候选池** | `ResearchCandidatePool.vue` | 「多周期K线」/ StockLink | **`StockKlineModal`** | `chartCode` |
| **7 机会投影抽屉** | `OpportunityProjectionDrawer.vue` | 股票链 | **`StockKlineModal`** | `chartCode` |
| **8 交易计划** | `TradePlanUpcoming.vue`（及说明表经父级） | StockLink | **`StockKlineModal`** | `chartCode` |
| **9 策略管理** | `stockStrategyManager.vue` | 开 K 对比 | **`StockKlineModal`** | code / name |
| **10 交易记录** | `TradingRecordManager.vue` | 行「K线」`openKlineChart` | **`StockKlineModal`** | code |
| **11 热股 / 排行 / 龙虎榜 / AI 荐股** | `HotStockList` / `rankTable` / `LongTigerRankList` / `aiRecommendStocksList` | 股票名链接 `openKline` | **`StockKlineModal`** | code / name |

**确认：** 上表个股多周期入口 **最终都进入同一个组件类型 `StockKlineModal`**（各页各自挂载一份实例，**非全局单例**，但组件与内嵌 Chart **同源**）。

### 1.2 旁路 / 非「多周期 Modal」入口（需知晓，非 A/B 现象主链）

| 入口 | 组件 | 与公共链关系 |
| --- | --- | --- |
| 市场指数面板 | `market.vue` **直接**嵌 `StockLightweightKlineChart`（无 Modal） | **同 Chart + 同 API**；指数码 |
| 持仓做 T 观察台 | `HoldingTPanel.vue` | **同** `GetStockEastMoneyKLine` + FE `klineCache`；**自绘 SVG**，**不经** `StockKlineModal` |
| 自选「日K」旧弹窗 | `stock.vue` `modalShow2` 等 | **ECharts 本地拼图**，非 Lightweight 多周期 |
| 研报/公告/行业资金等 | `KLineChart.vue` | **`GetStockKLine`**（另一套旧接口），ECharts |
| 列表火花线 | `stockSparkLine.vue` | 微型走势，非多周期工作台 |
| 信号回测 / 策略扫自选 | `SignalBacktestPanel` / `watchlistSignalScan.js` 等 | 直调 `GetStockEastMoneyKLine`，无 Modal |

→ **用户所述 A/B 均落在 §1.1 公共 Modal 链**；旁路不解释「两边同时停 09-04」。

---

## 二、前端组件链审计

### 2.1 调用关系图（个股多周期主链）

```text
[自选 / 跟踪机会 / 组合 / 机会 / 首页 / …]
        │  open / StockLink / showKline / applyStockClickAction
        ▼
┌───────────────────────┐
│   StockKlineModal     │  ← 各页各挂载；契约唯一身份 prop: code
│   (n-modal + 插槽)    │
└───────────┬───────────┘
            │ embedChart=true（默认）
            ▼
┌───────────────────────────────┐
│ StockLightweightKlineChart    │  多周期 / 指标 / LIVE poll / B.2 价位线
└───────────┬───────────────────┘
            │ loadData / refreshLatestPoll
            │ fetchEastMoneyKlineCached
            ▼
┌───────────────────────────────┐
│ utils/klineCache.js           │  进程内 Map 内存缓存
│ getOrFetch(klineCacheKey)     │
└───────────┬───────────────────┘
            │ Wails
            ▼
   GetStockEastMoneyKLine / Page
```

### 2.2 三项确认

| # | 问题 | 结论 |
| --- | --- | --- |
| 1 | 是否所有**个股多周期**入口共用 `StockKlineModal`？ | **是**（§1.1）；指数页可绕过 Modal 直嵌 Chart |
| 2 | 是否共用 `StockLightweightKlineChart`？ | **是**（Modal 内嵌；`market.vue` 亦直嵌同组件） |
| 3 | 是否存在多个 Modal / 老 K 线 / 特殊绕过？ | **有旁路**：`KLineChart`（旧 ECharts+`GetStockKLine`）、自选旧日 K Modal、`HoldingTPanel` SVG、火花线 — **均非 A/B 主路径** |

**不存在**「跟踪机会用另一套多周期引擎」的架构分裂；入口差异 **不能** 作为本新鲜度 bug 的主因。

---

## 三、前端缓存审计

**文件：** `frontend/src/utils/klineCache.js`  
**类型：** **仅内存 `Map`**（非 localStorage / 非 Pinia store）；进程级。

### 3.1 Key 设计

| 维度 | 是否包含 | 说明 |
| --- | --- | --- |
| stock_code / symbol | **是** | `symbol` 小写，常含市场前缀（`sh`/`sz`） |
| market 独立字段 | **否** | 一般编码进 symbol |
| period / timeframe | **是** | `klt`（如 `101`） |
| adjust type | **否** | FE key **不含**复权；后端 cache key **含** `adjust_flag` |
| 日历日 | **是** | 上海当日 `YYYY-MM-DD`（换日换 key） |

`klineCacheKey(symbol, timeframe, date?)` → `` `${sym}|${tf}|${d}` ``

### 3.2 缓存内容

| 字段 | 是否保存 |
| --- | --- |
| bars 全量 payload | **是**（`data`） |
| `expiresAt` | **是**（写入时 TTL） |
| 最后一根 K 线日期 | **否**（不单独存；只在 payload 内） |
| 更新时间 / 数据来源 | **否** |

### 3.3 命中逻辑与「09-04 vs 09-07」

| TTL | 值 |
| --- | --- |
| LIVE（`isKlineMarketLive`） | **5 min** |
| IDLE | **30 min** |

命中条件：同 key 且 `Date.now() <= expiresAt`。**不读**末 bar 交易日。

| 场景 | 是否仍直接返回 09-04 包？ |
| --- | --- |
| 同日（09-07）内，IDLE 写入止于 09-04，30min 未过 | **会** |
| LIVE poll | **仍走 `getOrFetch`** → 未过期则 **不请求后端** |
| 跨自然日 | key 含日期 → 新 key miss（换日可缓解，**不解决同日追赶**） |

→ 前端可放大「后端已落后仍展示」的窗口；**根因不单在 FE**。

---

## 四、后端 K 线服务审计

### 4.1 个股多周期主请求链

```text
StockLightweightKlineChart
  → GetStockEastMoneyKLine(code, name, klt, limit)
       → GetStockEastMoneyKLinePage(..., end="")
            → app.go: a.marketData.GetBars(...)
                 → EastMoneyKlineAdapter.GetBars
                      → EastMoneyKLineApi.GetKLineDataBefore(..., end latest)
                           ├── SQLite kline_cache
                           └── HTTP 东财 push2his …/kline/get
                                └── 失败：腾讯 / 新浪 fallback（同函数内）
```

**结论：** 所有 §1.1 入口调用 **同一 Wails API 族**；经 MD-001 `MarketDataService`，底层仍是 **同一** `EastMoneyKLineApi` + **`kline_cache`**（无第二层 facade 缓存）。

### 4.2 API 列表（与本专项相关）

| 路径 / 入口 | 函数 | 数据来源 | 缓存位置 |
| --- | --- | --- | --- |
| Wails | `App.GetStockEastMoneyKLine` | → Page | — |
| Wails | `App.GetStockEastMoneyKLinePage` | `marketData.GetBars` | 见下 |
| `marketdata` | `CompositeMarketDataService.GetBars` | `EastMoneyKlineAdapter` | **无二层缓存** |
| Adapter | `EastMoneyKlineAdapter.GetBars` | `GetKLineDataBefore` | — |
| data | `EastMoneyKLineApi.GetKLineDataBefore` | 东财 HTTP；失败腾讯/新浪 | **`kline_cache` 表** |
| data | `GetKLineData` / `GetKLineData2` | 同上 `end=20500101` | 同上 |
| 信号扫描 | `SignalScanApi.prepareStockBars` → `kline.GetKLineData` | **同** EastMoney API | **同** `kline_cache`（写穿） |
| 旧 UI | `GetStockKLine`（`KLineChart.vue`） | **另一路径** | 与多周期主链分离 |
| 直调旁路 | HoldingT / 回测 / 策略等 | 多仍为 `GetStockEastMoneyKLine` | 同 FE/BE 缓存语义 |

**多源关系：** 产品主链 **统一以东财为准**；腾讯/新浪是 **同请求失败时的 fallback**，不是「不同入口绑不同源」。  
另存在历史腾讯日 K Adapter（`TencentKlineAdapter`）等，**不是** Lightweight 多周期 Modal 默认路径。

---

## 五、`kline_cache` 审计

**文件：** `backend/data/kline_cache.go` · `backend/data/eastmoney_kline_api.go`

### 5.1 存储 Key / 元数据

| 字段 | 含义 |
| --- | --- |
| `stock_sec_id + klt + adjust_flag + end_key` | 唯一键；`latest` ← 空/`20500101` |
| `payload` | bars JSON |
| `bar_count` / `fetched_at` | 计数与拉取时间 |
| `last_bar_day` | **已写入**；**读路径决策未用于「日历新鲜度」** |

### 5.2 TTL（`klineCacheTTLAt`，读时按 **当前** LIVE/IDLE 重算）

| klt 类 | LIVE | 交易日 IDLE | 非交易日 IDLE |
| --- | --- | --- | --- |
| 日/周/月 `101/102/103` latest | **60s** | **6h** | **24h** |
| 分钟 latest | **30s** | **2h** | **24h** |

### 5.3 `GetKLineDataBefore` 决策序（Phase17-B.1）

```text
1) klineCacheGet：TTL 内 → 直接返回（无 last_bar_day 校验）
2) end=latest 且 !LIVE：GetStale → 有 bars 即返回（忽略 TTL，无日历校验）
3) LIVE 且 stale 够长：增量 HTTP merge；失败仍可 stale
4) 否则全量 HTTP → saveKLineCacheResult
```

### 5.4 重点回答：为何 09-04 在 09-07「仍有效」？

| 机制 | 为何仍返回 09-04 |
| --- | --- |
| **IDLE stale 短路** | 09-07 **午休 / 盘前 / 盘后** `klineMarketSessionLive=false` → 步骤 2 **故意不 HTTP**，只要库里有 bars（末根可以是 **上周五 09-04**） |
| **交易日 IDLE TTL=6h** | 即使不走 stale，步骤 1 也可在 6h 内把「止于 09-04」的 payload 当新鲜 |
| **无「末根 ≥ 当前期望交易日」** | `last_bar_day` 存在但 **不参与** 命中否决 → **日历落后仍算有效** |
| **FE 30min IDLE** | 后端若返回 09-04，前端继续粘住 |

→ **不是**「TTL 数字算错成永远」，而是 **策略上缺少交易日追赶条件** + **IDLE 省流量优先于日历正确性**。

---

## 六、LIVE 交易状态审计

### 6.1 定义对比

| | 前端 `isKlineMarketLive` | 后端 `klineMarketSessionLive` |
| --- | --- | --- |
| 文件 | `aShareSessionClock.js` | `kline_cache.go` |
| 时段 | 09:30–11:30、13:00–15:00 上海 | 同 |
| 周末 | 否 | 否 |
| 节假日 | **仅看星期**（无交易日历） | **`tradingcalendar.IsTradingDay`** |

### 6.2 对 2026-09-07（周一交易日）

| 墙钟 | LIVE？ | 旧日 K 为何可能不刷新 |
| --- | --- | --- |
| 连续竞价中 | **是**（前后端通常一致） | 短 TTL + 可增量；但 **FE getOrFetch** 可能仍握 IDLE 写入的 09-04；或增量失败回落 stale |
| **午休 11:30–13:00** | **否 → IDLE** | **典型**：步骤 2 直接 stale → **停在 09-04**；poll **停** |
| 盘后 | IDLE | 同上，直至扫描/全量写新 `last_bar_day` |

**若用户口语「盘中」含午休：** 系统 **不会** 进 LIVE，故 **不会** 按 LIVE 路径刷新 —— **符合现码，而非日历 bug  alone**。  
**若确在竞价中仍见 09-04：** 优先查 FE 内存未过期 + 后端 TTL/增量失败 fallback。

---

## 七、「生成快照」影响分析

### 7.1 调用链

```text
allStockList「生成快照」
  → runBackendSnapshotScan()
       → StartSignalScanSnapshot(...)   // app_signal_scan.go
            → SignalScanApi 后台扫描
                 → prepareStockBars / buildIndexCloseMap
                      → kline.GetKLineData(code, "101", …)
                           → GetKLineDataBefore(latest)
                                →（常 miss / 强制拉）HTTP 东财
                                → saveKLineCacheResult  ← 更新 payload + last_bar_day（可含 09-07）
```

### 7.2 为何快照后 K 线出现 09-07？

扫描为算信号 **大量读取 latest 日 K**，写穿 **共享 `kline_cache`**。随后自选/跟踪机会再开 Modal → 后端已是含 **09-07** 的 payload（或 TTL miss 后增量得到）→ 图表显示 07。  
**FE `clearKlineCache` 未见**与快照绑定；更多是后端已新 + FE TTL/换请求后 miss。

### 7.3 归类

| 选项 | 判定 |
| --- | --- |
| A. 正常刷新机制 | **否** — 无「刷新 K 线缓存」产品按钮/API |
| **B. 隐藏副作用** | **是** — 信号扫描顺带更新 K 缓存 |
| **C. 错误依赖** | **部分是** — 若运维/用户靠「生成快照」才能看到当日 K，则属错误依赖副作用 |

---

## 八、最终结论

### 8.1 多周期 K 线组件

| 选项 | 判定 |
| --- | --- |
| **A. 全部使用同一个公共组件（主产品）** | **是** — 个股多周期工作台统一 **`StockKlineModal` + `StockLightweightKlineChart`** |
| B. 多个组件 | 仅存在 **旁路**（旧 `KLineChart`、HoldingT SVG、指数直嵌等），**不是** A/B 现象根因 |

### 8.2 数据源

| 选项 | 判定 |
| --- | --- |
| **A. 统一数据源（主链）** | **是** — 东财 `GetKLineDataBefore` + 共享 `kline_cache`；经统一 Wails/MarketData 门面 |
| B. 多个数据源 | 仅 **fallback**（腾讯/新浪）与 **旧 `GetStockKLine` 旁路**；入口 A/B **不同源** 不成立 |

### 8.3 问题根因归属

| 代号 | 是否 |
| --- | --- |
| **A. 前端缓存** | **是（次）** — IDLE 30min / poll 不绕过 |
| **B. 后端缓存** | **是（主）** — IDLE stale + 长 TTL |
| **C. 交易日判断缺失** | **是（主）** — 有 `last_bar_day` 却不用于否决落后 latest |
| **D. 刷新机制缺失** | **是（次）** — IDLE 停 poll；无日历强制刷新；依赖扫描副作用 |
| **E. 入口差异** | **否** |

### 8.4 最小修复范围（禁止大重构）

与 [PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md](./PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md) 对齐；**只列建议，本审计不实现**：

| 文件 | 函数 | 修改建议 |
| --- | --- | --- |
| `backend/data/kline_cache.go` | **新增** `klineExpectedLatestBarDay` / `klineLatestCalendarFresh` | 交易日≥09:30（含午休）期望末根≥当日；非交易日/开盘前期望昨收 |
| `backend/data/kline_cache.go` | `klineCacheGet` | `endKey==latest` 且日历不新鲜 → 当 miss（`nil`）；可加短宽限防狂打 |
| `backend/data/eastmoney_kline_api.go` | `GetKLineDataBefore` | IDLE `GetStale` 短路前必须 `klineLatestCalendarFresh`；不新鲜则走既有增量/全量 |
| （可选）`frontend/src/utils/klineCache.js` 或 Chart `refreshLatestPoll` | `getOrFetch` / poll | LIVE poll 绕过内存缓存，或按末 bar 日失效 — **非首刀必需** |

**明确不做：** 新 Modal、拆入口、改 Snapshot 语义、改对外 Wails 契约、清库、调 TTL 常数作为主手段、全系统缓存重写。

---

## 九、禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 修改代码 / 清缓存 / 删库 / 调 TTL / 改 API / 修复 | 是 |

---

*Phase17.1 全系统多周期 K 线架构与数据链只读审计结束。*
