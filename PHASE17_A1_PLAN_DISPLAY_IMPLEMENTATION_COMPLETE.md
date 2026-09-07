# PHASE17-A.1 首页「我的计划」双槽展示 — 实现完成

**日期：** 2026-09-06  
**依据：** [PHASE17_A1_PLAN_DISPLAY_AUDIT.md](./PHASE17_A1_PLAN_DISPLAY_AUDIT.md)  
**方案：** **A（仅前端）** — 不改后端 / Upcoming 语义 / 生成链 / CandidatePool / DB  

---

## 1. 修改文件列表

| 文件 | 变更 |
| --- | --- |
| `frontend/src/components/InvestmentHome.vue` | 双槽 UI；加载 same-day-candidates；Source chip；详情深链 |
| `frontend/src/utils/homePlanDisplay.js` | **新增** 槽模型 / chip 映射 / 空态文案 |
| `frontend/src/utils/homePlanDisplay.test.mjs` | **新增** 单测（6 cases） |

**未改：** Go API、TradePlan schema、生成链、`GetUpcomingTradePlan`、`planContext` Upcoming 契约。

---

## 2. 修改说明

### 2.1 展示结构

```text
我的计划
├─ 今日执行      ← planContext.current_plan + same-day(today)
└─ 下一交易日准备 ← planContext.next_plan + same-day(next)
```

不再只渲染 `active_plan`。

### 2.2 每条计划卡片字段

| 字段 | 实现 |
| --- | --- |
| 状态 | `planUserStatus` → Tag（待确认 / 已准备 / 已锁定） |
| 来源 | Phase16.28 chip：`resolveProvenanceSourceBucket` → Strategy / Watchlist / Manual / 未知来源 |
| 股票数量 | upcoming `items.length`（同日额外候选无 items 时标「同日其他计划」） |
| 计划日期 | 槽 `trade_date` + 行内 `#planId` |
| 详情 | `router.push({ name: 'tradePlanUpcoming', query: { trade_date, plan_id } })` |

### 2.3 同日多计划

复用已有 `GET /api/tradeplans/same-day-candidates`（与 TradePlanUpcoming 同 API）。  
主卡片 = 槽内 upcoming plan；其余 candidate 列为 extra 行（排除 primary id）。

### 2.4 空态

| 槽 | 文案 |
| --- | --- |
| 今日 | `今日暂无执行计划` |
| 下一交易日 | `下一交易日暂无准备计划` |

不使用「系统无计划」。

### 2.5 兼容

- Beta「当前状态 / 流程下一步」：优先今日 plan，否则次日（`guidancePlan`）。  
- Upcoming Frozen 优先规则：**未改**。

---

## 3. 验证

### 3.1 单元测试

```text
node --test src/utils/homePlanDisplay.test.mjs
→ 6 pass / 0 fail
```

### 3.2 npm build

```text
Set-Location D:\stock\frontend; npm run build
→ ✓ built in ~2m 22s（exit 0）
```

### 3.3 Runtime 验收记录（逻辑 / 场景仿真）

本机未强制重启 Wails GUI；用与页面相同的 `buildDashboardPlanContext` + `buildHomePlanSlotModel` 做场景验收：

| # | 场景 | 结果 |
| --- | --- | --- |
| 1 | 手工 `t_sell` 在今日 | 今日执行：Manual + 只数；PASS |
| 2 | 自动 `after_close` 在次日 | 下一交易日准备：Strategy；PASS |
| 3 | 今日 Manual + 次日 Strategy 同时存在 | 双槽均 `hasPlan`；旧 `active_plan` 仅指向今日，次日仍展示；PASS |
| 4 | 同日多计划（t_sell + watchlist） | primary + extras=1（Watchlist）；PASS |
| 5 | **回归 bug：** 午前仅次日 after_close | 旧 active 为空；新：今日空态文案 + 次日 Strategy #60；PASS |

关键回归（#5）：

```text
OLD_ACTIVE null current
NEW_TODAY false 今日暂无执行计划
NEW_NEXT true Strategy 60
```

→ 自动计划可见性问题在展示层已关闭。

### 3.4 GUI 建议手测（可选）

1. 重启 / 刷新前端后打开「首页」  
2. 确认双槽标题与 Source chip  
3. 点「查看详情」进入交易计划页且带 `plan_id`  
4. 无计划日核对空态文案  

---

## 4. 边界确认

| 禁止项 | 状态 |
| --- | --- |
| 后端 API | 未改 |
| TradePlan schema / 生成链 | 未改 |
| CandidatePool | 未改 |
| Upcoming 选择语义 | 未改 |

---

## 5. 结论

Phase17-A.1 **实现完成（方案 A）**。  
首页可同时看到「今日执行」与「下一交易日准备」；`after_close` 不再因单槽 active 被误判为不存在。
