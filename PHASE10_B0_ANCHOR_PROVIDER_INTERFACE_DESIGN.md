# PHASE10_B.0 — AnchorProvider Interface Design

> **切片：** Phase10-B.0  
> **基线 commit：** `5beafb82897de1210c36a57e8f089cc8880ce163`（`5beafb8`）  
> **依据：** `PHASE10_B_ANCHOR_LAYER_DESIGN.md` §6.3 B.0  
> **性质：只读设计 — 不修改代码、不实现 B.1/B.2、不改 schema**  
> **日期：** 2026-08-05  

---

## 0. 切片目标与边界

### 0.1 目标

在 **不改变盘后选中语义结果** 的前提下，引入：

1. `ExecutionIntentAnchorProvider` 接口与 IO 模型  
2. 唯一生产源：`FollowedStockAnchorSource`（行为对齐现网 `defaultAfterCloseAnchor`）  
3. `strategy.populateAfterCloseExecutionIntent` 改为经 Provider 解析（可注入）  
4. 为 B.1+ 预留 Chain 插槽（B.0 **只挂 Followed**）

### 0.2 明确不做（本切片）

| 禁止 | 说明 |
|------|------|
| B.1 Kline Close 源 | 不读 `kline_cache` / `stock_kline_day` / `KlineService` |
| B.2 Candidate Snapshot | 不改 `CandidatePoolItem`、不新建价表 |
| B.3 Market Snapshot Store | 不建 EOD Store / Collector |
| stock_info 降级源 | 不接入 |
| 改晨间 Open / Limit / Volume / Readiness / Approve·Freeze | Phase10-A 冻结面 |
| 放宽 soft-fail | 无价仍不写 `selected` |
| 假价 / Open→ref | 禁止 |

### 0.3 成功标准（B.0）

| # | 标准 |
|---|------|
| 1 | 生产路径价源仍仅为 `followed_stock`（FollowPrice → Price） |
| 2 | 同输入下：`ref_price` / 是否 `selected` / soft-fail 与接线前一致 |
| 3 | `strategy` 不再直连 `FollowedStock` 查询（迁入 adapter） |
| 4 | 单测可注入 fake Provider；现有 populate 语义单测可迁移不降覆盖 |
| 5 | `marketdata/anchor` 无 TradePlan 状态机 / 下单 API |

---

## 1. 现网检查结论（`5beafb8`）

### 1.1 `afterCloseAnchor` 调用点

| 位置 | 角色 |
|------|------|
| `backend/strategy/after_close_intent_populate.go` | **唯一定义与生产调用** |
| `var afterCloseAnchorFunc = defaultAfterCloseAnchor` | 包级可替换函数（测试钩子） |
| `populateAfterCloseExecutionIntent` → `resolve(code, pool, tradeDate)` | 每 eligible item 调一次 |
| `BuildDraftTradePlanFromCandidatePool` | **唯一生产入口**调用 populate |
| `after_close_intent_populate_test.go` | 覆盖 `afterCloseAnchorFunc` 做 fake |

无其它包、无 HTTP、无 Cron 直接调用 `defaultAfterCloseAnchor`。

**现网签名：**

```text
defaultAfterCloseAnchor(stockCode string, pool *CandidatePool, tradeDate string) (afterCloseAnchor, bool)

afterCloseAnchor { Price float64; Source string; AsOf string }
```

**现网 Followed 行为（须 B.0 比特兼容）：**

1. `db.Dao == nil` 或 code 空 → miss  
2. `followed_stock WHERE stock_code=?` miss 或价均 ≤0 → miss  
3. 价：`FollowPrice`，若 ≤0 则 `Price`  
4. `AsOf`：有 `follow.Time` 用其日期，否则 `tradeDate`  
5. `Source`：默认 `prev_close`；`pool.Source == strategy_run` → `strategy_snapshot`（价仍来自自选）

### 1.2 Intent 数据结构（schema v4，不改）

**Plan 级（populate 写入，B.0 不变）：**

