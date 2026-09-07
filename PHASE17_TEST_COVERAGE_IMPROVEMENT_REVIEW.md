# PHASE17 Test Coverage Improvement Review

**日期：** 2026-09-07  
**性质：** **只读分析**（未改业务逻辑 / 未补测试代码）  
**范围：** Phase17 **C2** Explanation · **C3** HealthScore · **C4** 组合健康展示 · **Phase17.1** T-Suitability  

**产出：** 缺口清单 + **测试建议**（实现另开）。

---

## 0. 覆盖总览

| 阶段 | 后端单测 | 前端单测 | 整体 |
| --- | --- | --- | --- |
| **C2** | 强（场景 + nil/freshness 边界） | 无专用 mapper 测 | **较好** |
| **C3** | 强（场景 + clamp + nil） | `holdingHealthDisplay.test.mjs` 浅 | **较好** |
| **C4** | 依赖 C3 Exit 挂载（间接） | 仅 display util | **弱（UI/映射）** |
| **17.1** | 核心规则齐全 | **无** `holdingTSuitabilityDisplay` 测 | **中等** |

既有后端文件：

- `position_evaluation_explanation_test.go`（C2）  
- `holding_health_score_test.go`（C3）  
- `holding_t_suitability_test.go`（17.1）  
- FE：`holdingHealthDisplay.test.mjs`（C4 展示层一部分）

---

## 1. nil

### 现状

| 路径 | 已测？ |
| --- | --- |
| `EnrichHoldingWithExplanations(nil)` | ✅ C2 |
| `LoadExplanationSourceHints(nil)` | ✅ C2 |
| `EnrichHoldingWithHealthScores(nil)` | ✅ C3 |
| `BuildHoldingHealthScore(ex, nil)` | ✅ C3 |
| `EnrichHoldingWithTSuitability(nil)` | ✅ 17.1 |
| `BuildHoldingTSuitabilityFromStock(nil, …)` | ✅ 17.1 |
| `ProjectExitEvaluation(nil holding)` | ⚠ 未在 C2/C3/17.1 专测断言 `t_suitability` |
| FE `mapHoldingHealthScore(null/undefined)` | ❌ |
| FE `mapHoldingTSuitability(null)` / `mapPositionExplanation(null)` | ❌ |
| Drawer `suitability=null` / `health=null` 渲染 | ❌（无组件测） |

### 测试建议

1. **BE：** `ProjectExitEvaluation(nil, policy)` → Holdings 空、不 panic；可选断言不 panic 路径已含 Health/TSuit 补全逻辑。  
2. **FE：** 对 `mapPositionExplanation` / `mapHoldingHealthScore` / `mapHoldingTSuitability` 传入 `null` / `{}` / 缺 `level`，断言 `undefined` 或安全默认。  
3. **FE（可选）：** Drawer 空态文案快照 / 浅挂载（若引入 Vue test utils）。

---

## 2. 空数据

### 现状

| 路径 | 已测？ |
| --- | --- |
| Holdings 空 / Lots 空 + Enrich Explanation | ✅ C2 |
| Lots 空 + Enrich Health | ✅ C3 |
| T-Suit 零值 Input → unsuitable | ✅ 17.1 |
| Volatility bars `nil` → UNKNOWN | ✅ 17.1 |
| Health 空 `supportingFactors`/`riskFactors` 摘要 | ⚠ FE 仅测有因子；**未测空数组** |
| Exit 行无 `health_score` / `t_suitability` 时组合列「—」 | ❌ 无 FE 集成测 |

### 测试建议

1. **FE：** `buildHealthReasonSummary({ supportingFactors: [], riskFactors: [] })` → `[]`。  
2. **FE：** `tSuitLevelLabel('')` / `tSuitLevelTagType(undefined)` → `—` / `default`。  
3. **BE：** `EnrichHoldingWithTSuitability` 在 **gate map 无该 code** 时 → `CanSell=false` / `unsuitable`（契约锁定）。  
4. **BE：** holding `Holdings: []` 调 Enrich TSuit → 不 panic、仍空。

---

## 3. 异常行情

### 现状

