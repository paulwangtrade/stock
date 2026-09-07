# TradePlan Proposal Boundary（Phase6-A）

> 状态：**Design only（2026-07-21）**  
> 范围：定义 `TradePlanCandidate` → `TradePlanProposal` →（显式人工门）→ Existing `TradePlan` 的隔离边界  
> **本阶段禁止改代码**；禁止接 Execution / PaperBroker / Broker / Order；禁止改 `BuildTradePlan`、TradePlan DB 写入、Candidate Rank/Score、Decision Schema

---

## 0. 目标架构（冻结）

```text
DecisionSnapshot
        |
        v
TradePlanDraft              # EnableExecute=false
        |
        v
TradePlanCandidate          # Executable=false（Shadow）
        |
        v
TradePlanProposal           # Phase6-A：提案边界（本文件，未实现）
        |
        |  human gate + provenance OK
        |  ✗ shadow equal ≠ authorize
        v
Existing TradePlan          # 现网 ready / BuildTradePlan 链（仍独立）
        |
        ✗（Phase6-A 禁止）
        v
Execution / PaperBroker / Order
```

**Proposal 不是 TradePlan，也不是 Order。**  
**Proposal 通过 ≠ 可执行；仅表示「可进入人工确认 / 显式晋升讨论」。**

---

## 1. 为什么需要 Proposal（相对 Phase5）

| Phase | 产物 | 能力 | 缺口 |
|-------|------|------|------|
| 5-B | `TradePlanCandidate` | Draft → Candidate 投影 | 不碰 Existing Plan |
| 5-C | Shadow Diff | Candidate ↔ Existing Plan 对比报告 | `equal=true` 易被误读为授权 |
| **6-A** | **`TradePlanProposal`（设计）** | **显式「提案」态 + 门禁契约** | **仍不落库、不执行** |

Phase5-C 结论：强字段原生覆盖不足（Stop / DecisionID / SnapshotHash）；Status/Order/Fill 必须 ignore / forbidden。  
因此 Phase6 **不得**把 Shadow 结果直接接到 `BuildTradePlan` 或 `EnableExecute=true`。

---

## 2. 输入 / 输出契约

### 2.1 输入 A：`TradePlanCandidate`（已存在）

来源：`backend/tradeplan/candidate/`

前置条件（进入 Proposal 投影前必须满足）：

| 条件 | 要求 |
|------|------|
| `Executable` | 必须为 `false` |
| `Validation.OK` | 必须为 `true` |
| `SourceDecisionID` / `SourceSnapshotHash` | 非空 |
| `CandidateHash` | 非空且与内容一致（防篡改） |
| Intent | 仅为 `*_HINT`；禁止映射为执行命令 |
| Authority | 可读 Decision / CreateCandidate；**CreateTrade=false** |

### 2.2 输入 B（可选）：Shadow Diff

来源：`backend/tradeplan/shadow.CompareTradePlanCandidate`

用途：

- 作为 Proposal 的**附带证据**（coverage / field diffs / risk flags）
- **不得**单独作为 `Approved` / `Promote` 条件

硬规则：

```text
ShadowDiff.Equal == true  ──✗──>  Proposal.AuthorizedExecute
ShadowDiff.Equal == true  ──✗──>  TradePlan.Status=ready
ShadowDiff.Equal == true  ──✗──>  EnableExecute=true / CreateOrder
ShadowDiff.Equal == true  ──✓──>  Proposal.Evidence.ShadowEqual（仅观测）
```

### 2.3 输出：`TradePlanProposal`（设计类型，未实现）

建议未来包路径：`backend/tradeplan/proposal/`（Phase6-B+ 再实现）。

