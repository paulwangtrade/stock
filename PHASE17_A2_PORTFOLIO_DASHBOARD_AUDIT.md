# PHASE17-A.2 Portfolio Dashboard 字段语义增强 — 实现前审计

**日期：** 2026-09-06  
**性质：** **只读审计**（不改代码 / API / DB / 数据模型 / 不新增字段）  
**上游：**  
- [PHASE17_A_PORTFOLIO_UX_AUDIT.md](./PHASE17_A_PORTFOLIO_UX_AUDIT.md)  
- [PHASE17_A_HOME_ENTRY_AUDIT.md](./PHASE17_A_HOME_ENTRY_AUDIT.md)  
- [PHASE16_PORTFOLIO_DATA_FRESHNESS_AUDIT.md](./PHASE16_PORTFOLIO_DATA_FRESHNESS_AUDIT.md)  

**前提结论（沿用）：** Portfolio **不必重建**；主债是 **字段语义专业化与防误解**（账户盈亏 / 持仓盈亏 / 价格口径 / 更新时间）。

---

## 0. 执行摘要

| 维度 | 结论 |
| --- | --- |
| 页面能力 | 「我的组合」已具备双 API 资产、持仓分列价、累计/单票今日盈亏、来源 chip、provenance、K 线、卖出 |
| 账户「今日盈亏」真义 | **相对上一结算日报的权益差**（`daily_report_delta`），**不是** Σ 持仓今日浮盈，也**不随**行情 overlay 变 |
| 持仓「今日盈亏」真义 | **相对昨收** `(display − pre_close)×qty`；与账户今日 **故意不等** |
| 持仓「累计盈亏」真义 | **相对成本 · mark** `(mark − cost)×qty` |
| 最大误解风险 | 两处都叫「今日盈亏」；总权益跟 mark、行情列跟 Quote；无 `daily_pnl_basis` / 无行情时间戳 |
| 推荐实现 | **方案 A：纯前端语义增强**（改名、tooltip、basis、时间展示若已有 as_of）；B 仅当必须透出 Quote 时间；**不做 C** |

---

## 1. Portfolio Dashboard 页面能力

### 1.1 组件与数据源

| 区域 | 组件 | 数据来源 | API |
| --- | --- | --- | --- |
| 我的组合主页 | `PortfolioDashboard.vue` | Snapshot + Dashboard 拼装 | `GET /api/portfolio/snapshot?include_display=1` + `GET /api/portfolio/dashboard` |
| 首页「我的资产」卡片 | `InvestmentHome.vue` 内联 | home 聚合 Snapshot 投影 | `GET /api/investment/home` → `portfolio_summary` |
| 持仓表 | 同 Dashboard 内 `n-data-table` | `snapshot.positions` | 同上 snapshot |
| 来源 / 详情 | chip + `PortfolioProvenanceDrawer` | fill / provenance / TradePlan | `…/provenance`、`getTradePlanById` |
| Utils | `portfolioQuoteDisplay.js`、`portfolioSourceChip.js`、`portfolioSellEntry.js` 等 | 展示映射 only | — |

**无独立「持仓列表组件」文件**；列定义在 `PortfolioDashboard.vue` 的 `positionColumns`。

### 1.2 当前展示一览

**账户总览（组合页）**

| UI 标签 | 绑定 | 口径 |
| --- | --- | --- |
| 总权益 | `snapshot.equity` | cash + Σ(mark×qty) |
| 现金 | `snapshot.cash` | 账本现金 |
| 市值 | `snapshot.market_value` | Σ mark×qty |
| 今日盈亏 | `dashboard.summary.daily_pnl` | 见 §2 |
| 持仓数 | `snapshot.position_count` | 持仓行数 |

**风险**

| UI | 绑定 |
| --- | --- |
| 风险等级 | `dashboard.risk.risk_level` |
| 最大仓位比例 | `dashboard.risk.max_position_ratio` |

**首页「我的资产」**（更精简）

| UI | 绑定 | 相对组合页 |
| --- | --- | --- |
| 总资产 | `portfolio.equity` | 同权益；命名「总资产」 |
| 现金 / 股票市值 / 持仓数量 | cash / market_value / count | **无**今日盈亏、**无**风险 |

**持仓表（已有列，语义见 §3）**  
股票(K线) · 数量 · 成本 · 估值(mark) · 行情(display) · 市值(mark) · 累计盈亏 · 收益率 · 今日盈亏(单票) · 状态 · 来源 · 操作记录 · 卖出。

---

## 2. 账户级盈亏审计

### 2.1 来源

```text
GET /api/portfolio/dashboard
  → portfolio.Dashboard / ProjectDashboard
  → summary.daily_pnl
  → summary.daily_pnl_basis ∈ { daily_report_delta | unavailable }
```

计算（`dashboard_assemble.go`）：

```text
daily_pnl = 当前账户权益(equity_now) − 上一纸面日报权益(prior paper_sim_daily_reports.equity)
basis     = daily_report_delta   （有前日报）
          = unavailable          （无前日报 → daily_pnl=null，UI「—」）
```

