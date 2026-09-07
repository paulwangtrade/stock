# PHASE17.3 Holding Evaluation Snapshot — 设计审计

**日期：** 2026-09-07  
**性质：** 只读审计 + 设计（**本阶段不实现**）  
**目标：** 为未来「持仓复盘 / 卖出归因 / Alpha Research」提供可沉淀的历史评价能力。  

**原则：**

- 只记录评价  
- 不记录交易  
- 不影响执行  
- 不新增交易状态  
- 不改变 ExitEval 判定逻辑  

---

## 1. 当前能力审计

链路（已有、只读、请求时即时计算）：

```text
HoldingEval → Explanation (C2) → HealthScore (C3) → ExitEval（旁路拷贝，不驱动 Exit state）
```

**现状：无 `HoldingEvaluationSnapshot` 类型 / 表 / 持久化。** 评价仅活在 Observation API 内存投影中。

### 1.1 关键字段对照

| 字段需求 | HoldingEval | Explanation | HealthScore | 结论 |
| --- | --- | --- | --- | --- |
| `evaluation_time` / 等价时间 | View 级 `as_of`；行级无独立 `evaluation_time` | ✅ `evaluation_time` | ✅ `evaluation_time`（继承 Explanation） | **可用**（以 Explanation/Health 为准） |
| `stock_code` | ✅ | ✅ | ✅ | **可用** |
| `position_id` | ❌ 未透出 | ❌ | ❌ | **缺口**（账本有 `paper_sim_positions.id`，评价链未带入） |
| health_score / grade | — | — | ✅ `score` / `grade` | **可用** |
| supporting / risk factors | — | `hold_reasons` / `risk_hints` | ✅ `supporting_factors` / `risk_factors` | **可用**（Snapshot 建议取 Health 聚合结果） |
| price | ✅ `current_price` / `market_price` | ✅ `current_price` | 经 Explanation 间接 | **可用** |
| freshness | 行级 `quote_time`；无统一 freshness 枚举 | ✅ `freshness`（FRESH/STALE/UNKNOWN） | 经 Explanation | **可用** |

### 1.2 身份模型要点

- `paper_sim_positions`：**有**主键 `id`（即物理 position_id）。  
- HoldingEval / Attribution：**按 `stock_code`（+ account）聚合**；lots 用 `fill_id` / `plan_id`，**不暴露 position_id**。  
- 同一账户下 `(account_id, stock_code)` 唯一 → 运行时可从仓位表解析 `position_id`，但今日评价 DTO **尚未携带**。

### 1.3 可否生成 HoldingEvaluationSnapshot？

**可以。** 条件与限制：

| 项 | 裁定 |
| --- | --- |
| 能否从现有链投影一行 Snapshot？ | **是** — 在 HealthScore 完成后，对每个 `HoldingEvalStockRow` 做纯投影即可 |
| 是否已有历史时序？ | **否** — 需新增「评价快照」存储（或等价 append-only 记录）；本设计阶段只定模型，不落库实现 |
| 是否依赖 ExitEval？ | **否** — Snapshot 应挂在 HealthScore **之后、ExitEval 之前或并行旁路**；Exit 标签 **不入** Snapshot |
| `position_id` | 生成时需 **额外 lookup** `paper_sim_positions.id`；找不到可空（unattributed / 已平仓历史另议） |
| 是否影响执行？ | 设计上禁止：只读 append；无 Broker / Gateway / mark / TradePlan 写入 |

---

## 2. 设计：只读模型 `HoldingEvaluationSnapshot`

### 2.1 定位

**评价事实的时间点拷贝**（research / 复盘 / 归因输入），不是仓位账本，不是订单，不是 Exit 决策。

消费场景（未来）：

1. 持仓复盘：某日「当时认为持仓健康度如何」  
2. 卖出归因：卖出前后评价轨迹（**评价记录本身不含卖出事件**）  
3. Alpha Research：健康度 / 因子 / 价与 freshness 的面板特征  

### 2.2 字段（本阶段冻结建议）

