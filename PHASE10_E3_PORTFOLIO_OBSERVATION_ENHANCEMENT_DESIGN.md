# PHASE10-E.3 Portfolio Observation Enhancement Design

> **类型：** 只读设计 + 观察层实现（**不成交 / 不自动调仓 / 不改交易路径 / 不写交易表**）  
> **日期：** 2026-08-12  
> **前置：** [E.1](./PHASE10_E1_PORTFOLIO_OBSERVATION_ARCHITECTURE_DESIGN.md) · [E.2](./PHASE10_E2_PORTFOLIO_OBSERVATION_IMPLEMENTATION_REPORT.md) · Snapshot · Evaluation · Decision（D.8/D.9）· Rebalance（D.12）  
> **约束：** 不改 TradePlan / `RunDailyCandidateAndPlan` / FixedAmountSizer / PositionSizer / Gateway / Broker / PaperTradingJob / 成交 / Settlement；不生成真实 BUY/SELL Intent

---

## Verdict

E.2 已能回答「组合现在长什么样」。E.3 补齐「**为什么该关注**」：

- Decision History（状态变化观察，非信号）
- Holding Aging（持仓时长观察，非到期卖出）
- Holding Health Score（组合观察分，非 Strategy Score）
- Opportunity Cost（**本阶段只设计占位**，不实现复杂模型）
- Portfolio Health Summary（组合层计数，供 UI）

生产链路不变：`Strategy → CandidatePool → TradePlan → PaperTrading → Position`。  
观察链路仍停在 Rebalance Sandbox 之前，只证明系统**知道何时该关注风险**。

---

## 1. 现有链路审计（只读）

### 1.1 Portfolio Observation API（E.2）

`GET /api/papertrading/observation/portfolio`

| 块 | 已有 | 缺口 |
|----|------|------|
| Account Snapshot | equity / cash / MV / exposure / position_count | — |
| Evaluation 投影 | cost / price / pnl / return / `holding_days` | 未升成 Aging bucket / Health |
| Decision | `HOLD_*` + reason | **无历史序列** |
| Rebalance Diff | KEEP/ADD/INCREASE/DECREASE/REMOVE | — |
| 组合健康摘要 | 仅 decision 计数+权重 | 无 health / aging / risk 汇总 |

API 只读；失败变 `warnings`，不阻断。未改 `/holdings/evaluation`、`/rebalance`。

### 1.2 Holding Decision

```text
HoldingEvaluation  →  holdingdecision.EvaluateObservation  →  HoldingDecision
```

输入：现价、收益率、risk/profit/period 标签。  
输出：`HOLD_NORMAL` / `HOLD_WATCH` / `HOLD_REVIEW` / `EXIT_CANDIDATE`（默认关）。  
`action` 恒 `none`。缺失现价+收益率 → **维持 NORMAL，不升级**。

### 1.3 时间 / 成本字段（不要重复建设）

| 能力 | 现状 | E.3 策略 |
|------|------|----------|
| **Decision 状态历史** | **无表、无日终决策快照**。Decision 每次 GET 现算 | 新投影：可注入多日点；运行时仅「当前点」，**不写库** |
| **Holding 时间** | Evaluation `holding_days` = as_of − earliest lot `buy_date`（**日历日**） | **复用**，不重算 |
| **Position 创建时间** | `paper_sim_positions` **无 CreatedAt**，仅 `UpdatedAt` | 不用 position 行时间；用 fill/lot |
| **Fill 时间** | `paper_sim_fills.filled_at`；Attribution lot `trade_date` → Evaluation `buy_date` / `first_buy_date` | **复用** |
| **Cost Basis** | Snapshot `avg_cost`；Evaluation `avg_cost` / lot `cost_price` | **复用** E.2 `cost` |
| **Evaluation 持仓周期** | `SHORT_TERM≤5` / `MID_TERM≤20` / `LONG_TERM>20`（对齐 ExitPolicy 20 日） | **不改分类器**。E.3 Aging 用独立 5/30 桶 |

TradePlan 的 `DecisionSnapshot` 是策略/影子链路，**不是**持仓 Decision 历史。

