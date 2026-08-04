# PHASE10_A Final Acceptance Report

> **阶段：** Phase10-A Morning Intent Materialization  
> **状态：✅ 已完成冻结（Final Freeze）**  
> **日期：** 2026-08-05  
> **性质：** 只读验收整理 — **不修改代码**  
> **依据：**  
> - Phase10-A.1 Backend MVP  
> - Phase10-A.2 UI  
> - Phase10-A.3 Anchor Design（`PHASE10_A3_EXECUTION_INTENT_ANCHOR_DESIGN.md`）  
> - Golden Path（`PHASE10_A_GOLDEN_PATH_VALIDATION_REPORT.md`）  
> - 设计底稿：`PHASE10_A_MORNING_INTENT_MATERIALIZATION_DESIGN.md`  

---

## 1. 目标

打通受控闭环（**不**自动 Approve / Freeze / Paper / Cron）：

```text
Draft Intent（after_close_intent + selected）
        ↓
Morning Materialize（LimitPrices → TargetVolumes）
        ↓
Readiness Recheck
        ↓
（人工）Approve / Freeze —— 本阶段仅消费既有门禁，不改状态机
```

| 范围内 | 范围外（明确不做） |
|--------|-------------------|
| 物化编排 API + UI 接线 | 改 generate-next / upcoming 语义 |
| 复用已有 `MaterializeMorning*` | 改 `trade_plans` 表结构 |
| 禁假价；幂等 | 自动 Approve / Freeze / Execution |
| Anchor **设计**（A.3） | 实现统一 Anchor / Market Data Store |

---

## 2. 已完成能力

### 2.1 Phase10-A.1 — Backend MVP

| 交付 | 说明 |
|------|------|
| `POST /api/tradeplans/materialize-morning` | `{ plan_id }` |
| 编排 | precheck → LimitPrices → TargetVolumes → Readiness |
| 开盘价 | 复用 `RealtimeOpenPriceProvider`（无新拉数逻辑、无假价） |
| 返回 | `success`, `plan_id`, `materialized_items`, `readiness_ready`, `blockers`, … |
| 测试 | strategy + API selftests（Draft 可入、幂等、拒绝条件） |

主文件：`morning_intent_materialize.go`、`tradeplans_materialize_morning.go`、路由挂载。

### 2.2 Phase10-A.2 — UI

| 交付 | 说明 |
|------|------|
| 「早盘物化」按钮 | Draft + `after_close_intent` + 未冻结 |
| 调用 API | 展示成功/失败、`materialized_items` / Ready / blockers |
| 成功后刷新 | 计划详情 + Intent Readiness |
| 自测 | `tradePlans.materializeMorning.selftest.mjs` |
| 构建 | `npm run build` + `wails build -skipbindings -s`（A.2 Build Verification） |

### 2.3 Phase10-A.3 — Anchor Design（仅文档）

| 交付 | 说明 |
|------|------|
| 现状断点 | Intent 锚点唯一 `followed_stock` |
| Provider 契约 | `ref_price` / `ref_source` / `ref_as_of` / `confidence` |
| 优先级草案 | Market Snapshot → Candidate Snapshot → Kline → Followed |
| 归属裁定 | **实现进 Phase10-B**，不扩大 A |

### 2.4 构建 / 验证工件

| 工件 | 作用 |
|------|------|
| `PHASE10_A_GOLDEN_PATH_VALIDATION_REPORT.md` | plan#12 运行时 Golden Path |
| `TP-001_Phase10_Backlog.md` | Upcoming 选择语义（正交债务） |
| A.2 Build Report（会话） | exe 含「早盘物化」字符串 |

---

## 3. 验证结果

### 3.1 Golden Path（plan_id=12）

| 检查 | 结果 |
|------|------|
| 前置 | draft / after_close_intent；3× selected + ref_price |
| 物化 #1 | `success=true`，`materialized_items=3` |
| Spec | limit + volume + `intent=priced`；stage→`morning_materialized` |
| Open | Realtime 有效（非 pending_open） |
| 幂等 #2 | limit/volume **未覆盖**；idempotentSkip=3 |
| `readiness_ready` | **false**（2 票无锚点 Intent — 见 §5） |
| Approve / Freeze / Paper | **未跑**（符合范围） |

