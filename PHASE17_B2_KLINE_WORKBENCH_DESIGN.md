# PHASE17-B.2 K 线工作台能力设计审计

**日期：** 2026-09-06  
**性质：** **只读设计审计**（不改代码 / API / DB / 交易链）  
**目标：** 评估现有 K 线组件能否升级为 **股票研究 + 持仓操作工作台**  
**上游：**  
- [PHASE16_KLINE_COMPONENT_ARCHITECTURE_AUDIT.md](./PHASE16_KLINE_COMPONENT_ARCHITECTURE_AUDIT.md)  
- [PHASE16_KLINE_MODAL_CONTRACT_IMPLEMENTATION.md](./PHASE16_KLINE_MODAL_CONTRACT_IMPLEMENTATION.md)  
- [PHASE17_B1_KLINE_OPTIMIZATION_DESIGN.md](./PHASE17_B1_KLINE_OPTIMIZATION_DESIGN.md)  
- [PHASE17_A2_PORTFOLIO_DASHBOARD_AUDIT.md](./PHASE17_A2_PORTFOLIO_DASHBOARD_AUDIT.md)  

---

## 0. 执行摘要

| 问题 | 结论 |
| --- | --- |
| 现有 K 线能否做工作台底座？ | **能** — `StockKlineModal` → `StockLightweightKlineChart` 已具备多周期、指标、信号 markers、成本/止损止盈价位线、LIVE/IDLE 加载策略 |
| 是否需要新建第二套图表引擎？ | **否** |
| 是否需要新建交易链？ | **否** — 卖出复用 `SellDraftDialog` / paper draft；价位线保持观察/预警语义 |
| 最大缺口 | ① Portfolio 开 K 线未注入成本/股数 ② K 线旁无仓位摘要/卖出入口 ③ 成交/TradePlan 图上标记缺失 ④ 做 T 仍走旁路 SVG |
| 推荐路径 | **薄壳组装 + 接线**，分阶段 B.2-A→E；先持仓模式壳，后 markers / UX |

**可否进入后续编码切片：** **可以**（先冻结契约与范围，再开 B.2-A）。

---

## 1. 现有 K 线能力

### 1.1 组件分层

```text
StockLink / 各页 openStockKline
        ↓
StockKlineModal          ← 壳：code 契约、最大化、prepend/append/footer 插槽
        ↓ attrs 透传
StockLightweightKlineChart ← 引擎：lightweight-charts + 指标 + 信号 + 价位线
        ↓
klineCache.getOrFetch → Wails GetStockEastMoneyKLine(Page)
        ↓
MarketData / EastMoney + SQLite kline_cache（B.1 TTL / barCount / LIVE poll）
```

| 组件 | 路径 | 角色 |
| --- | --- | --- |
| `StockLink` | `frontend/src/components/StockLink.vue` | 统一点击开 K |
| `StockKlineModal` | `frontend/src/components/StockKlineModal.vue` | 弹窗壳；**唯一身份 prop = `code`** |
| `StockLightweightKlineChart` | `frontend/src/components/StockLightweightKlineChart.vue` | 主图表引擎 |
| `HoldingTPanel` | `frontend/src/components/HoldingTPanel.vue` | 做 T 观察（**旁路 SVG**，非 Lightweight） |
| `KLineChart.vue` / `stockSparkLine.vue` | 同目录 | 遗留 ECharts / 迷你线 — **不进工作台主路径** |

### 1.2 数据来源

| 层 | 能力 | 状态 |
| --- | --- | --- |
| Wails | `GetStockEastMoneyKLine` / `GetStockEastMoneyKLinePage` | **有** |
| 前端内存缓存 | `klineCache.js` LIVE 5min / IDLE 30min | **有**（B.1） |
| 后端 SQLite | `kline_cache.go` LIVE/IDLE TTL + latest barCount 命中 | **有**（B.1） |
| 轮询 | 默认 60s；`isKlineMarketLive` 门控 | **有**（B.1） |
| 历史左拖 | Page + end≠latest | **有** |

