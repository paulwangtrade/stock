# TradePlan Adapter Contract（Phase5-A）

> 状态：**Design only（2026-07-21）**  
> 范围：定义 `TradePlanDraft` → `TradePlanAdapter` → `TradePlanCandidate` 隔离边界  
> **本阶段禁止改代码**；禁止接 Execution / PaperBroker / Broker API / Order；禁止改 TradePlan 生命周期 / Decision Schema

---

## 0. 目标架构（冻结）

```text
DecisionSnapshot          # Fact / Explanation / Shadow（非交易授权）
        |
        v
TradePlanDraft            # Phase4：只读投影 + 生命周期隔离（EnableExecute=false）
        |
        v
TradePlanAdapter          # Phase5-A：映射契约（本文件）
        |
        v
TradePlanCandidate        # 中间产物：可审计候选，NOT 落库 TradePlan
        |
        ✗（本阶段禁止）
        v
Existing TradePlan        # 现有交易链入口（BuildTradePlan / ready）
        |
        v
Execution / PaperBroker / Order
```

**Adapter 输出必须是 `TradePlanCandidate`，不是 `models.TradePlan`。**  
避免映射结果直接进入 `CreatePlanWithItems` / `RunPaperOpenBuyOnce`。

---

## 1. 输入 / 输出契约

### 1.1 输入：`TradePlanDraft`（已存在）

来源：`backend/tradeplan/draft/`

前置条件（Adapter 调用前必须满足）：

| 条件 | 要求 |
|------|------|
| `Status` | 仅 `VALIDATED` 可进入 Adapter |
| `EnableExecute` | 必须为 `false` |
| `SourceDecisionID` / `SourceSnapshotHash` | 非空，且仍匹配 sealed snapshot |
| `VerifyDraftIntegrity` | OK |
| `VerifyDraftAuthority` | OK（可读 Decision；不可改；不可授权 Execution） |

### 1.2 输出：`TradePlanCandidate`（设计类型，未实现）

**不是** `models.TradePlan`。建议未来包路径：`backend/tradeplan/adapter/`（Phase5-B+ 再实现）。

```text
TradePlanCandidate
├── CandidateID              # 适配器生成的稳定 ID（非 DB PK）
├── SourceDraftID            # ← Draft.DraftID
├── SourceDecisionID         # ← Draft.SourceDecisionID
├── SourceSnapshotHash       # ← Draft.SourceSnapshotHash
├── DraftHash                # ← Draft.DraftHash（溯源）
├── TradeDate
├── StockCode / StockName
├── Side                     # 映射自 Payload.Side（校验枚举）
├── IntentKind               # 由 Action.Code 解释得到的「意图类别」，非执行命令
├── SuggestedTargetVolume    # ← Payload.TargetShares（建议量，非下单量）
├── SuggestedTargetAmount    # ← Payload.TargetAmount
├── SuggestedLimitPriceHint  # ← EntryPriceHint / zone（仅提示）
├── SuggestedStopPriceHint   # ← StopPrice（仅提示；现有 TradePlanItem 无此列）
├── MarketLevelHint          # ← Payload.MarketLevel
├── GateReadyHint            # ← Payload.GateReady（解释用）
├── MappingNotes[]           # 映射告警 / 降级说明
├── Executable               # 恒为 false（Phase5）
└── AdapterVersion           # 契约版本，建议 "1"
```

硬规则：

- `Executable == false` 恒成立（与 Draft `EnableExecute=false` 对齐）
- **禁止** `Action.Code → CreateOrder / EnableExecute=true / Status=ready`
- **禁止** Adapter 调用 `BuildTradePlan` / `CreatePlanWithItems` / `ExecutePlanItem` / Broker

---

## 2. Draft → Existing TradePlan 字段映射表

现有模型：`backend/models/trade_plan.go`（`TradePlan` + `TradePlanItem`）。

