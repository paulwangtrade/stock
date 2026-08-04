# PHASE10_A_MORNING_INTENT_MATERIALIZATION_DESIGN

> **阶段：** Phase10-A Morning Intent Materialization  
> **性质：只读设计 — 不实现代码、不改 Cron Spec 默认值、不改 generate-next / Readiness 规则 / Approve·Freeze·Execution**  
> **日期：** 2026-08-04  
> **依据：** `INTENT_MATERIALIZE_STATUS_DIAGNOSIS.md`、`PHASE6_5_6_13_MORNING_MATERIALIZATION_DESIGN.md`、`PHASE6_5_6_13_1/13_2` 实现报告、`PLAN17_READINESS_VS_RISK_DIAGNOSIS.md`  

---

## 0. 目标与非目标

### 0.1 目标

打通可操作闭环：

```text
Draft Intent（after_close / generate-next）
        ↓
Morning Materialize（复用已有 Materialize*）
        ↓
Readiness Ready（既有评估，不改规则）
        ↓
Approve（既有 Approve Gate）
```

Phase10-A **只补「接线 + 编排 + 触发面」**，不重写物化算法。

### 0.2 必须复用

| 已有能力 | 用途 |
|----------|------|
| `strategy.MaterializeMorningLimitPrices(planID, openPriceFn)` | Intent → `limit_price` / `priced` / `gap_skip` |
| `strategy.MaterializeMorningTargetVolumes(planID, opts)` | priced → `target_volume` / `size_skip` |
| `GET /api/tradeplans/readiness` | 物化后验证 Ready |
| `POST /api/tradeplans/approve` | Ready 后审批（本阶段仅消费，不改语义） |

### 0.3 非目标（硬禁止）

| 禁止 | 原因 |
|------|------|
| 在 generate-next / AfterClose 自动写 Spec | 违背 6.5.6.12/13 |
| 改 Readiness blocker 规则「放行未物化」 | 掩盖缺口 |
| 改 Approve / Freeze / Execution / 9:30 Buy 算量 | 另切片 |
| 默认改 `0 20 9` / `0 25 9` / `0 30 9` Spec 字符串 | ENV/JOB 风险；Job 仅作可选旁路 |
| 为缺价伪造开盘价 | 禁止假 priced |
| 改 `trade_plans` schema | 不需要 |

---

## 1. 现状缺口（设计前提）

```text
已有：MaterializeMorningLimitPrices / MaterializeMorningTargetVolumes（库函数 + 单测）
缺失：编排入口、HTTP/UI、默认 OpenPriceProvider、Cron 旁路
结果：generate-next 后 Readiness 合法 BLOCK（INTENT_NOT_MATERIALIZED / LIMIT / VOLUME）
```

额外事实：AfterClose 锚点失败时 `intent_status=""`，LimitPrices **只处理 `selected`** → 物化前可能需「Intent 可物化」前置检查（见 §5.1），**不在 AfterClose 写 Spec**。

---

## 2. 目标架构

```text
                    ┌─────────────────────────────┐
                    │  Trigger surfaces            │
                    │  A. HTTP/Wails API (MVP)     │
                    │  B. TradePlan UI 按钮        │
                    │  C. Optional Morning Job     │
                    └──────────────┬──────────────┘
                                   ▼
                    ┌─────────────────────────────┐
                    │  Orchestrator (NEW, thin)    │
                    │  RunMorningIntentMaterialize │
                    │  planID → price → volume     │
                    └──────────────┬──────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              ▼                    ▼                    ▼
   MaterializeMorning     MaterializeMorning     (read-only)
   LimitPrices            TargetVolumes          OpenPriceProvider
                                                   AccountSnapshot
                                   ▼
                         pricing_stage=morning_materialized
                                   ▼
                         Readiness / Approve（既有）
```

编排层 **仅顺序调用** 已有两函数 + 注入依赖；禁止复制价量公式。

---

## 3. 状态变化

### 3.1 Plan 级

