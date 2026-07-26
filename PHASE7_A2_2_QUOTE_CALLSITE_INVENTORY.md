# Phase7-A2-2 Quote Callsite Inventory

> 性质：**只读规划** — 未改源码、未生成 migration patch、未改 DB、未触碰 Paper/Execution 实现  
> 基线 HEAD：`d2b8f64bda079a6cb0467a849bd374c28192d1ba`（Phase7-A2-1 Golden Report Freeze）  
> 依据：`PHASE7_A2_CALLSITE_INVENTORY.md` §2 + 当前主源码重扫（排除 `build/*_wt`、`tmp_*`）

---

## 1. 当前调用链图

### 1.1 现状（生产热路径）

```text
Consumer（Watchlist / Monitor / AI / Agent / Paper / …）
        ↓
  （绝大多数直连）
        ↓
StockDataApi.GetStockCodeRealTimeData(codes...)
        ├── 腾讯 qt.gtimg.cn / 新浪 hq.sinajs.cn
        ├── 大批量自动分片
        └── 异步 upsert StockInfo（副作用）
        ↑
        │ 仅自选 batch：
GetStockInfosRealtimeBatch
        ├── FollowRealtimePriceCache.Partition（命中）
        └── miss → GetStockCodeRealTimeData → SetBatch
```

### 1.2 Phase7-A1 已就位、尚未挂载

```text
（无生产调用方）
        ↓
marketdata.QuoteService
  GetQuote / GetQuotes
        ↓
adapter.LegacyQuoteAdapter
        ↓
StockDataApi.GetStockCodeRealTimeData
```

目标态（未来迁移后，按调用方逐步切换）：

```text
Consumer
        ↓
marketdata.QuoteService
        ↓
LegacyQuoteAdapter
        ↓
StockDataApi.GetStockCodeRealTimeData
```

---

## 2. 调用点分类

### A. 非交易展示路径（Low → 优先试点）

| 标签 | 文件 / 函数 | 用途 | 缓存 | 风险 |
|---|---|---|---|---|
| **Watchlist / Dashboard** | `app.go`：`GetStockInfosRealtimeBatch` → `GetFollowRealtimeList` | 自选实时列表 | **有** `FollowRealtimePriceCache`（~5s） | Low |
| **Watchlist / Cron** | `app_windows/linux/darwin.go`：`MonitorStockPrices` → `GetStockInfos` | 定时监控/推送 | **无**共享 Quote 缓存 | Low |
| **Watchlist** | `app.go`：`MonitorFollowedStockCostPrices` | 跌破成本预警（成本在 Follow） | 无 | Low |
| **Watchlist** | `app.go`：`getStockInfo`；`app_linux.go`：`Greet` | 单只详情 / 示例 | 无 | Low |
| **Watchlist** | `app_domready.go` / `app_price_polling.go` | 预热 / 触发 Monitor | 经 batch | Low |
| **Watchlist** | `stock_data_api.go`：`Follow` 内取现价 | 加自选时写关注价 | 无 | Low |
| **AI** | `app.go`：`MonitorAiRecommendStockPrices` | AI 推荐价预警 | 无 | Low |
| **AI** | `ai_recommend_stocks_api.go` | 列表补现价 | 无 | Low |
| **AI** | `openai_stream.go` | 对话附带现价 | 无 | Low |
| **Agent** | `agent/tools/stock_price_info_tool.go`、`data_tools_wrapper.go` | Agent 查价 | 无 | **Low（A2-2-1 首选）** |
| **脚本** | `scripts/importwatchlist/main.go` | 导入补价 | 无 | Low |

字段需求：主要为 `Price` / `Open`/`High`/`Low`/`PreClose` / 涨跌 / `Bid`/`Ask` / 时间；Watchlist 在上层叠加 Follow 派生字段（成本、盈亏），**不进入** `marketdata.Quote`。

### B. 半交易观察路径（Medium — 延后）

| 标签 | 文件 / 函数 | 用途 | 影响 | 风险 |
|---|---|---|---|---|
| **Paper Mark** | `papertrading/realtime_price.go`：`MarkPrice` | EOD/结算盯市 | 影响权益展示与结算估值 | Medium |
| **Paper Dashboard** | `papertrading/dashboard.go` 等 | 读已存 `MarkPrice` 展示 | 展示；写入路径另计 | Medium |
| **Position display** | `paper_trading.go` / UI 持仓 | 市值 = Mark×量 | 观察面 | Medium |
| **交易记录** | `stock_data_api.go`：`resolveTradingRecordClosePrice`；持仓市值汇总 | 今日用实时价 | 记录/统计展示 | Medium |
| **SetPaperMarkPrice** | `app.go` → `PaperTradingApi.SetMarkPrice` | 外部写入市价 | 观察写入，非下单 | Medium |

说明：`MarkPrice` 观测可最终经 QuoteService，但须与 **成交用 OpenQuote** 解耦；A2-2-1 **不做**。

### C. 禁止迁移路径（High — A2-2 全程禁止）