### 1.3 指标体系与信号

| 能力 | 状态 | 备注 |
| --- | --- | --- |
| MA5/10/20/60 | **有** | toolbar 开关 |
| BOLL / OBV / MACD / KDJ / RSI | **有** | toolbar |
| 冰点 / 买卖点策略信号 | **有** | `strategySignals` + `icePointSignals.computeFullSignals` |
| 持仓「加」叠加 | **有** | `positionAwareSignals` + `costVolume` |
| 成本过滤卖点 | **有** | `costAwareSellSignals` |
| 信号回放 | **有** | `signalAsOfDay` / `signalReplayMode` |
| `createSeriesMarkers` | **有** | 主图标记通道 |

### 1.4 扩展接口（工作台关键）

**`StockLightweightKlineChart` props（节选）：**

| Prop | 用途 |
| --- | --- |
| `code` / `stockName` | 身份 |
| `costPrice` / `costVolume` | 成本线 + 加仓信号门槛 |
| `longEntryPrice` / `longStopLossPrice` / `longTakeProfitPrice` | 多单价位线（可拖拽） |
| `positionAwareSignals` / `strategySignals` | 信号模式 |
| `initialKlt` | 默认周期（默认 `'101'` 日 K） |
| `realtimeIntervalMs` | 轮询间隔；0 关闭 |
| `focusSignalTag` / 回放相关 | 列表对齐 / 历史 |

**Emits：** `update:longEntryPrice|longStopLossPrice|longTakeProfitPrice|costPrice`

**`StockKlineModal` 扩展面：**

| 机制 | 用途 |
| --- | --- |
| `v-bind="attrs"` | 图表选项透传（勿用 `chart-code`） |
| `#prepend` / `#append` / `#footer` | **工作台侧栏/操作条首选** |
| `embedChart` | 可换自定义内容 |
| 关弹窗销毁 | `:key` + `show` 控制；keep-alive 仍为 B.1-P2 暂缓 |

**最完整接线参考：** `stock.vue`（成本/止盈止损 + `position-aware-signals`）。  
**最弱接线参考：** `PortfolioDashboard.vue` 仅传 `code` / `stock-name`。

---

## 2. 持仓模式评估

### 2.1 目标能力 vs 现状

| 目标能力 | 现状 | 判定 |
| --- | --- | --- |
| 成本线 | `costPrice` → `createPriceLine`；有 prop 即显示 | **有**（Portfolio 未注入） |
| 开仓 / 止损 / 止盈线 | 同价位线系统；可拖、可点选；toolbar「设置价位线(预警)」 | **有**（预警语义，非下单） |
| 持仓数量展示 | `costVolume` 仅供信号逻辑；**无独立 UI 条** | **缺展示** |
| 盈亏区域 | 价位线有风险/目标幅度 hint（`longPositionStats`）；**无**图上盈亏填充区；浮盈在 `HoldingTPanel` | **部分** |
| 做 T 入口 | 路由 `/holding-t` → `HoldingTPanel`（SVG + 常空 markers） | **旁路有 / 未接主图** |
| Sell Draft | `SellDraftDialog` + `portfolioSellEntry`；挂在组合/持仓表 | **有（未挂 K 线）** |

### 2.2 复用结论

**可以复用同一 K 线页面做持仓模式**，推荐形态：

```text
StockKlineModal（或薄壳 PositionKlineWorkbench）
  #prepend  仓位摘要：数量 / 成本 / 累计浮盈 / 持仓今日浮盈（只读算式）
  图表      :cost-price :cost-volume :position-aware-signals="true"
            （可选 long-* 价位线）
  #footer   [卖出] → SellDraftDialog（注入 paper 持仓行）
            [做T]  → router 或后续换 Lightweight Modal
```

