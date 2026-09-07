# PHASE17-A.2 首页数据入口只读审计

**日期：** 2026-09-06  
**性质：** **只读审计**（不改代码 / API / DB / UI）  
**范围：** 交易流程首页「我的计划 / 今日机会 / 交易流程状态」  
**主入口：** `/` → `investmentHome` → `frontend/src/components/InvestmentHome.vue`  
**关联：** [PHASE17_A_PORTFOLIO_UX_AUDIT.md](./PHASE17_A_PORTFOLIO_UX_AUDIT.md) · Phase16.28 Origin / Opportunity · Phase16.28 Debt Register  

---

## 0. 执行摘要

| 问题 | 结论 |
| --- | --- |
| **自动计划为什么没显示？** | **不是** API 按 `source_session` 过滤掉 `after_close`。自动 Draft **会写入** `trade_plans`（`SourceSession=after_close`）。首页用 `GET /api/tradeplans/upcoming` + **前端 15:00 槽位 / 精确 trade_date 匹配**，常把「下一交易日自动计划」挡在「今日槽」外；同日若存在手工计划（如 `t_sell`），用户更容易只看到手工计划。分类偏 **B（前端槽位过滤）+ 单槽 Upcoming 语义**，偶发像 **A（今日精确日无返回）**。 |
| **今日机会为空是否合理？** | **经常合理。** 首页「今日机会」**不是** CandidatePool/机会页列表，而是 `decision_summary.opportunity_attention` 的**持仓对比启发式**；无达标分差 → 空态文案「今天没有单独机会提示」。 |
| **交易流程「已完成」是否真实？** | **不完全。** 可见的 Beta 五步是 **UI 引导**（由计划用户态推导），**不是** Day Monitor 的 materialize/approve/freeze/execution。真实流水线状态已聚合进 `GET /api/investment/home`，但被 `SHOW_LEGACY_HOME_BLOCKS=false` **隐藏**。 |
| **首页下一步最值得改什么？** | **P0：** 计划槽与 Upcoming 语义对齐（展示「下一交易日自动计划」+ source 标签）；**P1：** 机会空态分型（无池 / 无对比 / 接口失败）；**P2：** Beta 流程与 Day Monitor 状态解耦说明或轻量对齐。 |

---

## 1. 首页页面结构

### 1.1 组件树

```text
InvestmentHome.vue
├─ 今日交易流程（Beta 引导条）     ← 前端常量 BETA_TRADING_FLOW + planUserStatus
├─ 快捷按钮：发现机会 / 查看计划 / 查看组合
├─ 我的资产                       ← home.portfolio_summary（Snapshot）
├─ 今日机会                       ← home.decision_summary.opportunity_attention → opportunityCards
├─ 我的计划                       ← GET /api/tradeplans/upcoming ×2 + planContext.js
├─ 当前状态                       ← autoExecuteSentence(Track-B开关 + 计划态)
├─ StockKlineModal / OpportunityProjectionDrawer（懒加载解释）
└─ Legacy 运维块（SHOW_LEGACY_HOME_BLOCKS=false）
     ├─ 今日流程 pipelineSteps    ← home.trading_status（Day Monitor）【隐藏】
     ├─ 今日关注 daily_attention
     └─ 持仓状态等
```

| 区块 | 组件内实现 | 数据来源 | API |
| --- | --- | --- | --- |
| 资产 | 内联 statistic | Snapshot 投影 | `GET /api/investment/home` |
| 今日机会 | `n-data-table` | `opportunity_attention` → `toOpportunityCards` | 同上（嵌在 home） |
| 我的计划 | 内联卡片 | Upcoming TradePlan | `GET /api/tradeplans/upcoming?trade_date=`（今日 + 下一交易日） |
| Beta 交易流程 | 按钮条 | **无后端状态机**；由计划用户态推「下一步」 | 无独立 API |
| Track-B 文案 | 「当前状态」 | `today.enabled` | `GET /api/papertrading/dashboard/today` |
| 解释抽屉 | `OpportunityProjectionDrawer` | 点击懒加载 | 既有 opportunities projections（非 home 主路径） |

