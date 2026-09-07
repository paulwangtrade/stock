# PHASE17.4 Signal Outcome Projection — 设计审计报告

**日期：** 2026-09-07  
**性质：** 只读审计 + 设计（**本阶段不实现**）  
**目标：** 建立「信号结果归因」基础——记录信号产生后市场发生了什么。  

**明确不是：**

- 回测引擎  
- 策略优化  
- 选股改写  
- 生产交易变更  

**上游参照：**  
- [PHASE17_SIGNAL_RESEARCH_AUDIT.md](./PHASE17_SIGNAL_RESEARCH_AUDIT.md)  
- [PHASE16_D1_OUTCOME_MODEL_DESIGN.md](./PHASE16_D1_OUTCOME_MODEL_DESIGN.md)（成交域 Outcome，**域不同**）  
- [PHASE17_3_HOLDING_EVALUATION_SNAPSHOT_DESIGN.md](./PHASE17_3_HOLDING_EVALUATION_SNAPSHOT_DESIGN.md)  

---

## 0. 一句话结论

**原料充足，产品未接通：** Snapshot hit 已冻结 `signal_price` / `signal_time`；日 K 可算 T+1/3/5 与窗口 MFE/MAE；**尚无**挂在信号主键上的 `SignalOutcomeProjection`。  
本阶段只冻结只读模型与边界；**不建 Backtest Engine**。

---

## 1. 审计：信号来源

| 组件 | 角色 | 与 Outcome 的关系 |
| --- | --- | --- |
| **SignalScanSnapshot** | `signal_scan_snapshots`；`result_json` 存一批 hits | 信号发生批次锚点（`snapshot.id`） |
| **SignalScanHit** | JSON 内嵌；含 SignalEvent 最小字段 | **无独立 hit 主键**；研究行需合成 `signal_id` |
| **SignalPrice** | hit.`signal_price` + `signal_price_status`（frozen/missing/derived） | **冻结信号价**；禁止用 `NEW_PRICE` 回填（`NormalizeSignalScanHit`） |
| **CandidatePool / Item** | 池排序上下文；`SignalSnapshotID`、`SignalTag` | 桥到 TradePlan；**不是** forward 收益真源 |
| 图表冰点 markers | 前端按 K 重算 | **弱持久化**；不宜作本投影主键 |

### 1.1 信号身份缺口

| 需求 | 现状 |
| --- | --- |
| 稳定 `signal_id` | **无** DB 级 hit id |
| `stock_code` | ✅ `SECURITY_CODE` / `SECUCODE` |
| `signal_price` | ✅（可能 `missing`） |
| `signal_time` | ✅ 字符串；质量不一 |
| `signal_snapshot_id` | ✅ Snapshot.`id`；PoolItem 可回指 |

**合成 `signal_id` 建议（设计冻结，实现时二选一）：**

1. **推荐：** `"{snapshot_id}:{normalized_code}:{tag}:{signal_time}"`（缺省段用空占位）  
2. 或 SHA1 同上字段 → 定长 id  

**禁止：** 用 `CandidatePoolItem.id` / `TradePlanItem.id` / `fill.id` 冒充 signal_id（那是下游执行身份）。

---

## 2. 审计：交易链（旁路，非本投影必需）

```text
SignalScanSnapshot.hits (+ SignalPrice)
        ↓ signal_snapshot_id
CandidatePoolItem
        ↓ pool_id
TradePlan / TradePlanItem
        ↓
paper_sim_fills (buy/sell)
        ↓
Opportunity OutcomeProjection（成交 realized；域 = 机会/成交）
```

| 环节 | 可追踪？ | 对本投影含义 |
| --- | --- | --- |
| 信号 → Pool | 是 | 可选元数据；**不**驱动 T+N |
| Pool → TradePlan → Fill | 是 | **成交归因另域**（已有 Opportunity Outcome） |
| 未进池 / 未成交信号 | Snapshot 仍有 hit | **正是本投影要覆盖的主体** |

**裁定：** `SignalOutcomeProjection` **不依赖** Fill 是否存在。有成交可另链 Opportunity Outcome；本模型只回答「信号后价格路径」。

---

## 3. 审计：指标可否计算

基准约定（设计默认）：

- **锚价：** `signal_price`（`status=frozen` 优先；`missing` → 指标整行可空 / PENDING，**不**用现价伪造）  
- **锚日：** 由 `signal_time` 或 Snapshot.`trade_date` 解析交易日 `T0`  
- **序列：** 日 K（`obsfeedback` / marketdata Kline 同源思路）；窗口 = 锚日后第 1…N 个**有 K 的交易日**  
- **收益：** `(close_TN − signal_price) / signal_price`（比率；展示层可 ×100）  
- **MFE / MAE：** 窗口内相对锚价的最大有利 / 不利偏移（用 bar high/low；缺省用 close）

| 指标 | 可计算？ | 现网落点 | 缺口 |
| --- | --- | --- | --- |
| **return_t1** | ✅ 条件 | `obsfeedback` 有 horizon=1，但锚的是**持仓观测**，无 signal 主键 | 需信号锚定 Forward |
| **return_t3** | ✅ 条件 | obsfeedback **无** T+3 常量（仅 1/5/10） | 算法可复用 `nthTradingDayAfter`，扩展 N=3 |
| **return_t5** | ✅ 条件 | obsfeedback horizon=5；未挂 SignalHit | 同上 |
| **MFE** (`max_favorable_excursion`) | ✅ 条件 | obsfeedback 窗口 `max_profit`；Opportunity Performance **无** MFE 字段 | 信号锚价 + 日 K high |
| **MAE** (`max_adverse_excursion`) | ✅ 条件 | obsfeedback `max_drawdown` | 信号锚价 + 日 K low |

