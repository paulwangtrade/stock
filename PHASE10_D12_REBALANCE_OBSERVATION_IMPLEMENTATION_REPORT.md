# PHASE10-D.12 Rebalance Observation Implementation Report

> **类型：** 观察层实现（不成交 / 不生成 Intent / 不改交易路径）  
> **日期：** 2026-08-12  
> **前置：** [D.10 Target](./PHASE10_D10_TARGET_PORTFOLIO_ARCHITECTURE_DESIGN.md) · [D.11 Diff](./PHASE10_D11_REBALANCE_DIFF_ARCHITECTURE_DESIGN.md) · PortfolioSnapshot  
> **约束：** 无 BUY/SELL Intent、无 Order、无 Gateway/TradePlan/Sizer/模拟盘改动

---

## 1. 实现范围

### 1.1 新增包 `backend/rebalance`

| 文件 | 角色 |
|------|------|
| `types.go` | Current / Target / Diff Item / ActionCounts |
| `diff.go` | 纯函数 `Diff(current, target, policy)` |
| `current.go` | `CurrentFromSnapshot` + `AttachDecisions` |
| `target.go` | 观察用 `BuildObservationTarget`（非 TradePlan） |
| `diff_test.go` | 六类场景 + 无买卖动作 |

`action` ∈ `KEEP` / `ADD` / `INCREASE` / `DECREASE` / `REMOVE`。  
`intent_hint` 恒为 `none`。`switch_ok` 恒为 `false`（无成本模型 + 默认不允许 switch）。

### 1.2 观察用 Target（最小实现）

完整 D.10 CandidatePool 构造未接入（避免碰 TradePlan）。本阶段 Target 来源：

| 模式 | 行为 |
|------|------|
| 默认 | 当前持仓等权（`budget=min(1−0.15, 0.85)`）→ 权重漂移可见 |
| `target=identity` | 目标权重=当前权重 → 期望全 KEEP |
| `enter=` / `drop=` | 查询参数，便于观察 ADD/REMOVE（测试与调试） |

### 1.3 API（独立 GET，不污染 Evaluation）

```text
GET /api/papertrading/observation/rebalance
  ?target=identity
  &enter=sz000002,sz000003
  &drop=sz000001
```

响应：

```json
{
  "ok": true,
  "current": { ... },
  "target": { ... },
  "diff": {
    "counts": {
      "total_positions", "keep_count", "add_count",
      "increase_count", "decrease_count", "remove_count"
    },
    "items": [
      {
        "symbol", "current_weight", "target_weight", "delta_weight",
        "current_amount", "target_amount", "delta_amount",
        "action", "reason",
        "available_volume", "locked_volume", "executable_qty",
        "constraint_flags", "switch_ok", "switch_reason", "switch_group_id",
        "intent_hint": "none"
      }
    ]
  },
  "note": "调仓观察…不是交易建议…"
}
```

未改 `holdings/evaluation` 响应形状。

### 1.4 约束展示

| 字段 | 含义 |
|------|------|
| `available_volume` | T+1 可卖 |
| `locked_volume` + `t1_locked` | 当日锁仓 |
| `executable_qty` | REMOVE/DECREASE 可执行股数（≤ available；全锁则为 0） |
| `switch_ok` | 默认 false |
| `switch_reason` | `SWITCH_BLOCKED_DEFAULT` / `HOLD_NORMAL_BLOCKS_SWITCH` |
| `blocked_switch_count` | REMOVE∩ADD 配对组数 |

### 1.5 UI（只设计，本阶段不改 Vue）

在 `PaperTradingObservation.vue` 增加只读入口建议：

1. 标题：**调仓观察（Rebalance Observation）**  
2. 醒目标签：「这是调仓观察，**不是交易建议**，不生成买卖单。」  
3. 统计条：KEEP / ADD / INCREASE / DECREASE / REMOVE 计数  
4. 表格列：代码、现权重、目标权重、Δw、action、available、executable_qty、switch_ok  
5. 调用 `GET .../observation/rebalance`；失败不影响买入 UI  

**禁止：** 「卖出」「买入」「一键调仓」按钮。

---

## 2. 数据流

```text
paper_sim Snapshot
        │
        ▼
CurrentFromSnapshot  (+ optional Holding Decision states)
        │
BuildObservationTarget  ← 观察目标（identity / equal-weight / enter·drop）
        │
        ▼
rebalance.Diff  ← 纯计算
        │
        ▼
GET /observation/rebalance  JSON
        │
        ✕ BUY Intent / SELL Intent / Order / Gateway / TradePlan
```

价格口径：`snapshot_mark`（与 Allocator 一致）。不写 `mark_price`。

---

## 3. 测试结果

```text
go test ./backend/rebalance/ ./backend/api/
  -run "TestDiff_|TestCurrentFromSnapshot_|TestRebalanceObservationAPI_"
ok  go-stock/backend/rebalance
ok  go-stock/backend/api
```

| 用例 | 结果 |
|------|------|
| 完全一致（identity）→ 全 KEEP | PASS |
| 新增股票 → ADD | PASS |
| 减少股票 → REMOVE | PASS |
| 权重变化 → INCREASE/DECREASE | PASS |
| T+1 不可卖 + 锁仓 → executable_qty=0、t1_locked | PASS |
| SWITCH 配对 switch_ok=false | PASS |
| API GET；cash / mark 不变；无 BUY/SELL action | PASS |

---

## 4. 是否影响交易

**否。**

| 路径 | 影响 |
|------|------|
| TradePlan / 买入 / FixedAmountSizer（约 100000/票） | 无 |
| Gateway / 成交 / 模拟盘资金 | 无 |
| BUY/SELL Intent / Order | **未生成** |
| Holding Evaluation API | **未污染** |
| 数据库 | 无 schema / 无写入 |

本阶段只建立：

```text
Current  vs  Target  →  RebalanceDiff（观察）
```

Intent 与执行仍属后续切片。