权益本身来自 **mark 账本**（成交改 mark、约 15:05 Settlement 改 mark），**不含** GET 时 Quote overlay。

### 2.2 它代表什么？

| 说法 | 是否正确 |
| --- | --- |
| 账户净值相对「上一结算冻结点」的变化 | **是** |
| 盘中浮动盈亏（现价 vs 昨收汇总） | **否** |
| Σ 持仓表「今日盈亏」 | **否**（测试亦故意不等） |
| 未实现累计浮盈 Σ(mark−cost)×qty | **否**（那是 `pnl` / `ledgerPnl`） |

### 2.3 命名建议（展示层，不改 API 字段名）

| 现文案 | 建议展示名 | 副文案 / tooltip |
| --- | --- | --- |
| 今日盈亏（账户总览） | **账户今日盈亏** | 「相对上一结算日报的权益变化；含成交与日终估值，不含盘中行情 overlay」 |
| （缺失） | 依据标签 | 展示已有 `daily_pnl_basis`：`日报差` / `暂无上一日报` |
| 持仓列「今日盈亏」 | **持仓今日浮盈** 或 **相对昨收** | 「(行情价−昨收)×数量；≠ 账户今日盈亏」 |
| 累计盈亏 | **持仓累计浮盈** | 「相对成本 · 账本估值价」 |

---

## 3. 持仓级盈亏审计

### 3.1 Position / Snapshot 已有字段

| 字段 | 含义 | UI |
| --- | --- | --- |
| `avg_cost` | 成本 | ✅ 成本价 |
| `total_qty` 等 | 数量 | ✅ |
| `mark_price` | 账本估值价 | ✅ 估值价 |
| `market_value` | mark×qty | ✅ 市值 |
| `pnl` | (mark−cost)×qty | ✅ **累计盈亏** |
| `pnl_percent` | (mark−cost)/cost | ✅ **收益率** |
| `display_price` / source | Quote overlay | ✅ 行情价（无 live 则 —） |
| `display_pnl` | (display−cost)×qty | ⚠️ 仅 tooltip/旁路，**非主列** |
| `quote_pre_close` | 昨收 | 内部 |
| `today_pnl` | (display−pre_close)×qty | ✅ 列名「今日盈亏」 |
| `source` / `plan_id` | 非 position 表字段 | chip 前端 enrich |

### 3.2 已有 / 缺少 / 建议展示

| 能力 | 状态 |
| --- | --- |
| 单票累计收益 | **已有**（`pnl`）— 建议改名「累计浮盈」 |
| 单票今日涨跌（金额） | **已有**（`today_pnl`）— 建议改名「今日浮盈」 |
| 收益率 | **已有**（`pnl_percent`，mark 口径） |
| 单票今日涨跌幅 % | **缺展示**（可由 display 与 pre_close 前端算，属展示；本阶段不强制新 API） |
| 账户累计浮盈汇总 | 前端可 `ledgerPnl`（snapshot 已映射）— **总览未展** |
| `daily_pnl_basis` | API 有，**UI 未展** |

---

## 4. 当前价格链路审计（只记录）

```text
写入权威：
  Fill → mark_price := 成交价
  SettlementJob ~15:05 → mark_price := Realtime MarkPrice / Open fallback
  GET snapshot / overlay → 绝不写 mark

展示：
  行情源 QuoteService.GetQuotes（include_display=1）
    → display_price / display_quote_source / quote_pre_close / today_pnl
  UI：估值列=mark；行情列=仅 live/open；权益/市值/累计盈亏=mark
```

| 检查项 | 结论 |
| --- | --- |
| 当前价格是否实时？ | **行情列**：刷新瞬间的 Quote；**估值/权益**：账本 mark（可停滞至成交或 15:05） |
| 更新时间是否保存？ | Quote 有 `Time`/`FetchedAt`；**PositionView 未透出**；UI **无**「截至时刻」。Snapshot 有 `as_of`/`updated_at`，组合页 **未强调展示** |
| 来源是否可展示？ | 有 `display_quote_source`（live/open_fallback/persisted）与页级「含行情 overlay」Tag；行级可加强「行情/估值」chip |

**禁止本阶段：** 盘中写 mark、用 overlay 改 equity。

---

## 5. 首页组合摘要设计审计

### 5.1 现状（专业软件视角）

首页仅：总资产 · 现金 · 股票市值 · 持仓数量。  
缺：账户今日盈亏、仓位比、盈亏家数、价格新鲜度。对「交易驾驶舱」偏薄，但对「入口摘要」可接受——**组合页才是真相页**。

### 5.2 建议增量与优先级

**账户层**

| 项 | 优先级 | 说明 |
| --- | --- | --- |
| 账户今日盈亏 + basis 提示 | **P1** | home 已有 dashboard 链到 daily_pnl（经 portfolio_summary）；首页可展，命名勿与持仓混淆 |
| 仓位比例（市值/权益） | **P1** | 纯前端算，无新字段 |
| 累计浮盈（Σ pnl） | **P2** | 可用 snapshot 行求和或已有 ledger 映射 |
| 「可用资金」细分 | **P3** | 现现金≈可用；券商 buying power 无模型则勿假造 |

