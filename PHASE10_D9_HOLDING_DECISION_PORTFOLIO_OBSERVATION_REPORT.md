# PHASE10-D.9 Holding Decision Portfolio Observation Report

> **类型：** 设计 + 观察层实现（不成交 / 不 Rebalance / 不改交易路径）  
> **日期：** 2026-08-11  
> **前置：** Portfolio Snapshot（D.2/D.5）· Holding Evaluation（C.5-B.1）· Holding Decision Engine（D.8）  
> **约束：** 不改买入 / FixedAmountSizer / TradePlan / Execution Gateway / 成交 / 模拟盘资金；不实现卖出与 Rebalance

---

## 1. 现有链路分析

### 1.1 Observation API（只读）

| 路径 | 数据来源 | 回答的问题 | 缺口 |
|------|----------|------------|------|
| `GET /dashboard/today` · `positions` | `paper_sim` 仪表盘 | 今日成交 / 净持仓表 | 无决策、无组合风险分布 |
| `GET /observation/metrics` | job/run 统计 | 运行质量 | 与持仓决策无关 |
| `GET /observation/positions/attribution` | fills → lots | 持仓从哪笔 fill 来 | 无 PnL 决策 |
| `GET /observation/holdings/evaluation` | Attribution + Quote overlay | 现在持有什么、表现如何 | D.8 已旁路 `holding_decision`，仍无组合汇总 |
| `GET /observation/holdings/exit-evaluation` | Exit 复评旁路 | 是否需要复评 | **不是** Holding Decision；易与卖出混淆 |
| Portfolio Snapshot 包 | `paper_sim_accounts` + `mark_price` | equity / cash / MV / exposure | **无 HTTP**；价格是落库 mark，不是 overlay |

前端 `PaperTradingObservation.vue` 已展示：账户仪表盘持仓、Attribution、Holding Evaluation 单票表、Exit 复评。  
**未展示：** Holding Decision 状态、组合级决策计数、REVIEW 市值占比。Evaluation 客户端仍只解析 `body.evaluation`，忽略 D.8 的 `holding_decision`。

### 1.2 三层已有能力（未收口）

```text
PortfolioSnapshot     账户：equity / cash / MV / exposure / position_count
Holding Evaluation    单票事实：cost / overlay 现价 / pnl / return / days / risk·profit·period
Holding Decision      单票结论：HOLD_NORMAL | HOLD_WATCH | HOLD_REVIEW | EXIT_CANDIDATE(默认关)
```

缺口：没有把「账户数字 + 单票事实 + 单票结论」合成 **组合观察**。  
现有 Evaluation `summary` 只有 lot 计数与总市值，**没有** decision histogram，也没有 REVIEW 权重。

### 1.3 价格源不一致（需在观察层标明，不得偷偷统一）

| 层 | 价格 |
|----|------|
| Snapshot | 落库 `mark_price`（不写 overlay） |
| Evaluation / Decision | GET-time Quote overlay |

组合观察：**账户块用 Snapshot**；**决策市值权重优先 Evaluation overlay**（与决策同一事实）。本阶段不改 Snapshot、不写 `mark_price`。

### 1.4 本阶段不做

Rebalance Diff、Target Portfolio、卖单、Sizer、TradePlan、Execution、Vue 接线（UI 仅设计）。

---

## 2. 新增观察模型

包：`backend/holdingdecision`（纯函数 `BuildPortfolioObservation`）。

### 2.1 `PortfolioSummary`

| 块 | 字段 | 来源 |
|----|------|------|
| 账户 | `total_equity` `cash` `market_value` `exposure` `available_cash` `position_count` | Snapshot |
| Decision 统计 | `hold_normal_count` `hold_watch_count` `hold_review_count` `exit_candidate_count` | Decision 股票级 |
| 风险分布 | `risk_distribution.normal/watch/danger/unknown` | Evaluation `risk_state` |
| 决策市值 | `decision_market_value.*` + `hold_review_weight` / `hold_watch_weight` | Evaluation MV（缺则 Snapshot） |
| 组合结论 | `portfolio_decision_state` / `reason` | 持仓中最差决策档 |
| 单票关联 | `holdings[]` | Evaluation ⋈ Decision（`action` 恒 `none`） |

`exit_candidate_enabled` 透出策略开关（生产默认 `false`）。  
`action` 恒为 `none`：观察 ≠ 卖出建议 ≠ Rebalance。

### 2.2 单票 → 组合聚合

```text
15 只持仓（示例）
  Decision: NORMAL 10 · WATCH 4 · REVIEW 1 · EXIT_CANDIDATE 0
  REVIEW 市值权重 = Σ MV(REVIEW) / Σ MV(全部决策持仓)
  组合决策状态 = max(NORMAL < WATCH < REVIEW < EXIT_CANDIDATE)
```

