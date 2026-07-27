# Phase7-A2-3-1 Implementation Report

> 性质：**实现完成，等待人工审核** — **未** `git add` / **未** commit  
> 设计依据：`PHASE7_A2_3_1_QUOTE_MIGRATION_PLAN.md`  
> 基线：`6634b8d`（Phase7-A2-2-1 Agent Quote Migration）  
> 范围：Monitor 展示（`GetStockInfos`）+ Dashboard Watchlist（`GetStockInfosRealtimeBatch` / `GetFollowRealtimeList`）

---

## 1. 修改文件列表

| 文件 | 动作 | 说明 |
|---|---|---|
| `app_quote_bridge.go` | **新增** | `quoteToStockInfo` / `fetchRealtimeStockInfos`（仅市场字段） |
| `app_quote_bridge_test.go` | **新增** | fake QuoteService 注入、消费层字段 parity、缓存 hit |
| `app.go` | **修改** | `GetStockInfos` / `GetStockInfosRealtimeBatch` 经 QuoteService；`import marketdata` |
| `PHASE7_A2_3_1_IMPLEMENTATION_REPORT.md` | **新增** | 本报告 |

**未修改：** `interfaces.go`、`legacy_quote_adapter.go`、`stock_price_cache.go`（TTL）、Paper / Execution / Spec、前端、Agent 已迁路径、`getStockInfo` / 成本预警 / AI Monitor（仍旧链，属后续批次）。

---

## 2. 调用链变化

### Before

```text
Consumer（Monitor / Dashboard）
        ↓
GetStockInfos / GetStockInfosRealtimeBatch
        ↓
GetStockCodeRealTimeData
        ↓
StockDataApi
```

### After

```text
Consumer（Monitor / Dashboard）
        ↓
GetStockInfos / GetStockInfosRealtimeBatch
  ├── 时段过滤 / FollowRealtimePriceCache（保留）
  ├── miss/直取 → data.GetQuoteService().GetQuotes
  └── addStockFollowData（成本/盈亏等消费层，保留）
        ↓
QuoteService
        ↓
LegacyQuoteAdapter
        ↓
StockDataApi.GetStockCodeRealTimeData
```

`GetFollowRealtimeList` → 仍调用 `GetStockInfosRealtimeBatch`（无独立 diff，自动切换）。  
`MonitorStockPrices` → 仍调用 `GetStockInfos`（平台文件未改）。

---

## 3. 未触碰交易域声明

| 域 | 本实现是否修改 |
|---|---|
| Paper Trading / OpenQuote / paper_open_buy | **否** |
| Morning price materialization | **否** |
| Execution / Frozen Spec | **否** |
| TradePlan / Risk | **否** |
| Position cost calculation（`addStockFollowData` 逻辑） | **否**（仅取数入口替换；派生仍在消费层） |
| QuoteService interface | **否** |
| LegacyQuoteAdapter | **否** |
| Cache TTL | **否** |

---

## 4. 测试结果

```text
go test . -count=1 -run "QuoteToStockInfo|FetchRealtimeStockInfos|GetStockInfosWithQuoteService|GetStockInfosRealtimeBatchWithQuoteService|GetQuoteService_AccessInject"
→ ok

go test ./backend/data/ -count=1 -run "QuoteServiceAccess"
→ ok

go test ./backend/marketdata/ -count=1 -run "LegacyQuote"
→ ok
```

覆盖：

- fake QuoteService 可注入（`SetQuoteService` / `fetchRealtimeStockInfos`）
- 同一 fake Quote → 展示字段（price/open/high/low/volume/amount/timestamp）与旧字符串语义一致
- batch：首次 miss 调服务，二次 hit 不重复 `GetQuotes`；`FollowRealtimePriceCache` 仍工作
- `quoteToStockInfo` 不引入 cost/profit/alarm

---

## 5. Commit A 白名单建议

禁止 `git add -A`。建议仅：

```text
app.go
app_quote_bridge.go
app_quote_bridge_test.go
PHASE7_A2_3_1_IMPLEMENTATION_REPORT.md
```

可选（若设计文档尚未入库）：

```text
PHASE7_A2_3_1_QUOTE_MIGRATION_PLAN.md
PHASE7_A2_3_1_QUOTE_WATCHLIST_INVENTORY.md
```

建议 message：

```text
Phase7-A2-3-1: migrate watchlist/monitor quote fetch to QuoteService
```

**不做：** Golden（Commit B）；本任务停止于实现 + 报告。

---

## 6. 完成标准核对

| 标准 | 状态 |
|---|---|
| Monitor 展示路径迁移 | ✅ `GetStockInfos` |
| Dashboard Watchlist 路径迁移 | ✅ batch + FollowRealtimeList |
| QuoteService interface 未变 | ✅ |
| Adapter 未变 | ✅ |
| Cache 未变 | ✅ |
| Paper / Execution / Spec 无本任务 diff | ✅ |
| 测试 PASS | ✅ |
| git add / commit | **未执行**（按要求） |