| 字段 | 物化前（典型） | 物化成功后 |
|------|----------------|------------|
| `status` | `draft` | **仍 `draft`**（不 Approve） |
| `pricing_stage` | `after_close_intent` | `morning_materialized`（两步均完成后；与现 LimitPrices 写入条件对齐） |
| `pricing_policy_version` | ≥1 | 不变 |
| `risk_status` | 可已 `passed` | **不重算、不覆盖**（本阶段） |
| `approved_*` / `freeze_*` | 空 | 不变 |

Frozen / 非 draft → 两函数已有 **NO-OP**；编排应原样返回 `no_op`，不报成功 Ready。

### 3.2 Item 级

| 路径 | `intent_status` | `limit_price` | `target_volume` |
|------|-----------------|---------------|-----------------|
| 物化前 | `selected`（或空=不可价） | 0 | 0 |
| LimitPrices 成功 | `priced` | >0 | 不变（仍 0） |
| LimitPrices 跳空 | `gap_skip` | 0 | 0 |
| LimitPrices 缺开盘价 | 保持 `selected` | 0 | 0（`pending_open`） |
| TargetVolumes 成功 | `priced` | >0 | ≥100 |
| TargetVolumes 不可达 | `size_skip` | >0 | 0（或保持策略既有） |

Readiness：非 skip 的 buy 须 `priced ∧ limit>0 ∧ volume≥lot`；`gap_skip`/`size_skip` 不计入 tradeable，但可贡献「全 skip 且无缺失 Spec」类 Ready 形态（既有评估逻辑，不改）。

### 3.3 生命周期站位

```text
S1 after_close_intent ──(Phase10-A)──► S2 morning_materialized
                                              │
                                              ▼
                                         Readiness Ready?
                                              │
                                    Yes → Approve → Freeze → …
                                    No  → 见 §7 失败恢复
```

---

## 4. 编排契约（设计名）

```text
RunMorningIntentMaterialize(planID, opts) → MorningIntentMaterializeResult
```

### 4.1 输入 `opts`

| 字段 | 说明 |
|------|------|
| `OpenPriceFn` | 必填于生产路径；码→开盘/竞价价；失败返回 ok=false |
| `AccountSnapshot` / `RiskLimits` | 可选；nil 则沿用 `MaterializeMorningTargetVolumes` 默认 loader |
| `ForceVolume` | 透传 `MorningPositionMaterializeOpts.Force` |
| `Actor` / `Trigger` | `ui` / `api` / `cron`（仅日志与响应） |
| `DryRun` | **可选未来**；MVP 可不实现 |

### 4.2 输出（建议响应字段）

```text
ok, no_op, no_op_reason
plan_id, pricing_stage
price: { priced_count, gap_skip_count, pending_open_count, skipped_legacy }
volume: { sized_count, size_skip_count, idempotent_skip, skipped_other }
readiness_hint: { ready, blocker_codes[] }   // 编排末可选只读调用 Evaluate*；不写库
message, failed_step: "" | "precheck" | "price" | "volume" | "readiness_eval"
```

### 4.3 执行顺序（固定）

```text
1. Precheck（只读）
   - plan 存在；draft；!frozen；policy≥1
   - 统计 selected / empty-intent buy 数
   - selected==0 且存在 empty-intent → failed_step=precheck
     message=INTENT_NOT_SELECTABLE（引导补锚点/重建 Intent，不写 Spec）
2. MaterializeMorningLimitPrices(planID, OpenPriceFn)
3. 若 price 返回 fatal err → 停止（不跑 volume）
4. MaterializeMorningTargetVolumes(planID, opts)
5. （可选）EvaluateExecutionIntentReadiness 只读填 readiness_hint
6. 不调用 Approve / Freeze / Execution
```

**禁止**并行跑 price/volume（volume 依赖 priced）。

---

## 5. API 设计（MVP 必选）

### 5.1 HTTP

```text
POST /api/tradeplans/materialize-morning
Content-Type: application/json

{
  "actor": "ui:trade-plan-upcoming",
  "plan_id": 17
}
```

