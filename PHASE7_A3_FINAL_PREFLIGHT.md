# Phase7-A3 Final Preflight

> 性质：**提交前只读审计** — 不修改业务代码、不 `git add`、不 `commit`  
> 日期：2026-07-27  
> HEAD：`9bd227d` — `Phase7-A2-3-1: add quote migration golden verification report`  
> 目标：`QuoteService → Paper Observation Overlay → Dashboard DTO`（Dashboard 只读观察 Baseline）

---

## 审计结论（Executive Summary）

| 项 | 结论 |
|---|---|
| **六文件独立 Baseline** | ✅ 可形成闭合 `papertrading` 包；符号均在六文件内自洽 |
| **HEAD 编译影响** | ✅ HEAD **无** `papertrading` import；仅提交六文件不破坏现有 `go build ./backend/...` |
| **依赖边界（A 类生产）** | ✅ 无 broker / settlement / job / OpenQuote / execution 依赖 |
| **Observation 只读** | ✅ fallback 链正确；无 DB 写路径 |
| **测试边界** | ✅ `observation_test.go` 已解耦 Engine；纯内存 + fake QuoteService |
| **可提交** | ✅ **满足**（严格 path-level 暂存前提下） |

---

## 1. backend/papertrading 六文件 Baseline 可编译性

### 1.1 审计对象（相对 HEAD 均为 UNTRACKED）

| 文件 | 角色 | Baseline |
|---|---|---|
| `models.go` | `paper_sim_*` 模型 + `EnsureSchema` | ✅ |
| `config.go` | `IsEnabled` / 初始资金 | ✅ |
| `query.go` | 只读查询 | ✅ |
| `dashboard.go` | DTO + `GetDashboard*` + Overlay 接线 | ✅ |
| `observation.go` | Quote Overlay 核心 | ✅ |
| `observation_test.go` | Observation 单元测试 | ✅ |

### 1.2 包内符号闭合性

六文件交叉引用检查：

```text
dashboard.go
  → GetTodayStatus / GetDefaultAccount / GetPositions (query.go)
  → BuildObservationPositionRows / observationQuoteFetcher (observation.go)
  → PaperSim* 类型 (models.go)
  → IsEnabled (config.go)

observation.go
  → data.GetQuoteService / marketdata (HEAD 已有 A2)
  → DashboardPositionRow (dashboard.go)

query.go
  → models 类型 + db 只读 + data.TradePlanRepo.GetFrozenByTradeDate（只读）

observation_test.go
  → BuildObservationPositionRows（无 broker_test 符号）
```

**结论：** 六文件可独立组成 `package papertrading`，不依赖同包内 `broker.go` / `job.go` / `settlement.go` 等 B 类文件。

### 1.3 对全仓编译的影响

| 检查 | 结果 |
|---|---|
| `git grep papertrading` @ HEAD `9bd227d` | **无命中** — 现有已跟踪代码不 import `papertrading` |
| 工作区未跟踪 `app_paper_trading.go` | 调用 `PaperTradingJob` / `SettlementJob` — **不在本次提交范围**；若未来单独入仓需同批带入 job/settlement |
| 工作区未跟踪 `backend/api/papertrading.go` | 含 `/run` — **本轮排除** |

**预测：** 仅提交六文件后，`go build ./backend/...` **可通过**（新包无外部消费者）。

### 1.4 测试执行（工作区现状）

```text
go test ./backend/papertrading/ -count=1 -run "TestBuildObservationPositionRows_"
→ ok   go-stock/backend/papertrading   27.540s

go test ./backend/papertrading/ -count=1
→ ok   go-stock/backend/papertrading   22.054s（含 B 类测试文件，仅本地磁盘存在）
```

**提交后预期：** 仓库内仅含 `observation_test.go` 时，`go test ./backend/papertrading/` 只跑 Observation 单元测试，**不依赖** `broker_test.go`。

---

## 2. 依赖边界审计

### 2.1 A 类六文件 import 矩阵

| 文件 | 外部 import | 判定 |
|---|---|---|
| `models.go` | `db`, `gorm` | ✅ |
| `config.go` | 标准库 | ✅ |
| `query.go` | `db`, `data`（`GetFrozenByTradeDate` **只读**） | ✅ 读路径 |
| `dashboard.go` | `db` | ✅ |
| `observation.go` | `data`, `marketdata` | ✅ QuoteService |
| `observation_test.go` | `marketdata`, `papertrading`, `testify` | ✅ |

### 2.2 禁止依赖 — 未发现污染

| 禁止项 | A 类六文件 | 结果 |
|---|---|---|
| `execution` 包 | 无 import | ✅ |
| `broker` / `NewPaperBroker` | 无 import / 无调用 | ✅ |
| `settlement` / `SettlementJob` | 无 import / 无调用 | ✅ |
| `realtime_price` / `OpenQuote` | 无 import / 无调用 | ✅ |
| `PaperTradingJob` | 无调用 | ✅ |
| `price.go` / `DefaultOpenPriceProvider` | 无引用 | ✅ |