**持仓层（首页摘要）**

| 项 | 优先级 | 说明 |
| --- | --- | --- |
| 盈利/亏损股票数 | **P2** | 前端按 `pnl` 符号计数 |
| 持仓浮盈合计 | **P2** | 同累计浮盈 |

**行情层**

| 项 | 优先级 | 说明 |
| --- | --- | --- |
| 刷新/as_of 时间 | **P1**（组合页）/ **P2**（首页） | 先展示已有 as_of；Quote 时间需 B 才稳 |
| 数据来源一句 | **P1** | 「权益按估值；行情仅参考」固定文案 |

---

## 6. 与已有能力复用（禁止重复开发）

| 能力 | 复用方式 |
| --- | --- |
| Phase16.27/28 Source chip | 列表已有；语义增强勿新 taxonomy（Debt-1 未落地前复用 `portfolioSourceChip`） |
| Origin / strategy / signal | **抽屉 provenance** 已含；列表不重做 Origin 表 |
| Phase15 Provenance | `PortfolioProvenanceDrawer` + API |
| Fill / TradePlan | 操作记录摘要、chip enrich 已用 |
| `daily_pnl_basis` | **只展示**，不改计算 |
| `today_pnl` / `pnl` / `pnl_percent` | **改标签与 tooltip**，不改公式 |

---

## 7. 最小实现方案对比

### 方案 A — 纯前端展示增强（**推荐**）

| 内容 | 说明 |
| --- | --- |
| 组合页 | 账户「今日盈亏」→「账户今日盈亏」+ basis；持仓列改名；加强 tooltip；展示 as_of/刷新说明 |
| 首页 | 可选增加账户今日盈亏、仓位比、一句话口径 |
| API/DB | **零改动** |
| 风险 | 低；不碰交易链 |

### 方案 B — 少量 DTO 只读透出

| 内容 | 何时需要 |
| --- | --- |
| Snapshot display 透出 Quote `Time`/`FetchedAt` | 必须做「行情截至几点」且现有 as_of 不够 |
| 约束 | 只读字段；**不写** mark；**不改** equity 公式 |

### 方案 C — 新增接口

| 结论 | **不推荐** |
| --- | --- |
| 原因 | 双 API 已够；再聚合增加误解面 |

**推荐：先做方案 A；验证仍缺时间戳再开 B。**

---

## 8. 用户误解风险（汇总）

| ID | 风险 | 缓解（A） |
| --- | --- | --- |
| M1 | 账户「今日盈亏」被当成盘中浮盈 | 改名 + basis + tooltip |
| M2 | 账户今日 ≠ Σ 持仓今日 | 双列命名区分 + 页眉说明 |
| M3 | 行情价变了但总权益不动 | 固定文案：权益跟估值价 |
| M4 | 估值价像「过期现价」 | 列名「估值价」保留；旁注结算/成交更新 |
| M5 | 首页无盈亏像「系统没算」 | 首页补账户今日或链到组合 |
| M6 | Source chip 与 Origin 启发分叉 | 不扩 resolver；沿用 Debt-2 |

---

## 9. 专业化优化建议（落地顺序）

```text
P1  命名与 tooltip 隔离三套盈亏 + 展示 daily_pnl_basis + 口径条
P1  组合页 as_of /「手动刷新」新鲜度提示
P2  首页补账户今日盈亏、仓位比、盈亏家数
P2  行级行情/估值来源 chip 强化
P3  单票今日涨跌幅 %、可用资金文案细化
—— 仅当产品强需 ——
B   Quote 时间戳只读透出
```

**最小实现范围（进入编码时）：**

- 允许：`PortfolioDashboard.vue`、`InvestmentHome.vue`（资产区）、既有 display utils  
- 禁止：改 Go 计算、改 mark 写入、改 schema、新 portfolio API、重复 provenance/Origin  

---

## 10. 字段真实含义速查

| 展示位置 | 真实含义 | 公式/来源 |
| --- | --- | --- |
| 总权益/总资产 | 账本权益 | cash + Σ mark×qty |
| 账户今日盈亏 | 日报权益差 | equity_now − prior_report.equity |
| 市值 | 账本市值 | Σ mark×qty |
| 估值价 | 持仓记账价 | mark（成交/结算写入） |
| 行情价 | 展示用现价 | Quote.Price（刷新瞬间） |
| 累计盈亏 | 未实现相对成本 | (mark−cost)×qty |
| 收益率 | 同上比率 | (mark−cost)/cost |
| 持仓今日盈亏 | 相对昨收浮盈 | (display−pre_close)×qty |

---

## 11. 停止边界

- 本文只审计与命名/展示建议，**不编码**。  
- 不修改 API / DB / 模型 / 不新增后端字段。  
- 确认采用方案 A 后再开实现切片（建议称 Phase17-A.2-B 或实现任务）。
