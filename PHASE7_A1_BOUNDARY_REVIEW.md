# Phase7-A1 Boundary Review

> 性质：**只读验收** — 未改业务源码、未改 schema、未 git add / commit / reset / clean  
> 基线：Git tag **`v0.6.7-snapshot-freeze`**（commit `f1951b75b6ef172dac4d923906438d200d64df13`）  
> HEAD：`f1951b75b6ef172dac4d923906438d200d64df13`（与 tag 指向同一 commit）  
> 分支：`dev`

---

## 1. 交易链位置确认

```text
Market Data                          ← Phase7-A1 仅落在此层（只读 facade）
    ↓
Signal Snapshot
    ↓
Strategy
    ↓
Candidate
    ↓
Trade Plan
    ↓
Risk
    ↓
Paper
    ↓
Execution
```

**结论：** Phase7-A1 位于 **Market Data 层**；新增包不参与、不反向依赖 Trade Plan / Risk / Paper / Execution。

Frozen Spec 仍是交易价量唯一来源；本层无写路径、无下单/成交/风控决策。

---

## 2. Git 贡献范围

| 项 | 结果 |
|---|---|
| Phase7-A1 新增（未跟踪） | `backend/marketdata/`（6 个 Go 文件）、`PHASE7_A1_IMPLEMENTATION_REPORT.md` |
| Phase7-A1 对已跟踪文件改动 | **0** |
| 工作区整体 dirty | 预先存在：约 591 条 porcelain / 75 个已跟踪文件 diff（非 A1） |
| A1 是否混入 data/strategy/trade/paper/execution/frontend/app.go/schema | **否**（那些 dirty 文件早于本任务，且不在 A1 提交清单内） |

生产代码对 `go-stock/backend/marketdata` 的 import：**零**（除包内测试与 adapter）。未迁移任何调用方。

---

## 3. 逐文件边界审计

### 3.1 `interfaces.go`

| 检查 | 结果 |
|---|---|
| 只读 Market Data interface | **Pass** — `KlineService.GetBars` / `QuoteService.GetQuote(s)` |
| 模型仅市场字段 | **Pass** — `Bar` / `Quote` 无交易域字段 |
| 不含 Trade / Position / Order / Risk / Plan / Execution | **Pass**（仅注释声明禁止；无类型/字段） |

### 3.2 `kline_service.go`

| 检查 | 结果 |
|---|---|
| 仅 request normalization | **Pass** — 周期/复权常量、`NormalizeBarsRequest`、`FormatEndTime`、`ParseBarTime` |
| 无缓存迁移 | **Pass** — 无 cache 引用 |
| 不改变行情来源 | **Pass** — 本文件不取数；不 import `backend/data` |

### 3.3 `quote_service.go`

| 检查 | 结果 |
|---|---|
| 只读 quote 辅助 | **Pass** — `NormalizeCode(s)`、`FindQuote` |
| 无 position / cost / profit 字段 | **Pass** — 无结构体字段；`Quote` 定义在 `interfaces.go` 已排除 |

### 3.4 Adapter

| Adapter | 检查 | 结果 |
|---|---|---|
| `EastMoneyKlineAdapter` | 纯 wrapper | **Pass** — 规范化后委托 `GetKLineDataBefore`，映射 `KLineData`→`Bar` |
| | 不改旧 API 行为 | **Pass** — 未修改 `eastmoney_kline_api.go`；无新缓存/切片策略 |
| `LegacyQuoteAdapter` | 纯 wrapper | **Pass** — 委托 `GetStockCodeRealTimeData`，映射市场字段到 `Quote` |
| | 不改旧 API 行为 | **Pass** — 未修改 `stock_data_api.go`；显式跳过 Cost/Profit/Follow/Alarm |

依赖方向：`marketdata`（零业务依赖）← `marketdata/adapter` → `backend/data`。  
**无** `data` / `strategy` / `execution` → `marketdata` 的反向依赖。

### 3.5 测试中的交易域字样

`marketdata_test.go` 在 fake `StockInfo` 里**故意填入** `CostPrice` / `Profit` 等，用于验证 Adapter **不会**映射进 `Quote`；另有 `TestModelsStayMarketDataOnly` 反射守护。属验收测试，**不是**领域污染。

---

## 4. 架构边界结论

| 检查项 | 结论 |
|---|---|
| 落在 Market Data 层 | **Pass** |
| 不能反向依赖交易域 | **Pass** |
| 无调用方迁移 | **Pass** |
| 无交易链修改 | **Pass** |
| 无 schema / DB 修改 | **Pass** |
| 边界污染 | **未发现** |

---

## 5. 测试记录（本轮验收复跑）

| 命令 | 结果 |
|---|---|
| `go test ./backend/marketdata/... -count=1` | **PASS**（`ok go-stock/backend/marketdata` ~9.9s） |
| `go vet ./backend/marketdata/...` | **PASS** |
| `go build ./backend/...` | **PASS** |

### 既有失败（仅记录，禁止修复）

| 现象 | 原因 | 与 A1 关系 |
|---|---|---|
| `go test ./backend/data` 全包超时 | chromedp / 网络集成测试阻塞（此前已观测 ~629s） | 无关；A1 未改 `backend/data` |
| `go build ./...` → `main redeclared` | 未跟踪目录 `tmp_diag_upcoming` / `tmp_manual_generate_runtime_verify` / `tmp_phase67e_activate` 多 `func main` | 无关；未清理 |

---

## 6. Phase7-A2 准入建议

**可以进入 Phase7-A2 规划/实现准备**，前提：

1. 先完成 A2-0 调用点清点；  
2. 首个迁移 PR 限定非交易只读路径；  
3. Paper / Execution 取价最后迁移；  
4. A2 内禁止改信号公式与 schema。

---

## 7. 元数据

| 项 | 值 |
|---|---|
| 是否修改业务源码 | **否** |
| 是否修改 schema / DB | **否** |
| 是否 git add / commit | **否** |
| 输出文件 | `PHASE7_A1_BOUNDARY_REVIEW.md` |
