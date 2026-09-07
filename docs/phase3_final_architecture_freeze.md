# Phase3-Final Architecture Freeze

> 状态：**已冻结（2026-07-19）**  
> 范围：Phase3-PR1 ~ Phase3-PR4-E 完成后的交易入口架构验收  
> 约束：本文档仅作验收与防回退说明；**不修改业务代码、不调整调用链、不进入 Phase4**。

---

## 1. 当前架构图

### 1.1 总览（Execution 唯一编排）

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                         允许进入 ExecutionService                         │
├─────────────────┬─────────────────────┬─────────────────────────────────┤
│ A 自动交易       │ B 研究人工买入        │ C 模拟盘人工交易                  │
│ TradePlan       │ CandidatePool.vue   │ PaperTradingPanel.vue (normal) │
│      ↓          │      ↓              │      ↓                         │
│ PlanItemExecutor│ ResearchTradeIntent │ ManualTradeIntent              │
│      ↓          │      ↓              │      ↓                         │
│                 │ ResearchTradeFacade │ ManualTradeFacade              │
│      └──────────┴──────────┬──────────┴──────────┘                     │
│                            ↓                                            │
│                    ExecutionService                                     │
│                            ↓                                            │
│                      PreTradeCheck                                      │
│                            ↓                                            │
│                      ExecutionPort                                      │
│                     ↙           ↘                                       │
│              PaperBroker      RealBroker                                │
│                  ↓                 ↓                                    │
│           SubmitPaperOrder   RealStubOrder                              │
│           paper_orders/fills ExecutionReportHandler                     │
│                              real_stub_fills                            │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│ D 已知保留旁路（Phase3 未收口，不在本验收“唯一路径”范围内）                 │
│  PaperTradingPanel → SubmitPaperMarginOrder                             │
│  PaperTradingPanel → ConfirmBrokerOrderPlan → broker.Default().PlaceOrder│
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.2 路径明细

| 路径 | 入口 | Intent / 计划 | Facade / Adapter | 编排 | Port | 会计 / OMS |
|------|------|---------------|------------------|------|------|------------|
| **A 自动** | TradePlan / 开盘买入 | `TradePlanItem` | `PlanItemExecutor` | `ExecutionService` | Paper / Real | Paper 账本或 RealStub |
| **B 研究** | `CandidatePool.vue` | `ResearchTradeIntent` | `ResearchTradeFacade` | `ExecutionService` | Paper / Real | 同上 |
| **C 人工** | `PaperTradingPanel.vue`（普通单） | `ManualTradeIntent` | `ManualTradeFacade` | `ExecutionService` | PaperBroker | Paper 账本 |
| **D 旁路** | 见 §5 | — | — | **不经** ExecutionService | — | 各自实现 |

---

## 2. 执行入口规则

### 2.1 允许进入 `ExecutionService`

| 调用方 | 职责 |
|--------|------|
| `ResearchTradeFacade` | 研究 Intent → 临时 `TradePlanItem` → `ExecutePlanItem` |
| `ManualTradeFacade` | 人工 Intent → 临时 `TradePlanItem` → `ExecutePlanItem` |
| `PlanItemExecutor` | 自动 TradePlan / 开盘买入 → `ExecutePlanItem` |

编排顺序（冻结）：

```text
TradePlanItem (+ opts)
  → ExecutionService.ExecutePlanItem
  → PreTradeCheck
  → ExecutionPort.Submit
  → PaperBroker | RealBroker
```

### 2.2 禁止

| 禁止项 | 说明 |
|--------|------|
| `frontend` → `SubmitPaperOrder` | UI 不得直连纸面会计 |
| `api` → `SubmitPaperOrder` | HTTP/API 层不得直连纸面会计 |
| Facade → `SubmitPaperOrder` / `FillPaperOrder` / `NewPaperTradingApi` / `PaperTradingApi` | 必须经 ExecutionService |
| 任何新的 UI/API | 不得新增对上述纸面 API 的直接调用 |

