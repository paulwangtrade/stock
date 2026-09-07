# PHASE17.4.1 Signal Identity — 设计审计

**日期：** 2026-09-08  
**性质：** **只读设计**（不改代码 / 不建表 / 不接交易链）  
**目标：** 在实现 `SignalOutcomeProjection`（PHASE17.4）之前，冻结稳定 **`signal_id`** 锚点。  

**上游：**  
- [PHASE17_4_SIGNAL_OUTCOME_PROJECTION_DESIGN.md](./PHASE17_4_SIGNAL_OUTCOME_PROJECTION_DESIGN.md)  
- [PHASE17_SIGNAL_RESEARCH_AUDIT.md](./PHASE17_SIGNAL_RESEARCH_AUDIT.md)  
- Opportunity 稳定 id 先例：`backend/opportunity/opportunity_id.go`（`opp_` + SHA256 前缀）

**本阶段交付：** 本文档。完成后 **暂停**，等待授权再实现。

---

## 0. 一句话结论

**批次有 id、命中无 id：** `signal_scan_snapshots.id` 稳定；`SignalScanHit` 嵌在 `result_json` 内，**无 DB 主键 `signal_id`**。  
Outcome 需要的是 **hit 级**身份，不是 snapshot 级。  

**冻结方案：B（稳定 hash）为主，A（snapshot id）为必选组成字段，C（轻量投影）为可选物化——不建交易表。**

---

## 1. 审计：当前信号来源与字段

### 1.1 来源角色

| 来源 | 落点 | 角色 | 是否有稳定 hit 主键 |
| --- | --- | --- | --- |
| **SignalScanSnapshot** | `signal_scan_snapshots` | 扫描批次锚点；hits 在 `result_json` | 仅有 **`id`（批次）** |
| **SignalScanHit** | Snapshot JSON 内嵌 | 单票信号事实（含 SignalEvent 最小字段） | **无** |
| **CandidatePool / Item** | `candidate_pools` / `_items` | 选股桥：回指 snapshot + tag/score | Item 有 `id`，**禁止冒充 signal_id** |
| **Opportunity** | 只读投影 / `opportunity_id` | Signal→决策→计划统一视图 | 有 **`opp_*` hash**，域 = 机会，**≠ signal_id** |

### 1.2 字段对照（用户清单）

| 字段 | Snapshot | SignalScanHit | CandidatePoolItem | Opportunity（SignalBlock / List） |
| --- | --- | --- | --- | --- |
| **stock_code** | 无（在 hits） | ✅ `SECUCODE` / `SECURITY_CODE`（需归一化） | ✅ `stockCode` | ✅ `stock_code` / 由 hit 推导 |
| **signal_time** | 间接：`trade_date`+`session`；精确时刻在 hit | ✅ `signal_time` | ❌ 不存 | ✅ 有 hit 时：`signal_time` |
| **signal_price** | 无 | ✅ `signal_price` + `signal_price_status` | ❌ 不存 | ✅ 有 hit 时回填 |
| **signal_type** | 无独立枚举 | ✅ 近似 = `tag`（强/冰/趋…） | ✅ `signalTag` | ✅ `signal_tag` |
| **snapshot_id** | ✅ `id` | ❌（仅父级） | ✅ `signalSnapshotId` | ✅ `snapshot_id` |

**补充（审计发现，非清单但关键）：**

| 字段 | 说明 |
| --- | --- |
| `schema_version` | hit：`signal_event.v1` |
| `signal_price_status` | `frozen` / `missing` / `derived`；Normalize **禁止** NEW_PRICE 回填 |
| `strategy_id` / `session` | Snapshot 级元数据 |
| `opportunity_id` | `BuildOpportunityID(batchKey, secucode, signalTime, signalTag)` → `opp_`+hash8；**与 signal_id 分离** |

### 1.3 身份缺口（相对 Outcome）

```text
需要：signal_id（hit 级，可复现、跨 API 稳定）
现有：snapshot.id（batch）+ hit 字段组合（无正式合成契约）
禁止：candidate_pool_item.id / trade_plan_item.id / fill.id / opportunity_id 冒充
```