| 字段 | 类型建议 | 来源 | 说明 |
| --- | --- | --- | --- |
| `id` | uint / UUID | 生成时分配 | Snapshot 自身主键（持久化时）；纯内存投影可省略 |
| `stock_code` | string | HoldingEval | 必填 |
| `position_id` | *uint | `paper_sim_positions.id` | 可空；**不**伪造 lot/fill id |
| `evaluated_at` | time | Explanation.`evaluation_time` 或 view `as_of` | 评价时刻；≠ 成交时间 |
| `health_score` | int | HealthScore.`score` | 0–100 |
| `health_grade` | string | HealthScore.`grade` | A/B/C/D |
| `supporting_factors` | []string | HealthScore | 评价因子拷贝 |
| `risk_factors` | []string | HealthScore | 风险因子拷贝 |
| `price` | *float64 | Explanation.`current_price` 或 HoldingEval | 评价所用展示价；**不是**强制 mark 写入 |
| `freshness` | string 或结构化 | Explanation.`freshness.status` | FRESH/STALE/UNKNOWN（建议存 overall status；细节可另存 JSON 扩展） |

**建议显式不收录（边界）：**

- ExitEval label / ExitPolicy 阈值结果  
- BUY/SELL / 可卖数量 / 锁定状态  
- 订单、成交、计划生命周期  
- 账户 cash / equity / daily_pnl  
- mark_price 账本变更事件  

可选扩展（**非本阶段必填**，留给实现 RFC）：`account_id`、`schema_version`、`data_source_note`、完整 `freshness` 对象、`explanation` 摘要哈希。

### 2.3 投影伪契约（未来实现指引）

```text
输入：HoldingEvalStockRow（已含 Explanation + HealthScore）+ optional position_id
输出：HoldingEvaluationSnapshot
纯函数：不写 Broker / Settlement / ExitEval state
```

推荐挂载点（一旦实现）：

```text
BuildHoldingEvaluationObservation
  → EnrichHoldingWithExplanations
  → EnrichHoldingWithHealthScores
  → [可选] ProjectHoldingEvaluationSnapshots   ← 新增旁路
  → BuildExitEvaluation / ProjectExitEvaluation  ← 不读 Snapshot，逻辑不变
```

### 2.4 持久化（仅设计意图，本阶段不实施）

| 选项 | 说明 |
| --- | --- |
| A. 无表，仅 API 即时投影 | 满足「能生成」；**不满足**历史复盘 |
| B. append-only 评价表 | 满足复盘 / Research；与交易表隔离 |
| C. 日终批写 | 与 Settlement **解耦**：可同 cron 触发，但 **不得** 改 mark / 日报公式 |

**本阶段裁定：** 模型与边界先冻结；落库策略留待实现阶段另开变更。默认倾向 **B（评价专用 append-only）**，与交易账本分表。

### 2.5 硬禁止

1. **不要**新增交易状态字段或状态机。  
2. **不要**让 Snapshot 写回或改写 ExitEval。  
3. **不要**用 Snapshot 覆盖 `mark_price` / 触发卖出。  
4. **不要**把卖出归因事件混进本模型（归因消费 Snapshot，不反向污染）。  

---

## 3. 风险与缺口摘要

| 风险 | 缓解 |
| --- | --- |
| 缺 `position_id` | 投影时 join `paper_sim_positions`；平仓后 id 仍可保留在历史 Snapshot |
| 即时评价无历史 | 需后续持久化；设计已预留 `id` + `evaluated_at` |
| Health 与 Explanation 因子不完全同构 | Snapshot 统一采用 HealthScore 的 factors（已含 Explanation 标签映射） |
| 与 Phase17.2 quote freshness 两套口径 | Snapshot.`freshness` 跟 **Evaluation Explanation** 口径；组合页 quote freshness 不混入本模型 unless 显式映射文档 |

---

## 4. 阶段结论

| 问题 | 结论 |
| --- | --- |
| 现有链是否已具备评价时间与标的？ | **是**（`evaluation_time` + `stock_code`） |
| 是否已有 `position_id`？ | **否**（需从仓位表补） |
| 是否可生成 HoldingEvaluationSnapshot？ | **是**（纯投影；历史能力依赖后续存储） |
| 本阶段交付 | **本设计文档** |
| 下一步 | **暂停等待** — 不实现、不改 ExitEval、不建表，直至明确授权 |

---

*PHASE17.3 设计审计结束 · 等待指令。*
