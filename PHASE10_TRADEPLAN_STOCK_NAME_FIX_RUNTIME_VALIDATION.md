# PHASE10 TradePlan Stock Name Fix — Runtime Validation

> **类型：** Runtime Validation（未改代码、未 commit、未 push、未回填历史）  
> **日期：** 2026-08-08  
> **依据：** [PHASE10_TRADEPLAN_STOCK_NAME_FIX_IMPLEMENTATION_REPORT.md](./PHASE10_TRADEPLAN_STOCK_NAME_FIX_IMPLEMENTATION_REPORT.md)  
> **代码面：** 工作区未 commit 的 Universe SHORT_NAME 修复  
> **DB：** `D:\stock\build\bin\data\stock.db`

---

## Verdict

**PASS** — 真实 `RunAfterClosePlanWorkflow` 生成链路中，`sh600363` / `sz301677` 的 `trade_plan_items.stock_name` 均已非空落库；不依赖 API enrich / tushare。

---

## 1. 测试确认

### 1.1 本修复相关用例

```text
go test ./backend/strategy/ -count=1 -run "ParseStrategyRunItems|IsSTName|NormalizeTradeDate" -v
```

| 结果 | **PASS**（含 Legacy ABBR、仅 SHORT_NAME、ABBR 优先、ST 跳过） |

### 1.2 全量 `go test ./backend/strategy`

| 结果 | **FAIL**（与本修复无关） |
| 失败用例 | `TestBuildDraftTradePlan_UsesPositionSizerForAmount`（`tradingconfig_wire_test.go`：源码静态断言缺 `resolvePlanAmountViaSizer()`） |
| 判断 | **既有 wire 测试 / 脏工作区问题**，非 stock_name 映射回归 |

本报告以 **ParseStrategyRunItems\*** + **Runtime 生成** 为验收主证据。

---

## 2. 真实生成流程

| 项 | 值 |
|----|-----|
| 入口 | `strategy.RunAfterClosePlanWorkflow`（工作区源码 `go run`，非旧 exe） |
| 策略 | id=4「冰点超跌·出坑买点」 |
| 最新 run | **#50**（含两标的，字段=`SECURITY_SHORT_NAME`） |
| source_date | `2026-08-07` |
| trade_date | `2026-08-10` |
| CandidatePool | **#31** |
| TradePlan | **#32** draft v2 |
| riskPassed | true |

策略 JSON 名称字段（run=50）：

| SECURITY_CODE | SECURITY_SHORT_NAME |
|---------------|---------------------|
| 600363 | 联创光电 |
| 301677 | 欣兴工具 |

---

## 3. 生成结果（DB 直查）

### 3.1 `trade_plan_items`（plan_id=32）

| stock_code | stock_name | status | 断言 |
|------------|------------|--------|------|
| `sh600363` | **联创光电** | pending | **非空 PASS** |
| `sz301677` | **欣兴工具** | pending | **非空 PASS** |

### 3.2 上游 `candidate_pool_items`（pool_id=31）

| stock_code | stock_name |
|------------|------------|
| `sh600363` | 联创光电 |
| `sz301677` | 欣兴工具 |

名称在 **Universe → CandidatePool 落库** 已非空，Draft 透传一致。

---

## 4. 约束确认

| 检查项 | 结果 |
|--------|------|
| **未依赖 enrich** | **PASS** — 断言对象是 DB 列 `stock_name`，未走 HTTP DTO enrich；且 `tushare_stock_basic` 对 `301677` **仍无行**，`sz301677` 仍有名 → 不可能靠 tushare enrich 写库 |
| **未修改 schema** | **PASS**（本切片未改表） |
| **未影响已有字段优先级** | **PASS** — 单测 `AbbrPreferredOverShort` + Legacy ABBR 仍 PASS |
| 未 commit / push / 历史回填 | **PASS** |

对比修复前 plan #31（同两码）：DB `stock_name` 皆空；修复后 plan #32：皆有名。

---

## 5. 证据链

```text
run#50 SECURITY_SHORT_NAME
  → parseStrategyRunItems（工作区修复）
  → candidate_pool_items#31 stock_name 非空
  → trade_plan_items#32 stock_name 非空
  → （展示层 enrich 可选；本验证不依赖）
```

`RUNTIME_VALIDATION_PASS=true`

---

## 6. Known notes

1. 全量 `./backend/strategy` 包测试仍有无关 FAIL，需另单跟 `tradingconfig_wire_test`。  
2. 运行中旧 `go-stock.exe` **不含**本修复；本次用 `go run` 工作区代码写库。验收 UI 前需重建 exe。  
3. 旧 plan #31 空名未回填（按禁止项）。

---

## 7. Sign-off

| 项 | 结果 |
|----|------|
| 相关单测 | **PASS** |
| Runtime 生成 | **PASS**（plan #32） |
| sh600363 / sz301677 stock_name | **PASS** |
| 无 enrich 依赖 | **PASS** |
| Overall | **PASS**（可进入 scoped commit 验收） |
