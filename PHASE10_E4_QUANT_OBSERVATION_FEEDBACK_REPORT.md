# PHASE10-E.4 Quantitative Observation Feedback Loop Report

> **类型：** 只读观察闭环实现（不成交 / 不改 Decision 规则 / 不改交易路径）  
> **日期：** 2026-08-12  
> **设计：** [PHASE10_E4_QUANT_OBSERVATION_FEEDBACK_DESIGN.md](./PHASE10_E4_QUANT_OBSERVATION_FEEDBACK_DESIGN.md)

---

## Verdict

**PASS** — 新增 Observation Record → T+1/T+5/T+10 Outcome → Decision Performance Metrics 闭环。  
只读 API `GET /api/papertrading/observation/performance` + UI「持仓判断效果」。  
不写 `paper_sim_*`，不改 HoldingDecision / TradePlan / Gateway / Broker / Job。

---

## 1. 当前 Decision 链路（未改规则）

```text
paper_sim Snapshot
        ↓
Holding Evaluation（Attribution lots + Quote Overlay）
        ↓
holdingdecision.EvaluateObservation   ← D.8 规则不变
        ↓
portfolioobs.Assemble + Enhance       ← E.2 / E.3
        ↓
GET /observation/portfolio
```

Decision 输出仍为：`symbol` · `decision_state` · `decision_reason` · `AsOf`；`action=none`。

复用（不重复建设）：fill/`buy_date`、`cost_price`、`holding_days`、overlay 现价、日 K 线。

---

## 2. 新增观察闭环

```text
Decision（现算）
        ↓
CaptureFromObservation → Observation Record
        ↓
EvaluateOutcome（K 线 T+N + 沪深300/中证500）
        ↓
AggregateMetrics（按 HOLD_NORMAL / WATCH / REVIEW）
        ↓
GET /observation/performance + UI「判断效果」
```

生产：打开组合观察时捕获当日 Record → 独立 JSON（测试进程不写）。  
Performance GET **只读**，Outcome **现算不落库**。

---

## 3. 数据模型

### Record（当时系统看到的状态，非信号）

`observation_id, symbol, market, observation_time/date, decision_state/reason, health_score, holding_days, cost_price, market_price, unrealized_return, portfolio_weight, source=portfolio_observation`

### Outcome

`horizon, status(EVALUATED|INSUFFICIENT_DATA|PENDING), future_price/return, max_drawdown/profit, benchmark_return, alpha`

缺未来价 → 不评胜负；缺基准 → 仍 EVALUATED，alpha 为空。

### 存储

| 本阶段 | 未来（另切片） |
|--------|----------------|
| 内存 Store + `data/paper_observation_history.json` | `observation_history` / `observation_outcome` 表 |
| **不写** paper_sim_positions/orders/fills | 新 migration（须 registry 对齐现网 v7 之后） |

---

## 4. API / UI

```text
GET /api/papertrading/observation/performance?horizon=5&benchmark=csi300
```

返回：`samples / evaluated / decision_accuracy / win_rate / avg_return / avg_alpha` + `by_state.HOLD_NORMAL|WATCH|REVIEW`（含 WATCH `risk_capture_rate`）。

UI：模拟盘观察 → 组合观察 → **「持仓判断效果（观察）」**，文案 **「历史判断统计，不代表未来收益」**。

---

## 5. 测试结果

```text
go test ./backend/obsfeedback/ ./backend/api/
  -run "TestCaptureFromObservation|TestEvaluateOutcome_|TestAggregateMetrics_|TestObservationPerformanceAPI_|TestPortfolioObservationAPI_"
ok  go-stock/backend/obsfeedback
ok  go-stock/backend/api
```

| 用例 | 结果 |
|------|------|
| 生成 Observation Record | PASS |
| T+5 未来收益 + alpha（+8% / 基准 +3% / α +5%） | PASS |
| 缺少未来价格 → 不错误评价 | PASS |
| Benchmark 缺失 → API 仍 200，alpha 空 | PASS |
| GET performance 不改 cash / equity / mark_price / volume | PASS |

---

## 6. 数据库变化

**无 schema / migration。** 不碰 `schema_migrations`。  
可选 JSON 文件独立于模拟盘账本。

---

## 7. 是否影响交易链路

**否。**

| 路径 | 影响 |
|------|------|
| TradePlan / CandidatePool / Strategy / HoldingDecision / Risk / Sizer | 无 |
| Gateway / Broker / PaperTradingJob / 成交 / Settlement | 无 |
| 自动 BUY / SELL / 调仓 | **无** |

---

## 8. 修改文件

| 文件 | 角色 |
|------|------|
| `PHASE10_E4_QUANT_OBSERVATION_FEEDBACK_DESIGN.md` | 设计 |
| `backend/obsfeedback/*` | Record / Outcome / Metrics / Store / K 线适配 |
| `backend/api/paper_observation_performance.go` + `_test.go` | 只读 API |
| `backend/api/paper_observation.go` | 挂载 `/observation/performance` |
| `backend/api/paper_observation_portfolio.go` | 生产捕获 Record（测试跳过） |
| `frontend/src/api/observationPerformance.ts` | 客户端 |
| `frontend/src/components/PortfolioObservationPanel.vue` | 「判断效果」区块 |
| `PHASE10_E4_QUANT_OBSERVATION_FEEDBACK_REPORT.md` | 本报告 |
