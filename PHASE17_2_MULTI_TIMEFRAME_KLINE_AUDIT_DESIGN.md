# Phase17.2 Multi Timeframe KLine Audit

**日期：** 2026-09-07  
**文档：** `PHASE17_2_MULTI_TIMEFRAME_KLINE_AUDIT_DESIGN.md`  
**性质：** **只读架构审计 + 设计建议**（**未改代码 / 未新增字段 / 未重构 / 未修 bug**）  
**前置：** Phase17.1 KLine Freshness 已落地；本阶段目标是评估能否演进为「交易分析工作台」。

---

## 1. 当前架构

### 1.1 产品入口 → 公共组件

```text
自选 / 跟踪机会 / 机会 / 组合 / 首页 / 交易计划 / …
        │
        ▼
  StockKlineModal.vue          ← n-modal + 最大化；:key 含 max|normal
        │  v-bind attrs
        ▼
  StockLightweightKlineChart.vue  ← 多周期 / 指标 / 冰点信号 / 价位线 / poll
        │
        ├─ frontend klineCache.js (+ Phase17.1 日历失效)
        └─ Wails GetStockEastMoneyKLine / Page
              └─ MarketData → EastMoneyKLineApi → kline_cache (+ freshness)
```

旁路（非工作台主链）：`market.vue` 直嵌 Chart；`HoldingTPanel` SVG+同 API；旧 `KLineChart.vue`（ECharts + `GetStockKLine`）。

### 1.2 前端结构（职责）

| 层级 | 文件 | 职责 |
| --- | --- | --- |
| Modal 壳 | `StockKlineModal.vue` | `code` 契约、最大化布局、`chartMountKey`、插槽 prepend/footer（组合 B.2） |
| 图表核心 | `StockLightweightKlineChart.vue` | 周期 `INTERVALS`/`activeKlt`、指标、冰点标记、成本/多单价位线、LIVE poll |
| 信号算法 | `utils/icePointSignals.js` 等 | 冰/买/强/趋… **前端本地**基于 OHLC 计算 |
| 策略参数 | `utils/signalSettingsStore.js` + `signalSettings.js` | 全局 `ref` 配置（**非 Pinia**）；`screenStrategies` 下拉 |
| 加仓/冲减/成本过滤 | `addPositionSignals.js` / `rushReduceSignals.js` / `costAwareSellSignals.js` | 持仓上下文叠加 |
| 数据缓存 | `utils/klineCache.js` | 进程内存；日历 freshness（17.1） |
| 会话时钟 | `utils/aShareSessionClock.js` | LIVE/IDLE |

### 1.3 后端结构

| 层级 | 文件 | 职责 |
| --- | --- | --- |
| Wails | `app.go` `GetStockEastMoneyKLine*` | 统一入口 |
| Facade | `marketdata` + `EastMoneyKlineAdapter` | 无二层缓存 |
| 拉取/缓存 | `eastmoney_kline_api.go` + `kline_cache.go` | TTL + **17.1 交易日 freshness** |
| 扫描副作用 | `signal_scan_api.go` | 全市场拉日 K 写 cache（非图表正式刷新 API） |

**无独立「Signal 服务」给图表：** 图表上的冰点/买卖点 **不来自后端事件流**，而是 FE 对 bars 重算。

### 1.4 状态管理结论

| 状态 | 存放位置 | 共享范围 |
| --- | --- | --- |
| `activeKlt` / 指标开关 / `showIceSignals` | Chart **组件内 `ref`** | **仅当前实例**；最大化 remount 会丢 |
| `maximized` | Modal 内 `ref` | Modal 级 |
| `costPrice` / `costVolume` / `strategySignals` | **父页 props** | 打开时注入 |
| 策略参数 | `signalSettingsState`（模块级 ref） | 全局 |
| K 线 bars | Chart 内 `mergedRawRows` + FE/BE cache | 按 code+klt |

**无 Pinia Chart store；无跨最大化持久的 ChartSession。**

---

## 2. 数据流

### 2.1 K 线主数据

