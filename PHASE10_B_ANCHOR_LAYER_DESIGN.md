# PHASE10_B — Execution Intent Anchor Layer Design

> **阶段：** Phase10-B（实现归属；本文为设计）  
> **基线 commit：** `5beafb82897de1210c36a57e8f089cc8880ce163`（`5beafb8`）  
> **性质：只读设计 — 不修改业务代码、不改 schema、不实现 Provider**  
> **日期：** 2026-08-05  
> **上游：** `PHASE10_A3_EXECUTION_INTENT_ANCHOR_DESIGN.md`、`DATA-003_Phase10_Backlog.md`、`DATA-001_Phase10_Backlog.md`  
> **前提：** Phase10-A（Draft Intent → Morning Materialize → Readiness）已冻结；本文解上游锚点断点  

---

## 0. 结论先行

| 问题 | 判定 |
|------|------|
| Intent 为何经常无 `selected`？ | `defaultAfterCloseAnchor` **仅**读 `followed_stock`；`strategy_run` 候选常不在自选 |
| 根因 | **Anchor 源耦合**（DATA-003）+ 无统一 EOD 价快照（DATA-001 同族） |
| Phase10-A 是否再扩？ | **否** |
| Phase10-B 做什么 | **Execution Intent Anchor Layer**：统一 `ref_price` 解析契约 + 多源链，挂在 Market Data Layer 之上 |
| 与晨间 open | **正交**：`ref_price`=盘后锚；`open`/`open_ref_price` 仍走 Realtime Open |

---

## 1. 当前价字段来源盘点（`5beafb8`）

### 1.1 语义分流（先分清再设计）

| 字段 / 概念 | 时点 | 现网用途 | 谁写 |
|-------------|------|----------|------|
| `trade_plan_items.ref_price` | **盘后 T**（意图锚） | AfterClose Intent → 晨间算 limit | `populateAfterCloseExecutionIntent` |
| `trade_plan_items.ref_source` / `ref_as_of` | 随 ref | 审计 | 同上 |
| `trade_plan_items.open_ref_price` | **交易日开盘** | 晨间物化 | `MaterializeMorningLimitPrices` |
| `trade_plan_items.limit_price` | 物化后 | Order Spec | 晨间物化（由 ref±slip 或 open 规则） |
| 实时 `Open` / `Price` / `PreClose` | 拉数时刻 | UI / Paper fill / Mark | `GetStockCodeRealTimeData` 等 |
| 日 K `Close` | bar 日 | 图表 /（潜在）EOD 锚 | `kline_cache` / `stock_kline_day`（后者热路径未灌） |

**硬规则（沿用）：** 禁止用实时 **Open** 回填盘后 `ref_price`；禁止用预算/假价填 ref；缺价 soft-fail（不写 `selected`）。

### 1.2 `followed_stock`（现行唯一 Intent 锚点）

| 项 | 内容 |
|----|------|
| 模型 | `data.FollowedStock` → 表 `followed_stock` |
| 价字段 | `FollowPrice`（加入自选时价，**不随行情更新**）、`Price`（行情位，更新依赖其它路径） |
| 消费 | `strategy.defaultAfterCloseAnchor`（`after_close_intent_populate.go`） |
| 取值序 | **先** `FollowPrice`，≤0 再 `Price`；均 ≤0 → miss |
| `ref_as_of` | 优先 `follow.Time` 的日期；否则计划 `tradeDate`（常为 T+1，语义偏弱） |
| `ref_source` | 常量 `prev_close`；若 `pool.Source==strategy_run` 则标 `strategy_snapshot`（**价仍来自自选**，标签易误导） |

### 1.3 `CandidatePool` / `candidate_pool_items`

| 有 | `stock_code`, `rank`, `score`, `signal_*`, `decision_id`, `strategy_*`, … |
|----|----------------------------------------------------------------------------|
| **无** | open / close / last / ref / as_of / pre_close |

池角色 = **选股快照**，不是价快照。`BuildCandidatePool` 管线：Universe → score → SignalSnapshotEnhancer → Rank → 截断 → 落库；**全程不落价**。

`ConfigJSON` 可含 `session=after_close`、`source_date`、`signalSnapshotId` 等观测元数据，**不含行情**。

