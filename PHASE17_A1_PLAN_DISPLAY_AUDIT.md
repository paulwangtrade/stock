# PHASE17-A.1 首页「我的计划」展示优化 — 实现前审计

**日期：** 2026-09-06  
**性质：** **只读审计 + 设计建议**（不改代码 / 后端 / schema / 生成链 / DB）  
**上游：** [PHASE17_A_HOME_ENTRY_AUDIT.md](./PHASE17_A_HOME_ENTRY_AUDIT.md)  
**本阶段目标：** 优化首页计划**展示口径**，使「今日执行」与「下一交易日自动计划」同时可见；**不**修改 TradePlan 生成链。  

---

## 0. 执行摘要

| 项 | 结论 |
| --- | --- |
| 问题本质 | **展示槽位**：首页只渲染 `active_plan` 一槽；`current_plan` / `next_plan` **已算好但未并排展示** |
| 自动计划 | 已生成（`after_close` 等）；**不是**后端 Manual 过滤 |
| 推荐方案 | **方案 A（仅前端）** — 双槽展示 + Source 标签；必要时复用已有 `same-day-candidates` |
| 方案 B/C | 不优先；B 动 Upcoming 语义风险高；C 新聚合接口**非必要** |
| 可否进入实现 | **可以**（前端 only，边界清晰） |

---

## 1. 首页「我的计划」组件审计

### 1.1 组件

| 项 | 说明 |
| --- | --- |
| **组件** | **无独立子组件**；逻辑与模板内嵌于 `frontend/src/components/InvestmentHome.vue` |
| **辅助** | `frontend/src/utils/planContext.js`（`buildDashboardPlanContext`） |
| **状态映射** | `frontend/src/api/investmentHome.ts` → `planUserStatus` / `PlanUserStatusKey` |
| **对照页（可复用模式）** | `TradePlanUpcoming.vue` 已用 `buildUpcomingDisplayContext` **双槽** + `getSameDayCandidates` |

### 1.2 数据请求位置

```text
InvestmentHome.refresh()
  └─ loadPlanContext()
       ├─ getUpcomingTradePlan(shanghaiDate(now))     // resCurrent
       ├─ 由 resCurrent 推 nextTradingDay
       ├─ getUpcomingTradePlan(nextDay)               // resNext
       └─ planContext = buildDashboardPlanContext({ now, resCurrent, resNext })
```

**未请求：** `same-day-candidates`、`/api/tradeplans/plan?plan_id=`、home 内 `trading_status.plan_id`（不驱动本卡）。

### 1.3 API

| API | 首页用途 |
| --- | --- |
| `GET /api/tradeplans/upcoming?trade_date=` | 唯一计划数据源（调用 2 次） |
| `GET /api/tradeplans/same-day-candidates?trade_date=` | **未用**（Upcoming 页已用） |
| `GET /api/investment/home` | 不填充「我的计划」卡片 |

### 1.4 数据结构（前端实际消费）

```text
planContext
├─ today / active / as_of
├─ current_plan { kind, trade_date, title, plan|null, empty_text }
├─ next_plan    { kind, trade_date, title, plan|null, empty_text }
└─ active_plan  → 当前 UI 唯一绑定对象（= current 或 next 二选一）

plan (Upcoming DTO 子集，经 normalize)
├─ id, trade_date, status, source_session
├─ freeze.is_frozen / morning.materialization_status
└─ items[] { stock_code, stock_name, … }
```

### 1.5 当前展示规则

| 规则 | 行为 |
| --- | --- |
| 展示对象 | **仅** `displayPlanSlot = planContext.active_plan` |
| 15:00 前且交易日 | `active=current` → 标题「今日交易计划」；计划须 **trade_date === 今天** |
| 否则 | `active=next` → 「下一个交易日计划」 |
| 有计划 | Tag（`planUserStatus`）+ 只数 + 最多 5 只 `StockLink` |
| 无计划 | `empty_text`（如「暂无交易计划」） |
| Source | **不展示** `source_session` |
| 多计划同日 | **不支持**（Upcoming 单条 + 单槽） |

---

## 2. TradePlan 数据来源审计

### 2.1 Manual vs Automatic（现网）

| 类型 | `source_session`（典型） | 生成入口 |
| --- | --- | --- |
| **Manual** | `t_sell`、`exit_review` | 人工卖出 / 退出复评 Draft |
| **Automatic** | `after_close` | 盘后 workflow / `RunDailyCandidateAndPlan` → Draft Builder |
| **Automatic 变体** | `morning_rebuild`、`cash_rescale` | 早盘重建 / 现金缩放 |
| **Watchlist** | `watchlist` | 跟踪机会草稿 |

自动 Draft Builder 明确：`SourceSession = after_close`（`build_draft_trade_plan.go`）。

### 2.2 关键字段（首页需要 vs 已有）

