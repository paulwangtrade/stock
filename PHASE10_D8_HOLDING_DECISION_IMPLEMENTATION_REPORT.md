# PHASE10-D.8 Holding Decision Observation Implementation Report

> **类型：** 实现报告（观察层；不成交）  
> **日期：** 2026-08-11  
> **前置：** [D.7 Holding Decision Engine Design](./PHASE10_D7_HOLDING_DECISION_ENGINE_DESIGN.md) · Holding Evaluation + Quote Overlay  
> **约束：** 无卖出逻辑 / 无 Sell Order / 无 Execution Gateway / 不改 TradePlan / 不改买入 / 不改 FixedAmountSizer / 不改模拟盘成交 / 不改库表

---

## 1. 实现范围

### 1.1 新增包 `backend/holdingdecision`

| 文件 | 角色 |
|------|------|
| `types.go` | 状态、原因、Evidence、HoldingDecision、Policy、Engine |
| `evaluate.go` | 纯函数：`EvaluateObservation(eval, policy) → View` |
| `evaluate_test.go` | 五类规则 + 无卖出动作 + EXIT_CANDIDATE 默认关闭 |

引擎 **只读计算**：输入 Holding Evaluation 视图，输出 `HoldingDecision`。不访问 DB / Broker / Execution。

### 1.2 决策模型

| 字段 | 说明 |
|------|------|
| `symbol` | 股票代码 |
| `fill_id` | lot 主键（股票级聚合可为空） |
| `state` | `HOLD_NORMAL` / `HOLD_WATCH` / `HOLD_REVIEW` / `EXIT_CANDIDATE` |
| `reason` | 进入该状态的主因 |
| `evidence` | 现价、收益率、天数、risk/profit/period、quote_source |
| `action` | **恒为 `none`**（D.8 不产生交易动作） |

主因示例：

| state | reason | 含义 |
|-------|--------|------|
| `HOLD_WATCH` | `PROFIT_WEAKNESS` | 出现浮亏弱势 |
| `HOLD_WATCH` | `RISK_INCREASE` | 风险标签 WATCH |
| `HOLD_REVIEW` | `RISK_MATERIAL` | 风险标签 DANGER |
| `HOLD_NORMAL` | `DATA_MISSING` | 现价与收益率均缺，**不提高**等级 |
| `HOLD_NORMAL` | `NONE` | 常规盈利/持平区间 |

### 1.3 第一版保守规则

默认 `HOLD_NORMAL`。允许 `HOLD_WATCH`、`HOLD_REVIEW`。

`EXIT_CANDIDATE`：**默认关闭**（`DefaultPolicy().ExitCandidateEnabled = false`）。仅当测试显式打开策略，且同时满足 REVIEW + LONG_TERM + DANGER 时才产出。仍 **不是卖出建议**（`action=none`，`next_hint=CONSIDER_EXIT_EVAL`）。

生产路径使用 `DefaultPolicy()`，不会生成 EXIT_CANDIDATE。

### 1.4 Observation 接入（只增观察字段）

`GET /api/papertrading/observation/holdings/evaluation` 在原有 `evaluation` 旁增加：

| JSON 键 | 含义 |
|---------|------|
| `holding_decision` | 完整决策视图（按票 + lots） |
| `decision_state` | 组合最差决策状态（空持仓 → `HOLD_NORMAL`） |
| `decision_reason` | 对应最差状态的主因 |

**不**改 Evaluation 计算、**不**写 `mark_price`、**不**改买入成交、**不**改 PositionSizer。

### 1.5 未改动

- TradePlan / 买入路径 / `FixedAmountSizer`（仍约 100000/票）
- Execution Gateway / Paper fill / 卖单
- 数据库 schema
- Rebalance / Target Portfolio / Exit Evaluation 生产接线
- 前端 Vue（本阶段仅后端观察 API）

---

## 2. 数据流

```text
paper_sim fills + positions
        │
        ▼
Holding Evaluation（已有，C.5-B.1 overlay）
  symbol / cost / current_price / pnl / return
  holding_days / risk_state / profit_state / period_state
        │  只读拷贝事实
        ▼
holdingdecision.EvaluateObservation  ← 纯函数，无 DB
        │
        ▼
HoldingDecision View
  state + reason + evidence + action="none"
        │
        ▼
Observation JSON（evaluation 旁路 sibling）
        │
        ✕ 不进入 Broker / Gateway / TradePlan
```

决策等级映射（有价格或收益率时）：

```text
risk DANGER     → HOLD_REVIEW   (RISK_MATERIAL)
risk WATCH      → HOLD_WATCH    (RISK_INCREASE)
profit LOSS     → HOLD_WATCH    (PROFIT_WEAKNESS)   // 且非上两项
else            → HOLD_NORMAL   (NONE)
price+return 均缺 → HOLD_NORMAL (DATA_MISSING)      // 不抬级
```

股票级状态 = 其 lots 中 **最差** 一档。

---

## 3. 测试结果

```text
go test ./backend/holdingdecision/ ./backend/api/ -count=1 -timeout 90s -run "TestEvaluate_|TestHoldingsEvaluationAPI_"
ok  go-stock/backend/holdingdecision
ok  go-stock/backend/api
```

| 用例 | 期望 | 结果 |
|------|------|------|
| 正常盈利持仓 | `HOLD_NORMAL` / `NONE` | PASS |
| 轻微风险（WATCH / -6%） | `HOLD_WATCH` / `RISK_INCREASE` | PASS |
| 明显风险（DANGER / -12%） | `HOLD_REVIEW` / `RISK_MATERIAL`；默认无 EXIT_CANDIDATE | PASS |
| 缺数据（无价无收益率） | `HOLD_NORMAL` / `DATA_MISSING`，不抬级 | PASS |
| 浮亏但 risk NORMAL | `HOLD_WATCH` / `PROFIT_WEAKNESS` | PASS |
| 不会生成卖出动作 | JSON 无 EXIT_NOW / FORCE_CLOSE；`action=none` | PASS |
| EXIT_CANDIDATE 仅显式开启 | 默认 REVIEW；`ExitCandidateEnabled` 才候选 | PASS |
| API sibling | `holding_decision` + `decision_state` + `decision_reason`；evaluation 仍在 | PASS |
| 不改输入 | Evaluation 视图不被引擎改写 | PASS |

---

## 4. 是否影响交易

**否。**

| 路径 | 影响 |
|------|------|
| 买入 / TradePlan | 无 |
| FixedAmountSizer / 约 100000/票 | 无 |
| 模拟盘成交 / fill | 无 |
| Execution Gateway | 无接入 |
| 卖出 / Sell Order | **未实现；引擎永不输出卖出动作** |
| 数据库 | 无 schema / 无写入 |
| Holding Evaluation 事实层 | 只读消费，不改公式 |

本阶段只让系统 **计算并观察** 持仓决策状态。后续 Exit / Rebalance 不得从本包直接下单。