```text
用户选周期 (klt)
  → loadData / getOrFetch(FE)
  → GetStockEastMoneyKLine(code, name, klt, limit)
  → GetKLineDataBefore(latest)
       → freshness? → HTTP 东财 (+腾讯/新浪 fallback)
  → applySeriesFromRaw → lightweight-charts Candlestick + Volume
  → (可选) LIVE poll 60s（仅连续竞价）
```

### 2.2 冰点 / 买卖点

```text
showIceSignals=true 且 klt ∈ DAILY_LIKE {101,102,103,104,106}
  → computeFullSignals(OHLCV, signalOpts)     // 前端
  → createSeriesMarkers(candleSeries, markers)
  → 可选：成本过滤卖点 / 加仓「加」/ 冲减「冲」
分钟周期：提示「仅适用于日K、周K、月K…」并 detach markers
```

### 2.3 持仓 / 卖出（组合路径）

```text
PortfolioDashboard
  → 注入 costPrice/costVolume + prepend 持仓摘要
  → footer「卖出计划」→ SellDraftDialog → t-sell Draft
  （图表本身不读 TradePlan 成交点）
```

### 2.4 策略下拉

```text
signalSettings.screenStrategies[]
  → getScreenStrategyOptions()
  → selectedSignalStrategyId → getScreenSignalOptions(id)
  → 改变 iceThreshold 等参数后重算 markers
```

仍是 **同一套冰点算法 + 参数变体**，不是多套独立策略引擎挂载。

---

## 3. 当前能力

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| 多周期切换 | **有** | 1/5/15/30/60 分 + 日/周/月/季/年 |
| 日 K / 周 K / 分钟 | **有** | 同 API、不同 klt |
| 冰点/买卖点 | **有** | FE 计算；日类周期开启 |
| Lightweight Charts | **有** | `lightweight-charts` **^5.1.0**（非 Charting Library） |
| 最大化 | **有** | Modal 全屏；**会 remount Chart** |
| 成本线 / 现价线 | **有** | `createPriceLine`；组合注入成本 |
| 多单止损止盈线 | **有** | 工具栏「多单」标注 |
| B.2 持仓摘要 + 卖出 | **有** | Modal 插槽，非 Chart 内层 |
| Freshness（盘中追日） | **有** | Phase17.1 |
| TradePlan / 成交点上图 | **弱/无** | 计划在页外；图上无 plan_id 事件层 |
| 持仓做 T 工作台 | **弱** | HoldingT 旁路 SVG；主 Modal 未统一 T 计划点 |
| 后端 Signal 事件流 | **无** | 扫描结果进列表/快照，不直接驱动 Chart markers |

---

## 4. 已发现问题

### 4.1 TradingView Branding

| 项 | 结论 |
| --- | --- |
| 依赖 | **TradingView Lightweight Charts™**（开源轻量库），**不是**付费 Charting Library |
| 当前 `createChart` | `chartThemeOptions` **未显式**设置 `layout.attributionLogo` |
| 许可 | LICENSE / README：需标明 TradingView 为产品创作者；可用 `attributionLogo` 满足链接要求 |
| 商业化影响 | Logo/角标影响「自有交易工作台」观感；**完全抹掉需合规评估** |

**建议（不实现）：**

| 方案 | 说明 |
| --- | --- |
| **A. 继续使用并规范 branding** | **推荐近期**：显式 `attributionLogo: true`（或按官方要求展示链接），产品文案区分「图表引擎由 TradingView Lightweight Charts 驱动」 |
| B. 未来替换自研/其他图表层 | 中长期：若强品牌/强交互（画线订单、T 回合），再评估 ECharts/自研 canvas；成本高 |
| C. 其他 | 保留 LW Charts 作主图，自研仅 Overlay DOM 层（信号/订单气泡） |

→ **不建议**在未读许可条款前「静默隐藏」；优先 **A 合规展示 + 产品包装**。

### 4.2 周期与信号兼容（错配风险）