**不存在**独立「我的计划组件 / 今日机会组件」文件；均为 `InvestmentHome.vue` 内区块。

### 1.2 刷新链路

```text
refresh()
  ├─ getInvestmentHome()                    // 聚合读模型
  ├─ fetchTrackBPaperTradingEnabled()
  └─ loadPlanContext()
       ├─ getUpcomingTradePlan(shanghaiDate(now))
       └─ getUpcomingTradePlan(nextTradingDay)
       └─ buildDashboardPlanContext({ now, resCurrent, resNext })
```

---

## 2. 首页数据流图

```text
                    ┌─────────────────────────────┐
                    │     InvestmentHome.vue      │
                    └──────────────┬──────────────┘
           ┌───────────────────────┼───────────────────────┐
           ▼                       ▼                       ▼
 GET /api/investment/home   GET …/tradeplans/upcoming   GET …/papertrading/dashboard/today
           │                       │                       │
           ▼                       ▼                       ▼
 ┌─────────────────┐     Frozen优先 → Draft        enablePaperTrading
 │ Snapshot 资产    │     trade_date >= 查询日
 │ Dashboard 盈亏   │              │
 │ Decision 决策    │              ▼
 │  └ opportunity_  │     planContext.js
 │     attention    │     · 交易日&lt;15:00 → 槽「今日」须 trade_date==今天
 │ Daily Summary    │     · 否则 → 槽「下一交易日」
 │ Day Monitor      │              │
 │ Daily Attention  │              ▼
 └────────┬─────────┘     「我的计划」只展示 active_plan 一槽
          │
          ├─► 今日机会表 ← opportunityCards（对比机会，非池列表）
          ├─► Beta 流程 ← 仅用计划态（不用 Day Monitor）【可见】
          └─► Legacy「今日流程」← trading_status【隐藏】
```

---

## 3. 「我的计划」问题定位（重点）

### 3.1 首页调用的计划接口

| 调用 | 用途 |
| --- | --- |
| `GET /api/tradeplans/upcoming?trade_date={today}` | 当前槽候选 |
| `GET /api/tradeplans/upcoming?trade_date={nextDay}` | 下一交易日槽 |
| **未调用** | list / draft 列表 / `same-day-candidates` / `plan?plan_id=`（首页） |
| home 内 `trading_status.plan_id` | **不**驱动「我的计划」卡片 |

### 3.2 自动计划是否写入 TradePlan？

| 路径 | 写入？ | `source_session` |
| --- | --- | --- |
| 盘后 `RunAfterClosePlanWorkflow` → Draft Builder | ✅ | `after_close`（`TradePlanSourceAfterClose`） |
| 晨间 fallback `RunDailyCandidateAndPlan` → 同上 Builder | ✅ | `after_close` |
| 手工卖出等 | ✅ | `t_sell` / `exit_review` 等 |
| Watchlist 草稿 | ✅ | `watchlist` |

→ **自动计划会进库**；问题不在「没生成」。

### 3.3 Upcoming 查询过滤（后端）

`TradePlanRepo.GetUpcomingTradePlan(today)`（`trade_plan_visibility.go`）：

| 条件 | 有？ |
| --- | --- |
| `trade_date >= today` | ✅ |
| Frozen：`status=ready AND freeze_at IS NOT NULL` 优先 | ✅ |
| 否则最新 Draft：`status=draft` | ✅ |
| 排序：`trade_date ASC, plan_version DESC, id DESC` | ✅ |
| **`source_session` 过滤** | ❌ **无** |
| **creator 过滤** | ❌ **无** |
| limit 多条 | ❌ 只取 **First 一条** |

→ 后端**不会**因为「自动」而丢弃；只会返回**一条**「最早 trade_date 的 Frozen，否则 Draft」。

### 3.4 前端槽位过滤（关键）

`buildDashboardPlanContext`（`planContext.js`）：