### 1.4 Strategy / Signal Snapshot（勿与价快照混淆）

| 名称 | 实际含义 | 含价？ | 被 Intent 用？ |
|------|----------|--------|----------------|
| `CandidatePool` + `Source=strategy_run` | 策略候选版本 | 否 | 仅影响 `ref_source` 字符串 |
| `SignalScanSnapshot` | 全市场信号扫描结果 | 信号行可有展示价字段，**非 Intent 契约** | Enhancer 用 tag/score；**populate 不读价** |
| `ref_source=strategy_snapshot` | 标签 | — | 误导：价≠策略快照价 |

**结论：** 今日不存在「Strategy Snapshot 价源」；要建的是 **Market / Candidate Price Snapshot**，不是复用信号 JSON。

### 1.5 `stock_info`

| 项 | 内容 |
|----|------|
| 表 | `stock_info`（`data.StockInfo`） |
| 相关列 | `PreClose`、`Open`、`Price`（多为 **string**）、`Date`/`Time`、`FollowPrice` 等 |
| 写入 | UI/拉数路径分散（DATA-001） |
| Intent | **populate 不读** |
| 风险 | 覆盖不全（plan#19 实证候选码常无行）；`Date` 不一定等于盘后 `source_date` |

可作为 Chain **降级源**，不可当唯一真相。

### 1.6 K 线缓存与正式日线

| 层 | 位置 | 说明 |
|----|------|------|
| HTTP TTL 缓存 | `kline_cache`（`KLineCacheRecord`：secid+klt+adjust+end_key → JSON payload，`last_bar_day`） | 图表/东财路径热缓存；TTL 秒～日级；**非可审计 EOD Store** |
| 正式表 | `stock_kline_day` / `stock_kline_minute` + `StockKLineRepo` | 已建模；**生产热路径未稳定 Upsert**（Phase7 文档结论） |
| Market Data 读口 | `marketdata.KlineService.GetBars` | Phase7 接口 + adapter；Agent 等已部分迁移 |

**潜力：** 对齐 `source_date` 的日线 `Close` = 最强本地 `prev_close` 候选。  
**缺口：** 缺「按 code×trade_date 取 close」的 Anchor 门面；批处理灌库与 Intent 未接线。

### 1.7 Realtime provider

| 项 | 内容 |
|----|------|
| 实现 | `papertrading.RealtimeOpenPriceProvider` → `GetStockCodeRealTimeData` |
| 输出 | `OpenQuote` → **今日 Open**；`MarkPrice` → 最新价/Open |
| Phase10-A 接线 | `existingRealtimeOpenPriceFn` → 晨间 `MaterializeMorningLimitPrices` |
| 规则 | 缺/无效 open → `pending_open`，禁止假价 |
| 与盘后 Intent | **禁止**写入 `ref_price` |

另：`marketdata.QuoteService` 提供统一 `Quote{Price,Open,PreClose,…}`，属只读行情层；**尚未**被 `defaultAfterCloseAnchor` 使用。

### 1.8 来源能力矩阵

| 来源 | 有价？ | as_of 清晰？ | 覆盖 strategy_run？ | 今日被 Intent 用？ |
|------|--------|-------------|---------------------|-------------------|
| followed_stock | 部分 | 弱 | **常否** | **是（唯一）** |
| CandidatePool | 否 | — | 码有价无 | 否 |
| SignalScanSnapshot | 非契约 | 弱 | 信号覆盖≠候选 | 否 |
| stock_info | 部分 | 弱 | 常不全 | 否 |
| kline_cache / kline_day | 潜在强 | 可强 | 取决于是否拉过/灌过 | 否 |
| QuoteService.PreClose | 实时快照 | 日历日弱 | 依赖外网 | 否 |
| Realtime Open | 交易时段 | 开盘日 | 依赖外网 | **仅晨间 Spec** |

---

## 2. 当前调用链与 Intent 断点