| 字段 | 现网 AfterClose 值 |
|------|-------------------|
| `DefaultEntryRule` | `LIMIT_REF_PLUS_SLIP` |
| `DefaultMaxSlippage` | `0.03` |
| `PricingPolicyVersion` | `1` |
| `PricingStage` | `after_close_intent` |

**Item 级 Intent（锚点相关）：**

| 字段 | 成功锚点 | soft-fail / 非 eligible |
|------|----------|-------------------------|
| `RefPrice` | `>0` | `0` |
| `RefSource` | `prev_close` / `strategy_snapshot` | `""` |
| `RefAsOf` | 日期字符串 | `""` |
| `EntryRule` | `LIMIT_REF_PLUS_SLIP` | `""` |
| `MaxSlippage` | `0.03` | `nil` |
| `IntentStatus` | `selected` | `""` |
| `LimitPrice` / `TargetVolume` / `OpenRefPrice` | 强制 `0` | 强制 `0` |
| `PricedAt` / `PricedBy` | 清空 | 清空 |

Eligible：side 空或 `buy`；status 非 `skipped`/`error`。

**B.0 不改表、不加列。** `confidence` 仅 Provider 返回值内可选携带；**默认不落库**（避免 schema；观测走日志即可）。

### 1.3 `populateAfterCloseExecutionIntent` 接口

```text
// 现网
func populateAfterCloseExecutionIntent(
  plan *models.TradePlan,
  items []models.TradePlanItem,
  pool *models.CandidatePool,
)
```

- 副作用：改 `plan` 默认 Intent 字段 + 逐条改 `items`  
- 不落库（由 `CreatePlanWithItems` 负责）  
- 不改 item 数量 / Risk 字段  

`source_date` 已在 after_close 工作流写入 `pool.ConfigJSON`（`source_date`），但 **现网 anchor 未读**。B.0 可解析进 `AnchorContext.SourceDate` 供日志与后续切片；**Followed 适配器仍忽略其对 AsOf 的影响**（兼容）。

### 1.4 `marketdata` 包结构（现状）

```text
backend/marketdata/
  interfaces.go          // KlineService, QuoteService, Bar, Quote, 错误哨兵
  quote_service.go       // NormalizeCode(s), FindQuote
  kline_service.go       // Period/Adjust 规范化
  marketdata_test.go
  adapter/
    legacy_quote_adapter.go
    eastmoney_kline_adapter.go
```

- 边界：只读行情；禁止 TradePlan/Risk/Order 字段  
- **尚无** `anchor/` 子包  
- adapter 已依赖 `backend/data`（先例：Followed 适配可同样依赖）

---

## 2. AnchorProvider 接口定义（B.0）

### 2.1 包路径

```text
backend/marketdata/anchor/
  doc.go                 // 包边界说明
  types.go               // Context / Result / RefSource 常量
  provider.go            // Provider 接口 + 校验辅助
  followed.go            // FollowedStockAnchorProvider（B.0 唯一切实源）
  followed_test.go
  // 预留（B.0 可只放空文件或注释，不实现逻辑）：
  // chain.go            // ChainProvider — B.0 可提供「单元素链」薄封装
```

包名：`anchor`（import：`go-stock/backend/marketdata/anchor`）。

### 2.2 接口

```go
// Provider resolves after-close Execution Intent price anchors.
// ok=false means no reliable price; callers must soft-fail (no fabricated prices).
type Provider interface {
    Resolve(ctx Context) (Result, bool)
}
```

命名：对外文档称 **ExecutionIntentAnchorProvider**；Go 类型名 **`Provider`**（短名，包路径已限定域）。

### 2.3 输入模型 `Context`

```go
type Context struct {
    StockCode  string // required；建议 TrimSpace，不做市场前缀改写（与 Quote NormalizeCode 一致）
    TradeDate  string // 计划交易日（通常 T+1）；YYYY-MM-DD
    SourceDate string // 盘后源日 T；可空（B.0 Followed 不依赖）
    PoolID     uint   // 可选；B.0 不用于取价
    PoolSource string // 如 models.CandidatePoolSourceStrategyRun
    Session    string // 如 "after_close"；可选观测
}
```