| 规则 | 行为 |
| --- | --- |
| 交易日且上海时间 **&lt; 15:00** | `active = current` → 标题「今日交易计划」 |
| 否则（含周末 / ≥15:00） | `active = next` → 「下一个交易日计划」 |
| `current` 取计划 | `pickExactPlan`：**必须** `plan.trade_date === 今天` |
| `next` 取计划 | `pickOnOrAfterPlan`：`trade_date >= nextDay` |

与 Upcoming 语义错位示例：

```text
盘后自动生成：trade_date = 下一交易日，source_session=after_close，status=draft

用户在交易日 10:00 打开首页：
  upcoming(today) 可能直接返回「明天的 after_close Draft」（>= today 最早）
  pickExactPlan(…, 今天) → trade_date≠今天 → null
  → 「我的计划」空：「暂无交易计划」

同日若用户建了 t_sell Draft（trade_date=今天）：
  pickExactPlan 命中 → 首页只看见手工计划
```

### 3.5 分类结论（A / B / C）

| 分类 | 是否适用 | 说明 |
| --- | --- | --- |
| **A. 根本没有返回** | 部分 | 对「精确今天」：若库中无今日 plan，upcoming 仍可能返回明日 plan，但前端当「无今日计划」 |
| **B. 返回但被前端过滤** | **主因** | `pickExactPlan` / 15:00 切换 / 只展示 `active_plan` 一槽 |
| **C. 返回但组件不支持** | 弱 | 卡片能展示任意 session；**未展示** `source_session` 标签，易误判「只有手工」 |
| 后端按 Manual 过滤 | **否** | 无 `source_session` 过滤 |

### 3.6 完整链路（我的计划）

```text
RunAfterClose / RunDailyCandidateAndPlan
  → CandidatePool + BuildDraftTradePlanFromCandidatePool
  → trade_plans (source_session=after_close, 常为下一交易日)
        │
        ▼
GET /api/tradeplans/upcoming?trade_date=D
  → Frozen优先 else Draft；无 session 过滤；单条
        │
        ▼
planContext.buildDashboardPlanContext
  → active 槽 + exact/onOrAfter 过滤
        │
        ▼
InvestmentHome「我的计划」
  → planUserStatus / 最多 5 只股票名
  → 无 Source chip、无 Origin、无同日多计划列表
```

---

## 4. 「今日机会」问题定位（重点）

### 4.1 真实数据来源

| 用户以为 | 实际 |
| --- | --- |
| 机会页 / Signal Snapshot 列表 | ❌ 首页主表不用 |
| CandidatePool 全量 | ❌ 仅 Decision 取 **当日池 Top1** 作候选 |
| daily_attention OPPORTUNITY | 有映射函数 `pickOpportunityItems`，**当前可见表未用** |

**实际路径：**

```text
home.Service
  → decision.Service.Evaluate(trade_date=今天)
       → CandidatePool.GetLatestByTradeDate(今天) → Top 候选 + Score
       → 持仓 InvestmentScore
       → buildOpportunity：分差 ≥ 7.5 才进 Highlights
  → decision_summary.opportunity_attention
  → frontend toOpportunityCards(…, limit=10)
  → 今日机会表
```

空态文案：`今天没有单独机会提示`（强调「单独提示」，不是「市场无票」）。

### 4.2 空白原因分类

| 类型 | 条件 | 是否「合理空」 |
| --- | --- | --- |
| **O1 今日无池/无候选** | `GetLatestByTradeDate(今天)` 无池或无 item | 合理；与盘后池日期（常为下一交易日）易错位 |
| **O2 有候选但对比未达标** | 有 Candidate 但 Highlights 空（分差 &lt; 7.5 或持仓分缺失） | **合理**；产品语义=「值得对比的机会」 |
| **O3 决策包失败 / DEGRADED** | decision 缺失 | 应标失败，现与 O2 同空表 |
| **O4 前端映射丢弃** | `toOpportunityCards` 要求 candidate_code+score **且** 至少一条合法 highlight | 有 attention 无 highlight → 空 |
| **O5 首页读错接口** | — | **否**；读对了，但语义≠机会页 |

