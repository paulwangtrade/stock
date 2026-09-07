# PHASE17_C3 Holding Health Score — 实现报告

**日期：** 2026-09-07  
**阶段：** Phase17 C3  
**前置：** [PHASE17_C3_IMPLEMENTATION_AUDIT.md](./PHASE17_C3_IMPLEMENTATION_AUDIT.md)  
**状态：** **完成（单元测试通过）**

---

## 1. 新增能力

### `HoldingHealthScore`（持仓质量评价，非交易决策）

| 字段 | 说明 |
| --- | --- |
| `stock_code` / `evaluation_time` | 身份与时点 |
| `score` | 0–100 |
| `grade` | A / B / C / D |
| `grade_label` | 健康持有 / 正常观察 / 重点关注 / 风险较高 |
| `supporting_factors` | 支持因素（加分项） |
| `risk_factors` | 风险因素（扣分项） |
| `explanation` | 关联 `PositionEvaluationExplanation` |
| `data_source_note` | 声明：质量评价，非卖出信号 |

**代码：** `backend/papertrading/holding_health_score.go`  
**挂载：** `HoldingEvalStockRow.health_score` · `ExitEvaluationStockRow.health_score`

---

## 2. 评分规则 v0

| 项 | 分值 |
| --- | --- |
| 基础分 | **50** |
| 盈利 `PROFIT` | **+10** |
| 盈利扩大 `PROFIT_EXPANDING`（收益 ≥5%） | **+10** |
| `SIGNAL_ACTIVE` | **+15** |
| `TREND_SUPPORT` | **+15** |
| `PROFIT_PROTECTION` | **+10** |
| `SIGNAL_EXPIRED` | **-15** |
| `LOSS_CONTROL` | **-15** |
| `PRICE_STALE` | **-10** |
| `NO_SOURCE_TRACE` | **-10** |
| 最终 | **clamp 0–100** |

**等级：** 80–100 A · 60–79 B · 40–59 C · 0–39 D（表示健康度，不是卖出建议）

---

## 3. 数据来源（复用，无新表）

| 来源 | 用途 |
| --- | --- |
| HoldingEval | `profit_state` / `unrealized_return` → PROFIT / PROFIT_EXPANDING |
| PositionEvaluationExplanation | `hold_reasons` / `risk_hints` → 其余加减分 |
| Freshness | 已体现为 `PRICE_STALE`，评分不二次计算 |

链路：`Position → HoldingEval → Explanation → HoldingHealthScore → ExitEval`

---

## 4. 对交易链影响

**无影响。**

- 不写 Broker / Gateway / fills  
- 不生成 BUY / SELL  
- 不改 TradePlan 执行  
- Exit state 仍仅由既有 TIME/LOSS/PLAN 阈值决定，**不**读取 HealthScore  
- 无数据库迁移  

---

## 5. 测试

| 场景 | 期望 | 结果 |
| --- | --- | --- |
| 盈利 + Signal 有效 + 趋势正常 | 高分（≥80），含 SIGNAL_ACTIVE | PASS |
| 亏损 + Signal 失效 | 评分下降，LOSS_CONTROL / SIGNAL_EXPIRED | PASS |
| 无来源 | NO_SOURCE_TRACE | PASS |
| 价格过期 | PRICE_STALE | PASS |

---

*Phase17 C3 Holding Health Score 实现结束。*