| Draft 字段 | Existing TradePlan / Item | 处理策略 |
|------------|---------------------------|----------|
| `sourceDecisionId` | **无对应列** | **新增建议**：未来在 `TradePlanItem` 或 plan 扩展 JSON 保留溯源；Adapter 阶段写入 `TradePlanCandidate.SourceDecisionID` |
| `sourceSnapshotHash` / `snapshotHash` | **无对应列** | **新增建议**：同上，写入 Candidate；禁止静默丢弃 |
| `draftId` / `draftHash` | **无对应列** | Candidate 保留；不进现有表直至显式 migration |
| `payload.side` | `TradePlan.Side` / `TradePlanItem.Side` | **可映射**（字符串 `buy\|sell\|none`）；`none` 不得生成可执行 item |
| `payload.targetShares` | `TradePlanItem.TargetVolume` | **可映射为建议量**；注意现网 `BuildTradePlan` 常不填 Volume，执行时按金额/行情重算 |
| `payload.targetAmount` | `TradePlanItem.TargetAmount` | **可映射**；与现网 PlanFilter 金额语义对齐需校验 |
| `payload.entryPriceHint`（price zone / instant） | `TradePlanItem.LimitPrice` | **弱映射 / 提示**；现网开盘路径多用实时行情，LimitPrice 常为空 |
| `payload.stopPrice` | **无对应列** | **缺失字段**：仅进 Candidate `SuggestedStopPriceHint`；禁止伪造进 Order |
| `payload.actionCode` | **无对应列** | **禁止直接控制**执行；仅映射为 `IntentKind`（解释） |
| `payload.actionLabel` | `TradePlanItem.Reason`（可选） | 展示/备注；不得驱动下单 |
| `payload.allowDraft` | `TradePlan.EnableExecute` | **禁止推导**：`AllowDraft=true` ≠ `EnableExecute=true` |
| `enableExecute`（Draft） | `TradePlan.EnableExecute` | Draft 恒 false；Candidate.Executable 恒 false；**禁止**写出 true |
| `payload.marketLevel` | `TradePlan.MarketLevel` | 可提示映射；现网来自 RiskFilter |
| `payload.gateReady` | **无直接列** | 解释字段；不可单独放行执行 |
| `payload.stockCode/Name/tradeDate` | Item / Plan 同名字段 | **直接映射** |
| `status`（CREATED/VALIDATED…） | `TradePlan.Status`（draft/ready/executing…） | **命名空间不同**；禁止 `VALIDATED`→`ready` 自动跃迁 |
| Candidate Pool `Rank/Score` | `TradePlanItem.Score` / Priority | Adapter **不读取、不改写** Rank/Score（Phase3/4 冻结） |

### Action.Code 处理（冻结）

```text
Action.Code ──✗──> CreateOrder
Action.Code ──✗──> TradePlan.Status=ready
Action.Code ──✗──> EnableExecute=true
Action.Code ──✓──> TradePlanCandidate.IntentKind（解释分类）
```

建议 `IntentKind` 枚举（设计）：`ENTER_HINT | WAIT_HINT | WATCH_HINT | REDUCE_HINT | BLOCKED_HINT | OTHER`  
由 Code 投影，**不授权**。

---

## 3. 现有 TradePlan 生命周期（只读扫描结论）

### 3.1 创建入口

| 入口 | 路径 | 说明 |
|------|------|------|
| 主构建 | `strategy.BuildTradePlan` | CandidatePool + `risk.PlanFilterResult` → 落库；**直接 `Status=ready`** |
| 调试/日批 | `strategy.BuildTradePlanForDate` / `RunDailyCandidateAndPlan` | 池 → PlanFilter → BuildTradePlan |
| App 暴露 | `App.BuildTradePlan` / `App.RunDailyCandidateAndPlan` | Wails TEMP/调试与 cron 9:20 |
| 落库 | `data.TradePlanRepo.CreatePlanWithItems` | 写 `trade_plans` + `trade_plan_items` |

**注意：** 现网创建时已是 `ready`，模型虽有 `TradePlanStatusDraft` 常量，构建路径未走独立 draft 态。

### 3.2 状态流转（Plan）

```text
ready ──TryBeginExecute(CAS)──> executing
executing ──FinishPlanCAS──> done | partial | failed | skipped
ready/executing ──superseded──> superseded（同日新版本）
stuck executing ──ReconcileTradePlan──> done/partial/failed/...
```

常量：`draft | ready | executing | done | partial | skipped | failed | superseded`  
（`backend/models/trade_plan.go`）

Item：`pending | filled | skipped | error`（执行回写 OrderID/Fill）

### 3.3 消费者

| 消费者 | 用途 |
|--------|------|
| `data.RunPaperOpenBuyOnce` | 取当日 `ready` Plan → CAS executing → 行情/手数 → 下单 |
| `data.daily_trading_status` | 观测 Plan 阶段 |
| `data.trade_analysis_repo` | 分析 Pool↔Plan |
| `data.trade_plan_reconcile` | 卡住 executing 和解 |
| UI / App | 查询今日计划、手动触发开盘买 |

### 3.4 Execution 调用链（与 Decision 隔离）

```text
RunPaperOpenBuyOnce
  → TradePlanRepo.GetReadyByTradeDate (status=ready)
  → TryBeginExecute (ready→executing)
  → 行情 → calc volume
  → PlanItemExecutor / ExecutionService.ExecutePlanItem
  → PaperBroker / RealBroker（broker 包）
  → Order / Fill 回写 TradePlanItem
```