```text
TradePlanProposal
├── ProposalID                 # 提案稳定 ID（非 DB PK / 非 Plan ID）
├── SourceCandidateID          # ← Candidate.CandidateID
├── SourceDraftID              # ← Candidate.SourceDraftID
├── SourceDecisionID           # ← Candidate.SourceDecisionID
├── SourceSnapshotHash         # ← Candidate.SourceSnapshotHash
├── CandidateHash              # ← Candidate.CandidateHash（完整性）
├── TradeDate / StockCode / StockName
├── Side
├── ProposedTargetVolume       # ← TargetShares（提案量，非下单量）
├── ProposedLimitPriceHint     # ← EntryPriceHint
├── ProposedStopPriceHint      # ← StopPriceHint（现网无列则仅提案内保留）
├── IntentKind                 # 解释；不授权
├── Evidence
│   ├── ShadowEqual            # bool（观测）
│   ├── MissingFields[]
│   ├── DifferentFields[]
│   └── RiskFlags[]
├── Gate
│   ├── Status                 # DRAFT | REVIEWABLE | BLOCKED | REJECTED
│   ├── HumanRequired          # 恒 true（Phase6）
│   ├── AutoPromoteForbidden   # 恒 true
│   └── BlockReasons[]         # 如 missing_provenance / forbidden_field
├── AuthorizedExecute          # 恒 false（Phase6-A/B 设计冻结）
├── PromoteToLivePlanAllowed   # 恒 false（直至显式后续阶段 + 人审）
└── ProposalVersion            # 契约版本，建议 "1"
```

硬规则：

- `AuthorizedExecute == false` 恒成立  
- `HumanRequired == true` 恒成立  
- `AutoPromoteForbidden == true` 恒成立  
- **禁止** Proposal 调用 `BuildTradePlan` / `CreatePlanWithItems` / `RunPaperOpenBuyOnce` / Broker  
- **禁止** Proposal 写入 `trade_plans` / `trade_plan_items`（本阶段甚至不实现 repo）

---

## 3. Proposal Boundary（允许 / 禁止）

### 3.1 允许

```text
Candidate ──✓──> TradePlanProposal（投影 + Gate 评估）
Shadow Diff ──✓──> Proposal.Evidence（只读附带）
Proposal ──✓──> Observability / Audit 报告
Proposal ──✓──> 人工 Review UI（未来；只读展示）
```

### 3.2 禁止（冻结）

```text
Proposal ──────────────✗──────────> Execution / PaperBroker / Order
Proposal ──────────────✗──────────> BuildTradePlan
Proposal ──────────────✗──────────> CreatePlanWithItems / TradePlan DB 写入
Shadow.Equal ──────────✗──────────> AuthorizedExecute=true
Candidate.Executable ──✗──────────> true（仍由 Candidate 层冻结）
Action.Code / IntentKind ──✗─────> CreateOrder / Status=ready
Decision ──────────────✗──────────> Execution（Authority 断路保持）
```

### 3.3 与现网生产路径的关系

现网保持不变：

```text
CandidatePool → PlanFilter → BuildTradePlan(ready) → PaperBroker
```

Decision / Draft / Candidate / Proposal 链与上图 **并行隔离**；Phase6-A **不合并**。

---

## 4. Gate 规则（设计）

### 4.1 `REVIEWABLE` 最低条件（仍不可执行）

全部满足才可将 Gate.Status 标为 `REVIEWABLE`：

1. Candidate 前置条件全部 OK（§2.1）  
2. `Side` ∈ {`buy`,`sell`}（`none` → `BLOCKED`）  
3. 无 `forbidden_field`（Order / Fill / Executing 出现在对照 Plan 时 → `BLOCKED`，不得当对齐成功）  
4. 溯源字段齐全：`SourceDecisionID` + `SourceSnapshotHash` + `CandidateHash`  
5. `AuthorizedExecute` 仍为 false；仅表示「人可看」

### 4.2 `BLOCKED` 典型原因

