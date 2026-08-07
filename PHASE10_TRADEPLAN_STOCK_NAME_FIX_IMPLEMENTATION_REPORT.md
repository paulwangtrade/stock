# PHASE10 TradePlan stock_name Mapping Fix — Implementation Report

> **类型：** 实现报告（未 commit，待验收）  
> **日期：** 2026-08-08  
> **依据：** [PHASE10_TRADEPLAN_STOCK_NAME_FIX_DESIGN.md](./PHASE10_TRADEPLAN_STOCK_NAME_FIX_DESIGN.md)、[PHASE10_TRADEPLAN_STOCK_NAME_AUDIT.md](./PHASE10_TRADEPLAN_STOCK_NAME_AUDIT.md)

---

## Verdict

**实现完成，单测 PASS。** Universe 已识别 `SECURITY_SHORT_NAME`；既有名称键优先级未变。未改 enrich / schema / frontend / trade_plan / 历史数据。

---

## 1. 修改文件

| 文件 | 变更 |
|------|------|
| `backend/strategy/universe.go` | `parseStrategyRunItems` 名称 `firstString` 在既有键后追加 `SECURITY_SHORT_NAME`, `security_short_name` |
| `backend/strategy/universe_test.go` | 新增 SHORT_NAME / 优先级 / ST / 遗留 ABBR 用例 |

**未改：** `stock_name_enrich.go`、frontend、schema、`build_draft_trade_plan.go`、DB 回填。

---

## 2. 字段优先级（实现后）

```text
SECURITY_NAME_ABBR
security_name_abbr
stockName
StockName
name
Name
SECURITY_SHORT_NAME      ← 新增
security_short_name      ← 新增
```

---

## 3. 验证结果

```text
go test ./backend/strategy/ -count=1 -run "ParseStrategyRunItems|IsSTName|NormalizeTradeDate" -v
```

| 用例 | 结果 |
|------|------|
| `TestParseStrategyRunItems_LegacyAbbrStillWorks` | **PASS**（原有 ABBR） |
| `TestParseStrategyRunItems_ShortNameOnly` | **PASS**（仅 SHORT_NAME → `sz301677=欣兴工具`, `sh600363=联创光电`） |
| `TestParseStrategyRunItems_AbbrPreferredOverShort` | **PASS**（ABBR 优先于 SHORT） |
| `TestParseStrategyRunItems_LowerShortNameKey` | **PASS** |
| `TestParseStrategyRunItems_ShortNameSTSkipped` | **PASS** |
| `TestIsSTName` / `TestNormalizeTradeDate` | **PASS** |

验收对照：

| 要求 | 状态 |
|------|------|
| 原有名称字段测试 PASS | ✅ |
| 仅 `SECURITY_SHORT_NAME` 可解析名称 | ✅ |
| `sz301677` 类无 tushare 股票可得 `stock_name`（解析层） | ✅（不依赖 tushare enrich） |

---

## 4. 已知限制 / 后续

1. **已落库空名 plan（如 #31）不会自动修复** — 需重新 generate-next / after_close。  
2. **Runtime UI 抽检**未在本切片执行（待验收时生成新计划确认）。  
3. enrich / tushare 缺口仍在，但落库有名后 UI 不再依赖该兜底。

---

## 5. 约束核对

| 约束 | 状态 |
|------|------|
| 不改 enrich | ✅ |
| 不改 schema | ✅ |
| 不改 frontend | ✅ |
| 不回填历史 | ✅ |
| 不改 trade_plan 逻辑 | ✅ |
| 不 commit | ✅ |

---

## 6. 建议验收步骤

1. Review diff：`universe.go` + `universe_test.go`  
2. 重跑：`go test ./backend/strategy -count=1 -run ParseStrategyRunItems`  
3. Runtime：`POST /api/tradeplans/generate-next` 后查 `trade_plan_items.stock_name` 非空（含无 tushare 标的）  
4. 通过后 scoped commit（另指令）
