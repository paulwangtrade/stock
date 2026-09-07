# PHASE17-A Portfolio UX 与数据一致性增强 — 实现前审计

**日期：** 2026-09-06  
**性质：** **只读审计**（不改代码 / API / DB / 数据结构 / 不实现）  
**目标：** 为后续 Portfolio UX 增强划定**最小改动范围**  
**上游：**  
- [PHASE16_PORTFOLIO_DATA_FRESHNESS_AUDIT.md](./PHASE16_PORTFOLIO_DATA_FRESHNESS_AUDIT.md)  
- [PHASE16_27_PORTFOLIO_UX_CLOSURE_AUDIT.md](./PHASE16_27_PORTFOLIO_UX_CLOSURE_AUDIT.md)（部分条目已被 16.27-P0/P1/P2 **落地覆盖**，以本文件「现码」为准）  
- [PHASE16_28_DEBT_REGISTER.md](./PHASE16_28_DEBT_REGISTER.md)（Source taxonomy / resolver）  
- Phase15-B1 Live Overlay · Phase16.23 Holdings UX · Phase16.27-P0/P1/P2  

---

## 0. 执行摘要

| 维度 | 结论 |
| --- | --- |
| 主入口 | `#/portfolio` → `PortfolioDashboard.vue`（「我的组合」） |
| 数据架构 | **双 API**：`snapshot?include_display=1`（资产+持仓+行情 overlay）+ `dashboard`（账户今日盈亏/风险/今日成交） |
| 「当前价不一致」根因 | **设计分层**：账本 `mark_price`（成交/日终结算写入）≠ GET 时 `display_price`（Quote overlay）；权益/市值/累计盈亏跟 mark；行情列跟 Quote；**仅手动刷新**、UI **未展示** Quote `Time`/`FetchedAt` |
| P1「股票点 K 线」 | **已具备**（持仓名 `StockLink` → `StockKlineModal`） |
| P1「个股盈亏」 | **已具备**：累计盈亏 / 收益率（mark）+ 单票今日盈亏（昨收口径）；≠ 账户「今日盈亏」 |
| P2「来源」 | **浅标签 + 抽屉已有**；列表无策略/信号摘要；resolver 与 Origin 可能分叉（见 16.28 Debt） |
| 可否进入实现 | **可以** — 建议优先 **展示澄清 + 新鲜度 UX**，禁止为「一致」而盘中写 mark / 改权益口径 |

**一句话：** 组合页闭环反馈能力在 16.23/16.27 后已较完整；Phase17 主债是 **价格口径与新鲜度可理解性**，不是从零补列。

---

## 1. 当前架构

### 1.1 页面入口与结构

```text
productMenu「我的组合」→「持仓列表」
       ↓
router name=portfolioDashboard  path=/portfolio
       ↓
PortfolioDashboard.vue
  ├─ refresh()（onMounted + 手动按钮；无盘中轮询）
  │    ├─ GET /api/portfolio/snapshot?include_display=1
  │    └─ GET /api/portfolio/dashboard
  │    └─ enrichSourceChips（best-effort：fill.plan_id / provenance → getTradePlanById）
  ├─ 账户总览：总权益 / 现金 / 市值 / 今日盈亏 / 持仓数
  ├─ 风险：riskLevel / maxPositionRatio
  ├─ 持仓表：股票(K线) / 数量 / 成本 / 估值 / 行情 / 市值 / 累计盈亏 / 收益率 / 今日盈亏 / 状态 / 来源 / 操作记录 / 卖出
  ├─ 今日成交表（dashboard.fills）
  ├─ SellDraftDialog
  ├─ PortfolioProvenanceDrawer
  └─ StockKlineModal
```

**相关文件（只读清单）**

| 层 | 路径 |
| --- | --- |
| 页 | `frontend/src/components/PortfolioDashboard.vue` |
| 抽屉 | `frontend/src/components/PortfolioProvenanceDrawer.vue` |
| API | `frontend/src/api/portfolioSnapshot.ts` · `portfolioDashboard.ts` · `portfolioProvenance.ts` |
| Utils | `portfolioQuoteDisplay.js` · `portfolioSourceChip.js` · `portfolioSellEntry.js` · `portfolioProvenanceDisplay.js` · `stockDisplayAdapters.js` |
| 可复用 | `StockLink.vue` · `StockKlineModal.vue` · `SellDraftDialog.vue` |
| 后端读模型 | `backend/portfolio/readmodel/*` · `backend/api/portfolio_dashboard.go` · `portfolio_snapshot.go` · `portfolio_provenance.go` |
| 账本 | `backend/papertrading/models.go` · `broker.go` · `settlement.go` · `realtime_price.go` |

