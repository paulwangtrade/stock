# PHASE10 TradePlan stock_name Mapping Fix Design

> **类型：** 只读设计（不修改代码 / 不 commit / 不 push）  
> **日期：** 2026-08-08  
> **依据：** [PHASE10_TRADEPLAN_STOCK_NAME_AUDIT.md](./PHASE10_TRADEPLAN_STOCK_NAME_AUDIT.md)  
> **目标：** Universe 识别 `SECURITY_SHORT_NAME`，使 `candidate_pool_items` / `trade_plan_items.stock_name` 正确落库

---

## 1. 问题摘要

现网策略 run `result_json.dataList` 使用东财短名字段 **`SECURITY_SHORT_NAME`**（例：联创光电 / 欣兴工具），而 `parseStrategyRunItems` 只认 `SECURITY_NAME_ABBR` 等键 → `StockName` 空串贯穿 CandidatePool → TradePlan → UI（部分股票再被 tushare enrich「修妆」）。

**最小修复范围：** 仅扩展 Universe 名称字段映射；不改 schema、不改 enrich、不做历史回填（回填可另开切片）。

---

## 2. 当前字段映射

文件：`backend/strategy/universe.go`（函数 `parseStrategyRunItems`）

### 2.1 代码解析（策略 JSON → UniverseCandidate）

| 语义 | 当前 `firstString` 键顺序（先匹配先生效） |
|------|------------------------------------------|
| **代码** | `SECUCODE`, `secucode`, `SECURITY_CODE`, `security_code`, `stockCode`, `StockCode`, `code`, `Code` |
| **名称** | `SECURITY_NAME_ABBR`, `security_name_abbr`, `stockName`, `StockName`, `name`, `Name` |
| **行业** | `INDUSTRY`, `industry` |

**缺失：** `SECURITY_SHORT_NAME` / `security_short_name`（以及前端已用的部分别名，见 §5 可选）。

### 2.2 策略 JSON parser 形态

```text
stock_strategy_runs.result_json
  → json.Unmarshal → models.StockStrategyRunView
  → view.DataList (any) → []map[string]any
  → 逐行 firstString(...)
```

- View 定义：`backend/models/stock_strategy.go` → `DataList any \`json:"dataList"\``
- **无强类型 row struct**；字段名完全依赖 map key 字符串匹配
- 实测 run=50 行形态：`SECURITY_CODE` + `SECURITY_SHORT_NAME` + `MARKET_SHORT_NAME`（**无** `SECURITY_NAME_ABBR`）

### 2.3 对照：前端已兼容 SHORT_NAME

`frontend/src/utils/stockCode.js` → `resolveStrategyRowName`：

```text
SECURITY_SHORT_NAME → SECURITY_NAME_ABBR → 股票名称 → SECURITY_NAME → name
```

后端 Universe 落后于前端，导致「策略页有名、TradePlan 落库无名」。

---

## 3. 缺失原因（因果）

```text
策略 dataList.SECURITY_SHORT_NAME = "欣兴工具"
        │
        ▼
parseStrategyRunItems nameRaw = firstString(仅 ABBR/stockName/name…)
        │  ← 全部未命中
        ▼
UniverseCandidate.StockName = ""
        │
        ▼
BuildCandidatePool → candidate_pool_items.stock_name = ""
        │
        ▼
BuildDraftTradePlanFromCandidatePool → trade_plan_items.stock_name = ""
        │
        ▼
GET upcoming：DB 空名 + enrich(tushare) 部分掩盖
```

**不是** Draft 写丢字段；**不是** UI 绑错；根因是 **Universe 名称键表过窄**。

---

## 4. Universe persistence 与 trade_plan_items 来源

| 步骤 | 文件 | stock_name 行为 |
|------|------|-----------------|
| 1. 解析 | `backend/strategy/universe.go` `parseStrategyRunItems` | 写入 `UniverseCandidate.StockName` |
| 2. 落池 | `backend/strategy/build_candidate_pool.go` | `CandidatePoolItem.StockName = c.StockName` → DB |
| 3. Draft | `backend/strategy/build_draft_trade_plan.go` `draftPlanItemsFromFilter` | `TradePlanItem.StockName = c.StockName`（透传） |
| 4. 展示 | `backend/api/stock_name_enrich.go` | 仅空名时 tushare 补全（**本设计不改**） |

Follow 路径（`loadFollowCandidates`）用 `followed_stock.Name`，不受本缺陷影响。

---

## 5. 字段优先级方案（最小修复）

### 5.1 原则

1. **保持已有键优先级不变**（先列者优先）  
2. **在现有序列之后**追加东财短名兼容键  
3. 不调整前端顺序；后端以「兼容补缺」为准，避免改变已有 ABBR 样本行为

### 5.2 建议名称键顺序（实现时一字不差按此序）

```text
SECURITY_NAME_ABBR          // 既有 #1
security_name_abbr          // 既有
stockName                   // 既有
StockName                   // 既有
name                        // 既有
Name                        // 既有
SECURITY_SHORT_NAME         // ★ 新增（现网策略主字段）
security_short_name         // ★ 新增（小写容错）
```

对应伪代码：

