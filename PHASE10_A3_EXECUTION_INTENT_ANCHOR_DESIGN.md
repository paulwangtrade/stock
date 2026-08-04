# PHASE10_A3 — Execution Intent Anchor Layer Design

> **阶段标签：** Phase10-A.3（设计切片编号）  
> **性质：只读设计 — 不实现代码、不改 generate-next / 物化 / Readiness / Approve·Freeze·Execution**  
> **日期：** 2026-08-05  
> **依据：** plan#19 / pool#18 只读诊断；`after_close_intent_populate.go`；`PHASE10_A_MORNING_INTENT_MATERIALIZATION_DESIGN.md`；DATA-001 / DATA-002（快照触发）/ 拟 DATA-003（锚点耦合）  

---

## 0. 结论先行

| 问题 | 判定 |
|------|------|
| 为何无 `selected`？ | `defaultAfterCloseAnchor` **仅**读 `followed_stock`；`strategy_run` 候选常不在自选 |
| 根因类别 | **Intent 锚点源耦合** + 缺少统一盘后价快照（与 DATA-001 同族） |
| 是否继续扩 Phase10-A？ | **否** — Phase10-A 已完成「物化接线」；锚点层属 **数据/价源基础设施** |
| 推荐归属 | **Phase10-B / Market Data Layer**（本 A.3 仅出设计与边界） |

Phase10-A 范围保持：Draft Intent → Morning Materialize → Readiness。  
**Anchor Layer 是上游输入契约**，不应塞进 A.1/A.2 的 API/UI 修补。

---

## 1. 当前价格来源盘点

### 1.1 `followed_stock`（现行唯一 Intent 锚点）

| 项 | 内容 |
|----|------|
| 表 | `followed_stock`（GORM `FollowedStock`） |
| 字段 | `FollowPrice`（加入自选时价，**不随行情更新**）、`Price`（行情位，更新依赖其它路径） |
| 消费方 | `strategy.defaultAfterCloseAnchor` |
| 语义 | 被标注为 `prev_close` 或 `strategy_snapshot`（仅改 source 字符串；价仍来自自选） |
| 优点 | 本地、无假价、已有 soft-fail |
| 致命缺口 | **候选 ∉ 自选 → 无锚点**；`Price` 可能过期；与 strategy_run 生命周期脱节 |

### 1.2 `CandidatePool` / `candidate_pool_items`

| 项 | 内容 |
|----|------|
| 有 | `stock_code`, `rank`, `score`, `signal_*`, `decision_id`, … |
| **无** | open / close / last / ref / as_of 等价格列 |
| 角色 | 选股快照，**不是**价快照 |
| 结论 | 今日 **不能** 作为 Intent `ref_price` 源，除非 Phase10-B 扩展「Candidate Snapshot 带价」 |

### 1.3 `stock_info`

| 项 | 内容 |
|----|------|
| 表 | `stock_info`（实时/半实时行情缓存行） |
| 字段 | `pre_close`, `open`, `price`, `follow_price`, … |
| 写入 | 各 UI/行情拉取路径（分散，见 DATA-001） |
| 与 Intent | **populate 不读** |
| 风险 | 覆盖不全（plan#19 五码均无行）；时刻不一定是「source_date 收盘」 |

### 1.4 K 线缓存

| 层 | 表/结构 | 说明 |
|----|---------|------|
| HTTP/东财缓存 | `kline_cache` | secid+klt 粒度 JSON payload；`last_bar_day` |
| 结构化日线 | `stock_kline_day` 等 | 仓库型日/分钟线 |
| 与 Intent | **populate 不读** | 具备「真正 prev_close」潜力，缺统一查询契约与 as_of 对齐 |

### 1.5 Realtime provider

| 项 | 内容 |
|----|------|
| 实现 | `papertrading.RealtimeOpenPriceProvider` → `GetStockCodeRealTimeData` |
| 用途 | **早盘** `MaterializeMorningLimitPrices` 的 **open**（非盘后 ref） |
| 规则 | 缺/无效 open → `pending_open`，禁止假价 |
| 与盘后 Intent | **正交**；不能用开盘价回填 `ref_price`（时点错误） |

### 1.6 来源能力矩阵

| 来源 | 有价？ | as_of 清晰？ | 覆盖 strategy_run？ | 今日被 Intent 使用？ |
|------|--------|-------------|---------------------|----------------------|
| followed_stock | 部分 | 弱（FollowPrice 加入日；Price 更新不定） | **否（常缺失）** | **是（唯一）** |
| CandidatePool | 否 | — | 码有、价无 | 否 |
| stock_info | 部分 | 弱 | 常不全 | 否 |
| kline_cache / kline_day | 潜在强 | 可强 | 取决于是否拉过 | 否 |
| Realtime open | 交易时段 | 开盘日 | 依赖外网 | 仅晨间 Spec |

---

## 2. 当前调用链（Intent 价相关）

