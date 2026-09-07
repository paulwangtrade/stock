# PHASE17 Trading Session / Execution Batch 架构审计（只读）

**日期：** 2026-09-07  
**性质：** **只读架构审计**（未新增模型 / 未改代码 / 未改交易链）  
**背景：** 未来可能并存 Strategy 生成计划、Watchlist 手工计划、模型推荐、多市场计划。  
**问题：** 是否需要显式 **TradingSession** 层？  

**上游：**  
- [PHASE10_C8B_TRADING_SESSION_STATE_DESIGN.md](./PHASE10_C8B_TRADING_SESSION_STATE_DESIGN.md)  
- [PHASE10_C_SESSION_SEPARATION_DESIGN.md](./PHASE10_C_SESSION_SEPARATION_DESIGN.md)  
- [PHASE10_D4_CAPITAL_ALLOCATION_ARCHITECTURE_DESIGN.md](./PHASE10_D4_CAPITAL_ALLOCATION_ARCHITECTURE_DESIGN.md) / D5  
- `models.TradePlan` · `paper_sim_runs` · SameDayCandidates  

---

## 0. 四问总答

| # | 问题 | 结论 |
| --- | --- | --- |
| 1 | 多来源同时存在是否冲突？ | **会争用，但不必然炸库**：同日可多 plan；执行入口 **只选一条 Upcoming**；未做跨来源预算合并 |
| 2 | 资金分配在哪里决定？ | **主要在 Draft 生成时的 Sizer**（默认 FixedAmount；可选 portfolio_aware）；**无**跨 plan 的统一 CapitalAllocation 会话层；现金不足靠 reject / cash_rescale |
| 3 | 执行批次是否缺失？ | **半有**：`paper_sim_runs` ≈ 单 plan 执行批次；**缺**跨 plan / 跨来源的 ExecutionBatch；Session A/B 幂等仍偏「单 plan×日」 |
| 4 | 未来是否需要 TradingSession / ExecutionBatch / CapitalAllocation？ | **多来源并行或组合级预算时需要**；单主策略路径可继续用现网 + 标注；**建议按阶段引入，不必立刻建新表** |

---

## 一、现网 TradePlan 来源与执行骨架

### 1.1 来源字段（已有）

| 字段 | 作用 |
| --- | --- |
| `id` / `plan_id` | 计划主键；fill/order/run 挂接 |
| `trade_date` | 执行日 |
| `source_session` | 来源观测：`after_close` / `morning_rebuild` / `cash_rescale` / `t_sell` / `exit_review` / `watchlist` … |
| `source_kind` / `parent_plan_id` | 缩放等血缘（cash_rescale） |
| `pool_id` | → CandidatePool（策略路径） |
| `side` | buy / sell |
| `status` + `freeze_at` | draft → ready(frozen) → executing → 终态；同日可 `superseded` |
| `amount_per_stock` / `max_names` | 单票金额与名额（Sizer 产物） |
| `decision_provider` / `allocation_version` 等 | 决策/分配元数据（G 系列） |
| `enable_execute` | 是否允许执行 |

**没有：** `TradingSession` 实体、`ExecutionBatch` 实体、跨 plan 的 `capital_budget_id`。

### 1.2 执行链（单 plan）

```text
Cron / Manual
  → Gateway.RunExecution(session A|B, fillMode)
  → 选定 Frozen TradePlan（Upcoming 规则）
  → PaperSimRun (trade_date, plan_id)     ← 观测 + 幂等
  → Orders / Fills
```

`source_session` 影响：**产品分桶、晨间物化隔离卖单、UI 候选列表**；**不是**资金会话 ID。

### 1.3 「Session」一词的多重含义（易混）

| 用语 | 现网含义 | 是否实体层 |
| --- | --- | --- |
| `source_session` | **计划来源标签** | 列字段 |
| fillMode Session A/B | **开盘/收盘成交窗口** | Gateway/Job 策略 |
| Signal `midday`/`close` | **扫描时段** | Snapshot |
| FE `tradingSession.js` | **A 股时钟/K 线 bar 选择** | 前端工具 |
| C.8B Trading Session State | **交易日就绪状态机（设计）** | **未完整产品化为独立表** |

---

## 二、多来源并存：是否冲突？

### 2.1 已支持并存的证据

- 同日可存在 Strategy（`after_close`）与 Watchlist（`watchlist`）等多条非终态 plan。  
- `ListSameDayCandidates`：**只读列出**多来源候选，**不改** Upcoming 选择。  
- 卖单（`t_sell` / `exit_review`）与买单物化 **隔离**（`sell_isolation`）。  
- 同日新 Frozen 可 **supersede** 旧 ready。

### 2.2 冲突形态（真实风险）

| 冲突类型 | 现状 | 后果 |
| --- | --- | --- |
| **执行选主** | `GetUpcomingTradePlan`：按日优先 frozen ready，再 `plan_version DESC` | 其它来源 draft/ready **可能永不自动执行** |
| **资金争用** | 各 plan 独立 sizing；无跨 plan 预留 | 先执行者耗尽现金 → 后者 `insufficient_cash` reject |
| **名额/重复票** | 无跨 plan 去重闸 | 可能同票出现在策略计划与手工计划（靠人工/风控） |
| **幂等** | Run 键偏 `(trade_date, plan_id)` | 同日多 plan 可各跑一趟；**不是**一个 Batch 协调 |
| **多市场** | 基本 A 股单市场假设 | 尚无 market_id 会话边界 |