| BlockReason | 来源 |
|-------------|------|
| `missing_decision_provenance` | Plan/Candidate 缺 DecisionID |
| `missing_snapshot_hash` | 缺 SnapshotHash |
| `missing_stop_mapping` | Stop 无法落到现网列（风险提示；策略可选 block 或仅 flag） |
| `forbidden_field` | Order/Fill/Executing |
| `shadow_equal_not_authorization` | 文档级提醒：equal 不能放行（实现时写入 Notes，非放行码） |
| `intent_not_executable` | Intent 仅为 HINT |
| `auto_promote_forbidden` | 任何自动晋升尝试 |

### 4.3 明确非门禁条件

以下 **不得** 单独打开执行：

- Shadow `equal=true`
- Candidate 与 Existing Plan Side/Shares 一致
- Draft `VALIDATED`
- Gate `REVIEWABLE`

---

## 5. 溯源列设计（Migration Design Only）

> **本阶段只设计，不跑 migration，不改 models。**

### 5.1 建议落点（二选一，推荐 A）

**方案 A（推荐）：Item 级溯源列**

| 列 | 类型建议 | 说明 |
|----|----------|------|
| `source_decision_id` | `varchar(64)` indexed | ← Proposal/Candidate |
| `source_snapshot_hash` | `varchar(128)` | sealed snapshot |
| `source_candidate_id` | `varchar(64)` | 可选 |
| `source_proposal_id` | `varchar(64)` | 可选 |
| `stop_price_hint` | `double` nullable | 非强制进 Order |
| `intent_kind` | `varchar(32)` | 解释；不驱动执行 |

**方案 B：Plan 扩展 JSON**

- `provenance_json` / `proposal_evidence_json`  
- 优点：少改列；缺点：难索引、难 CAS 一致性校验

### 5.2 写入时机（未来，非 6-A）

仅在 **显式人工确认门通过** 且调用方为独立 `PromoteProposal`（名称示意）时写入；  
**禁止** 由 Shadow batch、cron、或 Decision Action 自动写入。

### 5.3 与现网 `ready` 的冲突

现网 `BuildTradePlan` **创建即 `ready`**。  
若未来 Proposal 晋升：

| 策略 | 说明 | Phase6-A 立场 |
|------|------|----------------|
| P0 | 晋升产物仍走独立表 / 独立 status 命名空间 | 推荐讨论 |
| P1 | 写入 Existing Plan 但强制 `EnableExecute=false` + 非 ready | 需改生命周期（大改，另开阶段） |
| P2 | 直接 `ready` | **永久禁止由 Decision/Proposal 自动触发** |

Phase6-A **不选择** P2；不实现 P0/P1。

---

## 6. Authority 对齐（设计）

现有角色（已实现）：

| Role | CreateCandidate | CreateTrade |
|------|-----------------|-------------|
| `TRADEPLAN_CANDIDATE_SHADOW` | YES | NO |
| `TRADEPLAN_DRAFT_FUTURE` | NO | NO |
| `EXECUTION_FORBIDDEN` | NO | NO |

建议新增（**未实现**）：

| Role | 能力 |
|------|------|
| `TRADEPLAN_PROPOSAL_SHADOW` | Read Decision + Emit Proposal；`CreateTrade=false`；`AuthorizedExecute=false` |
| （未来）`TRADEPLAN_PROMOTE_HUMAN` | 仅人审工具角色；仍不直连 Broker；与 Execution 分离 |

冻结：

```text
TRADEPLAN_PROPOSAL_SHADOW.CreateTrade = false
TRADEPLAN_PROPOSAL_SHADOW ──✗──> Execution
```

---

## 7. 字段映射：Candidate → Proposal →（未来）Plan

