# Phase12-E.2 Portfolio-aware Selection 实现报告

**日期**：2026-08-20  
**依据**：`PHASE12_E_CANDIDATE_SELECTION_LAYER_AUDIT.md`、`PHASE12_E1_CANDIDATE_SELECTOR_IMPLEMENTATION_REPORT.md`  
**范围**：扩展 `backend/selection` 纯函数。未接入 CandidatePool / TradePlan / PlanFilter / Sizer / Execution。不访问数据库、不调用 Portfolio API。

---

## 1. 选择顺序（每票）

1. 空代码 → `invalid_candidate`  
2. 同 symbol（已见过、保留 Rank 更高者）→ `duplicate_symbol`  
3. 在 `existing_positions` 中 → `already_holding`（**不占** max_selected_names）  
4. 已满 N → `over_name_limit`  
5. 现金预检开启且再选一票会超过 `available_cash` → `cash_limit`  
6. 否则入选 `rank_top`

现金预检仅当 `estimated_amount_per_name > 0`。这是选择层只数上限，**不是** Sizer，也不写金额进计划。

`PortfolioSnapshot` 仍不读取（留给 `sector_limit` / `risk_limit`）。常量已预留，本阶段不应用。

`CandidateDecision.SkippedReason` JSON 为 `skip_reason`；`SkipReason` 同值别名。

---

## 2. 测试

```
go test ./backend/selection -count=1
```

`ok`

| 用例 | 结果 |
| --- | --- |
| 30 选 5 | 仍通过 |
| 已持仓 | rank1 持仓被 skip，名额让给后续 |
| 重复代码 | 保留更高 Rank，另一条 `duplicate_symbol` |
| 现金不足 | 15 万 / 10 万估额 → 只选 1；现金 0 → 全 `cash_limit` |
| 未设估额 | `available_cash=0` 不启用现金过滤（兼容 E.1） |

---

## 3. 修改文件

| 文件 | 说明 |
| --- | --- |
| `backend/selection/types.go` | 原因常量、`EstimatedAmountPerName`、`skip_reason` |
| `backend/selection/select.go` | 持仓 / 去重 / 现金 |
| `backend/selection/select_test.go` | E.2 用例 |
| `PHASE12_E2_PORTFOLIO_AWARE_SELECTION_REPORT.md` | 本报告 |

未改交易链。

---

## 4. 下一步

E.3 接线 Builder：`Select` 后再 `PlanFilter`。默认产品需确认已持仓 skip 是否作为生产开关。