**结论：** 多来源 **允许共存于库**，执行层是 **「单赢家 Upcoming」**，不是 **「多计划联合会话」**。冲突表现为**选主与预算**，而非主键碰撞。

---

## 三、资金分配在哪里决定？

```text
[生成 Draft]
  Snapshot (可选)
    → allocation 包 / portfolio_aware Sizer（可选路径）
    → 或 FixedAmount（默认常数/票）
  → TradePlan.amount_per_stock + items.target_volume
  → （可选）cash_rescale 二次缩放

[执行]
  Broker 扣现金 / reject
  ← 无「本 Session 总预算」运行时账户
```

| 层 | 现网 | 备注 |
| --- | --- | --- |
| Capital Allocation（D4/D5） | `allocation` / `allocationengine` **有计算包** | 早期报告称未接 Default 路径；portfolio_aware 为可选模式 |
| PositionSizer | FixedAmount **默认**；portfolio_aware **可选** | 决定**单 plan** 怎么分 |
| Cash rescale | 生成后按可用现金缩放 | **事后修补**，非会话预算 |
| 跨 Watchlist + Strategy | **无** | 两套 draft 各算各的 |

**资金分配决策点 = 各来源自己的 Draft 生成时，而非 TradingSession。**

---

## 四、执行批次是否缺失？

### 4.1 已有近似物

| 对象 | 粒度 | 作用 |
| --- | --- | --- |
| `paper_sim_runs` | **一个** `(trade_date, plan_id)` + trigger | 执行账本、幂等、观测 |
| Gateway session A/B | 时间窗 / 价格模式 | 开盘或收盘 fill |
| Plan `executing`→终态 | 单 plan 生命周期 | CAS 防重入 |

### 4.2 相对「ExecutionBatch」的缺口

| 能力 | 现状 |
| --- | --- |
| 一批内多 plan（策略买 + watchlist 买 + 卖）统一编排 | **无** |
| Batch 级预算 / 优先级 / 部分成功策略 | **无** |
| Batch 级幂等（跨 plan） | **无** |
| 观测模式 A+B 同日同 plan | 设计有；幂等键历史上缺 session（C 设计文档） |

**结论：** 有 **Plan-Run**，缺 **Batch**；多来源并行时批次层缺口会放大。

---

## 五、未来是否需要三层？

### 5.1 建议裁定

| 概念 | 是否需要 | 何时 |
| --- | --- | --- |
| **CapitalAllocation** | **需要（逻辑层）**；表可选 | 多来源抢现金 **之前**就应先统一「本轮可花多少」；包已有雏形，缺的是 **跨 plan 会话绑定** |
| **ExecutionBatch** | **需要（当多 plan 同窗执行）** | Watchlist + Strategy 同日都要自动执行，或一笔运营触发多计划时 |
| **TradingSession** | **需要（编排层，未必先建表）** | 统一「交易日 + 市场 + 窗口 + 参与计划集合 + 预算」；可先做 **只读 Session View** 包装现有 cron/upcoming |

### 5.2 推荐演进（不新增模型于本审计）

```text
阶段 0（现网）
  单主 Upcoming + source_session 标签 + PaperSimRun

阶段 1（产品规则，少建表）
  SameDayCandidates 显式「选主 / 手工指定执行 plan」
  CapitalAllocation 接入所有 Draft 路径（含 watchlist）
  文档化：同日仅一买主计划可自动执行

阶段 2（真多来源并行）
  TradingSession（逻辑）：trade_date + market + window + budget_id
  ExecutionBatch：session 下有序执行 plan 列表 + 共享预算消耗
  Run 幂等含 session/batch

阶段 3（多市场）
  Session 增加 market_id；日历/规则分市场
```

### 5.3 何时**不必**上 TradingSession 实体

- 仍坚持 **每日单一自动买入 Frozen 计划**  
- Watchlist / 模型推荐仅 **人工 Approve 后替换或 supersede** 主计划  
- 卖单始终独立、不与买共享预算会话  

此时强化 **Upcoming 选主规则 + CapitalAllocation 单 plan** 即可。

---

## 六、风险摘要

| 风险 | 说明 |
| --- | --- |
| 误以为 `source_session` = TradingSession | 仅来源标签 |
| 多 ready 计划同时「以为都会执行」 | 实际只跑 Upcoming 赢家 |
| FixedAmount × 多来源 | 现金冲突隐蔽 |
| 过早建表 | 在选主策略未定前引入 Batch 会空转 |

---

## 七、合规确认

| 要求 | 状态 |
| --- | --- |
| 只读审计 | **是** |
| 未新增模型 | **是** |

---

## 附录 — 关键路径

- `backend/models/trade_plan.go` — source_session / 生命周期  
- `backend/data/trade_plan_candidates.go` — SameDayCandidates  
- `backend/data/trade_plan_visibility.go` — Upcoming 选主  
- `backend/papertrading/models.go` — `PaperSimRun`  
- `backend/papertrading/gateway.go` — RunExecution  
- `backend/morningpreparation/sell_isolation.go`  
- `backend/allocation*` / `positionsizing` — 资金与 sizing  

---

*Phase17 Trading Session / Execution Batch 架构审计结束。*