| 标签 | 文件 / 函数 | 原因 |
|---|---|---|
| **Execution / Frozen Spec** | `backend/execution/**` | 下单价量唯一来源是 Frozen Spec `limit_price` / `target_volume`；包内**无** `GetStockCodeRealTimeData`；antifallback 明确禁止 live quote 回写 Spec |
| **Paper 成交价** | `papertrading/broker.go` → `Price.OpenQuote` | Paper 成交开仓价；经 `RealtimeOpenPriceProvider.OpenQuote` | 
| **Paper 开仓准备** | `data/paper_open_buy.go` 默认行情源 | 开仓准备价 | 
| **Morning limit 物化** | `strategy/morning_price_materialize.go`（可注入 RealtimeOpen） | Draft plan limit；Freeze 前改价敏感 | 
| **Order price source** | SafetyGate / SubmitIntent 组装 | 必须读 Spec，禁止 runtime live 覆盖 |

---

## 3. 推荐迁移顺序（按风险）

### Low（允许进入 A2-2-1）

1. **Agent 查价工具**（`stock_price_info_tool` / `data_tools_wrapper` 行情工具）— 镜像 A2-1 K 线试点  
2. AI 推荐列表补现价 / `openai_stream` 附带现价  
3. AI / 自选成本预警 Monitor（只读告警）  
4. `scripts/importwatchlist`（离线）

### Medium（A2-2 中后期，单独 PR + Golden）

1. Watchlist：`GetStockInfos` / Monitor（注意与 batch 缓存关系）  
2. Watchlist：`GetStockInfosRealtimeBatch`（**保留** `FollowRealtimePriceCache` 于消费层或后续再收敛）  
3. 交易记录收盘/市值展示  
4. Paper **仅** `MarkPrice` 观测路径（禁止同时改 `OpenQuote`）

### High（本阶段禁止）

1. `RealtimeOpenPriceProvider.OpenQuote` / Paper Broker 成交  
2. `paper_open_buy` 行情源  
3. `MorningOpenPriceFunc` 装配  
4. 任何 Execution / Frozen Spec / SubmitIntent 价量路径  

---

## 4. QuoteService 是否满足当前需求

| 能力 | A1 现状 | 结论 |
|---|---|---|
| **batch quote** | `GetQuotes([]string)` → 委托 `GetStockCodeRealTimeData` | **满足** |
| **single quote** | `GetQuote` | **满足** |
| **market fields only** | `Quote` 仅行情字段；排除 cost/profit/position/order | **满足** |
| **timestamp** | `FetchedAt` + 上游 `Date`/`Time` | **满足**（观测足够） |
| **cache** | Adapter **不**含 TTL；`FollowRealtimePriceCache` 仍在 App 层 | **A2-2-1 不要求**进接口；Watchlist 迁移时继续在消费层复用现有缓存即可 |

Agent / AI 试点所需：`Price`/`Open`/`PreClose`/`High`/`Low`/`Change*`/`Bid`/`Ask` — 均已在 `Quote`。

---

## 5. 是否需要扩展 A1 interface

**A2-2-1（Agent Quote 试点）：不需要扩展。**

原因：

- `GetQuote` / `GetQuotes` 已覆盖单只与批量；  
- 字段集足够 Agent/AI 展示；  
- 缓存属策略层，不应塞进只读接口（与 A2-1「不改缓存」一致）；  
- 禁止为 Paper/Execution 新增 `OpenForFill` 之类方法（避免交易域泄漏）。

**后续若迁 Watchlist batch：** 仍优先「消费方继续用 `FollowRealtimePriceCache`，miss 时调 `QuoteService`」，**不必**把 cache 写进 `QuoteService` 接口。仅当多消费者强制共享同一缓存门面时，再开独立设计（非 A2-2-1）。

---

## 6. 是否允许进入 A2-2-1 实现

**允许进入 A2-2-1，但范围必须收紧为：**

| In Scope | Out of Scope |
|---|---|
| Agent 查价工具 → `QuoteService` / `LegacyQuoteAdapter` | Paper `OpenQuote` / `paper_open_buy` |
| 懒加载注入（类比 `SetKlineServiceFactory`，避免 data↔adapter 环） | Morning / Execution / Frozen Spec |
| Price/Open Golden（旧 `GetStockCodeRealTimeData` vs `GetQuote`） | 改共享 TTL / 删除旧 API |
| 文档与选择性 commit | Watchlist batch 全量迁移（可列为 A2-2-2） |

门禁建议：

1. 先出 `PHASE7_A2_2_1_AGENT_QUOTE_MIGRATION_PLAN.md`（设计 only）  
2. 实现仅 Agent 工具 + Golden  
3. Preflight 后再 commit  

---

## 7. 与 A2-1 的平行关系

| | A2-1 Kline | A2-2-1 Quote（建议） |
|---|---|---|
| 试点 | Agent 东财 K 线 | Agent 实时查价 |
| 接口 | `KlineService.GetBars` | `QuoteService.GetQuote(s)` |
| Adapter | `EastMoneyKlineAdapter` | `LegacyQuoteAdapter` |
| Golden | OHLC 15/15 PASS（已冻结） | Price/Open（及关键字段）数值对齐 |
| 交易链 | 未触及 | **必须**未触及 |

---

## 8. 元数据

| 项 | 值 |
|---|---|
| 是否修改源码 | **否** |
| 是否 migration patch | **否** |
| 是否触碰 Paper/Execution 代码 | **否** |
| 输出 | `PHASE7_A2_2_QUOTE_CALLSITE_INVENTORY.md` |
| 建议下一步 | A2-2-1 Agent Quote Migration **设计文档**（仍不实现，除非用户另行下达） |