| 字段 | 库/API 有？ | 首页当前用？ |
| --- | --- | --- |
| `source_session` | ✅ Upcoming DTO | ❌ 未展示 |
| `trade_date` | ✅ | ✅（槽标题旁） |
| `status` / freeze | ✅ | ✅（经 `planUserStatus`） |
| `items` / 数量 | ✅（upcoming 带 items） | ✅（只数 + 名称） |
| `strategy_name` / score | item 级有；Origin 有 | ❌ 首页未展 |
| product `source` bucket | same-day-candidates 有 | ❌ 首页未调 |

### 2.3 首页「为什么选中某计划」

1. **后端 Upcoming**：`trade_date >= 查询日`，Frozen（ready+freeze_at）优先，否则 Draft；**无** session 过滤；**只返回一条**。  
2. **前端槽位**：`active` 只留一槽；`current` 再用 `pickExactPlan(今天)` 卡死。  
3. 结果：下一交易日 `after_close` 在「今日槽」时段常不可见；今日 `t_sell` 更容易成为用户唯一看见的计划。

---

## 3. 当前过滤逻辑分析

### 3.1 Upcoming 实际包含什么？

| 问 | 答 |
| --- | --- |
| 今日计划？ | 仅当存在 `trade_date>=D` 的 Frozen/Draft **且**（前端）精确落到今日槽时 |
| 下一交易日计划？ | 查询 `trade_date=next` 或 `>=D` 最早一条时常是次日自动计划 |
| 自动计划？ | ✅ 可被 Upcoming 选中（无 session 黑名单） |
| 手工计划？ | ✅ 同上；同日多条时 Upcoming **只给一条**（版本/id 规则） |

### 3.2 `trade_date = today` 精确匹配的问题

```text
当前逻辑（首页）：

  upcoming(D) ──► 可能返回 trade_date=D+1 的 after_close
        │
        ▼
  pickExactPlan(…, today=D) ──► 丢弃（日期不等）
        │
        ▼
  用户看见「暂无交易计划」
  （同时 next_plan 里可能已有自动计划，但 UI 不渲染该槽）
```

**未来建议逻辑（展示层，不改 Upcoming 语义）：**

```text
我的计划
├─ 今日执行（trade_date = 日历今日；非交易日可隐藏或标「非交易日」）
│    ├─ 主卡片：exact upcoming / 或 same-day 列表摘要
│    └─ 多来源：t_sell / after_close（若同日并存）并列或列表
└─ 下一交易日（始终展示槽位，即使今日槽非空）
     └─ 自动 Strategy（after_close 等）/ Watchlist 等 + 数量 + 状态 + source
```

要点：

- **同时表达**「今天要执行的」与「正在准备的下一日自动计划」  
- 不再用「单一 active 槽」吞掉另一半信息  
- Upcoming 仍可 Frozen 优先；**展示**用双槽 +（可选）同日候选列表补全

---

## 4. 设计目标确认

### 4.1 产品目标（本阶段）

首页「我的计划」应同时表达：

1. **今天需要执行的计划**（含手工卖出等）  
2. **下一交易日准备中的自动计划**（Strategy / Watchlist 等）

### 4.2 建议信息架构

```text
我的计划
│
├─ 今日执行
│   · 计划 #id · Manual/Strategy/… · 状态（待确认/已准备/已锁定）
│   · N 只 · source_session 文案或 chip
│   · （同日多条时）多行摘要，不止 Upcoming 冠军一条
│
└─ 下一交易日 YYYY-MM-DD
    · 计划 #id · Strategy（after_close）…
    · N 只 · 状态
```

每条建议字段：

| 字段 | 来源 |
| --- | --- |
| 股票数量 | `items.length`（upcoming）或按需 `getTradePlanById` |
| 来源 | `source_session` → 既有 label/chip 映射 |
| 状态 | 复用 `planUserStatus`（勿新造状态机） |

**非目标：** 改生成链、改 Frozen 优先、首页嵌完整 Origin 表、改 DB。

---

## 5. 复用能力检查（禁止重复开发）

| 能力 | 路径 | 首页复用方式 |
| --- | --- | --- |
| 双槽上下文 | `planContext.js`：`current_plan`/`next_plan` **已存在** | **直接并排渲染**（最小改动） |
| Upcoming 双槽（更完整） | `buildUpcomingDisplayContext`（TradePlanUpcoming） | 可迁到首页或抽共享 presenter |
| 同日多来源列表 | `getSameDayCandidates` + `mapTradePlanSourceBucket` | 今日/次日槽下列出多 plan |
| Source 中文会话文案 | `resolveTradePlanSourceLabel`（`tSellFlow.js`） | 行内「盘后计划/人工卖出…」 |
| Source 产品桶 | `resolveTradePlanSourceBucketLabel` / same-day `source` | 「量化策略/人工关注/人工卖出」 |
| Chip 英文枚举 | `portfolioSourceChip.js`（16.27/16.28） | Strategy/Watchlist/Manual；对齐 Debt-1 前先复用、勿新 taxonomy |
| `planUserStatus` | `investmentHome.ts` | 状态 Tag |
| Origin / strategy / score | Origin API + 16.28-B | **链接到交易计划页**；首页最多一行 strategy 摘要（若 item 已有），不重做 Origin Drawer |
| StockLink / K 线 | 已有 | 保持 |