补充旁路（**不经 Decision**）：

- `execution.ResearchTradeFacade` / `ManualTradeFacade`：Intent → 临时 `TradePlanItem` → `ExecutePlanItem`

`backend/broker/*`：**不 import** TradePlan / Decision（Broker 只接 Execution 订单接口）。

`backend/tradeplan/draft/*`：**禁止** import execution / broker（已冻结）。

---

## 4. Adapter 边界与禁止关系

### 4.1 允许

```text
Decision → Draft → Adapter → TradePlanCandidate
Decision → Observability / Shadow Compare
```

### 4.2 禁止（冻结）

```text
Decision ──────────────✗──────────> Execution
Decision.Action.Code ──✗──────────> CreateOrder
Draft.AllowDraft ──────✗──────────> EnableExecute=true
Adapter ───────────────✗──────────> models.TradePlan 落库
Adapter ───────────────✗──────────> BuildTradePlan / RunPaperOpenBuyOnce
TradePlanCandidate ────✗──────────> Broker API
```

### 4.3 与 Phase3-D Authority 对齐

| 角色 | Read Decision | Modify | CreateTrade |
|------|---------------|--------|-------------|
| `TRADEPLAN_DRAFT_FUTURE` | YES | NO | NO |
| `EXECUTION_FORBIDDEN` | NO | NO | NO |

未来若增加 `TRADEPLAN_ADAPTER` 角色：建议 **仅 Read Draft + Emit Candidate**；`CreateTrade=false`。

---

## 5. 映射风险与缺失字段

### 5.1 高风险

1. **现网 Plan 创建即 `ready`**：若误把 Candidate 当成 Plan 落库，会立刻进入开盘执行资格。  
2. **Action.Code 语义过强**：ENTER 易被误当作下单许可。  
3. **Volume 语义不一致**：Draft 有 `targetShares`；现网执行常按 `AmountPerStock`/行情重算。  
4. **无 Decision 溯源列**：丢失 `sourceDecisionId` / hash 会导致无法审计与 stale 检测。

### 5.2 缺失字段（相对 Existing TradePlan）

| 能力 | Draft/Decision 有 | TradePlan 表 |
|------|-------------------|--------------|
| Decision 溯源 | DecisionID / SnapshotHash | 无 |
| Stop | StopPrice | 无 |
| GateReady | 有 | 无 |
| Zone 全文 | EntryZone | 仅可能 LimitPrice |
| Draft 生命周期 | CREATED/VALIDATED… | 不同枚举 |

### 5.3 低风险 / 可对齐

- `tradeDate` / `stockCode` / `stockName` / `side` / `targetAmount` / `marketLevel` 形状接近。

---

## 6. 未来接入步骤（不在 Phase5-A 实现）

| 阶段 | 内容 | 仍禁止 |
|------|------|--------|
| **5-B** | 实现 `adapter` 包：`DraftToCandidate()` + golden | 落库 / Execution |
| **5-C** | Candidate 批次聚合（多标的）+ 与 Pool 对齐校验（只读） | Rank 重算 |
| **6-A** | 可选：TradePlan 表增加溯源列 migration 设计 | 自动 ready |
| **6-B** | 人工确认门：Candidate → 显式 `BuildTradePlan` 输入 | Decision 直连执行 |
| **永不** | Decision/Draft 自动 `EnableExecute` | — |

---

## 7. 禁止事项清单（验收用）

- [ ] 不修改 Execution / PaperBroker / Broker API / Order 代码  
- [ ] 不修改 TradePlan 生命周期与 `BuildTradePlan`  
- [ ] 不修改 Decision Schema  
- [ ] 不实现 Adapter 代码（Phase5-A Design only）  
- [ ] 文档明确：输出为 `TradePlanCandidate`，不是 `TradePlan`  
- [ ] `Action.Code` 禁止直接控制执行  

---

## 8. Phase5-A Design Report 摘要

### 当前 TradePlan 架构

日批：`CandidatePool → PlanFilter → BuildTradePlan(ready) → 落库`；  
执行：`ready → executing → ExecutePlanItem → Broker → Order/Fill`。  
Decision / Draft **未**接入该链。

### Draft 映射风险

溯源字段缺失、创建即 ready、Volume/金额双轨、Action 易被误授权。

### Adapter 边界

只产 `TradePlanCandidate`；`Executable=false`；保持 Decision→Execution 断路。

### 缺失字段

`sourceDecisionId`、`snapshotHash`、`stop`、`gateReady`、zone 细节。

### 未来接入步骤

见 §6；下一步实现仅为 5-B Candidate 映射 harness。

### 禁止事项

见 §7；本阶段 **零代码行为变更**（仅本设计文档）。
