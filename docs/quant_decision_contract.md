# QuantDecision Phase0 — Contract Freeze

> 状态：**契约冻结（2026-07-20）**  
> 范围：只定义统一决策模型与字段映射；**不接生产写路径、不改交易逻辑**  
> 产物：`backend/models/quant_decision.go`、本文档  
> 约束：不修改 Execution / PaperBroker / TradePlan 生命周期 / Candidate·Risk 实现

---

## 1. 目标

消除 Watchlist 前端量化层与 Candidate / Risk / TradePlan 后端层的**语义重复**：

| 层 | 职责 |
|----|------|
| **QuantDecision** | 唯一业务语义：Signal / EntryZone / Gate / Risk / Size / Action |
| **Candidate / TradePlan** | 未来复用 Decision 核字段（本 Phase 不改表） |
| **Execution** | 只消费 Plan 生命周期（本 Phase 不动） |
| **Watchlist UI** | 只投影 Decision；禁止二次判定 Action |

**SchemaVersion = 1**（`models.QuantDecisionSchemaVersion`）。

---

## 2. 审计范围

| 模块 | 路径 | 角色 |
|------|------|------|
| WatchlistStockCard | `frontend/src/components/WatchlistStockCard.vue` | 展示 |
| buyChecklist | `frontend/src/utils/buyChecklist.js` | Gate |
| buyPositionSizing | `frontend/src/utils/buyPositionSizing.js` | Size |
| buyPriceRange | `frontend/src/utils/buyPriceRange.js` | EntryZone |
| icePointSignals | `frontend/src/utils/icePointSignals.js` | Signal |
| signalActionHint | `frontend/src/utils/signalActionHint.js` | Action（前端派生） |
| holdingPositionAdjust | `frontend/src/utils/holdingPositionAdjust.js` | HoldingBias |
| CandidatePool | `backend/models/candidate_pool.go` | 入池 |
| Risk PlanFilter | `backend/risk/plan_filter.go` + `plan_context.go` | Risk |
| TradePlan | `backend/models/trade_plan.go` | 计划意图 + 生命周期 |

---

## 3. 当前字段 → QuantDecision 映射表

### 3.1 WatchlistStockCard / 前端扫描

| 当前字段 / 文案 | 来源 | QuantDecision | 备注 |
|-----------------|------|---------------|------|
| 信号主标 `强/趋/…` | `signal.tag` ← `summarizeBuySignal` | `Signal.Tag` + `Signal.TagKind` | TagKind 由 Tag 归类 |
| `减30%` / `加20%` / `早减` | `sellPositionPct` / `addPositionPct` / `rushReducePct` | `Signal.RatioPct` + TagKind | 比例语义随 TagKind |
| `statusText` | icePointSignals | `Signal.Summary` | tooltip |
| `daysAgo` / `recentSignalDaysAgo` | scan entry | `Signal.DaysAgo` | |
| `signalScore`（隐式） | `calcSignalScore` | `Signal.Score` | 卡片未展示 |
| `signalBar` / confirmBar | summary | `Signal.BarIndex` / `ConfirmBar` | |
| `effectiveSignalDayKey` | summary | `Signal.DayKey` | |
| 操作副标 `可买/等回踩/仅观察/先风控/…` | `resolveSignalActionHint` | `Action.Code` + `Action.Label` | **冲突点**：现为前端独立派生 |
| 持仓辅助标签 | `holdingAdvice` | `HoldingBias.*` | 无主信号时单独展示 |
| 区间标签 `延后参考` 等 | `buyPriceRangeLabel` | **展示投影**（不进核字段） | 由 `EntryZone.DaysAgo/DeferMode` 推导 |
| 区间数值 `XX~XX` | `buyPriceRange.text` | `EntryZone.Text` + Low/High | |
| `instantPrice` / mode / deferMode | buyPriceRange | `EntryZone.*` | |
| `清单 XX%` / `就绪` | `quantChecklist` | `Gate.Score` / `Ready` / `Items` | 非胜率 |
| 仓位文案 / `不足一手` | `quantPlan` | `Size.*` + `Blockers` | BindingConstraint 待补齐 |
| `止损≈` | `quantPlan.stopPrice` | `Size.StopPrice` | |
| 大盘级别（隐式） | `marketModeKey` / holdingAdvice | `Regime.Level/Key` | |
| 草稿可否 | `canCreateDraft` | `Action.AllowDraft` | |
| `盈亏比` / `今盈亏` / 成本展示 | 行情+持仓 | **不映射** | 展示层独有 |
| 卡片 tint | 主题 | **不映射** | |

### 3.2 buyChecklist → Gate