**旁路（非本页主路径，本审计不展开改）**

| 路由/组件 | 关系 |
| --- | --- |
| `/holdings` `HoldingPositions.vue` | 隐藏菜单；非 snapshot 主读模型 |
| `/portfolio-decision-dashboard` | 决策观察另一产品面 |
| `TradePlanUpcoming` | 亦调 snapshot/dashboard，计划辅助 |

### 1.2 当前展示字段（持仓表）

| UI 列 | 绑定字段 | 口径 |
| --- | --- | --- |
| 股票 | `stockName`/`stockCode` | `StockLink` 可点 K 线 |
| 数量 | `totalQty`（tooltip 可卖/锁定） | 账本 |
| 成本价 | `avgCost` | 账本 |
| 估值价 | `markPrice` | 账本 mark |
| 行情价 | `displayPrice`（仅 live/open） | Quote overlay；缺则 `—`（**不用 mark 冒充**） |
| 市值 | `marketValue` | **mark×qty**；tooltip 可附行情市值 |
| 累计盈亏 | `pnl` | `(mark−cost)×qty` |
| 收益率 | `pnlPercent` | 相对成本 · mark |
| 今日盈亏（单票） | `todayPnl` | `(display−pre_close)×qty`；缺价则 `—` |
| 持仓状态 | `position_state` | PositionState |
| 来源 | chip | `source_session`→Strategy/Watchlist/Manual/未知 |
| 操作记录 | 今日 fill 摘要 +「详情」 | dashboard fills / provenance |
| 操作 | 卖出 | paper 可卖时 |

账户总览「今日盈亏」= dashboard `summary.daily_pnl`（**账户级**，见 §5）。

---

## 2. 数据流图

```text
┌──────────────────── 写入权威（执行/结算）────────────────────┐
│  Fill（broker）          → mark_price := fill_price           │
│  SettlementJob ~15:05    → mark_price := Realtime MarkPrice   │
│                            / Open fallback；写 updated_at      │
│  GET snapshot / overlay  → 绝不写 mark / 不改 equity            │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌── paper_sim_positions (AvgCost, MarkPrice, Qty, Name, …) ────┐
│                              │                                 │
│                              ▼                                 │
│              portfolio.Snapshot（账本投影）                      │
│                              │                                 │
│         ┌────────────────────┼────────────────────┐            │
│         ▼                    ▼                    ▼            │
│  include_display=0    include_display=1      dashboard         │
│  mark only            + QuoteService.GetQuotes                 │
│                       → display_price / today_pnl              │
│                       （FetchedAt/Time 有上游字段，              │
│                         PositionView 当前未透出）                │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    PortfolioDashboard.vue
         权益/市值/累计盈亏 ← mark
         行情列/单票今日盈亏 ← display（刷新瞬间）
         账户今日盈亏 ← daily_report 权益差
```

```text
持仓来源浅链路（现有）：

Position → (今日 Fill.plan_id | provenance.trades) → TradePlan.source_session
         → resolveProvenanceSourceBucket → chip
「详情」→ GET …/provenance → trades + origins(strategy/signal/reason)
         （Origin 全量解释；列表不展示）
```

---

## 3. API → UI 字段映射

### 3.1 `GET /api/portfolio/snapshot`

| API 字段 | UI | 已用？ |
| --- | --- | --- |
| `cash` / `equity` / `market_value` / `position_count` | 账户总览 | ✅ |
| `trade_date` / `disclaimer` / `data_source_note` | 页眉/提示 | ✅（disclaimer） |
| `updated_at` / `as_of` | — | ⚠️ 前端映射未强调展示 |
| `positions[].stock_code` / `stock_name` | 股票列 | ✅ |
| `total_qty` / `available_qty` / `locked_qty` | 数量 | ✅ |
| `avg_cost` | 成本价 | ✅ |
| `mark_price` | 估值价 | ✅ |
| `market_value` / `pnl` / `pnl_percent` | 市值/累计盈亏/收益率 | ✅ |
| `display_price` / `display_quote_source` | 行情价列 | ✅ |
| `display_market_value` / `display_pnl` / `display_pnl_percent` | 主要在 tooltip / 未主列 | ⚠️ display_pnl 未作主列（避免与累计盈亏混淆） |
| `quote_pre_close` / `today_pnl` | 单票今日盈亏 | ✅ |
| `position_state.*` | 持仓状态 / 卖出门禁 | ✅ |
| 行内 `plan_id` / `source` | **snapshot 无** | 由前端 enrich |

