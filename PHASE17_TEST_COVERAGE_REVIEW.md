# PHASE17 Test Coverage Review（C1 / C2 / C3）

**日期：** 2026-09-07  
**范围：** Phase17 C1（审计）· C2 Explanation · C3 HealthScore  
**动作：** 审查既有测试；**补充简单边界测试**；**未改业务逻辑**  

---

## 0. 总览

| 阶段 | 代码交付 | 测试现状（审查前） | 审查后 |
| --- | --- | --- | --- |
| **C1** | 仅 `PHASE17_C1_POSITION_MODEL_AUDIT.md` | **无代码 / 无单测**（预期） | 无需补测 |
| **C2** | `position_evaluation_explanation.go` | 4 场景 + source/freshness | + nil/空、UNKNOWN freshness、不可解析 signal_time、Exit 接线、标签文案 |
| **C3** | `holding_health_score.go` | 4 场景 + grade 映射 | + nil enrich、nil stock、分数 clamp 0/100、空 lots enrich |

```text
go test ./backend/papertrading -run "PositionEvaluationExplanation|HoldingHealthScore|ClassifyExplanation|ClassifyEvaluation|MapHoldingHealth|ExplainTag|ProjectExitEvaluation_Builds|Explanation_" -count=1
→ ok
```

---

## 1. 既有覆盖（审查前）

### C2 — `position_evaluation_explanation_test.go`

| 用例 | 覆盖点 |
| --- | --- |
| Signal+盈利+趋势 | Hold 标签；Exit 透传 Explanation |
| 无来源 | `NO_SOURCE_TRACE` |
| 价格过期 | `PRICE_STALE` |
| 亏损扩大 | `LOSS_CONTROL` + `SIGNAL_EXPIRED` |
| SourceType / Freshness 快乐路径 | 分类器 |

### C3 — `holding_health_score_test.go`

| 用例 | 覆盖点 |
| --- | --- |
| 高分 A | 加分因子 + Exit 挂 HealthScore |
| 亏损+过期 | 扣分因子 |
| 无来源 | score 降低 |
| PRICE_STALE | 相对新鲜价降分 |
| Grade 映射 | A–D |

### 异常 / 空数据（审查前缺口）

| 风险点 | 审查前 |
| --- | --- |
| `Enrich*(nil)` | 未测 |
| `LoadExplanationSourceHints(nil)` | 未测 |
| Freshness 无时间戳 → UNKNOWN | 未测 |
| Signal 有价无合法时间 | 未测（不误标 EXPIRED） |
| Health `stock=nil` | 未测 |
| Score >100 / <0 clamp | 未测（高分路径未断言 100） |
| `ProjectExitEvaluation` 自动补 Explanation+Health | 仅间接 |

---

## 2. 空数据 / Panic 风险

| 路径 | 代码防护 | 测试 |
| --- | --- | --- |
| `EnrichHoldingWithExplanations(nil)` | 早退 | **已补** |
| `EnrichHoldingWithHealthScores(nil)` | 早退 | **已补** |
| `LoadExplanationSourceHints(nil)` | 返回 `{}` | **已补** |
| `holding.Holdings` 空 / lots 空 | 循环 0 次或照常 Build | **已补** |
| `BuildHoldingHealthScore(..., nil)` | 用 Explanation.PnL/标签 | **已补** |
| `ClassifyEvaluationDataFreshness(nil,nil)` | UNKNOWN | **已补** |

**结论：** 纯投影路径对 nil 视图安全；DB 加载路径在 `db.Dao==nil` 时跳过查询（C2 设计），本轮未加 DB fixture（见 §4）。

---

## 3. 循环依赖风险

| 边 | 状态 |
| --- | --- |
| `papertrading` → `tradeplanorigin` | **已避免**（C2 用 models 直读 Snapshot hits） |
| `opportunity` → `papertrading` | 既有；C2/C3 **未加重** |
| `portfolio/provenance` → `papertrading` | 既有；Health/Explanation **不反向 import provenance** |

**测试建议：** 保持 C2/C3 单测为 `papertrading_test` 纯函数 + 内存 DTO；DB 集成测另开，避免拉 `opportunity` 成环。

**本轮：** 未发现因 C2/C3 引入的新 import cycle；`go test` 编译通过。

---

## 4. Fixture 需求

| 类型 | 是否需要 | 说明 |
| --- | --- | --- |
| 内存 HoldingEval / Hint | **已有** | 主路径足够 |
| SQLite Beta fixture（plan+pool+snapshot） | **可选后续** | 测 `LoadExplanationSourceHints` 真连库；非 C2/C3 阻塞 |
| 黄金 JSON 快照 | **暂不需要** | 标签集合小，断言码即可 |

---

## 5. 本轮补充的测试（仅测试，无业务改动）

**C2 文件追加：**

- `TestExplanation_NilAndEmptySafe`
- `TestExplanation_FreshnessUnknownWhenNoTimestamps`
- `TestExplanation_SignalPresentWithoutParseableTime_NotExpired`
- `TestExplainTagLabelZH`
- `TestProjectExitEvaluation_BuildsExplanationWhenMissing`

**C3 文件追加：**

- `TestHoldingHealthScore_NilStockAndNilEnrich`
- `TestHoldingHealthScore_ScoreClampTo100`
- `TestHoldingHealthScore_ScoreClampToZero`
- `TestEnrichHoldingWithHealthScores_EmptyLotsRow`

---

## 6. 仍可后续加强（非必须）

1. `LoadExplanationSourceHints` 的 DB 集成测（fixture：TradePlan + CandidatePoolItem + Snapshot JSON）  
2. API 层：`exit-evaluation` 响应含 `explanation` / `health_score` 的契约测（`backend/api`）  
3. 前端 C4 Health 展示：已有 `holdingHealthDisplay.test.mjs`；与本审查范围（C1–C3）分离  

---

## 7. 合规确认

| 要求 | 状态 |
| --- | --- |
| 审查 C1–C3 相关测试 | **是** |
| 允许补简单测试 | **已补** |
| 禁止改业务逻辑 | **是**（仅 `*_test.go`） |

---

*Phase17 Test Coverage Review 结束。*
