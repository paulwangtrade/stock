# PHASE10-E.4 Quantitative Observation Feedback Loop Design

> **类型：** 只读观察闭环（**不成交 / 不改 Decision 规则 / 不改交易路径 / 不写 paper_sim**）  
> **日期：** 2026-08-12  
> **前置：** Snapshot · Holding Evaluation · Holding Decision (D.8) · Portfolio Observation (E.2) · Health/Aging (E.3)  
> **约束：** 不改 TradePlan / CandidatePool / Strategy / HoldingDecision / Risk / PositionSizer / Gateway / Broker / PaperTradingJob；不自动 BUY/SELL / 调仓

---

## Verdict

现有链路能回答「**现在**怎么判断」。E.4 补上「**当时**的判断，后来市场如何」：

```text
Decision（现算，不改规则）
        ↓
Observation Record     ← 当时系统看到的状态（不是交易信号）
        ↓
Future Market Outcome  ← T+1 / T+5 / T+10 价格 + 基准
        ↓
Performance Metrics    ← 按 HOLD_NORMAL / WATCH / REVIEW 统计
        ↓
只读 API / UI「判断效果」
```

**不评价单次判断是否该买卖。** 只统计样本分布，供后续策略改进。  
现网买入链不变：`TradePlan → Gateway → PaperTrading → Settlement` · FixedAmount 100000。

---

## 1. 现有能力审计（只读，不重复建设）

### 1.1 Holding Decision（D.8，不改规则）

```text
HoldingEvaluation → holdingdecision.EvaluateObservation → StockDecision
```

| 已有 | 字段 |
|------|------|
| symbol | `StockDecision.Symbol` |
| decision_state | `HOLD_NORMAL` / `HOLD_WATCH` / `HOLD_REVIEW` / `EXIT_CANDIDATE`（默认关） |
| decision_reason | `NONE` / `DATA_MISSING` / `PROFIT_WEAKNESS` / `RISK_INCREASE` / `RISK_MATERIAL` |
| evaluation_time | `View.AsOf`（RFC3339；来自 Evaluation `as_of`） |
| action | 恒 `none` |

E.4 **不修改** `evaluate.go` 规则。

### 1.2 Portfolio Observation（E.2/E.3）

`GET /api/papertrading/observation/portfolio`

| 块 | 已有 |
|----|------|
| account snapshot | cash / equity / MV / exposure / n |
| holdings | cost / current_price / return / holding_days / first_buy_date / health / aging |
| decision summary | counts + overlay weights |
| history | 运行时仅当前点，**不落库** |

E.4 从 `portfolioobs.Observation` **投影** Record，不重算 Decision。

### 1.3 价格 / 时间 / 成交（复用）

| 能力 | 现状 | E.4 |
|------|------|-----|
| Fill 时间 | `paper_sim_fills.filled_at`；Attribution `trade_date` → Evaluation `buy_date` | 不新造 fill 表 |
| Buy / cost | lot `cost_price`；Observation `cost` | Record.`cost_price` 复用 |
| Holding days | Evaluation 日历日 `as_of − first_buy_date` | Record.`holding_days` 复用 |
| Quote overlay | Evaluation GET 时腾讯/回退；**不写 mark_price** | Record.`market_price` 用观察时 overlay/cost |
| K 线 | `EastMoneyKLineApi.GetKLineData` 日线（腾讯回退） | Outcome 用日线 T+N，不新建行情库 |
| Quote history 表 | **无独立 tick 历史表** | 不用；T+N 走 K 线 close/high/low |

---

## 2. Observation Record

「当时系统看到的状态」，**不是**交易信号。

```text
observation_id        obs-{YYYY-MM-DD}-{symbol}
symbol
market                SH / SZ / BJ
observation_time      RFC3339
observation_date      YYYY-MM-DD（去重键之一）
decision_state
decision_reason
health_score          可空
holding_days
cost_price            可空
market_price          可空（无价则日后 Outcome=INSUFFICIENT）
unrealized_return     可空
portfolio_weight      可空
source                portfolio_observation
```

同一 `(observation_date, symbol)` **一天一条**（后写覆盖）。  
`source` 标明来自组合观察投影，禁止写成 `BUY`/`SELL`/`SIGNAL`。

捕获时机（实现）：

- 纯函数 `CaptureFromObservation(obs)` — 测试主路径  
- 生产：`GET /observation/portfolio` 成功组装后 **追加到独立 Store**（见 §6）  
- **测试进程不写文件、不污染全局 Store**  
- `GET /observation/performance` **只读**，不写 Record

---

## 3. Outcome Evaluation

对每条 Record，在交易日 K 线上取观察日之后第 N 根日线：

| Horizon | 含义 |
|---------|------|
| T+1 | 下一交易日收盘 |
| T+5 | 第 5 个交易日收盘（默认展示） |
| T+10 | 第 10 个交易日收盘 |

```text
future_price
future_return          (future_close − market_price) / market_price
max_drawdown           窗口内 min(low)/market_price − 1   （≤0）
max_profit             窗口内 max(high)/market_price − 1
benchmark_return       同期基准
alpha                  future_return − benchmark_return
status                 EVALUATED | INSUFFICIENT_DATA | PENDING
```

| 缺失 | 行为 |
|------|------|
| 无 market_price / ≤0 | **INSUFFICIENT_DATA**，不评胜负 |
| 尚无足够未来 K 线 | **PENDING**，不评胜负 |
| K 线失败 | INSUFFICIENT，**API 不 500** |
| 无基准 K 线 | 仍可 EVALUATED；`benchmark_return`/`alpha` 为空，**不失败** |