同一 snapshot 内通常「一码一 hit」（`BuildHitIndex` 按归一化代码覆盖）；但 **同码多 tag / 多时刻** 理论上可能，故 identity **必须含 type + time**，不能只用 `snapshot_id + code`。

---

## 2. 设计：`SignalIdentity`

### 2.1 定位

只读研究身份对象：**描述「哪一次扫描里的哪一次命中」**。  
不写 Broker、不改 Pool/TradePlan、不参与成交 Outcome。

### 2.2 字段冻结（建议）

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| **`signal_id`** | string | ✅ | 稳定合成 id（见 §3）；建议前缀 `sig_` |
| **`stock_code`** | string | ✅ | 归一化 sina 码（如 `sh600363`）；构建时统一 `NormalizeStockCode` |
| **`signal_type`** | string | ✅* | = hit.`tag` / pool.`signalTag`；缺省用空串占位（仍参与 hash） |
| **`signal_time`** | string | ✅* | 原串保留；缺省空串；**不**在 identity 层强行解析 |
| **`signal_price`** | float64 | 条件 | 展示/Outcome 锚价；`missing` 时为 0，**仍可生成 id**（id 不依赖价） |
| **`source_snapshot_id`** | uint | ✅** | = `SignalScanSnapshot.id`；无 snapshot 的 ad-hoc 信号 → 0 + 降级策略（见下） |
| **`created_at`** | time | 投影层 | 身份物化或首次投影时刻；**不**等于 `signal_time` |

\* 空值允许，但必须进入 hash（空占位），避免「缺字段导致 id 漂移协议不明」。  
\** 生产扫描路径上几乎总有；前端冰点重算 marker **不**纳入本 identity 真源（17.4 已裁定弱持久化）。

### 2.3 建议可选元数据（非 id 输入）

| 字段 | 用途 |
| --- | --- |
| `signal_price_status` | Outcome 是否可算 |
| `secucode_raw` | 保留东财原始码便于对照 |
| `session` / `strategy_id` / `trade_date` | 来自 Snapshot；可冗余缓存 |
| `schema_version` | 契约版本 |
| `opportunity_id` | **可选外链**；禁止双向等同 |

### 2.4 与 Opportunity 身份对照

| | `signal_id` | `opportunity_id` |
| --- | --- | --- |
| 问句 | 这次**信号命中**是谁 | 这次**机会视图**是谁 |
| 输入 | snap + code + type + time | batchKey + secucode + time + tag |
| 前缀建议 | `sig_` | 现网 `opp_` |
| 无 hit 时 | 不应伪造完整信号身份 | 仍可从 pool tag 生成机会 id |

二者可并行；Outcome Projection **主键 = signal_id**，可选附带 `opportunity_id`。

---

## 3. 方案裁定：A / B / C

### 3.1 选项定义

| 方案 | 含义 |
| --- | --- |
| **A. 复用已有 snapshot id** | 直接用 `source_snapshot_id`（或仅 `snap:id`）当 signal_id |
| **B. 生成稳定 hash** | 对规范字段串做 SHA256（或等价）→ 定长 `sig_…` |
| **C. 新增轻量 projection** | 独立只读投影/可选 append-only 研究表物化身份行 |

### 3.2 评估

| 方案 | 优点 | 缺点 | 裁定 |
| --- | --- | --- | --- |
| **A alone** | 实现零成本；批次可导航 | **一 snap 多 hit**；无法作为 Outcome 行主键 | ❌ 不可单独作 `signal_id` |
| **B** | 与现网 `opp_*` 同模式；无新表；可复现；不碰交易链 | 依赖字段规范化契约；空 time/tag 需冻结规则 | ✅ **主方案** |
| **C** | 可索引、可审计、可挂 Outcome 外键 | 有实现/存储成本；易被误解为事件表 | ⚪ **可选后续**；非身份冻结前置 |

### 3.3 冻结结论