```text
POST /api/tradeplans/generate-next
  → RunAfterClosePlanWorkflow(sourceDate=T)
       ① NextTradingDay → tradeDate = T+1
       ② BuildCandidatePool(tradeDate, session=after_close)
            // 码+分，无价
       ③ BuildDraftTradePlanFromCandidatePool(pool)
            → FilterPoolForTradePlan
            → populateAfterCloseExecutionIntent(plan, items, pool)
                 → defaultAfterCloseAnchor(code, pool, tradeDate)
                      → followed_stock ONLY
                      → miss → intent="" / ref=0     ✦ 断点 A（DATA-003）
            → CreatePlanWithItems
       ④ EvaluateDraftTradePlanRisk

POST /api/tradeplans/materialize-morning   （Phase10-A 已接线）
  → MaterializeMorningLimitPrices(openPriceFn=RealtimeOpen…)
       → 要求 intent_status=selected && ref_price>0
       → 否则 legacy_skip → materialized_items=0   ✦ 断点 B（断点 A 的下游症状）
  → MaterializeMorningTargetVolumes
  → EvaluateExecutionIntentReadiness
```

| 断点 | 位置 | 性质 |
|------|------|------|
| **A** | populate / `defaultAfterCloseAnchor` | **根因**：无可靠盘后锚 |
| **B** | 晨间 Limit 物化 | **症状**：无 selected → 全 legacy_skip |
| **C** | Readiness / Approve | **正确阻断**：未物化不应 Ready |

Phase10-B 修 **A**；不放宽 B/C 的假价捷径。

---

## 3. AnchorProvider 抽象

### 3.1 放置位置（推荐）

```text
backend/marketdata/          // Phase7 已有 Quote/Kline 只读层
  interfaces.go              // 已有 KlineService / QuoteService
  anchor/                    // Phase10-B 新建（设计名）
    provider.go              // ExecutionIntentAnchorProvider
    chain.go                 // 优先级链
    sources/
      market_snapshot.go     // EOD Store（完整目标）
      candidate_snapshot.go  // 建池带价（可选切片）
      kline_close.go         // 经 KlineService / Repo
      stock_info.go          // 降级
      followed.go            // 兼容末位
```

**为何挂在 `marketdata` 下：**

- 锚点是**市场数据投影**，不是 TradePlan / Risk / Execution 决策  
- 复用 Phase7 `KlineService` / `QuoteService` 边界（只读、无 TradePlan 字段泄漏）  
- `strategy` 只依赖接口，去掉对 `FollowedStock` 的直连  

备选包名 `backend/intentanchor/` 亦可；须仍 **禁止** 包内出现下单/Approve/Freeze。

### 3.2 接口契约（设计语言）

```text
ExecutionIntentAnchorProvider

Resolve(ctx AnchorContext) (Anchor, ok bool)

AnchorContext:
  stock_code    string     // 规范化代码
  trade_date    string     // 计划交易日（通常 T+1）
  source_date   string     // 盘后日历日 T（锚价所属日）
  pool_id       uint       // 可选
  pool_source   string     // strategy_run | follow | …
  session       string     // after_close | …

Anchor (ok=true):
  ref_price     float64    // > 0
  ref_source    string     // 稳定枚举 §3.3
  ref_as_of     string     // YYYY-MM-DD = 价所属交易日（通常 = source_date）
  confidence    float64    // 0..1，观测用；默认不阻断 selected

ok=false:
  不伪造价；调用方 soft-fail（与现网一致）
```

`strategy.afterCloseAnchorFunc` 演进为对该接口的薄适配；单测继续注入 fake。

### 3.3 `ref_source` 枚举（目标）

| 值 | 含义 |
|----|------|
| `market_snapshot` | 统一 EOD / 盘后 Market Snapshot Store |
| `candidate_snapshot` | 建池时写入、与 `pool_id` 绑定的带价快照 |
| `kline_close` | 日线收盘（对齐 source_date） |
| `stock_info_pre_close` | `stock_info.pre_close`（降级） |
| `followed_price` | `followed_stock.Price` |
| `followed_follow_price` | `followed_stock.FollowPrice`（最低） |

现网 `prev_close` / `strategy_snapshot`：**兼容映射**，新写入应用上表；禁止再用 `strategy_snapshot` 表示「其实来自自选」。

### 3.4 confidence（观测）

