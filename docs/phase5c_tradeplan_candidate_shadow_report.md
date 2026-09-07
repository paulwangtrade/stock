# Phase5-C — TradePlanCandidate Shadow Batch Comparison Report

> 日期：2026-07-21  
> 状态：**Shadow only**（不改变生产路径）  
> 产物：`backend/tradeplan/shadow/` + `testdata/tradeplan_candidate_shadow_report.json`

---

## 1. 目标与边界

```text
DecisionSnapshot → TradePlanDraft → TradePlanCandidate
                                            |
                                         compare (shadow)
                                            |
                                            v
                                   Existing TradePlan
```

**禁止：** 修改 `BuildTradePlan`、TradePlan DB 写入、Execution / PaperBroker 接入、Candidate Rank/Score、Decision Schema。

---

## 2. 字段覆盖率

对 Candidate 侧关注字段（Side / TargetShares / EntryPriceHint / Stop / Intent / DecisionID / SnapshotHash）做评估：

| 指标 | 值（fixture batch） |
|------|---------------------|
| `candidateFieldsCovered` | **1.0**（7/7 字段均进入评估；含「缺失」也算已评估） |
| 可双向对齐的强字段 | Side、TargetShares、EntryPriceHint |
| 结构性缺失（Existing Plan 无列） | `plan.stopPrice`、`plan.sourceDecisionId`、`plan.snapshotHash`；Intent 仅能经 `Reason` 弱对齐 |

**结论：** 强映射约 **3/7 ≈ 43%** 有原生列；其余为缺失或弱代理。报告中的 `candidateFieldsCovered=1.0` 表示「均已扫描」，不等于「均可无损映射」。

---

## 3. 差异统计（Golden batch）

| Case | 结果 |
|------|------|
| Case1 Candidate≈Plan | `equal=true` |
| Case2 Shares 不同 | `field_diff=TargetShares`，`equal=false` |
| Case3 Status 不同 / executing | `ignored`（Status）；Executing → `forbidden_field`；语义字段仍可 `equal=true` |
| Case4 Order/Fill 出现 | `forbidden_field` |

典型字段统计（见 JSON `fieldStats`）：

- **Side**：equal 主导  
- **TargetShares**：存在 different（Case2）  
- **EntryPriceHint**：equal 或 weak_match（一侧为 0 时）  
- **Status / Item.Status**：ignored  
- **OrderID / Fill / Executing**：forbidden_field  

---

## 4. 缺失字段

| 缺失 | 风险含义 |
|------|----------|
| `plan.stopPrice` | Stop 无法落到现网 Plan 列 |
| `plan.sourceDecisionId` | 无法溯源 Decision |
| `plan.snapshotHash` | 无法做 snapshot 一致性 |
| `plan.intent`（无专用列） | 仅 `Reason` 弱代理 IntentKind |

RiskFlags 常见：`missing_stop_mapping`、`missing_decision_provenance`、`price_weak_match`、`intent_weak_match`、`forbidden_field`。

---

## 5. 风险列表

1. **创建即 ready**：Existing Plan 生命周期与 Candidate 影子语义不同；Status 必须 ignore。  
2. **执行层污染**：Order/Fill/Executing 一旦出现必须标 `forbidden_field`，不得当对齐成功。  
3. **量价双轨**：Shares/LimitPrice 与开盘重算路径可能不一致 → Shares/Price 差异属预期 shadow 信号。  
4. **溯源空洞**：无 DecisionID/Hash 列则无法做 stale / registry 闭环。  

---

## 6. 验收命令

```powershell
Set-Location D:\stock
go test ./backend/tradeplan/shadow/ -count=1
go test ./backend/tradeplan/candidate/ -count=1
go test ./backend/tradeplan/draft/ -count=1
go test ./backend/decision/authority/ -count=1
```

依赖冻结：`shadow` 生产代码不 import execution/broker，不引用 `BuildTradePlan`。

---

## 7. 是否具备进入 Phase6 条件

| 条件 | 状态 |
|------|------|
| Candidate Shadow 可批量对比 Existing Plan | **是** |
| 生命周期/执行字段隔离（ignore / forbidden） | **是** |
| 生产路径未改 | **是** |
| 字段原生覆盖不足（stop/溯源） | **缺口明确** |
| Candidate → 落库 TradePlan 自动接线 | **未做（禁止）** |

**Phase6 建议门槛：**

- 仅在「人工确认门 + 溯源列设计」就绪后，才考虑 Candidate → TradePlan 的**显式**接入；  
- Phase6 **不得**把 Shadow `equal=true` 直接等同于可执行授权；  
- 优先补齐：Decision 溯源字段设计、Stop/Intent 落库策略、与开盘 Volume 重算规则的对齐说明。

**结论：具备进入 Phase6 设计讨论的条件；不具备自动执行接线条件。**
