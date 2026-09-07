# PHASE17-B.2 实现前审计（组合持仓 → K 线工作台）

**日期：** 2026-09-07  
**性质：** **只读审计**（不改代码 / 不新增 API / 不改交易链 / 不建 migration / 不重构组件）  
**目标：** 冻结 **B.2-A（组合开 K 注入成本/股数）** 与 **B.2-B（摘要 + footer 薄壳）** 的最小实现范围  
**上游：**  
- [PHASE17_B2_KLINE_WORKBENCH_DESIGN.md](./PHASE17_B2_KLINE_WORKBENCH_DESIGN.md)  
- [PHASE17_A2_PORTFOLIO_IMPLEMENTATION_COMPLETE.md](./PHASE17_A2_PORTFOLIO_IMPLEMENTATION_COMPLETE.md)  
- [PHASE17_B1_KLINE_OPTIMIZATION_IMPLEMENTATION_COMPLETE.md](./PHASE17_B1_KLINE_OPTIMIZATION_IMPLEMENTATION_COMPLETE.md)  

---

## 0. 执行摘要

| 问题 | 结论 |
| --- | --- |
| 组合点股票能否带持仓上下文开 K？ | **能** — 列表行已有完整 snapshot 字段；缺口仅在 **未传入 Modal** |
| 是否需要新 API？ | **否** — 复用 `GET /api/portfolio/snapshot?include_display=1` |
| 是否需要新交易组件 / 执行链？ | **否** — 复用页内已有 `SellDraftDialog` |
| 是否需要改 K 线引擎核心？ | **否** — props + Modal 插槽已够；`stock.vue` 已是成本注入范本 |
| 最小第一批 | **B.2-A 接线 + B.2-B 摘要/footer**（仅 `PortfolioDashboard.vue` 为主） |
| 可否进入编码 | **可以**（确认本文件后开实现切片） |

**一句话：** 数据与扩展点都已具备；B.2 是 **Portfolio 开 K 的组装接线**，不是新能力建设。

---

## 1. K 线入口链路

### 1.1 关键路径与组件

| 角色 | 文件 |
| --- | --- |
| 组合页 | `frontend/src/components/PortfolioDashboard.vue` |
| 点击链 | `StockLink` ← `adaptPortfolioPosition(row)` ← `renderStockNameKlineLink` |
| 打开动作 | `openStockKline` → `applyStockClickAction`（`frontend/src/utils/stockDisplay.js`） |
| 适配器 | `frontend/src/utils/stockDisplayAdapters.js` → `adaptPortfolioPosition` |
| Modal | `frontend/src/components/StockKlineModal.vue` |
| 图表 | `frontend/src/components/StockLightweightKlineChart.vue`（attrs 透传） |

### 1.2 当前数据流

```text
持仓表行 row (SnapshotPositionRow)
    ↓ renderStockNameKlineLink
adaptPortfolioPosition(row) → StockLink model (klineKey / name / click_action)
    ↓ @open
openStockKline(model)
    ↓ applyStockClickAction
klineModal = { visible, title, chartCode, stockName }   ← 仅身份字段
    ↓
<StockKlineModal
  :code="chartCode"
  :stock-name="stockName"
/>                                                   ← 无 cost / qty / position
    ↓ v-bind="attrs"
StockLightweightKlineChart                           ← 未收到 costPrice / costVolume
```

### 1.3 当前传入参数

| 参数 | 有？ | 说明 |
| --- | --- | --- |
| `code` / `chartCode` | **有** | `applyStockClickAction` 写入 |
| `stockName` / `title` | **有** | 同上 |
| `costPrice` / `costVolume` | **无** | `stock.vue` 已有范本，组合页未接 |
| `positionAwareSignals` | **无** | 自选页可开，组合未开 |
| 持仓行引用（供摘要/卖出） | **无** | `klineModal` 未存 `row` |
| SellDraft 与 K 线联动 | **无** | 卖出仅在表「操作」列；与 Modal 独立 |

### 1.4 Position context

| 问题 | 结论 |
| --- | --- |
| 点击时是否仍持有完整 position row？ | **是** — `render` 闭包内有 `row`；但 `openStockKline` 只收 StockLink `model`，**丢掉了 row** |
| 页面内存是否仍有 positions？ | **是** — `positions` computed 来自 snapshot，可按 code 回查 |

**缺口定性：** 不是缺数据，是 **入口未把 row 上下文带进 Modal 状态**。

**对照范本（应复制接线方式，不复制业务逻辑）：**  
`stock.vue` 对 Modal 已传 `:cost-price` / `:cost-volume` / `:position-aware-signals` 等。

---

## 2. 持仓数据可复用字段审计

数据源（已有，**无需新接口**）：