### 1.4 本阶段不做

自动卖出、30 日到期平仓、改 Sizer/Gateway/Broker/Job、写 Intent、写 `paper_sim_*`、持久化决策历史表、完整机会成本模型、CandidatePool 对比。

---

## 2. 分层

```text
GET /observation/portfolio          ← 薄，不写规则
        ↓
portfolioobs.Assemble               ← E.2 投影（不重算引擎）
        ↓
portfolioobs.Enhance                ← E.3 纯计算
        ├── AgingBucket(holding_days, first_buy_date)
        ├── ScoreHealth(eval + decision + aging)
        ├── ProjectDecisionHistory(points)   ← 运行时仅当前点
        └── OpportunityCost stub             ← available=false
        ↓
Holding Evaluation / Holding Decision / Snapshot / Diff
```

禁止把 Aging/Health/History 公式塞进 `handlePortfolioObservation`。  
全部纯函数：**不写库、不改 mark_price、不触发交易**。

---

## 3. Decision History Observation

### 3.1 目标

观察同一股票 Decision **状态变化**（例：NORMAL → WATCH → REVIEW）。  
**不是交易信号**；未来可作 Exit Decision **参考输入**。

### 3.2 模型

```text
DecisionHistoryPoint { as_of, state, reason?, source }  // source=current|injected
DecisionHistory {
  symbol
  points[]          // 按 as_of 升序
  transition_count  // 相邻 state 变化次数
  complete          // ≥2 个点才为 true
  note              // 仅当前点时说明「无持久化快照」
}
```

### 3.3 数据来源（诚实边界）

| 模式 | 行为 |
|------|------|
| 运行时 GET | 每票 **1 个当前点**（今日 Decision）；`complete=false` |
| 单测 / 未来 Job | 注入多日 `points` → 可观察 NORMAL→WATCH→REVIEW |

**不**在 GET 时写历史表。缺失历史 **不得编造** 昨日状态。

---

## 4. Holding Aging Observation

### 4.1 目标

持仓时长风险观察。 **禁止**「满 30 天自动卖出」。

### 4.2 字段

| 字段 | 含义 |
|------|------|
| `holding_days` | **复用 Evaluation**（日历跨度，非交易日日历） |
| `first_buy_date` | 最早 lot `buy_date` |
| `holding_period_bucket` | E.3：`SHORT` / `MEDIUM` / `LONG` / `UNKNOWN` |
| `is_aging` | `bucket == LONG` |

### 4.3 分桶（E.3，独立于 Evaluation 5/20）

| 桶 | 条件 |
|----|------|
| `SHORT` | 有买入日且 `holding_days < 5`（含当日 0 天） |
| `MEDIUM` | `5 ≤ holding_days ≤ 30` |
| `LONG` | `holding_days > 30` |
| `UNKNOWN` | 无 `first_buy_date` 且无法信任天数 → **不升为 LONG** |

`is_aging` 只打标，**不**改 Decision，**不**生成 REMOVE/SELL。

---

## 5. Holding Health Score（组合观察分）

### 5.1 定位

纯观察指标。 **不是**交易评分，**不能替代** Strategy Score。

### 5.2 输入 / 输出

输入：Evaluation（价、盈亏、risk、profit）+ Decision + Aging bucket。  
输出：`health_score`（0–100 或 null）+ `health_level`。

| 分数 | 等级 |
|------|------|
| 80–100 | `HEALTHY` |
| 50–80（不含 80） | `NORMAL` |
| 30–50（不含 50） | `WATCH` |
| <30 | `RISK` |
| 现价与收益率皆缺 | `UNKNOWN`（score 省略） |

### 5.3 公式（v1，可审计）

基准 `100`，只减不加：

| 信号 | 扣分 |
|------|------|
| Decision `HOLD_WATCH` | −20 |
| Decision `HOLD_REVIEW` | −45 |
| Decision `EXIT_CANDIDATE` | −55 |
| Risk `WATCH` | −15 |
| Risk `DANGER` | −30 |
| Profit `BREAKEVEN` | −5 |
| Profit `LOSS` | −15 |
| Aging `MEDIUM` | −5 |
| Aging `LONG` | −10 |