| 原则 | 说明 |
| --- | --- |
| 不新建交易链 | 卖出继续 `createTSellDraft` / 既有 upcoming 流 |
| 不改持仓模型 | 摘要字段来自 snapshot / follow 行，前端展示 only |
| 账户隔离 | paper_sim 卖出行 ≠ 自选 follow；禁止混塞 |
| 价位线文案 | 保持「预警/观察」，避免被理解成委托 |

### 2.3 Portfolio 最小接线（设计建议，本阶段不编码）

打开 K 线时对齐 `stock.vue`：

- `:cost-price="row.avgCost"`
- `:cost-volume="row.totalQty"`
- `:position-aware-signals="true"`
- 可选 `#footer` 挂已有 `SellDraftDialog`（`canShowSellButton(row)`）

---

## 3. 交易标记评估

| 标记类型 | 现状 | 判定 |
| --- | --- | --- |
| 策略 / 冰点 Signal | `createSeriesMarkers` + 多标签样式（冰/买/强/趋/…） | **有** |
| 持仓「加」 | `positionAwareSignals` | **有** |
| T 买 / T 卖 | `chartMarkers.js` 框架（BUY/SELL →「T买/T卖」）；HoldingT `chartMarkersByCode` **常空** | **框架有、数据空** |
| 真实成交买/卖点 | Lightweight 主图 **无** fill→marker 映射 | **缺** |
| TradePlan 计划点 | 仅 StockLink 开图；图上无 plan/item 时间点 | **缺** |
| Provenance 联动 | 抽屉独立于图表 | **缺联动** |

### 3.1 设计原则（若后续做）

1. **只读叠加**：fill / plan 时间对齐到 bar → `createSeriesMarkers`；不改执行链、不写 mark。  
2. **分层开关**：信号层 / 成交层 / 计划层 / T 层，默认不全开，防糊。  
3. **语义隔离：**  
   - Signal = 策略观察  
   - Fill = 历史成交事实  
   - TradePlan = 计划意图（未必成交）  
   - 价位线 = 人工预警  
4. **先 fill，后 plan**：成交时间轴复盘价值更高；plan 点易与信号混淆。

### 3.2 不做 / 慎做

- 不在图上直接「一键下单」到真实券商  
- 不用 markers 改账户权益或今日盈亏口径（A.2 语义保持）  
- 不把 HoldingT SVG markers 与 Lightweight 混成两套真相

---

## 4. UI 优化评估

| 面 | 现状 | 评估 |
| --- | --- | --- |
| Logo 水印 | `App.vue` `n-watermark` **`content:''`（空）**；图表无品牌水印 | **无实质水印**；若要品牌/合规水印，应加 chart overlay，勿依赖空全局 watermark |
| 默认周期 | `initialKlt` 默认 **日 K (`101`)** | 研究合适；做 T/日内应允许入口传 `5`/`15` |
| 工具栏 | 周期 + 指标簇 + 策略 + 多单价位线，信息密度高 | 建议 **模式切换**：研究模式 / 持仓模式（折叠次要指标） |
| 加载状态 | `loading` / `loadingHistory` + `NSpin` + 文案 | **够用**；打开成本仍受 Modal 销毁重建影响（B.1-P2） |
| 插槽 | Modal prepend/append/footer | **已具备工作台扩展点**，优先用插槽而非改引擎 |
| 最大化 | Modal header 最大化 | **有** |

---

## 5. 能力矩阵（总表）

| 能力 | 有 / 部分 / 缺 |
| --- | --- |
| 多周期 Lightweight 图表 | **有** |
| Modal + StockLink 契约 | **有** |
| 东财数据 + 双层缓存 + LIVE 轮询 | **有** |
| 技术指标全套 | **有** |
| 策略/冰点 Signal markers | **有** |
| 成本/开仓/止损/止盈价位线 | **有** |
| 持仓感知加仓信号 | **有** |
| K 线旁 qty / 浮盈摘要 | **缺** |
| Portfolio 注入成本开 K | **部分**（能开图，未传 cost） |
| 图上盈亏填充区域 | **缺**（仅幅度 hint） |
| 做 T 主路径统一 | **部分**（旁路 SVG） |
| Sell Draft 挂在 K 线 | **部分**（链有，入口不在图） |
| Fill / TradePlan 图上标记 | **缺** |
| Logo 水印 | **缺**（全局空） |
| 研究/持仓工具栏模式 | **缺** |
| Modal keep-alive | **缺**（已知暂缓） |

