# PHASE10-E.3 Portfolio Observation Enhancement Report

> **类型：** 只读观察增强（不成交 / 不自动调仓 / 不改交易路径）  
> **日期：** 2026-08-12  
> **设计：** [PHASE10_E3_PORTFOLIO_OBSERVATION_ENHANCEMENT_DESIGN.md](./PHASE10_E3_PORTFOLIO_OBSERVATION_ENHANCEMENT_DESIGN.md)  
> **前置：** E.2 Portfolio Observation API

---

## Verdict

**PASS** — 在既有 `GET /api/papertrading/observation/portfolio` 上增加 Aging / Health / Decision History / Health Summary 纯计算投影。  
Opportunity Cost **仅占位**（`available=false`）。不写库、不改 `mark_price`、不生成 BUY/SELL、不自动卖出。

---

## 1. 修改文件

| 文件 | 变更 |
|------|------|
| `PHASE10_E3_PORTFOLIO_OBSERVATION_ENHANCEMENT_DESIGN.md` | 审计 + 边界设计 |
| `backend/portfolioobs/types.go` | Health / Aging / History DTO；免责声明含「不会自动卖出」 |
| `backend/portfolioobs/aging.go` | `SHORT/<5` `MEDIUM/5–30` `LONG/>30`；缺买入日 → UNKNOWN |
| `backend/portfolioobs/health.go` | 观察分 0–100 → HEALTHY/NORMAL/WATCH/RISK；缺价+收益 → UNKNOWN |
| `backend/portfolioobs/history.go` | Decision 时间线投影；运行时仅当前点 |
| `backend/portfolioobs/enhance.go` | Assemble 后纯增强；机会成本 stub |
| `backend/portfolioobs/enhance_test.go` | 盈利/AGING/缺失数据/NORMAL→WATCH→REVIEW |
| `backend/portfolioobs/assemble.go` | 复用 first_buy_date / risk / profit；调用 `Enhance` |
| `backend/portfolioobs/assemble_test.go` | 空仓 + 联接用例覆盖 health/aging |
| `backend/api/paper_observation_portfolio_test.go` | 盈利 HEALTHY、长持仓 AGING 不卖、缺行情不升 RISK、GET 不改账本 |
| `frontend/src/api/portfolioObservation.ts` | 解析 health / aging / history |
| `frontend/src/components/PortfolioObservationPanel.vue` | 组合健康度 / 长持仓 / Decision 变化 |
| `PHASE10_E3_PORTFOLIO_OBSERVATION_ENHANCEMENT_REPORT.md` | 本报告 |

未改：TradePlan / Sizer / Gateway / Broker / Job / 成交 / Settlement / `/holdings/evaluation` / `/rebalance` / `paper_observation.go` 路由。

---

## 2. 是否影响交易链路

**否。**

| 路径 | 影响 |
|------|------|
| TradePlan / `RunDailyCandidateAndPlan` / FixedAmountSizer / PositionSizer | 无 |
| Gateway / Broker / PaperTradingJob / 成交 / Settlement | 无 |
| BUY/SELL Intent | **未生成** |
| 30 日自动卖出 | **无**（AGING 只打标） |

---

## 3. 测试结果

```text
go test ./backend/portfolioobs/ ./backend/api/
  -run "TestAssemble_|TestClassifyAging|TestScoreHealth_|TestProjectDecisionHistory_|TestEnhance_|TestPortfolioObservationAPI_"
ok  go-stock/backend/portfolioobs
ok  go-stock/backend/api
```

| 用例 | 结果 |
|------|------|
| 正常盈利 → Health HEALTHY | PASS |
| 长期持仓 → AGING，无 SELL | PASS |
| Decision NORMAL→WATCH→REVIEW | PASS（注入点） |
| 缺现价+收益率 → UNKNOWN，不升 RISK | PASS |
| GET 不改 cash / equity / mark_price / volume | PASS |

---

## 4. 是否改变数据库

**否。** 无 schema 变更；GET 不写 `paper_sim_positions` / `mark_price` / cash / equity。  
Decision History **不落库**（运行时仅当前观察点）。

---

## 5. 架构落点

```text
GET /observation/portfolio
        ↓
portfolioobs.Assemble（E.2 投影，不重算引擎）
        ↓
portfolioobs.Enhance（E.3 纯计算）
        ├── AgingBucket
        ├── ScoreHealth
        ├── ProjectDecisionHistory（当前点）
        └── OpportunityCost stub
```

进入 Paper Rebalance Sandbox 前，系统可解释：账户事实、盈亏、当前判断、是否过久、是否该盯紧。仍 **不能** 自动交易。