| 区间 | 条件 | 建议 |
|------|------|------|
| 0.9–1.0 | market_snapshot 且 as_of==source_date | 正常 |
| 0.7–0.85 | kline_close 对齐 source_date | 可接受 |
| 0.4–0.6 | stock_info / 自选 Price | 可 selected；可选 Readiness WARN |
| &lt;0.4 或 ok=false | 不可靠 | 不写 selected |

**硬规则：** confidence 不能代替 `ref_price>0`。

---

## 4. 数据优先级（source_date = T，trade_date = T+1）

```text
1. Market Snapshot（T 收盘 / 官方 EOD）
       ↓ miss
2. Candidate Snapshot（建池瞬间 per code，绑定 pool_id）
       ↓ miss
3. Kline Close（stock_kline_day 优先；其次经 KlineService/kline_cache 解析 T close）
       ↓ miss
4. stock_info.pre_close（as_of 可接受时；confidence 降）
       ↓ miss
5. followed_stock.Price
       ↓ miss
6. followed_stock.FollowPrice
       ↓ miss
   ok=false → soft-fail
```

### 4.1 排序理由

| 优先级 | 理由 |
|--------|------|
| 1 Market Snapshot | 日历对齐、可复现、覆盖全市场候选（DATA-001 目标） |
| 2 Candidate Snapshot | 建池自洽；全局 Store 未就绪时仍可推进 Intent |
| 3 Kline Close | 本地已有潜力；需统一 Close(code,T) API |
| 4–6 | 兼容与降级；**Followed 不得再当唯一源** |

### 4.2 明确禁止

| 禁止 | 原因 |
|------|------|
| Realtime **Open** → `ref_price` | 时点错误；与晨间 Spec 混淆 |
| `limit_price` / 预算反推 ref | 假 Intent |
| 静默把候选插入 `followed_stock` | 污染自选；掩盖数据层缺口 |
| 放宽 Readiness「无锚也 Ready」 | 掩盖断点 A |

---

## 5. 与 Market Data Layer 的关系

```text
┌─────────────────────────────────────────────────────────┐
│  Market Data Layer（Phase7 基础 + Phase10-B 扩展）        │
│                                                         │
│  QuoteService / KlineService     ← 只读行情（已有）       │
│  Collector / EOD Snapshot Store  ← DATA-001（B 建设）    │
│  ExecutionIntentAnchorProvider   ← 本层投影（B 新建）    │
│       Chain: Snapshot → Candidate → Kline → … → Followed│
└───────────────────────────┬─────────────────────────────┘
                            │ Anchor only
                            ▼
┌─────────────────────────────────────────────────────────┐
│  strategy.populateAfterCloseExecutionIntent             │
│       → 写 item.ref_* / intent_status=selected|""       │
└───────────────────────────┬─────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│  Phase10-A Morning Materialize（已冻结）                 │
│       Open ← RealtimeOpenPriceProvider（独立）            │
│       Limit/Volume ← ref + open + 规则                   │
└─────────────────────────────────────────────────────────┘
```

| 边界 | 规则 |
|------|------|
| Market Data | 只提供价与 as_of/source；**不**写 TradePlan、**不**触发 Approve/Freeze |
| Anchor Layer | 只解析盘后 Intent 锚；**不**算 limit/volume |
| Strategy | 薄消费；soft-fail 行为保持 |
| Paper / Execution | 仍只认 Frozen Spec；本阶段不改成交价源 |

与 DATA-001：Anchor 是 **消费侧统一入口**；Snapshot Store 是 **供给侧**。可同阶段分期交付，但接口先定。

---

## 6. Phase10-B 实施范围

### 6.1 In Scope

1. `ExecutionIntentAnchorProvider` 接口 + ChainProvider  
2. Followed 适配器（行为兼容现网，修正 `ref_source` 语义）  
3. Kline Close 适配（经 `KlineService` 和/或 `StockKLineRepo`）  
4. `populateAfterCloseExecutionIntent` 改为注入 Provider；传入 `source_date` / `pool_id`  
5. 可观测：日志/可选 metrics（code, source, confidence, miss reason）  
6. 单测：fake Provider、soft-fail、禁止假价  
7. （推荐切片）Candidate 建池写价旁路表或 item 扩展列  
8. （完整目标）EOD Market Snapshot Store + 盘后 ensure（与 DATA-001 对齐）  

### 6.2 Out of Scope（本阶段不做）