JOIN 键：`symbol`（股票级）。Lot 明细仍在 Evaluation / Decision 原 API，本 DTO 不重复展开 fill。

缺数据：沿用 D.8（不抬决策等级）；缺 MV 的票计入个数、权重按 0。

---

## 3. API 设计

**选择：独立 GET `/api/papertrading/observation/holdings/summary`。**

不把组合汇总塞进 Evaluation API：

| 方案 | 结论 |
|------|------|
| 扩展 evaluation 响应 | 会把账户 Snapshot 与单票 overlay 混在同一 payload；Evaluation 已有 `holding_decision` sibling |
| **独立 `/holdings/summary`** | 与 metrics / evaluation 同级只读资源；职责是组合收口；GET、不写库 |

数据流：

```text
GET /holdings/summary
  ├─ portfolio.Service.Snapshot()          只读 paper_sim
  ├─ BuildHoldingEvaluationObservation()   只读 + overlay（不写 mark）
  ├─ holdingdecision.EvaluateObservation() 纯计算 DefaultPolicy
  └─ BuildPortfolioObservation(...)        纯聚合
        → JSON { ok, summary }
        ✕ Broker / Gateway / TradePlan / 卖单 / Rebalance
```

只读：不改 cash、不改 `mark_price`、不改 fills。

---

## 4. UI 设计（本阶段不改 Vue）

目标：让用户看见账户状态、组合风险、决策分布、单票评价。  
**禁止**把 Decision 画成「建议卖出」或操作按钮。

建议在 `PaperTradingObservation.vue` 持仓分析区增加 **「组合观察」** 条（只读标签）：

1. **账户状态**（Snapshot）  
   权益 / 现金 / 市值 / 敞口 / 持仓数。脚注：账户市值来自落库 mark，与评价 overlay 可能不一致。

2. **组合风险状态**  
   展示 `portfolio_decision_state` 为观察标签：`持有正常` / `关注` / `复评`。  
   文案：「这是持仓观察结论，不是卖出指令。」  
   `EXIT_CANDIDATE` 若未来打开：显示「退出评估候选（仍非卖出）」。

3. **持仓决策分布**  
   四段计数：NORMAL / WATCH / REVIEW / EXIT_CANDIDATE。  
   旁注 REVIEW、WATCH 的市值占比（`hold_review_weight`）。

4. **单票详细评价**  
   复用现有 Holding Evaluation 表，增加只读列 `decision_state` + `decision_reason`（来自 summary.holdings 或 evaluation sibling）。  
   不增加「卖出」「减仓」按钮。

颜色建议：NORMAL 默认、WATCH info、REVIEW warning；不要用 error/红色「卖出」语义。

接线：`getPaperHoldingsSummary()` → 本 API；与现有 evaluation 并行 GET。失败时不影响买入交易 UI。

---

## 5. 测试结果

```text
go test ./backend/holdingdecision/ ./backend/api/ -count=1 -timeout 120s
  -run "TestBuildPortfolioObservation_|TestEvaluate_|TestHoldingsSummaryAPI_|TestHoldingsEvaluationAPI_"
ok  go-stock/backend/holdingdecision
ok  go-stock/backend/api
```

| 用例 | 结果 |
|------|------|
| 空组合 → 全 0 计数、`HOLD_NORMAL`、`action=none` | PASS |
| 正常盈利组合 → 全部 `HOLD_NORMAL` | PASS |
| 含 WATCH → 计数与 `hold_watch_weight` 正确 | PASS |
| 含 REVIEW → 组合状态 REVIEW，权重 = REVIEW 市值/总市值 | PASS |
| Decision 统计正确（含 EXIT 默认 0） | PASS |
| GET 后 cash / `mark_price` 不变；JSON 无 EXIT_NOW / 卖出 / Rebalance | PASS |
| POST `/holdings/summary` → 405 | PASS |
| 既有 Evaluation / D.8 Decision 用例 | PASS |

---

## 6. 是否影响交易

**否。**

| 路径 | 影响 |
|------|------|
| 买入 / TradePlan / FixedAmountSizer（约 100000/票） | 无 |
| Execution Gateway / 成交 / 模拟盘资金 | 无 |
| 卖出 / Rebalance | **未实现** |
| 数据库 | 无 schema、无写入 |
| Snapshot / Evaluation 公式 | 只读消费 |

本阶段建立稳定观察链：

```text
Portfolio Snapshot → Holding Evaluation → Holding Decision → Portfolio Observation
```

为未来 Target Portfolio → Rebalance Engine 提供只读基础，**不**在本层产生组合变更指令。