### 4.3 有数据 / 无数据展示路径

| 路径 | 行为 |
| --- | --- |
| **有** Highlights | 表：股票 / 信号标签 / 评分 / 分差「风险」/ 行情列恒 `—` / 「解释」懒加载 ProjectionDrawer |
| **无** | `n-empty` +「发现更多机会」→ `stockScreen` |

### 4.4 状态分类建议（实现时用，本阶段不编码）

| 状态码 | 含义 | 建议文案方向 |
| --- | --- | --- |
| `OPP_NONE_POOL` | 当日无 CandidatePool | 今日尚无候选池（可提示看下一交易日池） |
| `OPP_NO_COMPARE` | 有池但无达标对比 | 暂无相对持仓的突出机会（正常） |
| `OPP_READY` | 有卡片 | 展示 Top N |
| `OPP_DEGRADED` | decision/home quality 降级 | 数据不完整，请刷新或查机会页 |
| `OPP_ERROR` | home 请求失败 | 与整页 error 一致 |

---

## 5. 交易流程状态审计

### 5.1 可见「今日交易流程」（Beta）

步骤：发现机会 → 生成计划 → 准备价格 → 批准冻结 → 模拟成交  

| 项 | 事实 |
| --- | --- |
| 状态来源 | **仅** `planUserStatus(upcomingPlan)` → `currentFlowStepKey` |
| 与 TradePlan 生命周期 | 间接（有无计划 / 是否物化 PASS / 是否冻结） |
| 与 Execution / Position | **无**（「模拟成交」一步只跳转组合页） |
| 「已完成」含义 | 索引 &lt; 当前步 → UI 标 done，**不**表示 cron/任务已跑 |

推导逻辑摘要：

- 无计划 → 当前「生成计划」  
- 待确认 → 「准备价格」  
- 已准备 → 「批准冻结」  
- 已锁定 → 「模拟成交」  

→ **可出现：** 流程条显示前步「已完成」，但 `RunDailyCandidateAndPlan` / 盘后任务未执行；或库中已有 after_close 计划，因槽位空而流程停在「生成计划」。

### 5.2 隐藏的真实流水线（Day Monitor）

`home.trading_status` ← `tradingdaymonitor.Build`：

- TradingEvent（当日）  
- 辅以 `GetLatestByTradeDate(trade_date)` + MorningReadiness  

字段：`materialize_status` / `approve_status` / `freeze_status` / `execution_status` / `settlement_status` / `plan_id`  

前端 `pipelineSteps` **已映射**，但包在 `SHOW_LEGACY_HOME_BLOCKS === false` 内 → **用户看不见**。

### 5.3 一致性风险

| 风险 | 说明 |
| --- | --- |
| 假完成 | Beta「已完成」≠ Monitor PASS |
| 假未开始 | 自动计划已在库，槽位空 → 引导仍停在生成 |
| 双真相 | home 同时持有 Monitor 与 Beta，只展示后者 |
| Track-B | `enablePaperTrading=false` 时「当前状态」强调观察态；与流程条「模拟成交」并存，需文案区分 |

---

## 6. 与 Phase16.28 能力关联（禁止重复建设）

| 能力 | 首页现状 | 建议 |
| --- | --- | --- |
| TradePlan Origin | 未用 | 计划卡可链到 Upcoming Origin / 浅 Source chip（复用 `portfolioSourceChip` / Origin display） |
| Signal Snapshot | 未用 | 机会「解释」已走 ProjectionDrawer；勿另造信号表 |
| CandidatePool | Decision 仅 Top1 | 列表仍以机会页为准；首页勿再造池 UI |
| Portfolio provenance | 未用 | 资产区已有「查看组合」；持仓溯源留在 Portfolio |
| Opportunity 解释列（16.28） | 抽屉复用 | 保持懒加载，不在 home 重复全表 |