**禁止：** 新建 Source 枚举文件、新建首页专用 Upcoming API、复制一份 Origin 投影逻辑。

---

## 6. 最小实现方案对比

### 方案 A — 仅前端调整展示（**推荐**）

| 项 | 内容 |
| --- | --- |
| 改动面 | `InvestmentHome.vue`（+ 可选小函数于 `planContext.js` / display util） |
| 数据 | **继续**现有 2× upcoming；`planContext` 已含双槽 |
| UI | 固定两区：**今日执行** + **下一交易日**；展示 `source_session` + `planUserStatus` + 只数 |
| 增强（仍前端） | 对今日 / 次日各调一次已有 `getSameDayCandidates`，多行展示同日 Manual+Automatic |
| 后端 | **零改动** |
| 生成链 | **不动** |

**优点：** 符合约束；立刻消除「自动计划消失」错觉；复用 Upcoming 页模式。  
**缺点：** Upcoming 单条在「主卡片」上仍可能不是用户想点的那条——用 same-day 列表补齐即可。

### 方案 B — 调整现有查询范围

| 项 | 内容 |
| --- | --- |
| 含义 | 改 `GetUpcomingTradePlan` 过滤（例如偏爱 after_close、或返回多条） |
| 风险 | 影响 TradePlanUpcoming、执行准备、Monitor 辅读等 **全局语义** |
| 与约束 | 易踩「改后端 / 改选择规则」；**不推荐为本阶段** |

### 方案 C — 新增聚合接口

| 项 | 内容 |
| --- | --- |
| 含义 | 如 `GET /api/investment/home/plans` 一次返回今日+次日+多来源 |
| 必要性 | **低** — 现有 upcoming×2 + same-day-candidates 已够拼装 |
| 与约束 | 非禁止但 **过度**；优先 A |

### 推荐

> **采用方案 A。**  
> 实施顺序建议：  
> 1）双槽并排（只用已有 `current_plan`/`next_plan`）+ Source 标签；  
> 2）按需加 `same-day-candidates` 多行；  
> 3）「查看计划」深链带 `trade_date` / `plan_id`（Upcoming 已支持同类导航）。

---

## 7. 风险分析

| 风险 | 等级 | 缓解 |
| --- | --- | --- |
| 双槽与 Beta「当前步」仍只绑一条 plan | 中 | 明确：流程引导仍可基于「今日执行优先，否则次日」；文案勿称全局唯一计划 |
| same-day 无 items → 只数为空 | 低 | 摘要先显示来源+状态；点击进 Upcoming；或懒加载 `getTradePlanById` |
| Source 文案与 Portfolio chip 两套 | 低 | 复用既有函数；Debt-1 未落地前接受中文会话标签 |
| 用户以为改了执行选计 | 中 | UI 标明「展示摘要，执行仍以计划页/Upcoming 规则为准」 |
| 非交易日「今日执行」空 | 低 | 隐藏今日槽或显示「今日非交易日」（对齐 Upcoming `showTodaySlot`） |

---

## 8. 当前问题定位（汇总）

1. **自动计划未显示原因：** 生成正常；首页 **只渲染 active 单槽** + 今日 **精确日期** 过滤，把次日 `after_close` 排除出视线。  
2. **数据链路：** Upcoming（无 session 过滤，单条）→ `buildDashboardPlanContext`（双槽已算）→ UI 只用 `active_plan`。  
3. **最小改动：** 前端双槽（+可选 same-day 列表）+ Source/状态/只数；**不改**后端与生成链。  
4. **可否进入实现：** **是** — 建议开 Phase17-A.1 实现切片，范围锁定方案 A。

---

## 9. 实现边界清单（进入编码时遵守）

**允许：**

- `InvestmentHome.vue` 模板/计算属性  
- `planContext.js` 展示辅助（不改变 upcoming API 契约）  
- 调用已有 `getSameDayCandidates` / `getTradePlanById`  
- 复用 `planUserStatus`、Source label/chip、StockLink  

**禁止：**

- 修改 Go 后端 / TradePlan schema / 生成逻辑 / CandidatePool / 执行链 / DB  
- 修改 `GetUpcomingTradePlan` Frozen 优先规则  
- 首页重做 Origin 全表  

---

## 10. 停止边界

本文只审计与设计建议，**不编码**。  
确认采用方案 A 后，再进入实现阶段。