**构建规则（strategy 侧）：**

| 字段 | 来源 |
|------|------|
| `StockCode` | `items[i].StockCode` |
| `TradeDate` | `plan.TradeDate` |
| `SourceDate` | 解析 `pool.ConfigJSON["source_date"]`；失败则 `""` |
| `PoolID` | `pool.ID`（pool 非 nil） |
| `PoolSource` | `pool.Source` |
| `Session` | ConfigJSON `session` 或常量 `after_close` |

### 2.4 输出模型 `Result`

```go
type Result struct {
    RefPrice   float64 // must be > 0 when ok=true
    RefSource  string  // 见 §2.5
    RefAsOf    string  // YYYY-MM-DD；可空（populate 仍可用 TradeDate 回填）
    Confidence float64 // B.0：Followed 建议 0.5；不落库
}
```

**Provider 契约校验（建议 `Valid(Result) bool` 辅助）：**

- `ok==true` ⇒ `RefPrice > 0` 且 `RefSource != ""`  
- `ok==false` ⇒ 调用方忽略 Result 内容  

### 2.5 `RefSource` 常量（B.0）

| 常量 | 值 | B.0 是否写入 |
|------|-----|--------------|
| `RefSourcePrevClose` | `prev_close` | **是**（兼容现网 Followed 默认） |
| `RefSourceStrategySnapshot` | `strategy_snapshot` | **是**（兼容：仅当 PoolSource=strategy_run） |
| `RefSourceFollowedPrice` | `followed_price` | 否（B.1+ 可选切换；B.0 默认关） |
| `RefSourceFollowedFollowPrice` | `followed_follow_price` | 否（同上） |
| `RefSourceKlineClose` 等 | … | **不实现取值**（常量可声明供编译期引用，无源） |

**B.0 默认：`LegacyFollowedLabels = true`**  
→ 继续写 `prev_close` / `strategy_snapshot`，保证与现网及依赖这些字符串的只读诊断一致。  
→ 新枚举仅预留；开启新标签属显式后续切片，不在 B.0 验收内。

### 2.6 Followed 实现要点

```text
FollowedStockAnchorProvider
  Resolve(ctx):
    if code empty or Dao nil → false
    load followed_stock by stock_code
    px = FollowPrice; if px<=0 { px = Price }
    if px<=0 → false
    asOf = follow.Time.date || ctx.TradeDate
    source = prev_close
    if ctx.PoolSource == strategy_run → strategy_snapshot
    confidence = 0.5
    return Result{px, source, asOf, 0.5}, true
```

禁止：写 DB、改自选、拉实时 Open、读 K 线。

### 2.7 Chain（B.0 最小形态）

可选薄封装，便于 B.1 插入而不改 populate：

```go
type Chain struct {
    Sources []Provider // B.0: []{ FollowedStockAnchorProvider{} }
}

func (c Chain) Resolve(ctx Context) (Result, bool) {
    for _, s := range c.Sources {
        if s == nil { continue }
        if r, ok := s.Resolve(ctx); ok && r.RefPrice > 0 {
            return r, true
        }
    }
    return Result{}, false
}
```

B.0 生产默认：`Chain{Sources: []Provider{FollowedStockAnchorProvider{}}}`  
**不得**在 Sources 中加入未实现的 Kline/Candidate（避免半截行为）。

---

## 3. 调用链变化

### 3.1 现网

```text
BuildDraftTradePlanFromCandidatePool(pool)
  → populateAfterCloseExecutionIntent(plan, items, pool)
       → afterCloseAnchorFunc(code, pool, tradeDate)
            → defaultAfterCloseAnchor → followed_stock
```

### 3.2 B.0 目标

```text
BuildDraftTradePlanFromCandidatePool(pool)
  → populateAfterCloseExecutionIntent(plan, items, pool)
       → build AnchorContext(code, plan, pool)
       → afterCloseAnchorProvider.Resolve(ctx)     // 包级可注入
            → Chain → FollowedStockAnchorProvider
       → 写 ref_* / selected | soft-fail（逻辑同现网）
```