`include_display=1`：**仅**展示 overlay；**不**改 `mark_price` / `equity` / DB。

### 3.2 `GET /api/portfolio/dashboard`

| API 字段 | UI | 已用？ |
| --- | --- | --- |
| `summary.daily_pnl` | 账户「今日盈亏」 | ✅ |
| `summary.daily_pnl_basis` | 已映射 TS | ❌ **页未展示**（易误解） |
| `summary.equity/cash/...` | 本页资产不用（用 snapshot） | 故意不用 |
| `risk.*` | 风险区 | ✅（行业集中度常空） |
| `trades.fills[]`（含 `plan_id`） | 今日成交 + 操作记录摘要 + chip enrich | ✅ |
| `positions[]`（dashboard 旧持仓块） | 本页持仓表 **不用** | 避免双源 |

### 3.3 `GET /api/portfolio/positions/{code}/provenance`

| 块 | 字段 | UI |
| --- | --- | --- |
| `position` | qty / avg_cost / mark | 抽屉 |
| `trades[]` | plan_id / fill / 时间 | 抽屉；chip 取最新 plan_id |
| `origins[]` | strategy / signal / reason / score | 抽屉 ExplanationTable |
| `reconcile` | 归因对账 | 抽屉 |

**无**列表级 inline「为什么持有」策略/信号摘要（需点详情或另开增强）。

### 3.4 能力对照（用户关心项）

| 关心项 | 是否已有 | 位置 |
| --- | --- | --- |
| 总资产/权益 | ✅ | snapshot.equity |
| 可用资金/现金 | ✅ | snapshot.cash（「可用」语义偏账户现金，非券商 buying power） |
| 持仓 | ✅ | snapshot.positions |
| 今日盈亏（账户） | ✅ | dashboard.daily_pnl |
| 风险 | ✅ | dashboard.risk |
| 当前价格 | ✅ 双列 | mark + display |
| 成本价 | ✅ | avg_cost |
| 持仓收益 | ✅ | pnl（累计未实现） |
| 收益率 | ✅ | pnl_percent |
| 股票来源 | ✅ 浅 | chip；详抽屉 |
| plan_id | ⚠️ | fills/provenance/chip tip；snapshot 行无原生字段 |

---

## 4. Position 数据模型

### 4.1 持久化 `PaperSimPosition`

| 字段 | 有？ |
| --- | --- |
| `stock_code` / `stock_name` | ✅ |
| `total_volume` / available / locked | ✅（qty） |
| `avg_cost` | ✅ |
| `mark_price` | ✅ |
| `market_value` / `pnl` | ❌ 表内不存；读模型现算 |
| `source` / `plan_id` | ❌ 持仓表无；经 Fill / attribution |

`UpdatedAt`：成交与结算时更新；**无**独立 `MarkedAt` 列（部分旧文档提及的 MarkedAt 非当前模型主路径）。

### 4.2 Lot / Fill

- Lot：`positionstate.LoadLotsByAccount` → 嵌套 `position_state`（可卖/新建仓等）  
- Fill：`paper_sim_fills` → dashboard 今日成交 + provenance 归因  

### 4.3 `mark_price` 来源与更新时间链路

| 事件 | 写入 mark？ | 说明 |
| --- | --- | --- |
| 买入/卖出成交 | **是** | `broker`：`mark_price = fill price` |
| 日终 Settlement（工作日 ~15:05） | **是** | `SettlementJob` + `RealtimeOpenPriceProvider.MarkPrice`（现价优先，否则 Open） |
| GET `/snapshot` | **否** | `readmodel.Evaluate` 明确无写 |
| `include_display` Quote | **否** | 只填 `display_*` / `today_pnl` |
| 盘中定时刷新 mark | **否** | 无盘中 mark job |

→ **盘中长时间无成交时，mark 可停滞在「上次成交价或昨收结算价」**；行情列刷新后可变 → 用户感知「当前价不一致」。

---

## 5. 当前价格一致性审计（问题记录，不修复）