| 当前 | QuantDecision |
|------|---------------|
| `items[].id/label/passed/required/detail` | `Gate.Items[]` |
| `score` (passCount/total) | `Gate.Score` |
| `ready` / `requiredPassed` | `Gate.Ready` / `RequiredPassed` |
| `checklistReadyScore` 配置 | `Gate.ReadyThreshold` |
| `blockers` 文案列表 | 可投影为 `Blockers` where `Layer=gate` |

### 3.3 buyPositionSizing → Size

| 当前 | QuantDecision |
|------|---------------|
| `ok` | `Size.OK` |
| `entryPrice` / `stopPrice` / `riskPerShare` / `confidence` | 同名 |
| `suggestedShares` / `suggestedAddShares` | `TargetShares` / `AddShares` |
| `suggestedAmount` | `TargetAmount` |
| `positionPct` | `PositionPct` |
| `reason` | `Size.Reason`；失败时拆 `Blockers` + `BindingConstraint` |

### 3.4 CandidatePoolItem

| 当前字段 | QuantDecision | 复用策略（未来） |
|----------|---------------|------------------|
| `StockCode` / `StockName` | `Instrument.*` | 直接复用 |
| `SignalTag` | `Signal.Tag` | 对齐枚举 |
| `SignalScore` | `Signal.Score` | 信号贡献分 |
| `Score` | 综合分（可写入 Meta 或扩展 ResearchScore；Phase0 用 `Signal.Score` 不覆盖池综合分） | **注意冲突**：两套 Score |
| `Reason` | `Signal.Summary` 或 Meta | 入池理由 ≠ statusText |
| `StrategyName` / `Version` | `Meta.Strategy*` | 复用 |
| `SignalSnapshotID` | `Meta.SignalSnapshotID` | 复用 |
| `Rank` | 不进 Decision 核（池序） | 池独有 |
| — | `Meta.CandidatePoolID` | 回链 |

### 3.5 Risk PlanFilter

| 当前 | QuantDecision |
|------|---------------|
| `PlanContext.MarketLevel` | `Regime.Level` | **唯一级别源（目标态）** |
| `PlanFilterItem.Allowed` | `Risk.Passed` | |
| `RiskCode` (`APPROVED` / `MARKET_LEVEL_BLOCKED` / …) | `Risk.Code` | 与 `risk.ReasonCode` 字符串对齐 |
| `RiskMessage` | `Risk.Message` | |
| `PlanRiskStatus*`（计划级） | 不进单票 Decision | 计划聚合独有 |
| 敞口/单票/日亏/现金上下文 | 输入 Size/Risk，不整包拷贝进 Decision | 快照可在 Meta/外部 |

### 3.6 TradePlan / Item

| 当前 | QuantDecision | 说明 |
|------|---------------|------|
| `TargetAmount` / `TargetVolume` | `Size.TargetAmount` / `TargetShares` | 意图复用 |
| `LimitPrice` | 可选自 `EntryZone` | 现多未用区间 |
| `RiskCode` / `RiskMessage` | `Risk.*` | |
| `MarketLevel`（计划头） | `Regime.Level` | 生成时冻结 |
| `Score`（Item） | 与 Candidate.Score 同源意图 | 与 Signal.Score 冲突风险 |
| `Status` ready/executing/… | **不映射** | 生命周期独有 |
| `OrderID` / Fill* | **不映射** | Execution 回写 |
| `Message` / CAS / reconcile | **不映射** | |

---

## 4. 字段冲突清单

| ID | 冲突 | 现状 | Phase0 裁定 |
|----|------|------|-------------|
| C1 | **「可买」vs 自动下单** | 前端 `checklistReady`/`zone` 即可显示「可买」；后端需 ready TradePlan + Enable | `Action.Code=ENTER` **≠** 执行授权；Watchlist Purpose 默认不触发下单 |
| C2 | **Gate vs Risk** | 清单% 像风控；PlanFilter 才是组合硬约束 | Gate=研究检查；Risk=`ReasonCode`；禁止混用文案 |
| C3 | **三套百分比** | 清单通过率%、仓位占权益%、加减仓比例% | 分属 `Gate.Score` / `Size.PositionPct` / `Signal.RatioPct`\|`HoldingBias.SuggestPct` |
| C4 | **两套 Score** | `CandidatePoolItem.Score`（综合）vs `SignalScore` vs 前端 `signalScore` | Decision 内 `Signal.Score`=研究信号分；池综合分保留在 Candidate，Phase0 不塞进 Decision 核 |
| C5 | **AmountPerStock vs 动态 Size** | Plan 固定金额；卡片 fixed-fractional 股数 | 并存；未来 Plan 可选用 `Size.TargetAmount`，本 Phase 不改 |
| C6 | **MarketLevel 双源** | 配置 `PlanMarketLevel` vs 前端 `getLastMarketModeKey()` | 目标：Regime 单一来源；Phase0 仅文档对齐语义 1–5 |
| C7 | **HoldingBias vs Action** | 无 tag 时「倾向加仓」像主信号 | Bias 不得把 Action 升为 ENTER；仅注解 |
| C8 | **不足一手笼统** | risk/敞口/级别均同一 reason 串 | 契约要求 `Size.BindingConstraint` + `Blockers.Layer` |
| C9 | **EntryZone vs LimitPrice** | 区间参考 vs 计划限价字段 | Zone 非委托；LimitPrice 仅策略显式限价时使用 |
| C10 | **SignalTag 字符集** | 自选含 减/止/冲/加/冰；全市场快照筛选偏 entry | TagKind 分流；入池策略自行过滤 |