```text
POST /api/tradeplans/generate-next
  → RunAfterClosePlanWorkflow(sourceDate)
       ① NextTradingDay → tradeDate (T+1)
       ② BuildCandidatePool(tradeDate, session=after_close)
            // 产出 pool items：码+分，无价；不刷新 Intent 锚点存储
       ③ BuildDraftTradePlanFromCandidatePool(pool)
            → FilterPoolForTradePlan
            → populateAfterCloseExecutionIntent(plan, items, pool)
                 → defaultAfterCloseAnchor(code, pool, tradeDate)
                      → followed_stock only
                      → miss → intent="" / ref=0   ✦ 断点
            → CreatePlanWithItems  // 计划级仍 pricing_stage=after_close_intent
       ④ EvaluateDraftTradePlanRisk

（另路径，Phase10-A 已接线）
POST /api/tradeplans/materialize-morning
  → MaterializeMorningLimitPrices(openPriceFn=RealtimeOpen…)
       → 要求 intent_status=selected && ref_price>0
       → 否则 legacy_skip → materialized_items=0
  → MaterializeMorningTargetVolumes
  → EvaluateExecutionIntentReadiness
```

**断点位置：** ③ populate，不是 物化 API，也不是 Readiness 规则过严。

---

## 3. Anchor Provider 抽象（目标契约）

### 3.1 接口（设计语言，非实现）

```text
ExecutionIntentAnchorProvider

Input:
  stock_code   string   // 规范化代码，如 sh603986
  trade_date   string   // Intent 所属计划交易日（通常 T+1）
  context?     {
    source_date     // 盘后源日 T（日历）
    pool_id         // 可选，用于 Candidate Snapshot
    pool_source     // strategy_run | follow | …
    session         // after_close | …
  }

Output (ok=true):
  ref_price    float64   // > 0
  ref_source   string    // 稳定枚举，见下
  ref_as_of    string    // YYYY-MM-DD（价所属交易日）
  confidence   float64   // 0..1 或分级，供观测；默认不阻断

Output (ok=false):
  不伪造价；调用方保持 soft-fail（不写 selected）
```

### 3.2 `ref_source` 建议枚举

| 值 | 含义 |
|----|------|
| `market_snapshot` | 统一 EOD/盘后 Market Snapshot Store |
| `candidate_snapshot` | 建池时写入的带价快照 |
| `kline_close` | 日线收盘（source_date 或上一交易日） |
| `stock_info_pre_close` | stock_info.pre_close（降级） |
| `followed_price` | followed_stock.Price |
| `followed_follow_price` | followed_stock.FollowPrice（最低优先） |

与现网兼容：今日写入的 `prev_close` / `strategy_snapshot` 可映射迁移，新实现应用上表枚举。

### 3.3 `confidence`（观测用）

| 等级（例） | 条件 | 建议 |
|------------|------|------|
| 0.9–1.0 | market_snapshot 且 as_of==source_date | 正常 |
| 0.7–0.85 | kline_close 对齐 source_date | 可接受 |
| 0.4–0.6 | stock_info / 过期自选 Price | WARN 可进 selected，Readiness 可加 warn（**不改 blocker 除非另立项**） |
| &lt;0.4 或 ok=false | 无可靠价 | 不写 selected |

**硬规则：** confidence 再高也不能代替 `ref_price>0`；**禁止**用计划金额/假价填 ref。

### 3.4 放置位置（逻辑包）

推荐独立于 UI / Cron：

```text
backend/marketdata/anchor/   // 或 backend/intentanchor/
  Provider interface
  ChainProvider (优先级链)
  sources: snapshot, candidate, kline, stockinfo, followed
```

`strategy.populateAfterCloseExecutionIntent` **只依赖接口**，不再直连 `FollowedStock` 查询。

---

## 4. 推荐优先级

在 **source_date = 盘后日历日 T**、**trade_date = T+1 计划日** 前提下：

```text
1. Market Snapshot (T 收盘 / 官方 EOD)
       ↓ miss
2. Candidate Snapshot（建池时对每码落价；与 pool_id 绑定）
       ↓ miss
3. Kline Close（stock_kline_day 或 kline_cache 解析出 T 或上一交易日 close）
       ↓ miss
4. stock_info.pre_close（仅当 as_of 可接受；confidence 降级）
       ↓ miss
5. followed_stock.Price
       ↓ miss
6. followed_stock.FollowPrice
       ↓ miss
   ok=false → soft-fail（现状行为）
```

### 4.1 为何此序

| 优先级 | 理由 |
|--------|------|
| Market Snapshot 最高 | 与日历日对齐、可复现、覆盖全市场候选 |
| Candidate Snapshot 次之 | 建池瞬间一致性；即使全局 Snapshot 未就绪，pool 自洽 |
| Kline Close | 已有本地缓存潜力；需统一「取 T 收盘」API |
| Followed 最低 | 保留兼容；**不得再当唯一源** |

### 4.2 明确禁止的回退

| 禁止 | 原因 |
|------|------|
| Realtime **Open** 写入盘后 `ref_price` | 时点错误；与晨间 Spec 混淆 |
| 用 `limit_price` / 预算反推 ref | 假 Intent |
| 把候选码静默插入 `followed_stock` | 污染自选语义，掩盖数据层缺口 |