```text
signal_id = B(稳定 hash)
source_snapshot_id = A 的字段角色（批次锚，必进 hash 输入）
C = 未来 SignalOutcome / SignalIdentity 物化时可选，非本阶段必做
```

**推荐 hash 输入（规范串，顺序固定）：**

```text
source_snapshot_id | stock_code_normalized | signal_type | signal_time
→ SHA256 → "sig_" + hex[:16]   // 16 hex = 8 bytes；与 opp_ 的 8 bytes 对齐量级，略加长降低碰撞观感
```

**刻意不进入 hash：** `signal_price`（避免 status 修补导致 id 漂移）、`created_at`、策略名展示串。

**可读调试串（非主键，可选并存）：**

```text
"{snapshot_id}:{stock_code}:{signal_type}:{signal_time}"
```

与 17.4 §1.1 建议一致；**产品主键仍用 `sig_` hash**，调试串可放 `signal_id_debug` 或日志。

**降级（无 snapshot）：**  
`source_snapshot_id=0` + `batch_fallback = trade_date|session|strategy_id`（对齐 `BatchKeyFromSnapshot` 无 id 分支）再 hash。冰点 marker / 纯前端重算 **不** 发正式 `signal_id`。

---

## 4. 约束（硬）

1. **不建交易表**（禁止 `paper_sim_*`、fill、order、broker 相关新表）。  
2. **不影响 CandidatePool**（不改 Score/Rank/生成/绑定逻辑；不回写 `signalSnapshotId`）。  
3. **不影响 TradePlan**（不改生成、冻结、执行意图）。  
4. **不改变生产交易链**（Broker / Settlement / Unlock / Exit 状态机）。  
5. **Identity ≠ 成交身份**：禁止用 pool item / plan item / fill id。  
6. **Identity ≠ Opportunity**：禁止把 `opp_*` 当作 `sig_*`。  
7. 若未来做 **C**：仅允许研究侧 append-only / 只读投影；**禁止**成为选股或下单输入。

---

## 5. 与 SignalOutcomeProjection 的衔接

```text
SignalScanSnapshot
    └─ SignalScanHit  ──BuildSignalIdentity──► SignalIdentity (signal_id, …)
                                                      │
                                                      ▼
                                            SignalOutcomeProjection
                                            (return_t1/t3/t5, MFE, MAE …)
```

| 规则 | 说明 |
| --- | --- |
| Outcome 行主键 | `signal_id` |
| 缺 `signal_price` | 身份仍可建；Outcome `quality=pending/missing_anchor` |
| Pool 仅有 tag、无 hit | **不**强制伪造完整 SignalIdentity；可标 `identity_status=unresolved` |
| 实现阶段默认 | 先纯函数 `BuildSignalIdentity(hit, snapshotID)`；持久化 C 延后 |

---

## 6. 阶段结论

| 问题 | 答案 |
| --- | --- |
| 四源字段是否齐全？ | Snapshot/Hit **基本齐**；Pool **缺 time/price**；Opportunity **派生**自 Hit/Pool |
| 稳定 `signal_id` 现状？ | **不存在**；需合成 |
| A/B/C？ | **B 为主 + A 作 source_snapshot_id + C 可选物化** |
| 本阶段交付 | **本设计文档** |
| 下一步 | **暂停** — 不实现 Identity / Outcome，直至授权 |

---

## 附录 — 关键路径

- `backend/models/signal_scan_snapshot.go` — Snapshot / Hit / SignalPrice 字段  
- `backend/models/candidate_pool.go` — `SignalTag` / `SignalSnapshotID`  
- `backend/data/signal_price.go` — Normalize（禁 NEW_PRICE）  
- `backend/opportunity/opportunity_id.go` — `opp_` hash 先例  
- `backend/opportunity/projection/types.go` — `SignalBlock`  
- `backend/research/provenance/signalindex.go` — 按 code 建 hit 索引（非 id）  

---

*PHASE17.4.1 Signal Identity 设计审计结束 · 暂停等待指令。*
