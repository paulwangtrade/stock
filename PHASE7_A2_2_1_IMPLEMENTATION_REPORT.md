# Phase7-A2-2-1 Implementation Report

> 性质：**实现完成，等待人工审核** — **未** `git add` / **未** commit  
> 设计依据：`PHASE7_A2_2_1_AGENT_QUOTE_MIGRATION_PLAN.md`  
> 基线 HEAD（开始时）：`90ce40d`（Phase7-A2-2 Quote Callsite Inventory）  
> 范围：仅 Agent Quote 查询路径 → `QuoteService` / `LegacyQuoteAdapter`

---

## 1. 修改文件

| 文件 | 动作 | 说明 |
|---|---|---|
| `backend/data/quote_service_access.go` | **新增** | `SetQuoteService` / `SetQuoteServiceFactory` / `GetQuoteService`（镜像 Kline access） |
| `backend/data/quote_service_access_test.go` | **新增** | 懒加载工厂 / Set 覆盖；无 DB |
| `backend/agent/tools/marketdata_wire.go` | **修改** | `init` 注册 `LegacyQuoteAdapter` 懒加载工厂（不在 init 里取数/访问 DB） |
| `backend/agent/tools/stock_price_info_tool.go` | **修改** | `QueryStockPriceInfo` → `QuoteService.GetQuotes`；市场字段 JSON DTO；抽出 `RenderGetStockInfo` |
| `backend/agent/tools/data_tools_wrapper.go` | **修改** | `GetStockInfo` 闭包改为 `RenderGetStockInfo(data.GetQuoteService(), codes)` |
| `backend/agent/tools/stock_price_info_tool_test.go` | **新增** | Agent 工具走 QuoteService、fake 注入、Adapter 委托、无 DB |
| `PHASE7_A2_2_1_IMPLEMENTATION_REPORT.md` | **新增** | 本报告 |

**未修改：** `backend/marketdata/interfaces.go`（QuoteService 接口）、Paper / OpenQuote / Morning / Execution / Frozen Spec / 交易链。

---

## 2. 调用链变化

### 2.1 旧

```text
QueryStockPriceInfo / GetStockInfo
        ↓
data.NewStockDataApi().GetStockCodeRealTimeData(...)
        ↓
StockDataApi（腾讯/新浪）
```

### 2.2 新

```text
QueryStockPriceInfo / GetStockInfo
        ↓
data.GetQuoteService()          ← interface；wire 懒加载
        ↓
marketdata.QuoteService
  GetQuotes / GetQuote
        ↓
adapter.LegacyQuoteAdapter      ← 仅 marketdata_wire 构造
        ↓
StockDataApi.GetStockCodeRealTimeData
```

### 2.3 Agent 入口确认

| 工具 | 状态 |
|---|---|
| `QueryStockPriceInfo`（`stock_price_info_tool.go`） | **已迁移** |
| `GetStockInfo`（`data_tools_wrapper.go`） | **已迁移** |
| 其它 Agent quote tool | **无**（仓库内 Agent 对 `GetStockCodeRealTimeData` 仅上述两处） |

### 2.4 注入模式

- 业务工具 **不** import `marketdata/adapter`；只依赖 `marketdata.QuoteService`。  
- `marketdata_wire.go`（唯一接线）注册工厂：`adapter.NewLegacyQuoteAdapter()`。  
- 工厂在 **首次** `GetQuoteService()` 时执行，避免 init 阶段访问 DB/Setting。  
- 服务未注入时：**硬失败**（`QuoteService 未初始化` / markdown 同文案），禁止静默回退旧 API。

### 2.5 输出边界

- `QueryStockPriceInfo`：兼容旧中文 JSON 标签的 **市场字段 DTO**（日期/时间/代码/名称/价量额/开高低昨收）；不含 cost/position/profit/follow/alarm/order。  
- `GetStockInfo`：markdown 仅市场行情字段。

---

## 3. 是否触碰交易域

| 域 | 是否触碰 |
|---|---|
| Paper Trading | **否** |
| OpenQuote / Broker 成交价 | **否** |
| Morning price materialization | **否** |
| Execution | **否** |
| Frozen Spec | **否** |
| QuoteService interface 扩展 | **否**（未发现不足，无需设计变更） |

---

## 4. 测试结果

```text
go test ./backend/agent/tools/ -count=1 -run "QueryStockPrice|RenderGetStockInfo|GetQuoteService|LegacyQuoteAdapter"
→ ok

go test ./backend/data/ -count=1 -run "QuoteServiceAccess"
→ ok

go test ./backend/marketdata/... -count=1 -run "LegacyQuote"
→ ok（既有 Adapter 委托测试）
```

覆盖点：

| 用例 | 验证 |
|---|---|
| `TestQueryStockPriceInfoJSON_UsesQuoteService` | Agent JSON 路径调用 `GetQuotes`；无交易域泄漏 |
| `TestRenderGetStockInfo_UsesQuoteService` | markdown 路径走 QuoteService |
| `TestGetQuoteService_FakeInjectNoDB` | `SetQuoteService` 注入；不触发 wire→`NewStockDataApi`→DB |
| `TestLegacyQuoteAdapter_DelegatesLegacyAPI_NoDB` | Adapter 委托 fake Legacy API；CostPrice/Profit 不进入 Quote |
| `TestQuoteServiceAccess_*` | 懒加载只创建一次；Set 覆盖 |

---

## 5. Commit A 建议文件列表

人工审核通过后，**仅**选择性 add（禁止 `git add -A`）：

```text
backend/data/quote_service_access.go
backend/data/quote_service_access_test.go
backend/agent/tools/marketdata_wire.go
backend/agent/tools/stock_price_info_tool.go
backend/agent/tools/stock_price_info_tool_test.go
backend/agent/tools/data_tools_wrapper.go
PHASE7_A2_2_1_IMPLEMENTATION_REPORT.md
```

可选同批（若尚未入库）：

```text
PHASE7_A2_2_1_AGENT_QUOTE_MIGRATION_PLAN.md
```

建议信息：

```text
Phase7-A2-2-1: migrate agent quote tools to QuoteService
```

**Commit B（本任务不做）：** Golden live/fixture 验证报告（`PHASE7_A2_2_1_GOLDEN_REPORT.md`）。

---

## 6. 元数据

| 项 | 值 |
|---|---|
| git add | **否** |
| commit | **否** |
| 等待 | 人工审核 |
| 下一步建议 | 审核通过后 Commit A；再开 Golden Commit B |