| 路径 | 已测？ |
| --- | --- |
| 5m bars close≤0 / 全无效 → UNKNOWN | ❌（仅 nil 与正常 low/active） |
| high&lt;low 交换后仍可算 | ❌（实现有 swap，无断言） |
| 不足 N 根（如 3 根）仍分类 | ❌ |
| NORMAL 振幅带（0.4%–1.0%） | ❌（仅 LOW / ACTIVE） |
| Quote 未来时间（quote &gt; asOf） | ❌ C2 freshness |
| 现价/成本为 0 或负 | ⚠ C2/C3 未专测异常价 |

### 测试建议

1. **`ClassifyVolatilityFromBars`：**  
   - 全 `Close: 0` → `UNKNOWN`  
   - `High < Low` → 仍得到有限 amp，非 panic  
   - 3 根 ACTIVE 样例 → `ACTIVE`  
   - 构造 amp≈0.6% → `NORMAL`  
2. **C2：** `QuoteTime = asOf.Add(+1h)` → freshness 行为文档化（绝对值 age 或 FRESH）并加断言。  
3. **C3：** `CurrentPrice=0` / 负 return 边界与 `PROFIT_EXPANDING` 阈值不误触发。

---

## 4. freshness 异常

### 现状

| 路径 | 已测？ |
| --- | --- |
| 无时间戳 → UNKNOWN | ✅ C2 |
| Price STALE → `PRICE_STALE` 标签 | ✅ C2 |
| Kline 可选 FRESH | ✅ C2 |
| Price FRESH + Kline STALE → overall | ⚠ 未断言 worst-of 合成 |
| 17.1：`Freshness=STALE` → unsuitable | ✅ |
| 17.1：仅 `HasPriceStaleTag`、Freshness 仍 FRESH | ❌ |
| 17.1：非法 freshness 字符串 → normalize UNKNOWN | ❌ |
| C2：刚好阈值边界（24h±1s） | ❌ |

### 测试建议

1. **C2：** quote FRESH + kline STALE → `Status == STALE`（worst-of）。  
2. **C2：** `PriceStaleAfter` 自定义阈值边界两侧各一例。  
3. **17.1：** `Freshness: "FRESH", HasPriceStaleTag: true` → 仍 `unsuitable` + `PRICE_STALE`。  
4. **17.1：** `Freshness: "garbage"` → 内部 UNKNOWN，叠加 VOLATILITY/HEALTH 规则可预期。

---

## 5. HealthScore 边界

### 现状

| 路径 | 已测？ |
| --- | --- |
| Grade 映射 A–D | ✅ |
| Score clamp 0 / 100 | ✅ |
| nil stock | ✅ |
| PRICE_STALE 降分 | ✅ |
| Grade 边界分（如 80/60/40 恰界） | ❌（仅 85/70/45/10） |
| `PROFIT_EXPANDING` 恰 5% 边界 | ❌ |
| 未知 HoldReasons/RiskHints 忽略 | ❌ |
| 17.1 Health C → caution | ❌（仅测 D） |
| 17.1 Health 空 → `HEALTH_UNKNOWN` caution | ❌（零值路径被 !canSell 掩盖） |
| Exit state **不因** Health/TSuit 改变 | ✅ 部分（高分 NORMAL）；缺「高风险 Health 仍不改 Exit」专测 |

### 测试建议

1. **`MapHoldingHealthGrade`：** 边界分 `80, 79, 60, 59, 40, 39`。  
2. **`PROFIT_EXPANDING`：** return `0.049` vs `0.05`。  
3. **17.1：** canSell+FRESH+ACTIVE + Health **C** → `caution` + `HEALTH_WATCH`。  
4. **17.1：** canSell+FRESH+NORMAL + Health `""` → `caution` + `HEALTH_UNKNOWN`。  
5. **Exit：** Health D + TSuit caution 的 holding → `Evaluation.State` 仍仅由 ExitPolicy 决定（与 TSuit Level 解耦断言）。

---

## 6. DTO 兼容性

### 现状

| 项 | 状态 |
| --- | --- |
| Go `json` snake_case（health / t_suit） | 有 |
| FE 同时读 snake + camel | `paperObservation.ts` map 已做 |
| **契约测试**（样例 JSON → map → 字段） | ❌ |
| `evaluation_time` / `evaluated_at` RFC3339 | 无 round-trip 测 |
| `price_age_seconds` vs FE `priceAge` 字符串 | FE 类型偏弱；无断言 |
| `mapHoldingHealthScore` 无 grade 且无 score → undefined | 实现有；**无测** |
| `mapHoldingTSuitability` 无 level → undefined | 实现有；**无测** |