**Debt 对齐：** 若首页加 Source 标签，应跟 Phase16.28 Debt-1/2（taxonomy / ResolveSource），避免第三套文案。

---

## 7. 已有能力 / 缺失能力

### 已有

- 单接口聚合资产 / 决策 / Monitor / Attention  
- Upcoming 只读计划 + 15:00 槽位切换框架  
- 机会对比卡片 + K 线 + 解释抽屉  
- Track-B 安全文案  
- Legacy 真实流水线数据（已加载未展示）  

### 缺失 / 弱

| ID | 缺失 | 优先级 |
| --- | --- | --- |
| H1 | 自动计划在「今日槽」不可见（槽位/日期语义） | **P0** |
| H2 | 计划卡无 `source_session` / 策略来源提示 | P0 |
| H3 | 同日多计划（after_close + t_sell）首页不可见（有 `same-day-candidates` 未用） | P1 |
| H4 | 机会空态未分型（易误解为「系统没机会」） | P1 |
| H5 | 机会候选池日期与「今天」可能错位（盘后池在次日） | P1 |
| H6 | Beta 流程与 Day Monitor 双轨未说明 | P1 |
| H7 | 行情价在首页机会表恒 `—`（已知） | P3 |
| H8 | Legacy 真实流水线对 Beta 用户不可见 | P2 |

---

## 8. Phase17-A 最小实现建议（只建议，不编码）

### 回答三问

1. **自动计划为什么没显示？**  
   已写入；Upcoming **不按手工过滤**；首页 **槽位精确日期 + 单条 Upcoming** 导致「下一交易日 after_close」在交易日午前常不进「我的计划」，手工当日计划更容易露脸。

2. **今日机会为空是否合理？**  
   在「无达标持仓对比」语义下 **合理**；若产品期望「池内股票列表」，则当前实现 **语义不符**（需改数据源，属产品决策，非小 bug）。

3. **下一步最值得改什么？**  
   **计划可见性与来源透明**（H1/H2），其次机会空态分型（H4），再次流程条与 Monitor 关系说明（H6）。

### 建议切片（仍属审计建议）

```text
17-A.2-B  我的计划：active 空时回退展示 next_plan；标签 source_session
17-A.2-C  今日机会：空态枚举 O1–O4 + 链到机会页/次日池提示
17-A.2-D  Beta 流程：文案「引导≠任务状态」；可选展示 Monitor 缩略
```

**边界：** 不改 Upcoming Frozen 优先语义、不改执行链、不新建机会 API；优先前端槽位与文案，必要读已有 `same-day-candidates` / home 已有字段。

---

## 9. 字段映射速查

### 我的计划

| UI | 字段 |
| --- | --- |
| 标题 | `active_plan.title` |
| 状态 Tag | `planUserStatus` ← status / freeze / morning.materialization_status |
| 只数 / 股票 | `plan.items`（最多 5） |
| 日期 | `active_plan.trade_date` |

### 今日机会

| UI | 字段 |
| --- | --- |
| 股票 | `opportunity_attention` highlights → candidate vs holding |
| 信号 | `user_label`（建议研究/关注） |
| 评分 | `candidate_score` |
| 「风险」列 | `score_gap`（实为分差） |
| 行情 | 无（固定 —） |

### Beta 流程

| UI | 字段 |
| --- | --- |
| 当前步 | 由计划用户态推导 |
| done/current | 前端 index 比较 |

### Home trading_status（隐藏）

| UI（legacy） | API |
| --- | --- |
| 晨间准备…日终结算 | `materialize_status` … `settlement_status` |

---

## 10. 停止边界

- 本文只审计、不编码。  
- 未改 API / DB / 首页 UI。  
- 本机 DB 探针因环境缺 CGO/sqlite CLI 未跑通；结论以**代码路径**为准，并与既有 Phase16 样本（`after_close` Draft 已落库）一致。  
- 实现须另开阶段，并复用 16.28 能力、避免第三套 Source/机会语义。