---

## 6. 风险与边界

| ID | 风险 | 缓解 |
| --- | --- | --- |
| W1 | 价位线被当成真实委托 | 文案固定「预警」；footer 卖出与价位线分区 |
| W2 | paper 卖出行与自选持仓字段混用 | SellDraft 仅接受 paper_sim 行；自选只展示 |
| W3 | Signal + Fill + Plan markers 过载 | 分层开关；默认仅 Signal |
| W4 | HoldingT 与 Lightweight 双轨 | B.2-C 收敛到同一引擎 |
| W5 | Modal 频繁销毁重建 | 沿用 B.1-P2；工作台高频场景再评估 keep-alive |
| W6 | 扩大图表职责导致难测 | 薄壳组装；引擎 props 保持稳定 |

**禁止纳入 B.2 主切片：** 新建行情接口、改 schema、改执行/Fill 写入、第三层缓存、ECharts 回归主路径。

---

## 7. 建议设计阶段（冻结草案）

| 阶段 | 范围 | 目标 | 依赖 |
| --- | --- | --- | --- |
| **B.2-A 契约接线** | Portfolio/持仓开 K 补 `costPrice`/`costVolume`/`positionAwareSignals` | 持仓成本线与加仓信号开箱可用 | 无后端 |
| **B.2-B 工作台壳** | Modal 插槽：仓位摘要 + footer `SellDraftDialog` | 「研究 + 持仓操作」同屏 | A；复用卖出链 |
| **B.2-C 做 T 对齐** | HoldingT → Lightweight（或深链 Modal，`initialKlt=5`） | 统一图表 | B 可选并行 |
| **B.2-D 成交/计划 markers** | 只读 fill（优先）→ markers；provenance 联动 | 复盘增强 | 只读 API 已有即可 |
| **B.2-E UX** | 研究/持仓工具栏模式、默认 klt 策略、可选水印、keep-alive 评估 | 体验 | 非阻塞 |

**最小可交付（建议首编）：B.2-A + B.2-B。**

---

## 8. 目标形态（概念）

```text
┌─────────────────────────────────────────────────────────┐
│  股票名 / 代码     数据截至 …     [研究|持仓] 模式      │
├──────────────┬──────────────────────────────────────────┤
│ 仓位摘要      │  Lightweight K 线                         │
│ qty / 成本    │  成本线 · 止损止盈（预警）                │
│ 累计浮盈      │  Signal markers（可关）                   │
│ 今日浮盈      │  （可选）Fill markers                     │
│ [卖出][做T]   │  指标 / 周期工具栏（持仓模式可折叠）      │
└──────────────┴──────────────────────────────────────────┘
```

实现上优先 **插槽拼装**，避免把摘要/卖出写死进 `StockLightweightKlineChart` 核心。

---

## 9. 结论

1. **现有 K 线组件足以作为「股票研究 + 持仓操作工作台」底座。**  
2. B.2 的本质是 **组合与接线**，不是重做图表或交易链。  
3. 持仓模式：成本线/价位线/信号已具备；缺的是 **摘要 UI + SellDraft 挂载 + Portfolio props**。  
4. 交易标记：Signal 已强；Fill/TradePlan/T 标记为增量；须分层、只读。  
5. UI：空水印、默认日 K、工具栏过满、加载可用；用模式化工具栏 + 插槽即可推进。  

**下一步：** 确认采用 B.2-A/B 最小范围后，再开实现切片（编码阶段另立项）。

---

*本文只读；未修改任何代码。*
