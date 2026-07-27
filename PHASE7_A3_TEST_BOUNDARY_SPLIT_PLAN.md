# Phase7-A3 Observation 测试边界拆分设计

> 性质：只读设计文档（不改代码、不 `git add`、不 commit）  
> 日期：2026-07-27  
> 目标：将 Observation 只读展示层测试与 Paper Engine 集成测试解耦

---

## 1. 当前测试依赖图

当前 `backend/papertrading/observation_test.go` 混合了两种测试形态：

```text
A. Observation Unit（前 3 个）
  observation_test.go
    ├─ fake QuoteService（stubObsQuoteService）
    ├─ fake Position（内存构造 PaperSimPosition）
    └─ BuildObservationPositionRows（纯函数路径）

B. Engine-backed Integration（第 4 个）
  observation_test.go
    └─ TestGetDashboardPositions_QuoteOverlayDoesNotWriteDB
         ├─ setupTestDB()          ← 定义于 broker_test.go
         ├─ enablePaperTrading()   ← 定义于 broker_test.go
         ├─ seedFrozenPlan()       ← 定义于 broker_test.go
         ├─ buyItem()              ← 定义于 broker_test.go
         └─ PaperTradingJob()      ← job/broker 执行路径
```

补充耦合点：

- `dashboard_test.go` 同样依赖 `setupTestDB/seedFrozenPlan/PaperTradingJob`
- `setupTestDB` 来自 `broker_test.go`，导致 Observation 测试对 Broker 测试基础设施存在隐式依赖

---

## 2. 测试分类（目标态）

### A 类：Observation 单元测试（允许进入 A3 Baseline）

**职责：** 仅验证 Observation 读路径逻辑，不触发执行引擎。  
**允许依赖：**

- fake QuoteService（stub/fake）
- fake Position（内存构造）
- fake Dashboard DTO（纯断言字段映射）
- 纯计算函数与只读拼装逻辑

**必须覆盖：**

- Quote Overlay（live）
- fallback（Quote 失败/空结果）
- PnL 计算正确
- ReturnRate 计算正确
- 不写 DB（建议通过“纯函数无 DB 依赖”结构保证）

### B 类：Paper Integration 测试（禁止进入 A3 Baseline 最小提交）

**职责：** 验证 Paper 引擎与观察读模型在真实 DB + 任务编排下的协同行为。  
**依赖：**

- DB（sqlite memory / gorm）
- Job
- Broker
- Settlement（若覆盖日终链路）
- TradePlan seed / frozen plan 流程

**典型用例：**

- `GetDashboardPositions` 在真实持仓下不写回 DB
- `PaperTradingJob` 产出后 Dashboard/Observation 读口径验证

---

## 3. 拆分方案

### 3.1 目标文件边界

```text
observation_test.go
  仅保留 A 类单元测试：
  - Quote Overlay
  - fallback
  - PnL
  - ReturnRate
  - （只读路径）不写 DB 的单元级证明

paper_integration_observation_test.go（新）
或 dashboard_integration_test.go（新）
  放置 B 类集成测试：
  - setupTestDB / seedFrozenPlan / PaperTradingJob 依赖
  - DB 前后对比（mark_price 不变）
```

### 3.2 用例迁移建议

将当前 `observation_test.go` 的第 4 个用例：

- `TestGetDashboardPositions_QuoteOverlayDoesNotWriteDB`

迁移到 Integration 文件，并归属到 B 类测试套件。

### 3.3 基础设施拆分建议

为避免再次污染，测试辅助函数分层：

- `testkit_observation_unit.go`：仅 fake/stub（无 DB）
- `testkit_paper_integration.go`：`setupTestDB/seedFrozenPlan/...`（DB + Engine）

这样可以从结构上防止 Observation Unit 无意引入 Engine 依赖。

### 3.4 CI/命令分层建议

建议后续分两档执行：

```text
快速单元（A 类）:
go test ./backend/papertrading -run ObservationUnit

集成回归（B 类）:
go test ./backend/papertrading -run Integration
```

（具体命名以后续实现时按现有测试命名规范落地）

---

## 4. 拆分后对 Phase7-A3 Baseline 的影响评估

## 4.1 是否满足 Baseline commit

**结论：满足，且更稳。**

原因：

1. Observation 只读层测试不再依赖 Broker/Job 基础设施
2. A 类测试文件可独立编译运行，不需要引入 B 类禁入文件
3. 与 Baseline 边界一致：`Dashboard 查询 + Observation 计算 + Quote 展示`

## 4.2 风险与收益

**收益**

- 消除测试污染与跨域耦合
- Baseline 提交边界更清晰
- 失败定位更快（展示层 vs 执行层）

**风险**

- 短期需要一次性迁移测试文件与命名（后续实现任务）
- 如果不同时调整测试命令，可能出现“未跑到集成测试”的假象

**缓解**

- 将 A/B 测试命名显式化（Unit/Integration）
- 在 PR/提交模板中分别列出 A/B 测试执行结果

---

## 5. 建议执行顺序（后续实现任务，不在本次执行）

1. 新建 Integration 测试文件，迁移第 4 个 DB 用例  
2. `observation_test.go` 仅保留 A 类用例  
3. 抽离 testkit（unit vs integration）  
4. 更新测试命令与文档  
5. 重新做 Commit Boundary Audit，确认 A3 Baseline 可最小提交

---

## 6. 最终结论

当前问题本质是**测试层级混放**，不是 Observation 业务逻辑错误。  
按本拆分方案执行后，可实现：

- Observation 单元测试纯读、纯计算、无引擎依赖
- Paper Integration 测试单独承载 DB/Job/Broker/Settlement 责任
- Phase7-A3 Baseline commit 边界与测试边界一致

---

## 7. 元数据

| 项 | 状态 |
|---|---|
| 修改源码 | 否 |
| git add | 否 |
| git commit | 否 |
| 输出文档 | `PHASE7_A3_TEST_BOUNDARY_SPLIT_PLAN.md` |