### 2.3 `SubmitPaperOrder` 定位（冻结）

> **Paper accounting primitive.**  
> UI / API / Façade must not call directly.

合法用途仅限：

1. `PaperBroker` 内部实现（`ExecutionPort` 适配）
2. 测试替身 / 单测

遗留说明：根目录 `App.SubmitPaperOrder` 的 Wails 导出可仍存在，但 **UI 契约测试禁止调用**；不得作为新功能入口。

---

## 3. Intent 边界

### 3.1 `ResearchTradeIntent`

| 项 | 内容 |
|----|------|
| 用途 | 研究候选确认交易（研究页人工买入） |
| 来源常量 | `research_source` = `signal_scan_snapshot` |
| strategyTag | `research_candidate` |
| **必须包含** | `candidate_snapshot_id`、`signal_score`、`signal_tag`、`research_source` |
| **禁止** | 承载普通模拟盘人工交易；与 Manual 模型合并或互借字段 |

注释约定：

> Research-origin execution intent. Not a manual order container.

### 3.2 `ManualTradeIntent`

| 项 | 内容 |
|----|------|
| 用途 | 模拟盘人工交易（普通单） |
| 来源常量 | `manual_source` = `paper_trading_panel` |
| strategyTag | `manual` |
| **必须包含** | `manual_source`、`order_kind`、`reason` |
| **禁止出现** | `candidate_snapshot_id`、`signal_score`、`signal_tag`、`research_source` |
| order_kind | 本阶段仅执行 `normal`；`margin_*` 预留且 Facade 拒绝执行 |

注释约定：

> Manual execution intent. Not a research signal container.

### 3.3 隔离原则（防语义污染）

- 两个 Intent **不得合并**为单一模型。
- 研究溯源字段不得进入 Manual；人工来源字段不得冒充研究信号。
- 两者均可映射为临时 `TradePlanItem`，但 **持久化模型保持分离**。

---

## 4. Paper / Real 边界

### 4.1 Paper

```text
ExecutionPort
  → PaperBroker
  → SubmitPaperOrder
  → paper_orders
  → paper_fills（及持仓/权益账本）
```

- `FillPaperOrder` 仅服务于纸面成交路径（含 PaperBroker.Fill / 会计层）。
- 前端 / API / Facade 不得直接调用。

### 4.2 Real

```text
ExecutionPort
  → RealBroker
  → RealStubOrder
  → ExecutionReportHandler
  → real_stub_fills
```

### 4.3 禁止（冻结）

| 禁止 | 原因 |
|------|------|
| Real 成交进入 `FillPaperOrder` | 污染纸面会计 |
| Real / Paper fill 表混用 | 账本与 OMS 语义混淆 |
| Real 路径写 `paper_fills` | 与 Phase2 Real 生命周期隔离冲突 |

---

## 5. 已知旁路（技术债，非本验收失败项）

| 旁路 | 当前调用链 | 状态 | 原因 |
|------|------------|------|------|
| **Margin** | `PaperTradingPanel` → `SubmitPaperMarginOrder` | Phase3 **未收口** | 暂无统一 Margin `ExecutionPort` |
| **BrokerConfirm** | `ConfirmBrokerOrderPlan` → `broker.Default().PlaceOrder` | Phase3 **未收口** | 券商确认占位流程，暂不进入 Execution 栈 |

说明：

- 上述旁路 **不属于**「Execution 唯一路径」验收范围。
- Phase3 冻结测试对普通单 / 研究买入已禁止 `SubmitPaperOrder`；Margin / BrokerConfirm **仍允许** 在 `PaperTradingPanel` 中保留。
- 收口工作留给后续 Phase（见 §7），**本文档发布后不得在 Phase3 名义下偷偷改调用链**。

---

## 6. 冻结测试与测试结果

### 6.1 防回退测试清单