---

## 5. 展示层独有（禁止进 Decision 核）

- 盈亏比（关注价→现价）、今盈亏、成本×股数排版  
- 关注天数、行业 tag、分时 spark  
- 卡片 tint / 颜色主题  
- Tooltip 排版与口语包装（由 `Action.Code` 映射）  
- 虚拟列表 / hover 性能状态  

---

## 6. Action 派生契约（文档级，未实现）

```
if Risk 未过或 Regime 禁开仓 → BLOCKED
else if Signal.TagKind == exit → REDUCE | EXIT_PARTIAL
else if TagKind == scale_in → SCALE_IN
else if TagKind == rush_reduce → REDUCE（或 EXIT_PARTIAL）
else if 无 entry 信号 → HOLD（可附 HoldingBias）
else if Gate 未 Ready → WATCH | WAIT_PULLBACK（按 Zone.Mode）
else if Zone.Mode == above → WAIT_PULLBACK
else if Size 未 OK → BLOCKED
else → ENTER

AllowDraft =
  (Action in {ENTER, SCALE_IN} && Size.OK) ||
  (Action in {REDUCE, EXIT_PARTIAL} && ExistingVolume > 0)
```

UI Label 仅为投影，例如 `ENTER` →「建议关注买入」（避免「可买」=授权感）。

---

## 7. 数据流（目标态）

```text
KLine / Quote
    → Signal Engine          → QuantSignal
    → EntryZone              → QuantEntryZone
    → Gate (checklist)       → QuantGate
    → Regime (1–5)
    → Risk PlanFilter        → QuantRiskSlice
    → Size Model             → QuantSize
    → derive Action/Blockers → QuantDecision
         ├→ Watchlist Projection（只读）
         ├→ CandidatePoolItem（复用核字段，未来）
         └→ TradePlanItem（意图字段，未来）
                → Execution / PaperBroker（不变）
```

Phase0：**仅定义结构**；生产仍走现有 JS Watchlist 与 Go Candidate→Risk→Plan 两套路径。

---

## 8. Go 模型位置

- 文件：`backend/models/quant_decision.go`  
- 测试：`backend/models/quant_decision_test.go`（JSON round-trip + schema 冻结）  
- **无** `TableName`、**无** AutoMigrate、**无** Wails 绑定要求  
- Risk.Code 字符串与 `backend/risk.ReasonCode` 对齐（如 `APPROVED`、`MARKET_LEVEL_BLOCKED`）

---

## 9. 非目标（本 Phase 明确不做）

- 不修改 `paper_open_buy` / Execution / PaperBroker  
- 不修改 TradePlan CAS / reconcile / 状态机  
- 不修改 PlanFilter 规则与 Candidate 生成  
- 不修改 WatchlistStockCard 行为  
- 不持久化 `quant_decisions` 表  

---

## 10. 后续阶段（索引）

| Phase | 内容 |
|-------|------|
| 0 | 本契约 + Go 类型（当前） |
| 1 | 前端投影收口：单一 `deriveAction`，弱化「可买」文案 |
| 2 | Go 引擎 / API 成为 Source of Truth，双跑 js_legacy |
| 3 | Candidate/TradePlan 挂 DecisionID |
| 4 | 退役前端第二套判定 |

---

## 11. 验收

- [x] 映射表覆盖 Card / Checklist / Sizing / Signals / Candidate / Risk / Plan  
- [x] 冲突清单 C1–C10 已裁定  
- [x] `quant_decision.go` 可编译  
- [x] `go test ./...` 通过（含 models 契约测试）  
- [x] 无交易逻辑 / Execution / PaperBroker / Plan 生命周期改动  