| 项 | 说明 |
|----|------|
| Phase10-A API/UI 再改 | 物化入口已够；锚点修好后自然有 selected |
| Approve / Freeze / Paper Job / Cron 旁路 | 不放宽门控 |
| TP-001 Upcoming 选择语义 | 正交 |
| 删除 `followed_stock` | 仅降为末位源 |
| 删除 `kline_cache` | 可并存；真相源迁正式日线/Snapshot |
| Realtime Open 假价 | 禁止 |
| 全市场 Collector 一次做完 | 可拆后续 DATA 切片 |

### 6.3 建议切片顺序（实现时）

| 切片 | 内容 | 验收焦点 |
|------|------|----------|
| **B.0** | 接口 + Followed 适配 + populate 接线 | 行为兼容；`strategy_run`∉自选仍 miss（预期） |
| **B.1** | Kline Close 源 + Chain | 有日线/可拉 K 的码 → `selected`+`ref>0`，不必在自选 |
| **B.2** | Candidate Snapshot（建池写价） | 新池与 Intent 同批次价一致 |
| **B.3** | Market Snapshot Store + ensure | 盘后工作流可复现 EOD；confidence 最高 |

每步保持 soft-fail + 禁止假价；**不**为「先出 Paper」跳过 B.1。

### 6.4 模块触点（实现时，非本文改码）

| 模块 | 改动性质 |
|------|----------|
| `backend/marketdata/anchor/*` | **新建** |
| `after_close_intent_populate.go` | 薄：换解析入口 |
| `build_draft_trade_plan` / after_close workflow | 传入 AnchorContext（source_date） |
| `BuildCandidatePool` | 仅 B.2+ 写价快照 |
| Readiness | 默认不改 blocker；可选后续 `LOW_ANCHOR_CONFIDENCE` WARN |
| 晨间物化 / RealtimeOpen | **不动算法** |

---

## 7. 验收标准（设计层 → 实现对照）

Phase10-B 完成后应满足：

1. `strategy_run` 候选 **不必** 在 `followed_stock`，在 Kline/Snapshot 可用时可得 `selected` + `ref_price>0`  
2. `ref_source` / `ref_as_of` 可审计，且 source 字符串与真实价源一致  
3. 无可靠价时仍不写 selected（与现 soft-fail 一致）  
4. Intent 已 selected 且开盘价可用时，晨间物化可 `materialized_items>0`（Phase10-A 路径不变）  
5. 无 Open→ref、无假价、无静默写自选  
6. Market Data 包边界：无 TradePlan 状态机泄漏  

实证对照（历史）：plan#19 / pool#18 五码均不在 `followed_stock` → 在 B.1+ 后同类场景应能锚上（前提本地/ Store 有 T 收盘）。

---

## 8. 相关文档与债务

| 文档 / ID | 关系 |
|-----------|------|
| `PHASE10_A3_EXECUTION_INTENT_ANCHOR_DESIGN.md` | A.3 设计原稿；本文升级为 Phase10-B 实施范围 |
| `PHASE10_A_FINAL_ACCEPTANCE_REPORT.md` | A 已冻结；锚点属上游 |
| `DATA-003_Phase10_Backlog.md` | 本断点债务登记 |
| `DATA-001_Phase10_Backlog.md` | 行情分散 / Store |
| `DATA-002_Phase10_Backlog.md` | 筛选快照人工触发（**勿混编号**） |
| `PHASE7A_MARKET_DATA_DESIGN.md` / `backend/marketdata` | 只读行情层基础 |
| `TP-001_Phase10_Backlog.md` | Upcoming；正交 |

---

## 9. Registry

```text
PHASE10-B   DESIGN       — Execution Intent Anchor Layer（本文）
DATA-003    OPEN         — Anchor 唯一耦合 followed_stock
DATA-001    OPEN         — Market Data 分散 / Snapshot Store
PHASE10-A   FROZEN       — 物化接线；不扩锚点实现
```

---

## 10. 本文边界声明

- 基于 commit **`5beafb8`** 只读分析  
- **未**修改任何业务代码 / schema / 测试  
- **未**实现 `AnchorProvider`  
- 下一步若开工：从 **B.0 → B.1** 起，单独立项与验收  
