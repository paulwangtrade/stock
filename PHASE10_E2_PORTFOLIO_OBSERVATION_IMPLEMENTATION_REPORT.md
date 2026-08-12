# PHASE10-E.2 Portfolio Observation Implementation Report

> **类型：** 只读观察层实现（不成交 / 不自动调仓 / 不改交易路径）  
> **日期：** 2026-08-12  
> **前置：** [E.1 架构](./PHASE10_E1_PORTFOLIO_OBSERVATION_ARCHITECTURE_DESIGN.md) · Snapshot · Evaluation · Decision（D.8/D.9）· Rebalance Diff（D.12）  
> **约束：** 无自动调仓、无自动买卖、无 TradePlan / `RunDailyCandidateAndPlan` / Gateway / Broker / 成交 / 买入路径改动；不实现 Rebalance 执行

---

## Verdict

**PASS** — 新增统一 `GET /api/papertrading/observation/portfolio` 与「模拟盘观察 → 组合观察」只读 UI。  
组装层只投影已有 Snapshot / Evaluation / Decision / Diff，**不重算引擎、不写库、不改 `mark_price`、不触发交易**。部分数据缺失时仍返回可观察结果（HTTP 200 + `warnings`）。

---

## 1. 实现范围

### 1.1 新增包 `backend/portfolioobs`

| 文件 | 角色 |
|------|------|
| `types.go` | `AccountSummary` / `DecisionSummary` / `RebalanceSummary` / `PositionRow` / `Observation` |
| `assemble.go` | `Assemble(snap, eval, decision, diff, target, warnings, now)` 纯投影 |
| `assemble_test.go` | 空组合 + NORMAL/REVIEW + REMOVE；JSON 无 BUY/SELL |

`action` 恒为 `"none"`。免责声明固定：

> 观察结果不是交易建议。不生成买卖单，不执行调仓。

### 1.2 API（独立 GET，不污染既有观察端点）

```text
GET /api/papertrading/observation/portfolio
  ?target=identity
  &enter=sz000099
  &drop=sz000003
```

未改：

- `GET /observation/holdings/evaluation`
- `GET /observation/rebalance`
- `GET /observation/holdings/summary`

响应：

```json
{
  "ok": true,
  "disclaimer": "观察结果不是交易建议。不生成买卖单，不执行调仓。",
  "portfolio": {
    "account": { "total_equity", "cash", "market_value", "exposure", "position_count" },
    "decision": {
      "normal_count", "watch_count", "review_count", "exit_candidate_count",
      "normal_weight", "watch_weight", "review_weight", "exit_candidate_weight"
    },
    "rebalance": { "add_count", "increase_count", "decrease_count", "remove_count", "keep_count" },
    "positions": [
      {
        "symbol", "stock_name",
        "current_weight", "current_amount",
        "target_weight", "target_amount",
        "cost", "current_price", "pnl", "return",
        "decision_state", "decision_reason",
        "rebalance_action", "rebalance_reason",
        "action": "none"
      }
    ],
    "warnings": []
  }
}
```

Snapshot / Evaluation 失败 → 写入 `warnings`，仍 200，不阻断页面。

### 1.3 数据来源（不重新计算）

```text
Portfolio Snapshot          → 账户事实（落库 mark）
Holding Evaluation          → 行情 overlay / 盈亏
Holding Decision            → HOLD_NORMAL / WATCH / REVIEW / EXIT_CANDIDATE
Rebalance Diff + 观察 Target → KEEP / ADD / INCREASE / DECREASE / REMOVE
        ↓
portfolioobs.Assemble（投影）
        ✕ 自动调仓 / BUY·SELL Intent / Order / Gateway / TradePlan
```

决策计数与权重复用 `holdingdecision.BuildPortfolioObservation`（D.9）。  
Diff 复用 `rebalance.Diff`（D.12）。观察 Target 仍为 identity / equal-weight / enter·drop，**不是**完整 D.10 Candidate Target。

价格口径：账户块用 Snapshot mark；评价/决策权重优先 overlay。UI 脚注标明，不静默混用。

### 1.4 前端

| 文件 | 角色 |
|------|------|
| `frontend/src/api/portfolioObservation.ts` | 独立客户端（不改混合 WT `paperObservation.ts`） |
| `frontend/src/components/PortfolioObservationPanel.vue` | 四模块面板 + 免责声明 |
| `PaperTradingObservation.vue` | 「组合观察」Tab 挂载面板 |

模块：

1. **账户状态** — 权益 / 现金 / 市值 / 敞口 / 持仓数  
2. **组合风险** — NORMAL / WATCH / REVIEW / EXIT_CANDIDATE 数量与权重  
3. **持仓决策列表** — 股票 / 当前仓位 / 盈亏 / Decision  
4. **模拟调仓观察** — 当前 / 目标 / Diff（例：当前 10% / 目标 0% → REMOVE）

醒目标签：**「观察结果不是交易建议」**。无「一键调仓」「买入」「卖出」按钮。API 失败不阻断买入模拟盘。

---

## 2. 测试结果

```text
go test ./backend/portfolioobs/ ./backend/api/
  -run "TestAssemble_|TestPortfolioObservationAPI_|TestHoldingsSummaryAPI_|TestRebalanceObservationAPI_|TestEvaluate_|TestDiff_"
```

| 用例 | 结果 |
|------|------|
| 空组合 | PASS（cash 保留，决策/Diff 计数为 0，positions 空） |
| 正常组合 + identity Target | PASS（KEEP + HOLD_NORMAL） |
| 有 WATCH / REVIEW | PASS |
| Rebalance 数据存在（drop + enter → REMOVE + ADD） | PASS |
| GET 不改变 cash / positions / `mark_price` | PASS |
| POST → 405；JSON 无 `"BUY"` / `"SELL"` | PASS |

既有 Evaluation / Summary / Rebalance / Diff / Decision 回归用例一并 PASS。

---

## 3. 是否影响交易

**否。**

| 路径 | 影响 |
|------|------|
| TradePlan / `RunDailyCandidateAndPlan` / 买入 / FixedAmountSizer | 无 |
| PaperTrading Gateway / Broker / 成交 / 模拟盘资金 | 无 |
| `paper_sim_*` 写入 / `mark_price` | **无**（GET 后 cash / volume / mark 不变） |
| Rebalance 执行 / Intent / Order | **未实现** |
| `/holdings/evaluation` · `/rebalance` 契约 | **未改** |

本阶段只建立：

```text
Snapshot + Evaluation + Decision + Diff  →  Portfolio Observation（只读）
```

---

## 4. 提交文件（scoped，非 `git add .`）

- `backend/portfolioobs/types.go`
- `backend/portfolioobs/assemble.go`
- `backend/portfolioobs/assemble_test.go`
- `backend/api/paper_observation_portfolio.go`
- `backend/api/paper_observation_portfolio_test.go`
- `backend/api/paper_observation.go`（仅挂载 `/observation/portfolio`）
- `frontend/src/api/portfolioObservation.ts`
- `frontend/src/components/PortfolioObservationPanel.vue`
- `frontend/src/components/PaperTradingObservation.vue`（仅「组合观察」Tab）
- `PHASE10_E2_PORTFOLIO_OBSERVATION_IMPLEMENTATION_REPORT.md`

未提交：Exit / Attribution / Execution / Risk 等其它 Phase WIP、D.10–D.20/E.1 设计稿、TradePlan / Gateway / Broker。