缺某一标签 → **该项扣 0**（不发明风险）。  
现价+收益率都缺 → 直接 `UNKNOWN`，**即使** Evaluation 把 risk 标成 NORMAL。

展示下限：数据完整且 Decision 为 `HOLD_REVIEW` / `EXIT_CANDIDATE` 时，`health_level` 不低于 `WATCH`（分数仍按公式）。**不把缺失数据抬到 RISK。**

长持仓盈利 + `HOLD_NORMAL`：约 90 → `HEALTHY`，仅 `is_aging=true`。

---

## 6. Opportunity Cost Observation（只设计）

长期持有是否挤占更好候选。需要：当前持仓表现 + CandidatePool + Target Portfolio。

```text
OpportunityCost {
  available: false
  level: UNKNOWN          // 未来 LOW | MEDIUM | HIGH
  note: 需 CandidatePool + Target 对比；E.3 不计算
}
```

本阶段 **不实现** 复杂模型，API 只回占位，避免假 HIGH。

---

## 7. Portfolio Health Summary

```text
PortfolioHealthSummary {
  portfolio_health          // 已知 health_level 的最差档；全未知/空仓 → UNKNOWN
  healthy_positions         // HEALTHY
  watch_positions           // Decision HOLD_WATCH
  review_positions          // Decision HOLD_REVIEW
  aging_positions           // is_aging
  risk_positions            // health_level RISK
  unknown_health_count
}
```

`portfolio_health` 用 Health 最差档（RISK > WATCH > NORMAL > HEALTHY），忽略 UNKNOWN。  
空仓：`UNKNOWN` + 全 0，不假装 HEALTHY。

---

## 8. API / DTO 增量

仍走 `GET /api/papertrading/observation/portfolio`。  
**不**新开写路径；**不**改 Evaluation / Rebalance 契约。

`Observation` 增加：`health`、`opportunity_cost`。  
`PositionRow` 增加：`first_buy_date`、`holding_period_bucket`、`is_aging`、`risk_state`、`profit_state`、`health_score`、`health_level`、`decision_history[]`、`opportunity_cost_level=UNKNOWN`。

免责声明扩展为：

> 观察结果不是交易建议。不生成买卖单，不执行调仓，不会自动卖出。

`action` 仍恒 `none`。JSON 不得出现交易动作 `"BUY"` / `"SELL"`。

---

## 9. UI（组合观察 Tab）

仅增强 `PortfolioObservationPanel`（模拟盘观察 → 组合观察）：

1. **组合健康度** — `portfolio_health` + healthy / watch / review / aging / risk 计数  
2. **长持仓观察** — `is_aging` 列表（天数 + bucket）；无卖出按钮  
3. **Decision 变化** — 时间线；仅当前点时标明「无历史快照」  

醒目标签：**观察结果不是交易建议 · 不会自动卖出**。

---

## 10. 测试计划

| # | 场景 | 期望 |
|---|------|------|
| 1 | 正常盈利 + HOLD_NORMAL + 短/中期 | Health `HEALTHY` 或高分 `NORMAL`；无 SELL |
| 2 | 长期持仓（>30 日）+ 仍盈利 | `LONG` / `is_aging`；**不**产生卖出 / Intent |
| 3 | 注入 Decision 点 | NORMAL→WATCH→REVIEW，`transition_count=2` |
| 4 | 缺现价与收益率 | `health_level=UNKNOWN`，**不**升级 RISK |
| 5 | GET API | `cash` / `equity` / `mark_price` / volume **不变** |

---

## 11. 进入 Rebalance Sandbox 前的观察完备性

E.3 后系统应能解释：

```text
账户事实（Snapshot）
+ 单票盈亏与成本（Evaluation）
+ 当前持仓判断（Decision）
+ 判断是否在变差（History，运行时仅当前点）
+ 是否过久（Aging）
+ 是否该盯紧（Health / Summary）
+ 组合差异（Rebalance Diff，仍不执行）
```

仍 **不能** 自动调仓。下一步才是 Paper Rebalance Sandbox（只读/仿真，非本切片）。
