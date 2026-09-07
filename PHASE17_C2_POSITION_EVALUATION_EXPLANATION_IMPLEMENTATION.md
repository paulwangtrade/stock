# PHASE17_C2 Position Evaluation Explanation — 实现报告

**日期：** 2026-09-07  
**阶段：** Phase17 C2  
**目标：** 在不改变交易执行、不自动卖出的前提下，增强持仓评价可解释性。  
**状态：** **完成（单元测试通过）**

---

## 1. 实现前能力确认（复用，不重复建模）

| 已有能力 | 复用方式 |
| --- | --- |
| HoldingEval / HoldingEvalObservation | 基础字段：`holding_days`、`avg_cost`、`current_price`、`unrealized_pnl`、`profit_state`、`risk_state`、`trend_state`、`quote_time` |
| ExitEval / ExitReviewOutcome | 链路保持；Explanation **附加**到 Exit 行，不改 state/reason 判定 |
| TradePlanOrigin 语义 | `source_session`、`strategy_name`、SignalSnapshot 字段；**不** import `tradeplanorigin`（避免 `papertrading↔opportunity` 环） |
| PositionProvenance | 同语义字段；本层自建轻量 `LoadExplanationSourceHints`（plan→pool→SignalScanHit） |
| SignalEvent / SignalPrice | 复用 `models.SignalScanHit` 内嵌 SignalEvent 字段（`signal_price` / `signal_time` / snapshot_id）；**无新表** |

**未新建：** 信号事件表、position_lots 表、自动卖出、TradePlan 执行改动、K 线缓存逻辑。

---

## 2. 新增结构

### `PositionEvaluationExplanation`

| 字段 | 含义 |
| --- | --- |
| `stock_code` / `evaluation_time` / `position_days` | 身份与时点 |
| `cost_price` / `current_price` / `pnl` | 来自 HoldingEval |
| `source_type` | `Strategy` \| `Watchlist` \| `Manual` \| `Unknown` |
| `signal_context` | `signal_snapshot_id` / `signal_time` / `signal_price` / `signal_tag` / `present` |
| `hold_reasons` / `risk_hints` | 解释标签（非交易） |
| `freshness` | `EvaluationDataFreshness` |
| `data_source_note` | 只读声明 |

### 解释标签（禁止 BUY/SELL）

**继续持有：** `SIGNAL_ACTIVE` · `TREND_SUPPORT` · `PROFIT_PROTECTION`  

**风险提示：** `SIGNAL_EXPIRED` · `LOSS_CONTROL` · `PRICE_STALE` · `NO_SOURCE_TRACE`

### `EvaluationDataFreshness`

- `price_age` / `kline_age`（可读）+ `*_seconds`
- `price_status` / `kline_status` / overall `status`：`FRESH` \| `STALE` \| `UNKNOWN`
- 默认：价格 >24h → STALE；K 线 >36h → STALE；缺时间 → UNKNOWN  
- **不修改** `kline_cache` / freshness 门闩实现

---

## 3. 接入链路

```text
Position (attribution)
  → HoldingEval (observation)
  → EnrichHoldingWithExplanations (+ optional SourceHints)
  → ExitEval (copies explanation onto stock row)
```

- `BuildExitEvaluation`：DB 路径加载 hints → enrich → ExitEval  
- `ProjectExitEvaluation*`：若 holding 尚无 explanation，用空 hint 现场生成（兼容旧测试）  
- Exit 的 `NORMAL/WATCH/REVIEW_REQUIRED` **逻辑未改**；Explanation 仅为附加 JSON 字段

**文件：**

| 文件 | 作用 |
| --- | --- |
| `backend/papertrading/position_evaluation_explanation.go` | 类型 + 标签 + Freshness + hints 加载 |
| `backend/papertrading/position_evaluation_explanation_test.go` | 四场景 + source/freshness |
| `holding_evaluation_observation.go` | `HoldingEvalStockRow.Explanation` |
| `exit_evaluation.go` | Build 接线 + `ExitEvaluationStockRow.Explanation` |

---

## 4. 测试结果

```text
go test ./backend/papertrading -run "PositionEvaluationExplanation|ClassifyExplanation|ExitEvaluation_" -count=1
ok
```

| 用例 | 期望 | 结果 |
| --- | --- | --- |
| 有 Signal + 盈利 + 趋势 UP | Hold：SIGNAL_ACTIVE / TREND_SUPPORT / PROFIT_PROTECTION | PASS |
| 无 Signal 来源 | `NO_SOURCE_TRACE` | PASS |
| 价格过期（>24h） | `PRICE_STALE` | PASS |
| 亏损扩大 | `LOSS_CONTROL`（+ 过期信号 → SIGNAL_EXPIRED） | PASS |

`ExitEvaluationAPI_*`（api 包）回归：**ok**（未改执行语义）。

---

## 5. 合规确认

| 禁止项 | 状态 |
| --- | --- |
| 修改自动交易 | **否** |
| 自动生成卖单 | **否** |
| 修改 TradePlan 执行 | **否** |
| 大规模 DB 迁移 | **否** |
| 修改 K 线缓存逻辑 | **否** |

---

## 6. 后续建议（非本阶段）

1. **C3** Hold/Sell Explain UI 卡片（消费 `explanation` JSON）  
2. **C4** T-suitability（独立评分，仍 suggest_only）  
3. 可选：盘中把 live quote_time 写入评价 freshness；KlineAsOf 由 ChartSession 注入  
4. 可选：统一走 `tradeplanorigin` 需先拆 `opportunity→papertrading` 依赖环

---

*Phase17 C2 Position Evaluation Explainability 实现结束。*
