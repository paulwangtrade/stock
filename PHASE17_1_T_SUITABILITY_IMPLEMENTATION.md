# PHASE17.1 Holding T-Suitability — Implementation Report

**日期：** 2026-09-07  
**范围：** Holding Intelligence 只读「做 T」适宜性投影 + 组合页展示  
**上游：** [PHASE17_1_T_SUITABILITY_DESIGN.md](./PHASE17_1_T_SUITABILITY_DESIGN.md)

---

## 做了什么

### 后端

| 文件 | 说明 |
| --- | --- |
| `backend/papertrading/holding_t_suitability.go` | `HoldingTSuitability` DTO、`BuildHoldingTSuitability`、5m 振幅 `ClassifyVolatilityFromBars`、`EnrichHoldingWithTSuitability` |
| `backend/papertrading/holding_t_suitability_test.go` | 覆盖可卖 / T+1 / stale / 低波动 / Health D / nil |
| `holding_evaluation_observation.go` | 行字段 `t_suitability` |
| `exit_evaluation.go` | 管道：… → HealthScore → **TSuitability** → ExitEval（Exit state 不变） |

**规则（v0）：**

1. `!can_sell` / T+1 锁仓 / `PRICE_STALE` → `unsuitable`  
2. 5m `(high−low)/close` → `ACTIVE` / `NORMAL` / `LOW`（无 MACD/RSI）  
3. Health 仅背景：C→`HEALTH_WATCH`、D→`HEALTH_RISK`（**不**因 D 自动 unsuitable）

**挂载：** exit-evaluation 行 `t_suitability`（方案 B）；无新表。

### 前端

| 文件 | 说明 |
| --- | --- |
| `holdingTSuitabilityDisplay.js` | 文案：可考虑 / 观察 / 条件不足 |
| `HoldingTSuitabilityDrawer.vue` | 只读原因；**无**买卖按钮 |
| `PortfolioDashboard.vue` | 「做 T」列 + Drawer |
| `paperObservation.ts` | 映射 `t_suitability` |

---

## 未改（硬边界）

- Broker / Settlement / mark_price / 交易执行  
- ExitEval 状态机阈值  
- 自动买卖  

---

## 测试

```text
go test ./backend/papertrading/ -run 'TSuitability|ClassifyVolatility|EnrichHoldingWithTSuitability'  → ok
```

覆盖：

| # | 场景 | 结果 |
| --- | --- | --- |
| 1 | 正常可卖 | `suitable` |
| 2 | T+1 锁仓 | `unsuitable` + `LOCKED_POSITION` |
| 3 | 行情过期 | `unsuitable` + `PRICE_STALE` |
| 4 | 低波动 | `caution` + `LOW_VOLATILITY` |
| 5 | Health D | `caution` + `HEALTH_RISK`（非 unsuitable） |
| 6 | nil / 零值 | 不 panic |

**`go test ./...`：** 全仓执行时另有 **既有** 失败（与本变更无关）：

- `backend/strategy`：`TestBuildDraftTradePlan_UsesPositionSizerForAmount`（期望源码含 `resolvePlanAmountViaSizerForDraft`）  
- `tmp/` 下探针包 build failed  

本阶段相关包 `papertrading` **通过**。

---

## 用户可见

| 列 | 含义 |
| --- | --- |
| 🟢 可考虑 | suitable |
| 🟡 观察 | caution |
| 🔴 条件不足 | unsuitable |

点击打开 Drawer 查看原因 / freshness / 波动 / 健康等级。

---

*Phase17.1 T-Suitability 实现完成 · 停止。*