---

## 5. 模块影响分析（若进入 Phase10-B 实现）

### 5.1 `strategy`（必改消费方，宜薄）

| 改动 | 说明 |
|------|------|
| `after_close_intent_populate.go` | `afterCloseAnchorFunc` → 注入 `AnchorProvider` |
| `BuildDraftTradePlanFromCandidatePool` | 传入 context（source_date, pool_id） |
| 测试 | 用 fake Provider；保留 soft-fail 单测 |

**不改：** LimitPrices / TargetVolumes 算法；晨间仍要 `selected`+`ref`+open。

### 5.2 Market Data Layer（主建设）

| 能力 | 说明 |
|------|------|
| EOD / Session Snapshot Store | 按 `trade_date×code` 存 close/pre_close/as_of/source |
| Collector 或复用现有拉数 | 与 DATA-001 / DATA-002（筛选快照调度）协调 |
| Kline Close 查询门面 | 封装 `kline_cache` / `stock_kline_day`，输出标准 Anchor |
| ChainProvider | 实现 §4 优先级 |

### 5.3 TradePlan / Candidate

| 改动 | 说明 |
|------|------|
| 可选：`candidate_pool_items` 增价列或旁路 `candidate_price_snapshots` | 支撑 priority 2 |
| `trade_plans` schema | **可不改**（仍用现有 item.ref_*） |
| generate-next 编排 | 可选：建池前/后触发 Snapshot ensure（属 B，非 A） |

### 5.4 Readiness

| 建议 | 说明 |
|------|------|
| **默认不改** blocker 集合 | 无 selected 仍 `INTENT_NOT_MATERIALIZED` 等 — 正确 |
| 可选 WARN | `LOW_ANCHOR_CONFIDENCE`（evidence: source/confidence）— 另切片 |
| 禁止 | 为「推进 Paper」放宽未锚定价即 Ready |

### 5.5 非本设计范围

- TP-001 Upcoming 选择语义  
- PaperTradingJob / Freeze 自动  
- Realtime open 假价  

---

## 6. Phase 归属判断

### 6.1 不应继续扩大 Phase10-A

| Phase10-A 已交付 | Anchor 层需求 |
|------------------|---------------|
| 物化编排 API + UI | 统一价源、Snapshot、Collector、多源链式回退 |
| 复用现有 Materialize* | 新建 Market Data / Anchor 包与可能的表 |
| 禁止假价 / 不改状态机 | 触及 DATA-001 同级基础设施 |

在 A 内「临时多读 kline/stock_info」会：

- 复制 DATA-001 债务（分散直连）  
- 无 as_of 契约 → 难复现  
- 把 Phase10-A 验收标准拖成数据平台  

### 6.2 应作为 Phase10-B

```text
Phase10-A   Intent 物化接线（已完成 A.1/A.2；A.3=本设计）
Phase10-B   Market Data Layer + Execution Intent Anchor Provider
Phase10-…   连续 Paper / JOB catch-up 等（依赖 B 有可靠 selected）
```

**建议债务登记：** Intent 锚点耦合 → **DATA-003**（勿占用已有 DATA-002「筛选快照人工触发」编号）。  
实现排期：**Phase10-B**，与 DATA-001 同包或紧后。

### 6.3 Phase10-B 最小切片建议（实现时，非本文开工）

1. `AnchorProvider` 接口 + Followed 适配（行为兼容）  
2. Kline Close 适配（覆盖 strategy_run 常用码）  
3. populate 切换到 Chain  
4. （可选）Candidate 建池写价快照  
5. （完整）Market Snapshot Store + 盘后 ensure  

每步保持 soft-fail + 禁止假价。

---

## 7. 验收标准（设计层）

实现 Phase10-B 后应满足：

1. `strategy_run` 候选 **不必** 在 `followed_stock` 也能在有 EOD/K 线时得到 `selected`+`ref_price>0`  
2. `ref_source` / `ref_as_of` 可审计  
3. 无价时仍不写 selected（与现 soft-fail 一致）  
4. 早盘物化在 open 可用时可 `materialized_items>0`（在 Intent 已 selected 前提下）  
5. Phase10-A API/UI **无需**为锚点再开例外通道  

---

## 8. 相关文档

| 文档 | 关系 |
|------|------|
| `PHASE10_A_MORNING_INTENT_MATERIALIZATION_DESIGN.md` | A 物化接线；本文件补上游 |
| `DATA-001_Phase10_Backlog.md` | 行情分散 / Store |
| `DATA-002_Phase10_Backlog.md` | 筛选快照人工触发（编号已占用） |
| `TP-001_Phase10_Backlog.md` | Upcoming 选择语义（正交） |
| plan#19 只读诊断（会话） | 实证：五码均不在 followed_stock |

---

## 9. Registry

```text
PHASE10-A.3  DESIGN-ONLY  — Execution Intent Anchor Layer；实现归属 Phase10-B / Market Data
DATA-003     OPEN         — Anchor 唯一耦合 followed_stock（建议编号；与口述 DATA-002 避让）
```