| 行为 | 现码 |
| --- | --- |
| 分钟 K + 冰点 | **自动不画 markers**，状态文案提示 |
| 日/周/月/季/年 + 冰点 | **仍计算并展示** |
| 算法假设 | `computeFullSignals` 按 **bar 序列**当「日级」逻辑（RSI 阈值、冰点间隔「日」等） |

**风险：** 周 K/月 K 上每根 bar≠一日，却套用日频参数 → **信号语义错配**（间隔、冰点密度失真），易被当成「正确买卖点」。

**尚无：** `Signal { strategy, timeframe, signal_time }` 契约；markers 不携带 timeframe 元数据。

### 4.3 最大化状态丢失

**根因（审计确认）：**

```86:88:frontend/src/components/StockKlineModal.vue
const chartMountKey = computed(
  () => `${props.chartKey}-${resolvedCode.value || 'nocode'}-${maximized.value ? 'max' : 'normal'}`,
)
```

```141:147:frontend/src/components/StockKlineModal.vue
    <stock-lightweight-kline-chart
      ...
      :key="chartMountKey"
```

最大化切换 → **销毁并重建** Chart → `onMounted` 仅从 props 恢复：

- `activeKlt = initialKlt || '101'`
- `showIceSignals = true` **仅当** `strategySignals` prop 为 true  
- 用户手动点的「冰点/买卖点」、非默认周期、指标组合、缩放 **全部丢失**

若观测「周期还在、信号没了」：常见于默认本就是日 K（看起来周期保留），但入口 **未** 传 `strategySignals`，手动开过冰点 → remount 后 `showIceSignals` 回到 false。

### 4.4 策略下拉扩展性

- 已有 `screenStrategies[]` + 下拉改参，但仍绑定 **同一 `computeFullSignals` /「冰点/买卖点」按钮**。
- UI 文案与实现强耦合「冰点」业务，不是通用 Strategy Signal Layer。
- 未来 MACD/均线/自定义策略需 **新计算器 + Overlay 注册表**，而非再堆 boolean。

### 4.5 Overlay 耦合

单组件内并行：

- K 线 + Volume  
- MA/BOLL/MACD/KDJ/RSI（indicator toggles）  
- 冰点 markers（策略）  
- 成本/现价/多单价位线（仓位）  
- 组合 Modal footer 卖出（交易，在壳层）

**无清晰 Layer 接口**；扩展 Trade/T 计划点易继续堆进 `StockLightweightKlineChart.vue`（已很大）。

### 4.6 其它

| 问题 | 说明 |
| --- | --- |
| HoldingT 双轨 | 观察台 SVG ≠ 主 Modal；做 T 产品叙事易分裂 |
| TradePlan 不上图 | 批准/成交与 K 线无统一事件时间轴 |
| FE/BE LIVE 日历 | FE 无完整节假日表（17.1 已部分缓解 freshness） |

---

## 5. 风险分析

| 风险 | 级别 | 说明 |
| --- | --- | --- |
| 周/月 K 展示日频冰点 | **高** | 误导交易决策 |
| 最大化丢 overlay | **中** | 工作台体验断裂 |
| 许可/品牌 | **中** | 商业化包装与 attribution 合规 |
| 巨型单文件 Chart | **中** | 后续 T/Trade 层难测难拆 |
| 复权/时区不一致 | **低–中** | 主链东财+上海格式化；分钟/日混用需保持 end 参数纪律 |
| Freshness 仅日历日 | **低** | 分钟实时仍靠 LIVE TTL；与「策略事件时间」正交 |
| 无后端 Signal 版本 | **中** | 回放/审计「当时信号」依赖 FE 重算，难与扫描快照严格对齐 |

### 5.1 多周期数据一致性

| 维度 | 现状 |
| --- | --- |
| 数据源 | 统一东财 `GetKLineDataBefore`（fallback 腾讯/新浪） |
| 复权 | 图表主路径多为空 `adjustFlag`；cache key **含** adjust |
| 时间戳 | 日：日期；分钟：时分；`timeVisible` 随周期切 |
| 时区 | 上海格式化 / 东财字段解析 |
| 缓存 | FE：code\|klt\|上海日；BE：secid+klt+adjust+end |
| Freshness | **latest** 日历门闩（17.1）；历史 end 不受影响 |