| 规则 | 说明 |
|------|------|
| `actor` | 必填（与 generate-next / approve 一致） |
| `plan_id` | 必填；**不**用「默认 upcoming」隐式选 plan，避免误物化旧日 Draft |
| 幂等 | 允许重复 POST（见 §8） |
| 鉴权 | 与现网 tradeplans AssetServer 同级 |

响应：映射 §4.2；HTTP 200 + `ok=false` 表示业务未达 Ready（与 generate-next 风格对齐），结构性错误用 4xx。

### 5.2 Wails（可选同构）

若桌面端不经 AssetServer：`App.MaterializeMorningTradePlanIntent(planID, actor) map[string]any`  
内部调用同一 Orchestrator，避免双实现。

### 5.3 明确不提供

- `materialize` 写入 Approve  
- 批量「物化全部 draft」无 actor（防误伤）— 若 Job 需要，见 §7 带 `trade_date` 过滤  

---

## 6. UI 入口（MVP 必选）

**位置：** `TradePlanUpcoming`（与「生成明日计划」同页）。

| 元素 | 行为 |
|------|------|
| 按钮「早盘物化 Intent」 | 对 **当前展示 plan.id** 调 materialize-morning |
| 启用条件 | `status=draft` ∧ 未冻结 ∧ `pricing_policy_version≥1` ∧ （建议）存在 Readiness blockers 含 Intent/Limit/Volume **或** `pricing_stage=after_close_intent` |
| 禁用文案 | 已 Frozen / 非 Draft / 无 planId / 物化中 |
| 成功 | toast + `refresh` + 拉 readiness；若 `readiness_hint.ready` 提示可审批 |
| 失败 / pending_open | 展示 `pending_open_count`、blocker 摘要；**不**清 Draft |
| 与 Approve | 仍受既有 `canApprove`（Readiness）约束；物化按钮 **不**替代审批 |

**不改** generate-next 按钮语义；文案可加 hint：「生成后需交易日早盘物化方可审批」。

---

## 7. Job 可选方案

| 方案 | 描述 | 推荐度 |
|------|------|--------|
| **J0 无 Job** | 仅 API+UI（Phase10-A MVP） | **默认交付** |
| **J1 旁路 Cron** | 新 key 如 `morning_intent_materialize`，Spec 建议 `0 15 9 * * 1-5`（开盘后、9:20 adopt 前）或 `0 22 9`（9:20 后）；**不改**现有三条 Spec | Phase10-A.1 |
| **J2 挂 9:20 尾** | `RunMorningPlanPreparation` 成功后对「当日未冻 Draft + after_close_intent」调用编排 | 改动面更大；需防 build_morning 裸 plan |

**J1 伪流程：**

```text
trade_date = today
candidates = draft ∧ trade_date=today ∧ policy≥1 ∧ !frozen ∧ stage∈{after_close_intent, morning_materialized?}
for each plan (最高 plan_version 优先):
  RunMorningIntentMaterialize(plan.id, cron actor, default OpenPriceProvider)
```

**启用：** 独立 enable 文件或配置位（对齐 after_close job 风格），默认 **off**，避免无开盘价时批量 pending。

**与 JOB-001：** 漏跑属 Phase10 债务；本设计不实现 catch-up，仅定义「可重跑幂等」。

---

## 8. 幂等规则

| 场景 | 行为 |
|------|------|
| 重复 LimitPrices | 已 `priced`/`gap_skip` 项按现函数跳过或稳定重写；`pending_open` 可在有价后转为 priced |
| 重复 TargetVolumes | `idempotent`（volume≥lot 且未 Force）→ 计数 IdempotentSkip，不破坏已有 volume |
| 已 `morning_materialized` 再调 | 允许；用于补全先前 pending_open / 未 sized 项 |
| Frozen / ready | NO-OP，`ok` 语义：`no_op=true`，**不**视为 Readiness 失败 |
| 并发双触发 | 依赖现有逐 item Update；接受「最后写者」；MVP 不做分布式锁；UI 按钮 generating 防抖 |

编排层 **不**引入新状态机列；幂等完全建立在已有 Materialize* 行为上。

---

## 9. 失败恢复

