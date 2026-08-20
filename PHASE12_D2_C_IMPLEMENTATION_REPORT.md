# Phase12-D2-C Home Plan Context 实现报告

**日期**：2026-08-20  
**依据**：`PHASE12_D1_DASHBOARD_DISPLAY_ARCHITECTURE_DESIGN.md`  
**目标**：首页「我的计划」按交易日与 15:00 切换今日 / 下一交易日，不改 upcoming 接口。

未改 `GET /api/tradeplans/upcoming`、TradePlan 结构、Freeze、Execution。

---

## 1. 规则（上海时区）

| 条件 | `active` | 标题 |
| --- | --- | --- |
| 交易日且 `now < 15:00:00` | `current_plan` | 今日交易计划 |
| 交易日 `now >= 15:00:00` | `next_plan` | 下一个交易日计划 |
| 周末 / 非交易日 | `next_plan` | 下一个交易日计划 |

盘前（含 00:00–09:00）仍是今日计划。切点对齐 `tradingwindow` 15:00，不用 09:00。

日历与后端 `tradingcalendar.Default` 一致：周末休市，法定假期未挂表（与 upcoming 的 `next_trading_day` 同源）。下一交易日优先用 upcoming 响应里的 `next_trading_day`。

---

## 2. Adapter

`frontend/src/utils/planContext.js` → `buildDashboardPlanContext`：

- `current_plan`：仅当 `plan.trade_date === 今日` 才采用（upcoming 在收盘后仍可能返回今日 draft，**不展示**）
- `next_plan`：第二次 `getUpcomingTradePlan(nextDay)`
- `active_plan`：按时钟选槽

首页两次只读调用现有 upcoming（与交易计划页双槽同构），展示层决定显示哪一个。股票预览仍用 D2-A `StockLink` / `StockKlineModal`。

---

## 3. 测试

```
node frontend/scripts/verify-plan-context.mjs
node frontend/scripts/verify-stock-display.mjs
node frontend/scripts/verify-opportunity-card.mjs
```

均为 `ok`。覆盖：

- 交易日上午 10:00 / 14:59:59 → `current`，主计划为今日
- 交易日 15:00 / 16:30 → `next`，今日 draft 仍留在 `current_plan` 但不展示
- 周六 / 周日 → `next`，不把工作日计划当成今日

后端相关：

```
go test ./backend/tradingcalendar
go test ./backend/api -run "TestNextTradingDay|TestIsTradingDay|TestTradePlansUpcoming_NextTradingDay"
```

均为 `ok`。未改这些包。

---

## 4. 修改文件

| 文件 | 说明 |
| --- | --- |
| `frontend/src/utils/planContext.js` | 时钟 / 日历 / 双槽装配 |
| `frontend/src/components/InvestmentHome.vue` | 两次 upcoming + 只渲染 `active_plan` |
| `frontend/scripts/verify-plan-context.mjs` | 时间语义测试 |
| `PHASE12_D2_C_IMPLEMENTATION_REPORT.md` | 本报告 |

未改：`backend/api/tradeplans.go`、Freeze、评分、机会卡。

---

## 5. 结论

首页计划上下文已按 15:00 与交易日切换。upcoming 查询语义不变。D2-A/B/C 展示层切片完成。