| ID | 观察 | 严重度 |
| --- | --- | --- |
| P17-1 | 行情列依赖 **手动刷新** 瞬间的 `GetQuotes`；无自动轮询 | 中 |
| P17-2 | `marketdata.Quote` 有 `Time` / `FetchedAt`，**PositionView 未透出** → UI 无法显示「行情截至几点」 | 中 |
| P17-3 | 权益/市值/累计盈亏跟 **mark**；行情列跟 **live** → 同屏双口径（设计如此，文案已部分说明） | 中（认知） |
| P17-4 | Quote 失败时行情列 `—`，估值列仍显示旧 mark → 正确不冒充，但「无行情」不够醒目 | 低 |
| P17-5 | 非交易日 / 上游延迟时 `live` 仍可能返回陈旧价；无 freshness SLA | 中 |
| P17-6 | Observation 页曾用 `display_pnl`；本页累计盈亏坚持 mark — 跨页需文档对齐 | 低 |

**结论：** 「与实际行情不一致」主要是 **账本新鲜度 + 刷新策略 + 时间戳缺失**，不是单一 bug。修复方向应优先 **UX/可选轮询/透出时间**，而非盘中写库改 mark（会冲击权益与 daily_pnl 语义）。

---

## 6. 盈亏字段审计（语义隔离）

| 层级 | 产品语义 | 现字段 | 公式/来源 | UI |
| --- | --- | --- | --- | --- |
| 账户级今日 | 相对上一结算日报的权益变化 | `daily_pnl` | equity_now − prior `paper_sim_daily_reports.equity` | 总览「今日盈亏」 |
| 单票今日浮盈 | 相对昨收 | `today_pnl` | (display − pre_close)×qty | 持仓「今日盈亏」 |
| 累计持仓收益 | 相对成本未实现 | `pnl` | (mark − cost)×qty | 「累计盈亏」 |
| （可选参考）行情浮盈 | 相对成本 · 行情价 | `display_pnl` | (display − cost)×qty | **未做主列**（避免抢义） |

**已有：** 上表前三行均已分离展示。  
**缺少/弱：**

- `daily_pnl_basis` 未上屏 → 用户不知「不可用/日报差」  
- 无「Σ单票今日盈亏」与账户今日盈亏的对照说明（二者**故意不等**）  
- 账户「可用资金」未单独于现金拆券商口径  

**禁止：** 用 `pnl` 冒充今日；用 `daily_pnl` 冒充单票；用 `display_pnl` 改写 equity。

---

## 7. 股票交互能力审计

| 能力 | 状态 | 复用点 |
| --- | --- | --- |
| 持仓名称/代码点开 K 线 | ✅ | `adaptPortfolioPosition` → `StockLink` → `applyStockClickAction` → `StockKlineModal` |
| 今日成交名称点 K 线 | ❌ | fill 列仅文本 |
| 独立行情页路由深链 | 可选 | 现以 Modal 为主，非强制跳转 |
| K 线组件 | ✅ | `StockKlineModal.vue` |

**若再增强「点名称开 K 线」（fills / 抽屉标题）：**

| 文件 | 改动性质 |
| --- | --- |
| `PortfolioDashboard.vue` | fillColumns 名称列改 `StockLink`（UI only） |
| `PortfolioProvenanceDrawer.vue` | 标题/代码链接触发同一 modal 或 emit（可选） |
| `stockDisplayAdapters.js` | 已有 `adaptPortfolioPosition` / `adaptProvenance`；多半够用 |
| **不必**新 API / 改 DB | — |

→ **P1「股票点击 K 线」对持仓表已完成**；剩余为次要入口补齐。

---

## 8. 来源追踪能力审计（结合 Phase16.28）

```text
Position → Fill(plan_id) → TradePlan → Origin Projection → Signal/Reason
```

| 能力 | 状态 |
| --- | --- |
| Provenance API | ✅ |
| 抽屉展示 strategy/signal/reason | ✅（复用 Explanation*） |
| 列表 Source chip | ✅（16.27-P2） |
| 列表内联策略名/信号/入选理由 | ❌ |
| 与 Origin `resolveOriginSourceBucket` 完全一致 | ❌（仅 `resolveProvenanceSourceBucket(session)`；见 Debt-2） |
| 复用 TradePlanOriginExplanationDrawer | ❌（provenance 自有抽屉；字段同源 Origin 块） |

**结论：** 「为什么持有」**可查**（详情）；**不可扫**（列表无 Origin 级摘要）。增强优先复用 provenance origins，避免新 API。

---

## 9. 已有能力 / 缺失能力

### 已有（勿重复实现）