**风险点：** 周 K 信号错配；分钟与日切换时 markers 策略不一致（分钟关、日开）用户可能误解为「丢数据」；二次缓存（FE+BE）需继续靠 freshness/日历失效协同。

---

## 6. 多周期设计建议

1. **引入 `Signal Timeframe Contract`（逻辑契约，本阶段不落库）：**

```text
SignalEvent {
  strategy_id: string
  timeframe: "1m"|"5m"|"15m"|"30m"|"60m"|"1d"|"1w"|"1M"|...
  signal_time: ISO/日键   // 信号归属时间
  tag: "冰"|"买"|...
  source: "client_compute"|"scan_snapshot"|...
}
```

2. **交互推荐（兼容现网）：**

| 用户操作 | 推荐行为 |
| --- | --- |
| 日 K + 冰点策略 | 展示（现状） |
| 切换到 5/30 分 | **隐藏**该策略 markers；条带提示「当前策略仅支持日K」 |
| 切换到周/月 | **默认隐藏**日频冰点；或提供「按日信号投影到周 bar（实验）」显式开关，**默认关** |
| 切换回日 K | 恢复 Session 内策略开关状态 |

3. **禁止：** 在无 timeframe 元数据时，把日频算法结果画在任意周期上（周/月现状应视为 **债务**）。

---

## 7. Signal Layer 设计建议

目标：从「布尔 showIceSignals」升级为可插拔层。

```text
SignalLayer {
  id, strategy_id, timeframe_compat[]
  compute(bars, ctx) → SignalEvent[]
  render(chartApi, events) → dispose()
  visible: boolean
}
```

- 与 K 线 Layer 解耦：换周期时 `compatible(activeTimeframe)` 决定显示/清空。  
- 扫描快照 / 后端事件可后接 `source=scan_snapshot`，与客户端重算并存。  
- **不要求本阶段加 DB 字段**；先 FE 契约与 Session。

---

## 8. Strategy Layer 设计建议

从：

```text
ice_point = true / 「冰点/买卖点」按钮
```

到：

```text
StrategyDescriptor {
  id, name
  timeframe_compat: string[]
  signal_types: string[]      // 冰/买/强…
  params_schema / params      // 现有 screenStrategies 可迁入
  compute_ref: "ice_full_v1" | "macd_cross_v1" | …
}
```

**扩展路径：**

1. 保留现有冰点为 `ice_full_v1` 默认策略。  
2. 下拉只切换 `StrategyDescriptor`，按钮改为「信号」总开关 + 策略选择。  
3. 新策略 = 新 `compute_ref` + 注册，**禁止**在模板写死第二种业务按钮。

---

## 9. ChartSessionState 设计建议

解决最大化丢失与工作台连贯性（**先设计，后实现**）：

```text
ChartSessionState {
  stock_code: string
  timeframe: string          // klt 或规范 timeframe
  overlays: {
    indicators: { ma, boll, macd, ... }
    signal_layer_id: string | null
    signal_visible: boolean
    position_lines: { cost, entry, stop, take_profit }
  }
  strategy_id: string
  fullscreen: boolean
  zoom_range: { from, to } | null
}
```

**落地原则：**

- 状态升到 **Modal 或 provide/inject / 轻量 store**，Chart 变为受控展示。  
- **去掉** `chartMountKey` 对 `max|normal` 的依赖（或仅改高度 resize，不 remount）。  
- 最大化只改 layout，**不**重置 Session。

---

## 10. 持仓做 T 扩展建议

### 10.1 现有可复用

| 能力 | 位置 |
| --- | --- |
| 成本 / 现价线 | Chart price lines + props |
| 持仓数量 | 组合 prepend / `costVolume` |
| 卖出计划入口 | Modal footer → SellDraft / t-sell |
| 买/卖类标记 | 冰点体系（语义≠T 回合） |

### 10.2 缺口（需预留接口，本阶段不实现）