### 测试建议

1. **FE 契约测（推荐 `.mjs` 或 vitest）：** 用最小 Go 风格 JSON fixture：

```json
{
  "health_score": {
    "stock_code": "sh600000",
    "score": 70,
    "grade": "B",
    "grade_label": "正常观察",
    "supporting_factors": ["PROFIT"],
    "risk_factors": [],
    "evaluation_time": "2026-09-07T14:00:00+08:00"
  },
  "t_suitability": {
    "stock_code": "sh600000",
    "level": "caution",
    "reasons": ["LOW_VOLATILITY"],
    "can_sell": true,
    "freshness": "FRESH",
    "volatility_status": "LOW",
    "health_grade": "A",
    "evaluated_at": "2026-09-07T14:30:00+08:00"
  }
}
```

断言 camel 字段与数组长度。  
2. **仅 camelCase 输入** 一条（兼容路径）。  
3. **残缺 DTO：** 缺 `level` / 缺 `grade`+`score` → mapper 返回 `undefined`。

---

## 7. 前后端字段一致性

### 对照（抽样）

| Go JSON | FE 映射目标 | 一致性风险 |
| --- | --- | --- |
| `hold_reasons` / `risk_hints` | `holdReasons` / `riskHints` | 低（已 map） |
| `supporting_factors` / `risk_factors` | `supportingFactors` / `riskFactors` | 低 |
| `grade_label` | `gradeLabel` | 低 |
| `volatility_status` | `volatilityStatus` | 低 |
| `can_sell` | `canSell` | 低 |
| `evaluated_at` vs Health `evaluation_time` | 两套时间字段名 | **中** — UI 勿混用；建议契约测分开断言 |
| Explanation `freshness.price_age_seconds` | FE `priceAge`（string） | **中** — 秒数字段可能未映射 |
| C4 Drawer 用 `healthFactorLabelZH` | 与 Go `ExplainTagLabelZH` 文案 | **中** — 中文可能漂移，无对照测 |
| 17.1 Drawer `tSuitReasonLabelZH` | Go reason 常量 | **中** — 新增 reason 易漏 FE 文案 |

### 测试建议

1. **常量对照表测：** Go 导出的 reason/tag 集合 ⊆ FE `T_SUIT_REASON_LABEL` / `healthFactorLabelZH` keys（或允许 fallback 字符串）。  
2. **Exit API 集成（可选）：** httptest 或 golden：响应含 `health_score` + `t_suitability`，FE map 后列渲染输入非空。  
3. **文案：** `ExplainTagLabelZH` vs `healthFactorLabelZH` 对同一 tag 的「是否刻意不同」写进测试注释；若应对齐则加相等断言。

---

## 8. 分阶段优先级建议

| 优先级 | 建议 | 阶段 |
| --- | --- | --- |
| **P0** | FE mapper 契约测（snake/camel/残缺/nil） | C4 + 17.1 |
| **P0** | 17.1：`HasPriceStaleTag`、Health C、空 Health、gate 缺失 | 17.1 |
| **P1** | Volatility 异常 bars + NORMAL 带 | 17.1 |
| **P1** | Freshness worst-of + 阈值边界 | C2 |
| **P1** | Health grade/expanding 边界分 | C3 |
| **P2** | `holdingTSuitabilityDisplay.test.mjs`（对称 C4） | 17.1 FE |
| **P2** | Exit 不因 Health/TSuit 改 state 回归 | C3/17.1 |
| **P3** | Drawer 空态组件测 | C4/17.1 |

---

## 9. 明确不做（本审查）

- 不修改业务逻辑  
- 不在本轮补写测试代码  
- 不要求新建 DB / E2E 全链路（可作为后续独立任务）

---

## 10. 结论

C2/C3 **后端核心与多数边界已覆盖**；C4 **偏展示 util**；17.1 **规则主路径已测**，缺口集中在：

1. 异常行情 / freshness 细节  
2. Health 与 T-Suit **交叉边界**（C、空 Health、tag 与 freshness 双触发）  
3. **前后端 DTO 契约与文案一致性**自动化  

按上表补测即可显著降低回归风险，**无需改业务代码**。

---

*PHASE17 测试覆盖增强审查结束 · 停止。*