### 3.3 strategy 侧接线设计

| 现有 | B.0 |
|------|-----|
| `afterCloseAnchorFunc` 函数变量 | 保留为 **测试适配层**，或改为 `afterCloseAnchorProvider Provider` |
| `defaultAfterCloseAnchor` | 删除或变为对 Followed Provider 的 1:1 委托（避免双实现） |

**推荐：**

```text
var afterCloseAnchorProvider marketdata/anchor.Provider = defaultAfterCloseAnchorProvider()

func defaultAfterCloseAnchorProvider() Provider {
    return anchor.Chain{Sources: []anchor.Provider{anchor.FollowedStockAnchorProvider{}}}
}
```

测试：`t.Cleanup` 恢复 Provider；fake 实现 `Resolve`。

若希望少改测试文件：保留

```text
afterCloseAnchorFunc = func(code, pool, tradeDate) (afterCloseAnchor, bool) {
    r, ok := afterCloseAnchorProvider.Resolve(ctxFrom(...))
    return afterCloseAnchor{r.RefPrice, r.RefSource, r.RefAsOf}, ok
}
```

作为临时桥；长期测试直接 mock `Provider`。

### 3.4 populate 伪代码（行为不变）

```text
for each item:
  force limit/volume/open_ref/priced_* = zero
  if !eligible → clear intent fields; continue
  ctx = Context{...}
  res, ok = provider.Resolve(ctx)
  if !ok || res.RefPrice <= 0 → soft-fail clear; continue
  write RefPrice/Source/AsOf, EntryRule, MaxSlippage, IntentStatus=selected
  // confidence: log only in B.0
```

**不改** `populateAfterCloseExecutionIntent` 的对外 Go 签名（仍三参），降低调用面噪声；Context 在函数内组装。

### 3.5 不变的下游

```text
CreatePlanWithItems
MaterializeMorningLimitPrices   // 仍要求 selected && ref>0
Readiness / Approve / Freeze    // 不改
```

---

## 4. 兼容旧 `followed_stock` 行为方案

### 4.1 兼容矩阵

| 行为点 | 现网 | B.0 要求 |
|--------|------|----------|
| 唯一生产价源 | followed_stock | 同 |
| 价优先级 | FollowPrice → Price | 同 |
| miss → 不 selected | 是 | 同 |
| AsOf | Time 日期 / tradeDate | 同（**不用** SourceDate 改写 AsOf） |
| Source 标签 | prev_close / strategy_snapshot | 同（Legacy 模式） |
| strategy_run 且不在自选 | miss | **仍 miss**（预期；B.1 才改善） |
| 禁止假价 | 是 | 同 |

### 4.2 双实现禁止

B.0 merge 后只保留 **一处** Followed 查询逻辑（`marketdata/anchor/followed.go`）。  
`defaultAfterCloseAnchor` 若保留，必须是纯委托，无第二套 SQL。

### 4.3 回归探针（实现时手工/单测）

| Case | 期望 |
|------|------|
| 自选有 FollowPrice>0 | selected，ref=FollowPrice，source 按 pool |
| FollowPrice=0，Price>0 | selected，ref=Price |
| 无自选行 | 不 selected，ref=0 |
| pool=nil | source=`prev_close`（若有价） |
| pool.Source=strategy_run 且有自选 | source=`strategy_snapshot` |
| Dao nil | miss |

### 4.4 与「修正 ref_source 语义」的关系

父设计允许未来改为 `followed_price` / `followed_follow_price`。  
**B.0 默认不切换**，避免静默改变审计字符串与外部诊断脚本。  
若需切换：另开「B.0.1 labels」微切片 + 更新断言，不混进本接口切片验收。

---

## 5. 测试方案

### 5.1 `marketdata/anchor` 单测