| 断言 | 测试文件 |
|------|----------|
| CandidatePool 禁止 `SubmitPaperOrder` / `PaperTradingApi`，须走 ResearchTrade API | `backend/api/phase3_research_ui_freeze_test.go`、`phase3_trade_ui_freeze_test.go` |
| PaperTradingPanel 普通单禁止 `SubmitPaperOrder`，须走 ManualTrade API；允许 Margin / BrokerConfirm | `backend/api/phase3_paper_panel_manual_trade_freeze_test.go`、`phase3_trade_ui_freeze_test.go` |
| Research / Manual Facade 禁止 PaperTrading 直连 | `backend/execution/phase3_research_execution_freeze_test.go`、`phase3_manual_execution_freeze_test.go`、`phase3_execution_entry_freeze_test.go` |
| `ExecutionService` 为人工/研究唯一执行编排入口 | `backend/execution/phase3_execution_entry_freeze_test.go` |
| `SubmitPaperOrder(` 不得出现在 api / facade / frontend 生产源码 | `backend/execution/phase3_execution_entry_freeze_test.go` |
| Intent 模型隔离（Research 必有研究字段；Manual 禁止研究字段） | `backend/api/phase3_trade_ui_freeze_test.go` |
| Manual / Research API 源码无 PaperTrading 旁路 | `backend/api/phase3_manual_trade_api_freeze_test.go` 等 |

### 6.2 验收执行结果

```text
go test ./backend/execution/ ./backend/api/ -count=1 -timeout 90s -run "Phase3_"
→ ok  go-stock/backend/execution
→ ok  go-stock/backend/api

（Phase3-PR4-E 全量）go test ./... -count=1 -timeout 120s
→ 全部通过（2026-07-19）
```

### 6.3 本验收明确未改动的代码

以下文件/能力在 Phase3-Final **不得因验收文档而修改**：

- `ExecutionService` / `ExecutionPort`
- `PaperBroker` / `RealStub*` / `SubmitPaperOrder` / `FillPaperOrder`
- CandidatePool API
- Vue 业务功能

---

## 7. 后续 Phase4 建议（仅建议，不启动）

| 优先级 | 建议项 | 说明 |
|--------|--------|------|
| P0 | Margin 收口 | `SubmitPaperMarginOrder` → Margin 感知的 ExecutionPort / Intent（独立 `order_kind`，仍不与 Research 混用） |
| P1 | BrokerConfirm 收口 | `ConfirmBrokerOrderPlan` 进入 Execution 栈或明确永久旁路并文档化 |
| P2 | 退役 `App.SubmitPaperOrder` Wails 导出 | 在 UI/API 零引用确认后删除遗留绑定 |
| P3 | 统一观测 | 对 A/B/C 三条路径打齐 `strategyTag` / Intent ID / ClientOrderID 指标 |

**停止线**：Phase3-Final 至此结束；**不进入 Phase4 实现**。

---

## 附录：Phase3 交付对照

| PR | 内容 | 状态 |
|----|------|------|
| PR1 | `ResearchTradeIntent` 模型 + Repo | ✅ |
| PR2 | `ResearchTradeFacade` → ExecutionService | ✅ |
| PR3-A/B | ResearchTrade API + CandidatePool UI | ✅ |
| PR4 设计 | Manual 与 Research 分离（不复用 Research Intent） | ✅ |
| PR4-A | `ManualTradeIntent` 模型 + Repo | ✅ |
| PR4-B | `ManualTradeFacade` → ExecutionService | ✅ |
| PR4-C | ManualTrade API | ✅ |
| PR4-D | PaperTradingPanel 普通单改走 ManualTrade | ✅ |
| PR4-E | 架构防回退冻结测试 + 注释 | ✅ |
| **Final** | 本验收文档冻结 | ✅ |

---

*Document ID: `phase3_final_architecture_freeze` — Architecture acceptance only.*
