# PHASE17_C3 Implementation Audit — Holding Health Score 前置确认

**日期：** 2026-09-07  
**阶段：** Phase17 C3（实现前审计）  
**依据：** C1 Position Model Audit · C2 Position Evaluation Explanation  
**结论：** **可直接在 Explanation 之上做规则评分；无需重复建模 / 无需改交易链。**

---

## 1. PositionEvaluationExplanation（C2）可复用字段

| 字段 | 用途（C3） |
| --- | --- |
| `pnl` / 成本 / 现价 | 盈亏方向辅助（主路径仍优先用 HoldingEval 收益比） |
| `source_type` | 已映射进标签（`NO_SOURCE_TRACE`）；评分不另建来源模型 |
| `signal_context` | 已映射进 `SIGNAL_ACTIVE` / `SIGNAL_EXPIRED` |
| `hold_reasons` | **加分因子真源**（SIGNAL_ACTIVE / TREND_SUPPORT / PROFIT_PROTECTION） |
| `risk_hints` | **扣分因子真源**（SIGNAL_EXPIRED / LOSS_CONTROL / PRICE_STALE / NO_SOURCE_TRACE） |
| `EvaluationDataFreshness` | PRICE_STALE 已由 Explanation 产出；评分只读标签，不重算 freshness |

**原则：** HealthScore **不重新分类标签**；只消费 Explanation + HoldingEval 已有盈亏指标。

---

## 2. HoldingEval 可复用指标

| 指标 | 落点 | C3 用法 |
| --- | --- | --- |
| 持仓状态 | `eval_state` / lots | 不参与打分（Exit 仍用） |
| 盈亏 | `unrealized_return` / `profit_state` / `unrealized_pnl` | 「盈利」「盈利扩大」规则输入 |
| 风险 | `risk_state` | 已由 Explanation → `LOSS_CONTROL`；不重复扣分 |
| 时间 | `holding_days` | 不直接进 v0 分（Exit TIME_REVIEW 保留独立） |
| `trend_state` | 已由 Explanation → `TREND_SUPPORT` | 评分读标签即可 |

---

## 3. ExitEval 如何读取 HoldingEval

```text
BuildExitEvaluation:
  BuildHoldingEvaluationObservation
  → LoadExplanationSourceHints + EnrichHoldingWithExplanations   (C2)
  → LoadExitContextByFillID
  → ProjectExitEvaluationWithExitPolicy
       └─ evaluateExitStock: 用 HoldingDays / UnrealizedReturn / ExitContext
          复制 Explanation；Exit state 仅由 ExitPolicy 阈值决定
```

- Exit **不**因 Explanation 标签改 state。  
- C3 目标：在 Explanation 之后插入 HealthScore，Exit **可读** HealthScore，**仍不**因 score 触发 SELL / 改 Exit state。

---

## 4. C3 设计边界

| 做 | 不做 |
| --- | --- |
| 规则分 0–100 + 等级 A–D | ML / 复杂模型 |
| 复用 Explanation 标签 | 重复计算 Signal/Trend/Freshness |
| 附加到 Holding / Exit JSON | 改 Broker / TradePlan 执行 |
| 声明「持仓质量评价」 | 卖出建议 / 自动调仓 |

---

## 5. 推荐实现落点

| 项 | 路径 |
| --- | --- |
| 模型 + 规则 | `backend/papertrading/holding_health_score.go` |
| 测试 | `holding_health_score_test.go` |
| 接线 | `EnrichHoldingWithHealthScores`；`BuildExitEvaluation` / `ProjectExitEvaluation*` |
| 字段 | `HoldingEvalStockRow.HealthScore` · `ExitEvaluationStockRow.HealthScore` |

---

*Phase17 C3 实现前审计结束 — 进入实现。*