| Candidate | Proposal | Existing TradePlan / Item | Phase6-A |
|-----------|----------|---------------------------|----------|
| Side | Side | Side | 设计可映射 |
| TargetShares | ProposedTargetVolume | TargetVolume | 设计可映射（提案量） |
| EntryPriceHint | ProposedLimitPriceHint | LimitPrice | 弱映射 / 提示 |
| StopPriceHint | ProposedStopPriceHint | **无列** | 仅 Proposal 保留；migration 见 §5 |
| IntentKind | IntentKind | Reason 弱代理 / 未来 `intent_kind` | 解释 only |
| SourceDecisionID | SourceDecisionID | **无列** | 溯源设计；不静默丢弃 |
| SourceSnapshotHash | SourceSnapshotHash | **无列** | 同上 |
| Executable | AuthorizedExecute | EnableExecute | **两侧恒 false** |
| — | Gate.Status | Plan.Status | **命名空间隔离**；禁止 `REVIEWABLE`→`ready` |
| — | Evidence.* | Order/Fill/Executing | **禁止写入**；对照时 forbidden |

不比较 / 不晋升：

| 字段 | 原因 |
|------|------|
| Status（Plan） | 生命周期不同 |
| Executing / ExecutedAt | 执行态 |
| OrderID / Fill* | Broker 结果 |
| Profit / 盈亏 | 结果层 |
| Rank / Score | 冻结；Proposal 不读不改 |

---

## 8. 依赖冻结

```text
backend/tradeplan/proposal/   （未来）
       |
       +--> candidate / draft / decision（只读）
       +--> shadow（只读 Evidence）
       |
       ✗--> execution
       ✗--> broker
       ✗--> BuildTradePlan / strategy 写路径
       ✗--> data.TradePlanRepo 写路径
```

Phase6-A 实际交付：**仅本设计文档**；不创建 `proposal` 包。

---

## 9. 与 Phase6-B+ 的边界（预告，不实现）

| 阶段 | 内容 | 仍禁止 |
|------|------|--------|
| **6-A**（本文件） | Proposal 边界契约 + Gate + 溯源列设计 | 一切代码与 DB |
| **6-B** | 实现 `proposal` 包 + golden；可选只读 UI | DB 写入 / Execution |
| **6-C** | 人工确认门 API（approve/reject）+ 审计日志 | 自动 promote / Broker |
| **6-D**（可选） | migration 溯源列 + 显式 Promote（人审后） | Decision 直连执行；shadow equal 放行 |
| **永不** | Shadow equal / VALIDATED / REVIEWABLE → 自动 ready / CreateOrder | — |

---

## 10. 禁止事项清单（验收用）

- [ ] 不修改 Execution / PaperBroker / Broker / Order  
- [ ] 不修改 `BuildTradePlan` 与 TradePlan 生命周期  
- [ ] 不写入 TradePlan DB  
- [ ] 不修改 Candidate Rank/Score、Decision Schema  
- [ ] 不实现 `proposal` 包代码（Phase6-A Design only）  
- [ ] 文档明确：`Shadow.equal ≠ AuthorizedExecute`  
- [ ] 文档明确：Proposal 输出不是 live `TradePlan`  
- [ ] 文档明确：`HumanRequired` + `AutoPromoteForbidden`

---

## 11. Phase6-A Design Report 摘要

### Proposal 是什么

Candidate 与 Existing TradePlan 之间的**提案隔离层**：携带溯源、量价提示、Shadow 证据与人工门状态；**不授权交易**。

### 核心断路

```text
equal / VALIDATED / REVIEWABLE ──✗──> ready / EnableExecute / CreateOrder
Proposal ──✗──> Execution
```

### 进入后续实现的前置

1. 人审 Gate 契约已文档冻结（本文件）  
2. 溯源列方案已选定（推荐 Item 级列）  
3. 与现网「创建即 ready」冲突策略已显式拒绝自动 P2  
4. Authority 新角色仅设计、CreateTrade 保持 false  

### 本阶段交付

仅：`docs/phase6a_tradeplan_proposal_boundary.md`  
**零代码行为变更。**

### 是否可进入 Phase6-B

**可以进入 Phase6-B 实现讨论 / 实现 Proposal 投影 harness（仍 Shadow / 无 DB / 无 Execution）。**  
**不可以进入自动执行接线。**