| failed_step / 现象 | 恢复动作 |
|--------------------|----------|
| `precheck` INTENT_NOT_SELECTABLE | 修复锚点（自选价）或重新 generate-next；**再**物化 |
| `price` 全 `pending_open` | 等待行情；稍后重试同一 API（幂等） |
| `price` 部分 gap_skip | 可接受；继续 volume；Readiness 可能靠 skip 或剩余 priced |
| `volume` size_skip 过多 → Ready=false | 调预算/现金/风控后 `Force` 重跑 volume，或接受不可交易 |
| 中途 DB 写失败 | 返回 error；部分 item 可能已更新 → **允许整单重试**（幂等） |
| 物化成功但 Ready=false | UI 展示 readiness blockers；不自动 Approve |
| 误物化错误 plan | 仅 draft 可写；Frozen 无伤；错误 draft 可再 generate 新 version |

**不提供**「回滚 Spec → 全 0」API（易与 AfterClose 契约混淆）；若需重置，另开设计。

---

## 10. OpenPriceProvider（生产默认）

| 优先级（建议） | 来源 | 标注 |
|----------------|------|------|
| 1 | 竞价/开盘专用（若未来有） | `auction`/`open` |
| 2 | 实时行情开盘价字段 | `realtime_open` |
| 3 | 实时最新价（仅文档降级，需打标） | `last_as_open_fallback` — **默认关闭** |

缺价 → `pending_open`，**禁止**用昨收写入 `limit_price` 冒充早盘 Spec。

AccountSnapshot：默认 Paper 快照（与现 `loadMorningAccountSnapshotDefault` 一致）。

---

## 11. 与 Approve / Readiness 的边界

```text
Phase10-A 成功定义（编排 ok）:
  两步无 fatal error，且非 frozen no_op 误报成功

产品「可审批」定义（页面）:
  既有 canApprove = Draft ∧ Readiness.blockers==0 ∧ …
  Phase10-A 只提高 Ready 概率，不改按钮公式
```

Risk PASS 与 Materialize **解耦**（保持现状）。

---

## 12. 切片与验收

### 12.1 实现切片（未来）

| 切片 | 内容 |
|------|------|
| **10-A.0** | Orchestrator + 单测（mock 已有 Materialize*） |
| **10-A.1** | HTTP `materialize-morning` + OpenPriceProvider 接线 |
| **10-A.2** | TradePlan UI 按钮 + refresh/readiness |
| **10-A.3** | （可选）Cron J1 + enable 开关 |

### 12.2 验收标准

1. Plan17 类 Draft：物化后 blockers 不再含 `LIMIT_PRICE_MISSING` / `TARGET_VOLUME_BELOW_LOT`（或转为合法 skip）。  
2. `INTENT_NOT_MATERIALIZED` 在仍有 `selected`/空 intent 未价时仍可出现；全 skip 或全 priced+volume 后消失或 Ready。  
3. 重复调用不损坏已 sized 量（幂等）。  
4. Frozen 调用 NO-OP。  
5. generate-next **仍不**写 Spec。  
6. 无 Approve/Freeze/Execution 副作用。

---

## 13. 风险与依赖

| 风险 | 缓解 |
|------|------|
| 开盘前点 UI | pending_open + 文案引导 |
| empty intent（锚点失败） | precheck 明确错误；不与 Spec 失败混淆 |
| Job 默认 on 无价 | enable 默认 off |
| 与 9:30 临场算量双轨 | 本阶段不改 Execution；文档标明 Spec 先服务 Approve/Freeze |

---

## 14. 总结

| 项 | 决定 |
|----|------|
| 核心手段 | **编排复用** LimitPrices → TargetVolumes |
| MVP 触发 | **API + UI** |
| Job | **可选** J1，默认关 |
| 状态 | draft 上 `morning_materialized`；不自动 Approve |
| 幂等 | 依赖现有 Materialize* |
| 失败 | 可重试；禁止假价；precheck 分离「无 selected」 |

**Phase10-A 设计完成；等待实现授权。本文不包含代码修改。**
