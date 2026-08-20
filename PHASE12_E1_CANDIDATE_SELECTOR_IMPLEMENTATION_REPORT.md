# Phase12-E.1 CandidateSelector 实现报告

**日期**：2026-08-20  
**依据**：`PHASE12_E_CANDIDATE_SELECTION_LAYER_AUDIT.md`  
**范围**：可测试的选择层纯函数原型。未接入 CandidatePool Builder / TradePlan / PlanFilter / Sizer / Execution。

---

## 1. 做了什么

新增包 `backend/selection`（无 DB、无 HTTP、不 import strategy/risk/papertrading）。

`Select(candidates, ctx) *SelectedCandidates`

E.1 只使用：

- Rank 升序（1 最优）；Rank≤0 排在有 Rank 之后，再按 Score 降序
- `MaxSelectedNames`（≤0 时默认 **5**，对齐计划 MaxNames）

**不读**（仅预留在 Context）：`ExistingPositions`、`PortfolioSnapshot`、`AvailableCash`。

| 结果 | reason |
| --- | --- |
| 入选 | `rank_top` |
| 超过上限 | `over_name_limit` |
| 空代码 | `invalid_candidate`（不占名额） |

入选条目带 `SelectionRank` 1..K。未改任何交易链文件。

---

## 2. 测试

```
go test ./backend/selection -count=1
```

`ok`

覆盖：

1. 30 个候选选 5 个（25 个 skipped）  
2. 乱序输入按 Rank 1..N 输出  
3. skipped_reason = `over_name_limit`  
4. 空候选 → 两列表皆空  
5. 补充：未排名靠 Score；预留持仓/现金不生效  

---

## 3. 修改文件

| 文件 | 说明 |
| --- | --- |
| `backend/selection/types.go` | Candidate / Context / Decision |
| `backend/selection/select.go` | `Select` |
| `backend/selection/select_test.go` | 单测 |
| `PHASE12_E1_CANDIDATE_SELECTOR_IMPLEMENTATION_REPORT.md` | 本报告 |

未改：`build_candidate_pool.go`、`build_draft_trade_plan.go`、`plan_filter.go`、sizer、execution。

---

## 4. 下一步（E.2，本切片不做）

Builder 改为 `Select` 后再 `PlanFilter(selected)`。E.1 默认仍不跳过已持仓。