**判定：Phase10-A 物化接线与幂等 — PASS。**

### 3.2 阴性对照（不作为 A 失败）

| 对象 | 现象 | 归类 |
|------|------|------|
| plan#19（最新 08-05） | selected=0 → 物化 0 | DATA-003 / 锚点，非 A API 故障 |
| Upcoming 默认见旧日 Draft | ASC 语义 | TP-001 |

### 3.3 验收边界声明

Phase10-A **冻结标准** =「有 selected Intent 时可物化并重检 Readiness，且禁假价、幂等」。  
**不等于**「任意 generate-next 计划必 Ready」——后者依赖上游锚点（Phase10-B）。

---

## 4. 已知限制

| ID / 主题 | 限制 |
|-----------|------|
| 锚点源 | 盘后 Intent 仍只认 `followed_stock` |
| Ready | 同计划混有无 Intent 行时，Readiness 仍 BLOCK |
| Open | 非交易时段 / 无开盘价 → `pending_open`，`materialized_items` 可为 0（诚实失败） |
| Upcoming | 默认非「最新生成计划」（TP-001） |
| UI stage 推断 | upcoming 无 `pricing_stage` 字段时靠 Readiness/lifecycle 推断 |
| 工作区 | 构建时 git dirty 面大；A 交付物已嵌入产物，但不等于整库洁净提交 |

---

## 5. 非本阶段问题（DATA-003）

**DATA-003 — Execution Intent Anchor Source Coupling**（口述曾用 DATA-002；**DATA-002 已占用「筛选快照人工触发」**，锚点债务用 **DATA-003**）

| 点 | 说明 |
|----|------|
| 现象 | `strategy_run` 候选 ∉ 自选 → 无 `selected` → 物化 legacy_skip |
| 影响 | 新票难进可执行 Intent；Paper 难连续 |
| 与 A 关系 | **上游输入**；A 已正确 soft-fail / 禁假价 |
| 处理 | **不在 Phase10-A 修复**；见 A.3 设计 → Phase10-B |

相关：DATA-001（行情分散）、DATA-002（筛选快照调度）。

---

## 6. 后续 Phase10-B 建议

1. **实现 Execution Intent Anchor Provider**（按 A.3 优先级链）。  
2. **Market Snapshot / Candidate 带价快照**，与 DATA-001 Store 同规划。  
3. populate 切换到 Provider；保持 soft-fail、禁止假价。  
4. 再跑「最新日 generate-next → 全票 selected → 物化 → Ready」验收。  
5. TP-001（Upcoming 语义）可并行 UX，勿与 B 混为同一 PR。  
6. Ready 后的 Approve→Freeze→Paper 连续交易属更后切片，依赖 B 稳定 Intent。

```text
Phase10-A  ✅ FROZEN — 物化接线 + UI + Anchor 设计 + Golden Path
Phase10-B  → Market Data / Intent Anchor 实现（DATA-003 + DATA-001）
```

---

## 7. Freeze 声明

**Phase10-A 已完成冻结。**

- 不再扩大 A 范围（不在本阶段实现 Anchor Provider / 改 generate-next / 改状态机）。  
- 后续缺陷若属物化 API/UI 回归，按维护修复；属锚点/行情层，开 Phase10-B 或 DATA-003。  
- 本报告为验收冻结记录；**禁止借「收尾」改代码。**

---

## 8. 文档索引

| 文档 | 角色 |
|------|------|
| `PHASE10_A_MORNING_INTENT_MATERIALIZATION_DESIGN.md` | A 设计底稿 |
| `PHASE10_A3_EXECUTION_INTENT_ANCHOR_DESIGN.md` | A.3 锚点设计 |
| `PHASE10_A_GOLDEN_PATH_VALIDATION_REPORT.md` | Golden Path 实证 |
| `TP-001_Phase10_Backlog.md` | Upcoming 债务 |
| 本文 | **Final Acceptance / Freeze** |
