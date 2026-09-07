# PHASE17.1 Dashboard UX 优化 — 实现完成

**日期：** 2026-09-07  
**依据：** [PHASE17_1_DASHBOARD_UX_AUDIT.md](./PHASE17_1_DASHBOARD_UX_AUDIT.md)  
**性质：** 仅展示层（**未改 API / CandidatePool / 策略 / daily_pnl 计算**）

---

## 0. 结果

| 项 | 结果 |
| --- | --- |
| 今日交易流程横向紧凑 | **PASS** |
| 今日机会按 stock_code 聚合 | **PASS** |
| 盈亏色 A 股红涨绿跌 | **PASS**（首页 + 组合） |
| `npm run build` | **PASS**（exit 0） |

---

## 1. 修改文件

| 文件 | 变更 |
| --- | --- |
| `frontend/src/components/InvestmentHome.vue` | 流程横向步骤条 CSS/DOM；`opportunityTableRows` 按 code 聚合；列：主要信号 / 信号数量 / 最大分差；`pnlColor` ← `marketColor.js` |
| `frontend/src/components/PortfolioDashboard.vue` | 删除本地绿涨红跌 `pnlColor`；改用 `marketColor.pnlColor` |

未改：后端、`opportunityCard.js` 适配器数据源、`router`、策略、Snapshot / daily_pnl 公式。

---

## 2. 分项说明

### 2.1 今日交易流程

- **保留：** `BETA_TRADING_FLOW`、`currentFlowStepKey`、`goFlowStep`、done/current/upcoming。
- **调整：** `flex-direction: row` + `→` 连接；步骤改为紧凑 pill；hint 进 `title`；状态色：当前橙、完成绿。
- **效果：** 显著降低首屏高度。

### 2.2 今日机会去重

- **聚合键：** `candidate_stock.code`（lowercase）。
- **每行展示：** 股票、主要信号（优先「建议研究」）、信号数量（原 highlight 条数）、评分、最大分差。
- **禁止项遵守：** 未改 API / CandidatePool / `buildOpportunity`。

### 2.3 盈亏颜色

| 约定 | 实现 |
| --- | --- |
| 盈利 | `marketUp` 红（`#ec0000` / CSS var） |
| 亏损 | `marketDown` 绿（`#00b578`） |
| 平盘 | `marketFlat` 灰（`#8c8c8c`） |

来源：`frontend/src/utils/marketColor.js` → `pnlColor()`。  
作用面：首页「账户今日盈亏」；组合总览 / 累计浮盈 / 持仓今日浮盈 / Modal 浮盈等原本地 `pnlColor` 调用点。

---

## 3. 验证

| 检查 | 方式 | 结论 |
| --- | --- | --- |
| 页面可编译 | `npm run build` | **PASS** |
| 布局 | 源码：横向 `.beta-flow-steps` | **PASS**（需 exe 目视确认高度） |
| 机会不重复 | 聚合 `Map` by code，一行一票 | **PASS**（逻辑） |
| 盈亏 A 股色 | 两页共用 `marketColor.pnlColor` | **PASS**（逻辑） |

桌面 GUI 点验：关闭旧 exe 后 `wails build -skipbindings -s` 再开新包。

---

## 4. 禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 新组件 | 是 |
| 后端接口 | 是 |
| 策略调整 | 是 |
| 改 daily_pnl 计算 | 是 |

---

*Phase17.1 Dashboard UX 实现完成。*