| 数据 | 用途 |
| --- | --- |
| `position_id` / `avg_cost` / `qty` / `available_qty` | Position Layer |
| `t_plan_points[]` {time, side, qty, plan_id} | T 计划/回合标记 |
| `fill_points[]` {time, price, side, plan_id} | Trade Layer 成交 |
| `current_quote` | 现价线持续刷新 |
| `session_mode: "observe"|"t_trade"|"analysis"` | 工作台模式 |

HoldingTPanel 应 **收敛进同一 Modal+Session**，避免第三套图。

---

## 11. 推荐实施顺序

| 序 | 阶段 | 内容 | 风险控制 |
| --- | --- | --- | --- |
| **0** | 合规小改（可选） | 明确 Lightweight Charts attribution 展示策略 | 不改交易逻辑 |
| **1** | ChartSessionState | 状态上移；最大化 **不 remount** | 专修 4.3；回归 B.2/Freshness |
| **2** | Signal Timeframe 门禁 | 周/月默认不展示日频冰点；分钟保持提示 | 专修错配；可开关 |
| **3** | StrategyDescriptor 适配 | 下拉/按钮去冰点硬编码，仍接 `ice_full_v1` | 无新策略也可做 |
| **4** | Overlay Layer 拆分（薄） | Signal/Position 渲染与 loadData 边界清晰 | 控制文件膨胀 |
| **5** | Trade / T 点上图 | 只读展示 plan/fill；接 SellDraft 已有链 | 不做自动下单 |
| **6** | （长期）图表引擎评估 | 仅当 Layer 稳定且品牌硬需求 | 对应 Branding-B |

**明确不做（本设计阶段）：** 改 API、加表字段、大重构、替换东财主源、重写冰点公式。

---

## 12. 审计合规确认

| 要求 | 状态 |
| --- | --- |
| 未修改任何代码 | **是** |
| 未新增数据库字段 | **是** |
| 不影响现有运行 | **是**（只读文档） |
| 问题均为审计发现 | **是** |
| 含未来实施路线 | **是**（§11） |

---

## 附录 A — 涉及文件列表

### 前端（核心）

- `frontend/src/components/StockKlineModal.vue`
- `frontend/src/components/StockLightweightKlineChart.vue`
- `frontend/src/utils/klineCache.js`
- `frontend/src/utils/aShareSessionClock.js`
- `frontend/src/utils/icePointSignals.js`
- `frontend/src/utils/signalSettingsStore.js`
- `frontend/src/utils/signalSettings.js`
- `frontend/src/utils/addPositionSignals.js`
- `frontend/src/utils/rushReduceSignals.js`
- `frontend/src/utils/costAwareSellSignals.js`
- `frontend/src/components/PortfolioDashboard.vue`（B.2 注入/footer）
- `frontend/src/components/HoldingTPanel.vue`（旁路）
- `frontend/src/components/market.vue` / `KLineChart.vue`（旁路）
- `frontend/package.json`（`lightweight-charts`）

### 后端（核心）

- `app.go`（`GetStockEastMoneyKLine*`）
- `backend/marketdata/**` + `adapter/eastmoney_kline_adapter.go`
- `backend/data/eastmoney_kline_api.go`
- `backend/data/kline_cache.go`（含 17.1 freshness）
- `backend/data/signal_scan_api.go`（扫描写 cache）

### 关联文档

- `PHASE17_1_KLINE_ARCHITECTURE_AUDIT.md`
- `PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md`
- `PHASE17_1_KLINE_FRESHNESS_FIX_IMPLEMENTATION_COMPLETE.md`

---

## 附录 B — 架构结论（一句话）

**多周期 K 线已形成统一 Modal+Lightweight Charts+东财缓存主链，具备工作台雏形；但 Session 未上提、信号缺 timeframe 契约、策略 UI 绑死冰点、最大化 remount 丢状态，尚不足以称为完整「交易分析工作台」——应按 §11 先 Session 与信号门禁，再 Layer/T 扩展。**

---

*Phase17.2 Multi Timeframe KLine 只读审计设计结束。*
