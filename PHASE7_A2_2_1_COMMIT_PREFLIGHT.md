# Phase7-A2-2-1 Commit Preflight

> 性质：**只读提交前检查** — 未改源码、未 `git add`、未 commit  
> 基线 HEAD：`90ce40d2a73b6b8f3f794bf6a574a7ce9c6375b2`  
> （`Phase7-A2-2: add quote service migration callsite inventory`）  
> 检查对象：Phase7-A2-2-1 Agent Quote Migration 未提交实现  
> 检查时间：2026-07-27

---

## 1. git diff --stat（A2-2-1 相关）

### 1.1 已跟踪文件修改

```text
 backend/agent/tools/data_tools_wrapper.go    |  30 +------
 backend/agent/tools/marketdata_wire.go       |   9 ++-
 backend/agent/tools/stock_price_info_tool.go | 115 ++++++++++++++++++++++++++-
 3 files changed, 120 insertions(+), 34 deletions(-)
```

### 1.2 未跟踪（本阶段新增）

| 路径 | 状态 |
|---|---|
| `backend/data/quote_service_access.go` | `??` |
| `backend/data/quote_service_access_test.go` | `??` |
| `backend/agent/tools/stock_price_info_tool_test.go` | `??` |
| `PHASE7_A2_2_1_IMPLEMENTATION_REPORT.md` | `??` |

### 1.3 接口 / Adapter

| 路径 | diff |
|---|---|
| `backend/marketdata/interfaces.go` | **无** |
| `backend/marketdata/adapter/legacy_quote_adapter.go` | **无**（复用 A1） |

---

## 2. 文件边界检查

### 2.1 允许清单对照

| 允许文件 | 实际存在于 A2-2-1 变更 | 结论 |
|---|---|---|
| `quote_service_access.go` | ✅ 未跟踪新增 | Pass |
| `quote_service_access_test.go` | ✅ 未跟踪新增 | Pass |
| `marketdata_wire.go` | ✅ 已修改 | Pass |
| `stock_price_info_tool.go` | ✅ 已修改 | Pass |
| `stock_price_info_tool_test.go` | ✅ 未跟踪新增 | Pass |
| `data_tools_wrapper.go` | ✅ 已修改 | Pass |
| `PHASE7_A2_2_1_IMPLEMENTATION_REPORT.md` | ✅ 未跟踪新增 | Pass |

### 2.2 允许清单外文件

| 文件 | 原因 | Commit A 建议 |
|---|---|---|
| `PHASE7_A2_2_1_AGENT_QUOTE_MIGRATION_PLAN.md` | 前序设计任务产出，未入库；**非本实现 diff** | **可选**一并入库；非必须 |
| `PHASE7_A2_2_1_COMMIT_PREFLIGHT.md` | 本只读预检产出 | **可选**；审核记录用 |

### 2.3 工作区其它 dirty（与 A2-2-1 无关）

工作区仍有大量预先存在的 Paper / Execution / TradePlan 等 dirty / untracked（例如 `backend/execution/**`、`backend/data/paper_*.go`、`backend/api/tradeplans.go` 等）。

**结论：** 这些**不是** A2-2-1 改动；Commit A **禁止** `git add -A`，必须 path-level 白名单 add。

---

## 3. 依赖检查

### 3.1 目标链

```text
Agent Tool（QueryStockPriceInfo / GetStockInfo）
        ↓
data.GetQuoteService() → marketdata.QuoteService
        ↓
LegacyQuoteAdapter（仅 marketdata_wire 工厂构造）
        ↓
StockDataApi.GetStockCodeRealTimeData
```

### 3.2 反模式扫描

| 检查项 | 结果 |
|---|---|
| Agent **生产代码**直接调用 `GetStockCodeRealTimeData` | **否**（`stock_price_info_tool.go` 无匹配；`data_tools_wrapper` GetStockInfo 已改走 `RenderGetStockInfo`） |
| 新增第二套 Quote Adapter | **否**（仅复用 `adapter.NewLegacyQuoteAdapter`） |
| bypass QuoteService | **否**（工具经 `GetQuoteService` / 可注入的 `QuoteService` 参数；未注入时硬失败） |
| 业务工具 import `marketdata/adapter` | **否**（仅 `marketdata_wire.go` + 测试文件） |
| 扩展 QuoteService interface | **否** |

说明：测试中的 `fakeRealtimeFetcher.GetStockCodeRealTimeData` 与 `stubQuoteService` 属单测桩，**不是**生产 bypass。

---

## 4. 交易域扫描（相对本 Commit 白名单）

对本阶段**拟提交文件**扫描：未修改下列域的生产路径。

| 域 | A2-2-1 白名单是否修改 | 结论 |
|---|---|---|
| Paper | 否 | Pass |
| Execution | 否 | Pass |
| TradePlan | 否 | Pass |
| Risk | 否 | Pass |
| Frozen Spec | 否 | Pass |
| OpenQuote | 否 | Pass |
| Morning price | 否 | Pass |

注意：工作区其它路径上的 Paper/Execution dirty **不得**进入本 commit。

---

## 5. 测试状态

| 命令 | 结果 |
|---|---|
| `go test ./backend/agent/tools/ -count=1 -run "QueryStockPrice\|RenderGetStockInfo\|GetQuoteService\|LegacyQuoteAdapter"` | **ok**（~9.1s） |
| `go test ./backend/data/ -count=1 -run "QuoteServiceAccess"` | **ok**（~28.1s） |
| `go test ./backend/marketdata/ -count=1 -run "LegacyQuote"` | **ok**（~10.0s；中途曾遇 Windows `unlinkat ... Access is denied` 环境干扰，重跑通过） |

---

## 6. 预检结论

### 6.1 是否建议 commit

**建议 commit（Commit A）** — 范围正确、调用链符合设计、交易域未触碰、相关单测通过。

前提：**仅** path-level 添加白名单；禁止 `git add -A`。

### 6.2 推荐 commit 文件列表

```text
backend/data/quote_service_access.go
backend/data/quote_service_access_test.go
backend/agent/tools/marketdata_wire.go
backend/agent/tools/stock_price_info_tool.go
backend/agent/tools/stock_price_info_tool_test.go
backend/agent/tools/data_tools_wrapper.go
PHASE7_A2_2_1_IMPLEMENTATION_REPORT.md
```

可选（审核需要时）：

```text
PHASE7_A2_2_1_AGENT_QUOTE_MIGRATION_PLAN.md
PHASE7_A2_2_1_COMMIT_PREFLIGHT.md
```

### 6.3 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| 工作区大量无关 dirty 被误加入 commit | **高（操作面）** | 严格白名单 `git add -- <paths>` |
| `QueryStockPriceInfo` JSON 从全量 `StockInfo` 改为市场字段 DTO | **低** | 设计已选兼容中文标签；无交易字段泄漏 |
| wire 工厂首次 `GetQuoteService` 仍会构造 `NewStockDataApi`（与 Kline 同） | **低** | 懒加载；测试用 `SetQuoteService` 注入避免 DB |
| 未做 live Golden（Commit B） | **信息性** | 本 commit 不含 Golden；后续单独观察 |

### 6.4 commit message 建议

```text
Phase7-A2-2-1: migrate agent quote tools to QuoteService

Route QueryStockPriceInfo and GetStockInfo through QuoteService
and LegacyQuoteAdapter instead of calling GetStockCodeRealTimeData
directly.
```

---

## 7. 元数据

| 项 | 值 |
|---|---|
| 是否修改源码 | **否** |
| 是否 git add / commit | **否** |
| 下一步 | 人工确认后执行 Commit A（另下指令） |