- `GET /api/portfolio/snapshot?include_display=1` → `portfolioSnapshot.ts` → `SnapshotPositionRow`  
- 来源/plan：页内 `sourceChipByCode`（fills + `getTradePlanById` 浅 enrich）+ provenance 抽屉  
- 卖出：`availableQty` / `canSell`（snapshot `position_state`）

| 目标字段 | Snapshot / 前端已有 | 映射 | 新 API？ |
| --- | --- | --- | --- |
| stock_code | `stockCode` | 直接 | **否** |
| stock_name | `stockName` | 直接 | **否** |
| qty | `totalQty`（总）；可卖用 `availableQty` | 摘要展示 total；卖出用 available | **否** |
| cost_price | `avgCost` | → Modal `:cost-price` | **否** |
| mark_price | `markPrice` | 摘要「估值」 | **否** |
| 当前价 | `displayPrice`（live/open）；缺则 — | 摘要「当前价」；勿用 mark 冒充 | **否** |
| floating_profit | **累计** `pnl`；(mark−cost)×qty | 摘要「持仓浮盈」优先用 `pnl`（与 A.2「累计浮盈」一致） | **否** |
| 持仓今日浮盈 | `todayPnl` | 摘要可选第二行 | **否** |
| plan_id | `sourceChipByCode[code].planId` | 可选展示 | **否** |
| provenance/source | chip bucket + `PortfolioProvenanceDrawer` | 摘要可显示 chip；详情仍抽屉 | **否** |

**原则落地：** B.2 摘要数值 **全部从前端已加载的 `row` 读取**；不重算权益、不请求新 portfolio API。

**注意：**

- 「持仓浮盈」产品文案若指相对成本 → 用 `pnl`，勿与账户「账户今日盈亏」混淆（A.2 已隔离）。  
- `displayPnl` 为 (display−cost)×qty，仅作旁注，**不作**主「持仓浮盈」默认，以免与表「累计浮盈」分叉。

---

## 3. K 线组件扩展点审计

### 3.1 `StockKlineModal`

| 能力 | 状态 | 用途（B.2） |
| --- | --- | --- |
| `code` 唯一身份 | **有** | 保持 |
| 其余图表选项 `v-bind="attrs"` | **有** | 透传 `cost-price` / `cost-volume` / `position-aware-signals` |
| `#prepend` / `#append` / `#footer` | **有** | **摘要条 + 卖出按钮首选** |
| `embedChart` | **有** | 本阶段保持 true |
| 全局 logo 水印 | App 级 `content:''` | **不依赖**；B.2 不做品牌水印 |

### 3.2 `StockLightweightKlineChart`

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| Props 扩展 | **有** | `costPrice` / `costVolume` / `long*` / `positionAwareSignals` / `strategySignals` / `initialKlt` … |
| 成本线 | **有** | `costPrice` → `createPriceLine`（有值即显示） |
| 开仓/止损/止盈线 | **有** | 预警语义；B.2 首批可不传 |
| Chart 内插槽 | **无独立 annotation 插槽** | 摘要放 **Modal prepend**，不必改引擎 |
| Footer | Modal `#footer` | 挂「卖出」触发已有 dialog |
| B.1 缓存 / LIVE poll | **已落地** | 扩展 props **不改变**加载策略 |

**结论：** 不需要为 B.2-A/B 改图表内核；优先 **Portfolio 接线 + Modal 插槽**。

---

## 4. SellDraftDialog 复用审计

| 项 | 现状 |
| --- | --- |
| 路径 | `frontend/src/components/SellDraftDialog.vue` |
| 组合页用法 | 已挂载；`openSellDialog(row)` → `:row` / `:trade-date` / `actor="ui:portfolio-sell"` |
| 所需参数 | `show`、`row`（paper_sim 持仓行）、`tradeDate`；可选 `actor` / `sourceSession` |
| `row` 关键字段 | `stockCode`、`availableQty`（可卖）、`totalQty`、`canSell` — helpers 在 `portfolioSellEntry.js` |
| API | 既有 `createTSellDraft`（`tradePlansTSell`）→ Upcoming；**不改执行链** |
| 从 K 线工作台打开 | **可以** — footer 按钮调用**同一** `openSellDialog(klinePositionRow)`，**禁止复制** Dialog |

**约束：**

- 仅 paper_sim 行（`canShowSellButton`）；不可卖时 footer 隐藏或 disabled + tooltip。  
- 不新建「K 线专用卖出组件」。

---

## 5. 最小修改范围

### 5.1 需要修改（建议）