### 2.3 边界说明（非污染，需知情）

| 项 | 说明 |
|---|---|
| `query.go` → `data.NewTradePlanRepo().GetFrozenByTradeDate` | **只读** Frozen Plan ID；不改 TradePlan 写路径 |
| `models.go` 含 `PaperSimOrder` / `PaperSimFill` / `PaperSimRun` | 数据模型定义；Baseline **无**成交/结算执行代码 |
| `dashboard.go` DTO 含 `executionId` 字段 | JSON 展示字段；非 import `execution` 包 |

**结论：** 无 import 或调用链污染；符合「Dashboard 只读观察」边界。

---

## 3. Observation 只读原则审计

### 3.1 DisplayPrice fallback 链

`observation.go` → `resolveDisplayPrice`：

```text
Quote.Price > 0     → live
else Quote.Open > 0 → open_fallback
else                → persisted MarkPrice
```

`fetchObservationQuotes`：`nil` / error / empty → 空 map → 全员 persisted。  
**判定：✅ fallback 正确。**

### 3.2 写路径检查

| 检查项 | 结果 |
|---|---|
| 写 `paper_sim_positions` | **无** — 仅构造 `DashboardPositionRow` |
| 更新 DB `mark_price` | **无** — `PersistedMarkPrice` 仅拷贝 `p.MarkPrice` |
| 修改 Fill | **无** — 未 import / 未调用 broker |
| 影响 Settlement | **无** — 未 import settlement |
| 影响 OpenQuote | **无** — 未 import price/realtime_price |
| `GetDashboardPositions` | 只读 `GetPositions` + Overlay 投影 |

**判定：✅ Observation 只读原则成立。**

---

## 4. Dashboard DTO 审计

### 4.1 新增 / 扩展字段（`DashboardPositionRow` / `DashboardPositions`）

| 字段 | 性质 | 判定 |
|---|---|---|
| `displayPrice` | 展示投影 | ✅ 仅 DTO |
| `persistedMarkPrice` | 库 Mark 快照透传 | ✅ 仅 DTO |
| `quoteSource` / `quoteUpdatedAt` | 行情来源元数据 | ✅ 仅 DTO |
| `observationMarketValue` | 行汇总观察市值 | ✅ 仅 DTO |
| `observationUnrealizedPnl` | 行汇总观察浮盈 | ✅ 仅 DTO |
| `observationEquity` | 现金 + 观察市值 | ✅ 仅 DTO |
| `quoteOverlay` | 是否至少一行 live/open | ✅ 仅 DTO |

账户级 `marketValue` / `equity` / `unrealizedPnl` **仍为库快照**（成交/结算后），与 `observation*` 双口径并存 — 设计预期。

### 4.2 兼容字段

| 字段 | 行为 | 判定 |
|---|---|---|
| `markPrice` | **= `displayPrice`**（注释明确 compat） | ✅ 旧 UI「当前价」列无需改 key |

**判定：✅ 新增字段均为展示投影；`markPrice` 兼容保持。**

---

## 5. 测试边界审计

### 5.1 `observation_test.go` 用例清单

| 用例 | 覆盖 | 依赖 |
|---|---|---|
| `TestBuildObservationPositionRows_FallbackPersistedMark` | nil service + Quote 错误 → persisted | fake QuoteService + 内存 Position |
| `TestBuildObservationPositionRows_LiveQuoteAndPnL` | live Quote、MV、PnL、ReturnRate | 同上 |
| `TestBuildObservationPositionRows_OpenFallback` | Open fallback、PnL | 同上 |
| `TestBuildObservationPositionRows_ReadOnlyNoMutation` | 输入 Position 不被修改（只读语义） | 内存 fixture |

### 5.2 已拆除的跨模块依赖

以下 **已不再出现** 于 `observation_test.go`：

- `setupTestDB`
- `enablePaperTrading`
- `seedFrozenPlan`
- `buyItem`
- `PaperTradingJob`
- `db.Dao` 查询
- `models.TradePlanItem`
- `broker_test.go` helper

### 5.3 禁止依赖 — 确认

| 禁止项 | `observation_test.go` | 结果 |
|---|---|---|
| `broker_test.go` | 无符号引用 | ✅ |
| `job` / `PaperTradingJob` | 无 | ✅ |
| `settlement` | 无 | ✅ |
| real DB migration | 无 | ✅ |

**说明：** 「不写 DB」现通过 **内存 Position 前后字段一致** 验证，而非 DB 前后对比；与 `PHASE7_A3_TEST_BOUNDARY_SPLIT_PLAN.md` 一致。DB 级集成断言留待后续 Integration 套件。

**判定：✅ 测试边界符合 Baseline 要求。**

---

## A. 可提交文件白名单

