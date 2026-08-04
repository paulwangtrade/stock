# DATA-003 — Execution Intent Anchor Source Coupling

> **ID:** DATA-003  
> **Suggested alias:** 用户拟用 **DATA-002**；该编号已用于 *Stock Screen Snapshot Depends on Manual Trigger*（`DATA-002_Phase10_Backlog.md`），故本项定为 **DATA-003**  
> **Status:** Accepted debt — tracked; **not fixed in this note**  
> **Priority:** 高（真实连续 Paper / 可执行 Intent 生命周期前）  
> **Target phase:** **Phase10-B / Market Data Layer**（与 DATA-001 统一价源一并规划）  
> **Nature:** Tracking only — no code / trading / schema change in this document  
> **Date:** 2026-08-05  
> **Evidence:** 只读诊断 plan#19 / pool#18；`defaultAfterCloseAnchor` → `followed_stock` only  

---

## 1. Problem

Execution Intent Materialization（盘后 Intent → 早盘物化）依赖 **`followed_stock` 作为唯一盘后价格锚点**（`FollowPrice` / `Price`）。

```text
BuildCandidatePool (strategy_run)
        ↓
BuildDraftTradePlanFromCandidatePool
        ↓
populateAfterCloseExecutionIntent
        ↓
defaultAfterCloseAnchor(code)
        ↓
followed_stock WHERE stock_code=?   ← 唯一软源
        ↓
缺失 / 价≤0 → soft-fail：不写 selected，ref_price=0
```

策略候选常为**新发现代码**（不在自选），导致：

- 计划级仍标 `pricing_stage=after_close_intent`
- 行级 **无 `selected` Intent**
- 早盘 `MaterializeMorningLimitPrices` 全部 `legacy_skip`
- `materialized_items=0` → 无法 Approve/Freeze → PaperTradingJob 无 Frozen Plan

**CandidatePool / CandidatePoolItem 无可用价格字段**；盘后工作流也不在 populate 前刷新行情写入锚点。

---

## 2. Impact

| Area | Effect |
|------|--------|
| 新发现股票 | 无法自动进入可执行 Intent 生命周期 |
| Paper Trading | 难以形成连续交易日建仓/换仓（依赖偶发「候选∈自选」） |
| 策略池 ↔ 执行层 | 通过自选表隐式耦合；pool 与 watchlist 生命周期不一致 |
| Phase10-A 物化 API/UI | 入口可用，但上游 Intent 空则永远无 Spec |
| 与 DATA-001 | 同属「无统一 Market Data / prev_close 快照」；本项是 **消费侧锚点耦合** |

---

## 3. Observed instance（2026-08-05 Draft #19）

| 项 | 值 |
|----|-----|
| pool | #18 `strategy_run` |
| items | sh603986 等 5 码 pending |
| `followed_stock` | 库内 89 行有价，**上述 5 码均 ABSENT** |
| intent | 全部 `intent_status=""`，`ref_price=0` |

---

## 4. Target direction（Phase10-B — design only now）

在 **Market Data Layer** 统一提供盘后锚点（例如）：

```text
Collector / EOD Snapshot Store
        ↓
  prev_close | as_of | source  per code
        ↓
  afterCloseAnchor（优先统一层，自选仅兼容回退）
```

原则备忘：

1. **禁止假价格** 不变；缺价仍 soft-fail，但价源不得仅绑自选。  
2. Candidate / Intent / Morning Open 应可读同一日快照契约。  
3. 不在 Phase10-A 范围偷偷改 `defaultAfterCloseAnchor` 绕过立项。

---

## 5. Non-goals（本记录）

- 不修改 `after_close_intent_populate.go` / generate-next / 物化 API  
- 不把策略码强行写入 `followed_stock` 作为正式方案  
- 不扩大 Phase10-A 范围  

---

## 6. Related

| ID | Relation |
|----|----------|
| **DATA-001** | 行情接入分散 / 未来 Market Data Store |
| **DATA-002** | 筛选快照人工触发（编号已占用；勿混淆） |
| **TP-001** | Upcoming 选择语义；正交 |
| Phase10-A | 物化入口已通；卡在本债上游 Intent |

---

## 7. Registry line

```text
DATA-003  P1/高  OPEN  — Intent 锚点仅 followed_stock；策略候选无法 selected；Phase10-B / Market Data Layer
```

**Disposition:** 正式登记为 **DATA-003**（用户口述 DATA-002 → 编号避让）。Phase10-B 与 DATA-001 一并处理。