| 文件 | 目的 | 批次 |
| --- | --- | --- |
| `frontend/src/components/PortfolioDashboard.vue` | ① `klineModal` 增加持仓上下文（row 或 cost/qty 字段）② 开 K 时注入 `:cost-price` / `:cost-volume` / 可选 `:position-aware-signals` ③ `#prepend` 仓位摘要 ④ `#footer` 触发既有 `openSellDialog` | **B.2-A + B.2-B** |

可选（仍属前端、零 API）：

| 文件 | 目的 | 是否必须 |
| --- | --- | --- |
| 小 util（如 `portfolioKlineWorkbench.js`） | 从 row 派生摘要展示模型，保持 Dashboard 薄 | **可选**；非必须新文件 |

### 5.2 不需要修改

| 类别 | 说明 |
| --- | --- |
| 后端 API | snapshot / dashboard / t-sell/draft / provenance **不动** |
| 数据模型 / schema | **不动** |
| 交易执行链 / Fill / Gateway | **不动** |
| `StockLightweightKlineChart` 核心 | **首批不动**（仅消费已有 props） |
| `SellDraftDialog` | **不动实现**（只多一个打开入口） |
| K 线缓存 / B.1 LIVE 门控 | **不动** |
| TradePlan / Opportunity / 首页双槽 | **不动** |
| 新建图表引擎 / Fill markers | **暂缓** |

---

## 6. 风险检查

| 风险 | 影响？ | 说明 / 缓解 |
| --- | --- | --- |
| K 线缓存 | **否** | 仍走 `getOrFetch`；仅多传展示 props |
| LIVE 轮询 | **否** | `setupPoll` / `isKlineMarketLive` 不变 |
| TradePlan | **低** | 卖出仍走既有 draft→Upcoming；与表列卖出同源 |
| Portfolio 性能 | **低** | 无新请求；摘要纯计算；Modal 仍按需挂载 |
| 关闭 Modal 丢上下文 | **可控** | row 存 `klineModal` 即可；切股时更新 |
| 价位线被当成委托 | **文案** | 成本线 = 观察；卖出仅 footer → SellDraft |
| 自选 vs paper 混淆 | **低** | 本切片只改组合页 paper_sim |
| Modal `chartMountKey` 重建 | **已知** | B.1-P2 keep-alive 仍暂缓；不扩大范围 |

---

## 7. 最终建议

### A. 推荐实现顺序

```text
B.2-A  组合开 K：保存 position row + 注入 costPrice/costVolume（+ 可选 positionAwareSignals）
         → 成本线立即可见
B.2-B  Modal #prepend 摘要（名称/代码/数量/成本/当前价/累计浮盈）
         + #footer「卖出」→ 复用 openSellDialog / SellDraftDialog
（验收） 周末 IDLE 秒开仍成立；卖出草稿仍进 Upcoming
```

### B. 第一批只实现

1. **PortfolioDashboard** 开 K 携带持仓上下文  
2. 向 Modal/Chart 注入 **成本价、股数**（复用图表成本线）  
3. **摘要条**（prepend）：名称/代码、数量、成本、当前价、持仓浮盈（`pnl`）  
4. **Footer 卖出入口**（条件同表列 `canShowSellButton`）  

不做：新 API、新 Dialog、改图表引擎、改缓存、改执行链。

### C. 明确暂缓

| 暂缓项 | 原因 |
| --- | --- |
| Fill 成交点图标 | B.2-D；需 markers 分层 |
| 自动卖出评价 / 退出引擎挂钩 | 超出工作台接线 |
| 新建图表引擎 | 禁止 |
| HoldingT SVG 统一 | B.2-C |
| Modal keep-alive | B.1-P2 / B.2-E |
| 止损止盈线默认打开 | 可选后续；避免与「卖出」语义抢戏 |
| 品牌 logo 水印 | 全局 watermark 为空；非本批 |

---

## 8. 目标体验对照（验收口径预告）

| 体验项 | 实现手段（编码时） |
| --- | --- |
| 股票名称/代码 | title + 摘要（已有 name/code） |
| 当前持仓数量 | `row.totalQty` |
| 成本价 | `row.avgCost` + `:cost-price` → 成本线 |
| 当前价 | `row.displayPrice`（无则 —） |
| 持仓浮盈 | `row.pnl` |
| 成本线 | 已有图表能力 |
| 卖出计划入口 | footer → 既有 `SellDraftDialog` |

---

## 9. 停止边界（编码前冻结）

- 本文 **只审计**；**禁止**在本阶段改代码。  
- 实现切片启动后仍须：**零新 API、零 schema、零执行链改动、零复制 SellDraftDialog**。  
- 主改文件收敛为 **`PortfolioDashboard.vue`（± 可选小 util）**。

**审计结论：可以进入 B.2-A/B 编码；范围已足够小且依赖已齐。**

---

*只读审计输出；未修改任何代码。*