### A1. 核心（P0 — 必须 path-level 暂存）

```text
backend/papertrading/models.go
backend/papertrading/config.go
backend/papertrading/query.go
backend/papertrading/dashboard.go
backend/papertrading/observation.go
backend/papertrading/observation_test.go
```

### A2. 可选同批（P1 — 前端 DTO 透传 / 只读 UI）

```text
frontend/src/api/paperObservation.ts
frontend/src/components/PaperTradingObservation.vue
```

### A3. 可选文档（P2 — 不进编译）

```text
PHASE7_A3_FINAL_PREFLIGHT.md
PHASE7_A3_IMPLEMENTATION_REPORT.md
PHASE7_A3_TEST_SPLIT_IMPLEMENTATION_REPORT.md
PHASE7_A3_COMMIT_BOUNDARY_REVIEW.md
PHASE7_A3_BASELINE_IMPLEMENTATION_PLAN.md
PHASE7_A3_TEST_BOUNDARY_SPLIT_PLAN.md
```

---

## B. 必须排除文件

| 路径 | 原因 |
|---|---|
| `backend/papertrading/broker.go` | 成交写路径 |
| `backend/papertrading/broker_test.go` | Engine 测试 + 被旧集成测依赖的 helper |
| `backend/papertrading/job.go` | `PaperTradingJob` 执行编排 |
| `backend/papertrading/job_test.go` | Job 测试 |
| `backend/papertrading/price.go` | OpenQuote 契约 |
| `backend/papertrading/realtime_price.go` | OpenQuote 生产实现 |
| `backend/papertrading/settlement.go` | EOD 写 `mark_price` |
| `backend/papertrading/daily_report.go` | 结算后产物 |
| `backend/papertrading/daily_report_test.go` | 结算依赖测试 |
| `backend/papertrading/dashboard_test.go` | 依赖 Job/Broker 播种 |
| `backend/api/papertrading.go` | 同文件含 `POST /run → PaperTradingJob` |
| `app_paper_trading.go` | Cron 调用 Job + Settlement |
| `backend/papertrading/logs/**` | 运行日志 |
| 工作区其它 dirty（execution / TradePlan 写 / Risk 等） | 非 A3 Baseline |

**禁止操作：** `git add -A`、`git add backend/papertrading/`（整目录）

---

## C. 当前风险

| # | 风险 | 严重度 | 缓解 |
|---|---|---|---|
| 1 | 误用整目录 `git add backend/papertrading/` 带入 B 类文件 | **High** | 仅逐文件白名单；提交前 `git diff --cached --name-only` 复核 |
| 2 | 本地磁盘仍有完整 papertrading 包，`go test` 全绿 **不代表** 仅六文件入仓后的测试集 | **Med** | 提交后仅跑 `observation_test`；Integration 另任务 |
| 3 | `query.go` 只读读 Frozen Plan — 语义上触及 TradePlan，但非写路径 | **Low** | 已登记；Dashboard 今日观察需要 planId |
| 4 | `models.go` 含 Order/Fill/Run 模型 — 易被误解为带入执行能力 | **Low** | 仅 schema/类型；无执行函数入仓 |
| 5 | 未提交 `api/papertrading.go` → HTTP Dashboard **不可用**直至 handler 拆分入仓 | **Med** | 预期；Baseline 先冻包；HTTP 另任务 |
| 6 | 未提交 `app_paper_trading.go` — 若未来单独入仓而无 job/settlement 会编译失败 | **Med** | 保持与 Engine 文件同批或延后 |
| 7 | 「不写 DB」测试为内存级，非 DB 集成级 | **Low** | 符合测试拆分设计；Integration 测试后续补 |

---

## D. 推荐 commit message

```text
Phase7-A3: establish paper observation baseline with quote overlay
```

可选 body（人工填写）：

```text
- Add read-only papertrading observation layer (QuoteService overlay)
- Extend Dashboard DTO with display/persisted mark and observation totals
- Keep markPrice compat for existing UI
- Add isolated observation unit tests (no broker/job dependency)
- Exclude broker, settlement, job, and /run API from this baseline
```

---

## 门禁检查清单（提交前人工）

| # | 检查 | 期望 |
|---|---|---|
| 1 | `git diff --cached` 仅含 A1（+ 可选 A2/A3） | ✅ |
| 2 | staged **无** broker/job/settlement/realtime_price/price | ✅ |
| 3 | staged **无** `api/papertrading.go`、`app_paper_trading.go` | ✅ |
| 4 | `go test ./backend/papertrading/ -run TestBuildObservationPositionRows` | ✅ 已通过 |
| 5 | 未使用 `git add -A` | ✅ |

---

## 元数据

| 项 | 值 |
|---|---|
| 业务代码修改 | **否** |
| git add / commit | **否** |
| 输出 | `PHASE7_A3_FINAL_PREFLIGHT.md` |
| 等待人工确认 | **是** |