| 测试 | 内容 |
|------|------|
| `TestFollowedResolve_FollowPricePreferred` | FollowPrice 优先于 Price |
| `TestFollowedResolve_FallbackPrice` | FollowPrice≤0 用 Price |
| `TestFollowedResolve_Miss` | 无行 / 双价≤0 / 空 code |
| `TestFollowedResolve_SourceLabels` | strategy_run → strategy_snapshot，否则 prev_close |
| `TestFollowedResolve_AsOfFromFollowTime` | Time 非零时 AsOf=该日 |
| `TestChain_OnlyFollowed` | 单源链与直调 Followed 一致 |
| `TestResult_OkRequiresPositivePrice` | Valid 辅助 |

DB：沿用 strategy 测试惯用的 sqlite 内存/`setup` 模式，或最小 AutoMigrate `FollowedStock`。

### 5.2 `strategy` 单测迁移

| 现有测试 | B.0 调整 |
|----------|----------|
| `TestBuildDraftTradePlanFromCandidatePool_PopulatesAfterCloseIntent` | fake 改挂 Provider 或桥接 func |
| `TestBuildDraftTradePlanFromCandidatePool_LimitAndVolumeStayZero` | 同上 |
| `TestPopulateAfterCloseExecutionIntent_DoesNotChangeItemCount` | 同上 |
| `TestCreatePlanWithItems_LegacyPlanUnaffected…` | **不改**（不经 populate） |

新增：

| 测试 | 内容 |
|------|------|
| `TestPopulate_UsesProviderSoftFail` | Provider miss → 不 selected |
| `TestPopulate_DefaultProviderFollowedParity`（可选集成） | 写入 followed 行后默认 Provider 与旧行为一致 |

### 5.3 非目标测试（B.0 不做）

- Kline / Candidate / stock_info 源  
- 晨间物化端到端（属 A / 后续）  
- 真实外网 QuoteService  

### 5.4 验收检查清单（实现 PR）

- [ ] `go test ./backend/marketdata/anchor/...`  
- [ ] `go test ./backend/strategy/ -run AfterClose|Populate|DraftTradePlan`  
- [ ] grep：`defaultAfterCloseAnchor` 无独立 SQL 副本  
- [ ] grep：`marketdata/anchor` 无 `Approve`/`Freeze`/`TradePlanRepo`  
- [ ] 无 schema migration  

---

## 6. 依赖与边界

```text
strategy  ──imports──►  marketdata/anchor  ──imports──►  data (FollowedStock), db
                              │
                              └── 不 import strategy / execution / papertrading
```

- 避免循环依赖：Provider 接口与 Followed 实现均在 `anchor`  
- `confidence` 不反向依赖 readiness 包  

---

## 7. 实施步骤建议（实现阶段参考，本文不执行）

1. 新增 `marketdata/anchor` types + `Provider` + Followed + 单测  
2. strategy：默认 Provider = Chain{Followed}；populate 改 Resolve  
3. 迁移/桥接测试钩子  
4. 删除或委托旧 `defaultAfterCloseAnchor` 体  
5. 跑 §5.4 清单  

**仍不包含：** Kline 源、Candidate 价列、EOD Store。

---

## 8. 与父文档对照

| 父文档 B.0 | 本文落点 |
|------------|----------|
| 接口 + Followed 适配 + populate 接线 | §2–§3 |
| 行为兼容；∉自选仍 miss | §4 |
| 不涉及 B.1/B.2 | §0.2、§5.3、§7 |

---

## 9. Registry

```text
PHASE10-B.0   DESIGN-ONLY  — AnchorProvider 接口 + Followed 兼容接线方案（本文）
PHASE10-B     DESIGN       — PHASE10_B_ANCHOR_LAYER_DESIGN.md
DATA-003      OPEN         — 唯一耦合 followed；B.0 不关闭该债，仅结构化入口
```

---

## 10. 边界声明

- 基于 **`5beafb8`** 与 `PHASE10_B_ANCHOR_LAYER_DESIGN.md` 只读细化  
- **未**修改任何业务代码 / 测试 / schema  
- **未**实现 Provider  
- 下一步若开工：按 §7 实现 B.0，验收 §0.3 + §5.4  