- 双 API 资产/报告拆分  
- mark vs 行情分列 + overlay 不进权益  
- 成本 / 估值 / 行情 / 市值 / 累计盈亏 / 收益率 / 单票今日盈亏  
- 持仓 K 线、卖出草稿、provenance 抽屉、来源 chip  
- PositionState、今日成交摘要  

### 缺失 / 弱（Phase17 候选）

| ID | 缺失 | 建议优先级 |
| --- | --- | --- |
| G1 | 行情时间戳（API 透出 + UI） | P1 |
| G2 | 盘中自动刷新策略（可关） | P1 |
| G3 | `daily_pnl_basis` 与双「今日盈亏」对照文案 | P1 |
| G4 | 行情失败/陈旧态更醒目 | P1 |
| G5 | 列表级来源增强（策略/信号短摘要） | P2 |
| G6 | Source resolver 与 Origin 对齐（Debt-1/2） | P2 |
| G7 | 成交表/抽屉 K 线入口 | P3 |
| G8 | 可选展示「行情市值/行情浮盈」参考列（严格标注） | P3 |
| G9 | 盘中写 mark 求「数字一致」 | **不做**（越权执行链） |

---

## 10. 风险点

1. **口径混用**：把 display 写进 equity / daily_pnl → 破坏 Paper 结算语义。  
2. **盘中 mark 写入**：看似「一致」，实则改变账户权益与日报差，触及 settlement 边界。  
3. **Source 启发式分叉**：与 TradePlan Origin 页不一致（16.28 Debt）。  
4. **轮询过频**：打爆 Quote 上游 / 卡 UI；需退避与交易时段门控。  
5. **扩大 API**：多数缺口可用现有字段 + 小透出（quote time）；避免新 portfolio schema。

---

## 11. Phase17 最小实现建议（仅建议，本阶段不编码）

### P1 — 优先（最小集）

| 项 | 建议做法 | 改动面 |
| --- | --- | --- |
| 当前价一致性（认知） | 页眉固定三行口径；行情列旁展示「截至 {quote_time/fetched_at}」；缺行情明确「非实时」 | 优先 UI；若无时间字段则 **小范围** snapshot display 透出 Quote.Time/FetchedAt（只读） |
| 刷新新鲜度 | 可选「交易时段每 N 秒刷新 snapshot(include_display)」+ 手动刷新保留 | 前端 only |
| 盈亏可理解 | 展示 `daily_pnl_basis`；单票「今日」tooltip 强调 ≠ 账户今日 | 前端 only |
| K 线 | **持仓已具备** — 不重复；可选 fills 补链（可降 P3） | UI |
| 个股盈亏 | **已具备** — 不重复造列；只补文案/空态 | UI |

### P2 — 来源展示增强

- 列表在 chip 旁增加短策略名（来自已拉的 plan / 或批量 provenance 缓存，注意 N+1）  
- 或点击 chip 已有抽屉，仅优化首屏「计划 #id · strategy」一行  
- Source taxonomy / ResolveSource 与 16.28 Debt-1/2 **同阶段或紧随**，避免再分叉  

### P3 — 体验

- 成交表/抽屉 K 线  
- 参考行情市值列  
- 风险区行业集中度空态说明  

### 明确不做（本增强边界）

- CandidatePool / TradePlan schema  
- 执行链 / Settlement 语义改写  
- 用 overlay 重算总权益作为「正式资产」  
- 为对齐 Origin 而改执行或 DB  

### 建议落地切片顺序

```text
17-B  文案 + daily_pnl_basis + 双今日盈亏说明     （纯前端）
17-C  行情时间戳透出 + 列展示                     （最小 API 只读字段 或先用 as_of）
17-D  可选盘中轮询                                 （前端）
17-E  来源列表摘要 / Debt-1·2 对齐                 （前端为主）
```

---

## 12. 停止边界

- 本文 **只审计、不编码**。  
- 不修改 API / DB / 执行链。  
- 实现须另开 Phase17-B+，并以本文件「已有 vs 缺失」防重复建设。

---

## 13. 冻结对照

| 问题 | Phase17-A 判定 |
| --- | --- |
| 当前价 vs 行情 | **已知分层 + 新鲜度债**；非未实现 overlay |
| 点股票开 K 线 | **已完成**（持仓） |
| 个股盈亏 | **已完成**（累计 + 单票今日） |
| 来源为什么持有 | **抽屉完成 / 列表浅标签**；深度列表为 P2 |