**数据不足时：** 未来 K 不足 → `PENDING` / 字段 null；**禁止**编造收益。

**与成交域区别：**

| | SignalOutcomeProjection | Opportunity OutcomeProjection |
| --- | --- | --- |
| 锚点 | signal_price @ signal_time | buy fill 价量 |
| 问句 | 信号后市场怎样 | 这笔买卖实现怎样 |
| 需要成交？ | 否 | 是（或 NO_TRADE） |

---

## 4. 设计：`SignalOutcomeProjection`

### 4.1 定位

只读研究投影：**信号发生 → 随后市场价格路径摘要**。  
不参与扫描参数、池排序、TradePlan 生成、Broker、Settlement。

### 4.2 字段（本阶段冻结）

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `signal_id` | string | 合成稳定 id（见 §1.1） |
| `stock_code` | string | 归一化代码 |
| `signal_price` | float64 | 冻结信号价；无效则不下假数 |
| `signal_time` | string / time | 信号时刻（保留原串 + 可选解析） |
| `return_t1` | *float64 | T+1 收盘相对信号价收益；不足则 null |
| `return_t3` | *float64 | T+3 |
| `return_t5` | *float64 | T+5 |
| `max_favorable_excursion` | *float64 | 窗口内最大有利偏移（建议窗口 = max horizon，默认至 T+5） |
| `max_adverse_excursion` | *float64 | 窗口内最大不利偏移 |

**建议可选元数据（非本阶段必填）：**  
`signal_snapshot_id`、`tag`、`strategy_id`、`session`、`signal_price_status`、`evaluated_at`、`quality`（complete/partial/pending）、`horizon_window`、`data_source_note`。

### 4.3 投影契约（未来实现指引）

```text
输入：SignalScanHit + snapshot_id (+ 日K bars)
输出：SignalOutcomeProjection
纯函数：读 K 线；不写 paper_sim_* / CandidatePool / TradePlan
```

伪流程：

```text
1. NormalizeSignalScanHit（保证不把 NEW_PRICE 当 signal_price）
2. 若 signal_price 无效 → 仅身份字段 + quality=pending/missing_anchor
3. 解析 T0 → 取日后 bars
4. return_tN = close(TN)/signal_price - 1（有第 N 根才填）
5. MFE/MAE = 窗口 high/low 相对 signal_price 的极值收益
```

**复用：** 可抽取/借鉴 `obsfeedback.nthTradingDayAfter` / `windowHighLow` 的数学，**新建信号域包装**；不要把 obsfeedback Record 误标为信号结果。

### 4.4 与相邻模型边界

| 模型 | 关系 |
| --- | --- |
| HoldingEvaluationSnapshot (17.3) | 持仓评价时点；**不**替代信号 forward |
| Opportunity Outcome | 成交 realized；可并行展示，**FK 可选**（via pool→plan），非必需 |
| `backtestEngine.js` | 单票交易模拟；**本设计不纳入、不扩展为引擎** |
| CandidatePool 评分 | **只读引用**；Outcome **禁止**回写 score/rank |

### 4.5 持久化（仅意图）

| 选项 | 说明 |
| --- | --- |
| A. 即时投影 API | MVP；适合按 snapshot 拉 hits 现算 |
| B. append-only `signal_outcome_projections` | cohort / 面板性能；与交易表隔离 |

本阶段**不建表、不实现 API**；默认倾向先 A 后 B。

---

## 5. 硬约束

1. **不改变生产交易**（Broker / Fill / Settlement / Unlock）。  
2. **不影响选股**（不改 CandidatePool 生成、Score、Rank、扫描命中规则）。  
3. **不建立 Backtest Engine**（无组合仿真、无参数寻优、无 walk-forward 框架）。  
4. **不**用成交价覆盖 `signal_price`。  
5. **不**把本投影结果喂回自动下单或 ExitEval 状态机。  

---

## 6. 阶段结论

| 问题 | 结论 |
| --- | --- |
| Signal 来源是否清晰？ | **是**（Snapshot → Hit + SignalPrice；Pool 为下游桥） |
| TradePlan / Fill 是否需要？ | **归因可选；forward 计算不需要** |
| T+1/3/5 / MFE / MAE？ | **数据可算；缺信号锚定投影产品** |
| 本阶段交付 | **本设计报告** |
| 下一步 | **暂停等待** — 不实现，直至授权 |

---

## 附录 — 关键路径

- `backend/models/signal_scan_snapshot.go` — Snapshot / Hit / SignalPrice  
- `backend/data/signal_price.go` — Normalize（禁 NEW_PRICE 回填）  
- `backend/models/candidate_pool.go` — `SignalSnapshotID`  
- `backend/opportunity/outcome/` — 成交域 Outcome（对照）  
- `backend/obsfeedback/` — T+N / 窗口极值算法参考（域勿混用）  

---

*PHASE17.4 设计审计结束 · 等待指令。*