```go
nameRaw := firstString(row,
	"SECURITY_NAME_ABBR", "security_name_abbr",
	"stockName", "StockName", "name", "Name",
	"SECURITY_SHORT_NAME", "security_short_name",
)
```

### 5.3 明确不在本切片

| 项 | 原因 |
|----|------|
| 把 `SECURITY_SHORT_NAME` 提到 ABBR 之前 | 违反「保持已有优先级」；且无证据表明需覆盖 ABBR |
| 改 `defaultStockNameLookup` / tushare | 治标；本切片治本落库 |
| 回填 plan #31 历史空名 | 数据运维另开；重新 generate-next 即可验证 |
| 扩展 `SECURITY_NAME` / `股票名称` | 可选 P1；非最小必改（前端有，后端可后续对齐） |

### 5.4 ST 过滤副作用（预期为正）

`isSTName(nameRaw)` 在名为空时几乎不生效。补上 SHORT_NAME 后，带 ST 前缀的短名会被正确跳过——与策略意图一致，应在测试中覆盖。

---

## 6. 修改文件列表

| 文件 | 变更 | 必选 |
|------|------|------|
| `backend/strategy/universe.go` | `parseStrategyRunItems` 名称 `firstString` 追加两键 | **是** |
| `backend/strategy/universe_test.go` | 增加 SHORT_NAME / 优先级 / ST 用例 | **是** |
| `PHASE10_TRADEPLAN_STOCK_NAME_FIX_DESIGN.md` | 本设计 | 文档 |
| `backend/api/stock_name_enrich.go` | 不改 | 否 |
| `frontend/**` | 不改 | 否 |
| DB / schema | 不改 | 否 |

**预估 diff：** ~2 行生产代码 + 若干单测。

---

## 7. 回归风险

| 风险 | 等级 | 说明 / 缓解 |
|------|------|-------------|
| 同时存在 ABBR 与 SHORT_NAME 且不一致 | 低 | 仍优先 ABBR；与「保持已有优先级」一致 |
| SHORT_NAME 含 ST，候选变少 | 低–中 | 正确行为；单测固定 ST 样例；观察 Candidate 数量日志 |
| 仅改映射、旧 plan 仍空名 | 低 | 预期；需新生成或另做回填 |
| Follow 路径回归 | 极低 | 不碰 `loadFollowCandidates` |
| enrich / UI | 极低 | 落库有名后 enrich 不再触发；展示更稳 |
| 性能 | 无 | 仅多两次 map 查找 |

---

## 8. 测试方案

### 8.1 单元测试（`universe_test.go`）

| 用例 | 输入 row | 期望 `StockName` |
|------|----------|------------------|
| `TestParseStrategyRunItems_ShortNameOnly` | 仅 `SECURITY_CODE` + `SECURITY_SHORT_NAME=欣兴工具` | `欣兴工具` |
| `TestParseStrategyRunItems_AbbrPreferredOverShort` | 同时有 ABBR=`招商银行`、SHORT=`招行` | `招商银行`（优先级） |
| `TestParseStrategyRunItems_LegacyAbbrStillWorks` | 仅 `SECURITY_NAME_ABBR` | 仍解析成功 |
| `TestParseStrategyRunItems_ShortNameSTSkipped` | SHORT=`ST示例` | 不进入结果集 |

实现注意：`parseStrategyRunItems` 当前非导出；可选：

- 同包直接测；或  
- 抽小函数 `strategyRowName(row map[string]any) string` 专测优先级（推荐，便于单测且改动仍最小）。

### 8.2 包级 / 集成（可选但建议）

```text
go test ./backend/strategy -count=1 -run "ParseStrategyRunItems|Universe|IsSTName"
```

手工：构造含 `SECURITY_SHORT_NAME` 的 fake run JSON → `BuildCandidatePool` → 断言 `candidate_pool_items.stock_name` 非空（可用现有 sqlite 测试夹具）。

### 8.3 Runtime 验收（实现后）

1. `POST /api/tradeplans/generate-next`（或 after_close）基于含 SHORT_NAME 的最新 strategy run  
2. DB：`trade_plan_items` 对 `sz301677` / `sh600363`（或当次标的）`stock_name` 非空  
3. UI：两行「名称」均有值，且不依赖 tushare 是否收录 `301677`

### 8.4 非目标验证

- 不要求旧 plan #31 自动变好  
- 不要求 tushare 补齐 `301677`

---

## 9. 实现切片建议（后续执行，非本设计）

1. 改 `universe.go` 键表 + 单测  
2. `go test ./backend/strategy`  
3. 可选 scoped commit：`fix(strategy): map SECURITY_SHORT_NAME into universe stock name`  
4. Runtime generate-next 抽检  

**本文件阶段：只读设计完成；未改代码、未 commit。**

---

## 10. Sign-off Checklist

| 项 | 状态 |
|----|------|
| 当前字段映射 | 已记录 |
| 缺失原因 | Universe 键表缺 `SECURITY_SHORT_NAME` |
| 修改文件列表 | `universe.go` + `universe_test.go` |
| 字段优先级方案 | 既有键不变，其后追加 SHORT_NAME |
| 回归风险 | 已列 |
| 测试方案 | 单测 + runtime 验收 |
| 未改代码 / 未 commit | ✅ |