禁止用成本价或 mark_price 伪造未来收益。

---

## 4. Decision Performance Metrics

**不评价单次。** 只对 `status=EVALUATED` 样本聚合。

### HOLD_NORMAL

| 指标 | 定义 |
|------|------|
| samples / evaluated | 记录数 / 已评价数 |
| win_rate | `future_return > 0` 占比 |
| avg_return | 平均 future_return |
| avg_alpha | 有基准时的平均 alpha |
| avg_max_drawdown | 平均最大回撤 |

### HOLD_WATCH

验证是否提前发现风险：

| 指标 | 定义 |
|------|------|
| avg_return | 平均未来收益（预期偏弱） |
| risk_capture_rate | `future_return < 0` 占比（风险被捕获） |

### HOLD_REVIEW

| 指标 | 定义 |
|------|------|
| avg_return | 平均未来收益（若 REVIEW 有效，预期更弱） |
| risk_capture_rate | 同 WATCH |

`EXIT_CANDIDATE` 若出现，统计口径同 REVIEW；默认规则仍关闭，样本通常为 0。

### 组合层

| 字段 | 定义 |
|------|------|
| samples / evaluated | 全状态 |
| win_rate | 全体 EVALUATED 中 `future_return > 0` |
| avg_return / avg_alpha | 全体 EVALUATED |
| decision_accuracy | NORMAL 且收益≥0，或 WATCH/REVIEW 且收益<0 的占比 |

---

## 5. Benchmark

不能只看个股涨跌。

| 基准 | 代码 | E.4 |
|------|------|-----|
| 沪深300 | `000300.SH` | **默认** `csi300` |
| 中证500 | `000905.SH` | `?benchmark=csi500` |
| 行业指数 | 对应行业 | **占位**：无映射则 `industry_benchmark.available=false`，不失败 |

`?benchmark=none` 只算绝对收益。

输出：`absolute_return`（= future_return）· `benchmark_return` · `alpha`。

---

## 6. 数据存储

**禁止写入** `paper_sim_positions` / `paper_sim_orders` / `paper_sim_fills`。

### 6.1 本阶段实现

| 层 | 方案 |
|----|------|
| 测试 | 内存 Store |
| 生产 | 进程内存 + 可选 JSON：`data/paper_observation_history.json`（相对 cwd，exe 下即 `build/bin/data/`） |
| Outcome | **现算不落库**（每次 performance GET 用 K 线回放） |

JSON 只存 Record，不存成交、不改 schema_migrations。

### 6.2 未来落库（不在本阶段 Apply）

```text
observation_history
  observation_id PK
  symbol, market
  observation_time, observation_date
  decision_state, decision_reason
  health_score, holding_days
  cost_price, market_price, unrealized_return, portfolio_weight
  source
  unique (observation_date, symbol)

observation_outcome
  observation_id + horizon PK
  status, future_date, future_price
  future_return, max_drawdown, max_profit
  benchmark_id, benchmark_return, alpha
  evaluated_at
```

独立表，**新 migration**（须 registry ≥ 现网 v7 之后另切片）。E.4 **不**加 v8，避免污染交易库与 schema 门禁。

---

## 7. API

```text
GET /api/papertrading/observation/performance
  ?horizon=5          // 1|5|10，默认 5
  &benchmark=csi300   // csi300|csi500|none
```

只读。POST → 405。

```json
{
  "ok": true,
  "disclaimer": "历史判断统计，不代表未来收益。不是交易建议。不生成买卖单。",
  "performance": {
    "horizon": 5,
    "benchmark": "csi300",
    "samples": 0,
    "evaluated": 0,
    "decision_accuracy": null,
    "win_rate": null,
    "avg_return": null,
    "avg_alpha": null,
    "by_state": {
      "HOLD_NORMAL": { "samples": 0, "evaluated": 0, "win_rate": null, "avg_return": null, "avg_alpha": null, "avg_max_drawdown": null },
      "HOLD_WATCH":  { "samples": 0, "evaluated": 0, "avg_return": null, "risk_capture_rate": null },
      "HOLD_REVIEW": { "samples": 0, "evaluated": 0, "avg_return": null, "risk_capture_rate": null }
    },
    "industry_benchmark": { "available": false, "note": "行业指数映射未启用" }
  }
}
```

空样本仍 **200**。不修改 cash / equity / mark_price / volume。

---

## 8. UI

模拟盘观察 → **组合观察** 内新增区块：

```text
持仓判断效果（观察）
HOLD_NORMAL   样本 / 5日胜率 / 平均收益
HOLD_WATCH    样本 / 风险捕获率 / 平均收益
HOLD_REVIEW   样本 / 平均收益
```

固定文案：**「历史判断统计，不代表未来收益」**。

---

## 9. 测试

| # | 用例 | 期望 |
|---|------|------|
| 1 | CaptureFromObservation | 生成 Record，无 BUY/SELL |
| 2 | 已知未来价 T+5 | future_return / alpha 正确 |
| 3 | 缺未来价 | INSUFFICIENT/PENDING，不计入 win_rate |
| 4 | 缺基准 | EVALUATED 仍成功，alpha 为空 |
| 5 | GET performance | 200；cash/equity/mark/volume 不变 |

---

## 10. 非目标

自动买卖、改 Decision/Risk/Sizer、写 Intent、接 Gateway、APPLY 调仓、持久化 Outcome 表、行业指数完整映射、用今日 Decision 伪造历史日期的「回测决策」。
