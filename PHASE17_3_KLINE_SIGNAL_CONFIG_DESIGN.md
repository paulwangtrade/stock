# Phase17.3 KLine Signal Config Design

**日期：** 2026-09-07  
**文档：** `PHASE17_3_KLINE_SIGNAL_CONFIG_DESIGN.md`  
**性质：** **只读架构审计 + 配置体系设计**（**未改代码 / 未新增 DB 字段 / 未实现功能 / 未改冰点算法**）  
**前置：** Phase17.1 Freshness · Phase17.2 多周期 K 线架构审计

---

## 1. 当前 Signal 架构

### 1.1 总览

```text
设置页 SignalSettingsPanel / screenStrategies
        │ serializeSignalParams → GetConfig.SignalParams (JSON 字符串)
        ▼
signalSettingsStore (模块级 ref)
        │ buildSignalOptions / getScreenSignalOptions(strategyId)
        ▼
StockLightweightKlineChart
        │ showIceSignals 开关 + selectedSignalStrategyId
        ▼
icePointSignals.computeFullSignals(bars, options)
        │ { iceEnter, buy, strictBuy, trendBuy, … sell… }
        ▼
createSeriesMarkers → Overlay markers
```

**并行消费方（同一算法）：** 选股扫描 / 自选信号卡 / 策略回测 / 量化提醒 — 多经 `summarizeBuySignal` / `computeFullSignals` + 同一套 `signalParams`。

### 1.2 分层现状

| 层 | 实现 | 角色 |
| --- | --- | --- |
| 配置 | `signalSettings.js` + Store + 设置页 | 参数包（已称「策略」） |
| 计算 | `icePointSignals.js` | **单体 Provider**：冰+买+强+趋+转+突+弹+止+减 |
| 展示 | `StockLightweightKlineChart.vue` | 布尔开关 + markers |
| Modal | `StockKlineModal.vue` | 透传 props；无信号语义 |
| 持久化 | 后端 `settings.SignalParams`（JSON text） | **非**独立 signal 表 |

**结论：** 已有「可配置参数」，但 **不是** 可插拔多 Signal Provider 体系；命名上的「选股策略」≈ **同一冰点族参数变体**。

---

## 2. 冰点信号现状分析

### 2.1 `icePointSignals.js`

| 项 | 内容 |
| --- | --- |
| 核心入口 | `computeTradeSignals`（冰/买/止/减基底）→ `computeFullSignals`（强/趋/转/突/弹等） |
| 输入 | `{ closes, opens, highs, lows, volumes, dayKeys, indexMa20ByDay }` + `options` |
| 输出 | 索引数组集合（`iceEnter`、`buy`、`strictBuy`…）+ MA/RSI 等辅助；再由 Chart 转 markers |
| 是否绑死日 K | **算法按 bar 序列**；**不声明 timeframe**。Chart 用 `DAILY_LIKE_KLT` 在 日/周/月/季/年 上启用 |
| 硬编码 | 默认阈值在函数默认参数与 `DEFAULT_SIGNAL_SETTINGS`；**运行参数可被 options 覆盖** |
| 与页面耦合 | Chart 内写死「冰点/买卖点」按钮与文案；计算模块本身可被扫描/回测复用 |

### 2.2 配置能力（已有，易被低估）

`signalSettings.js` 已包含大量可调项，例如：

- `common.iceThreshold` / `rsiPeriod` / `maPeriod` / `requireIndexBull`
- `strong.*` / `trend.*` / `breakout.*` / `reversal.*` / `rebound.*` / `sell.*`
- `screenStrategies[]`：多套参数 + `activeScreenStrategyId`
- `buildSignalOptions` → 传入 `computeFullSignals`

设置页已有卡片文案 **「K线信号参数」**（`settings.vue`），经 `SignalSettingsPanel` 编辑并随配置保存。

### 2.3 图表绑定

```text
strategySignals prop → onMounted 默认 showIceSignals=true
用户按钮 toggleIceSignals
activeKlt ∈ {101,102,103,104,106} → syncStrategySignals
分钟 klt → 提示并 detach（不画）
周/月 → 仍跑日频语义算法（Phase17.2 已标错配风险）
```

---

## 3. 当前问题

| # | 问题 | 说明 |
| --- | --- | --- |
| 1 | **Provider 未抽象** | 冰点族 = 唯一实现；无法注册「均线交叉」「MACD」为平行 Provider |
| 2 | **Signal ≠ 配置「策略」命名混乱** | UI「当前策略」实为参数包；与交易 Strategy→TradePlan 不同层 |
| 3 | **展示耦合** | Chart 按钮/文案/marker 样式绑死冰点业务 |
| 4 | **无统一 SignalEvent** | 输出是 barIndex 集合，缺 `timeframe/timestamp/type/strength` |
| 5 | **周期契约缺失** | 日频逻辑画在周/月 bar 上 |
| 6 | **多信号并行弱** | 一套 `computeFullSignals` 内多 tag，不是多 Provider 开关矩阵 |
| 7 | **Session 丢失** | 最大化 remount 可丢 `showIceSignals`（17.2） |
| 8 | **配置面已强、产品叙事弱** | 用户以为「页面硬编码」，实则参数在设置里，但 K 线未呈现「Signal 系统」心智 |

---

## 4. Signal Provider 设计

### 4.1 冰点定位（必答）

| 选项 | 是否 |
| --- | --- |
| A. K 线功能 | 否（K 线是数据/渲染底座） |
| B. 指标 | 部分像（用 RSI/MA），但产出是 **离散事件** 非连续指标线 |
| **C. Signal Provider** | **是（推荐）** |
| D. 策略 | 否（策略应消费多 Signal 做决策） |

**为什么选 C：**

1. 冰点/买卖点是对市场状态的 **离散标注**，符合 Signal 语义。  
2. 已可配置、可扫描、可回放，天然是 Provider 输出。  
3. 与 MACD 柱/均线 **指标层** 分离后，Chart 只订阅 `SignalEvent[]`。  
4. 交易 Strategy / 持仓做 T 可组合多个 Provider，而不把冰点升格为唯一「策略」。

```text
KLine Layer (bars)
    ↓
Signal Providers (ice_reversal_v1, ma_cross_v1, …)
    ↓ SignalEvent[]
Overlay / Scan / Research / (未来) Strategy
```

### 4.2 Provider 接口（设计，不实现）

```text
SignalProvider {
  id: "ice_full_v1"           // 现 computeFullSignals 封装
  name: "冰点反转族"
  timeframe_compat: ["1d"]    // 默认仅日；周月需显式投影策略
  param_schema: …             // 映射现有 common/strong/trend/…
  compute(bars, params, ctx) → SignalEvent[]
}
```

现网迁移：**不改算法**，仅把 `computeFullSignals` 包成 `ice_full_v1` Provider。

---

## 5. SignalConfig 模型建议

面向「设置 → K线信号参数」列表，**逻辑模型**（可先仍序列化进现有 `SignalParams` JSON，**不新增表字段**）：

```text
SignalConfig {
  id: string                 // e.g. ice_full_v1
  name: string               // 冰点反转信号
  enabled: boolean           // 全局/工作台默认是否参与
  timeframe: string | string[]  // "1d" | ["1d","1w"]
  parameters: object         // 现 screenStrategy.settings 子集或全量
  description: string
  provider_ref: string       // 计算实现 id
  ui: { group, order }
}
```

**与现网映射：**

| 现网 | 未来 |
| --- | --- |
| `screenStrategies[i]` | 可演进为 **同一 Provider 的多套 parameters 预设**（Profile），或拆成多 SignalConfig |
| `activeScreenStrategyId` | 「当前用于扫描/图表的参数 Profile」 |
| 单一冰点开关 | `SignalConfig.enabled` + Chart Session overlays |

**适合扩展：** 突破 / 趋势 / MACD / 自定义 = 新 `provider_ref` + 新 `parameters`，列表行增加即可。

**注意：** 短期勿把「冰点内部的强/趋/突」拆成 10 个独立 Provider（成本高）；可先 **一个 ice_full_v1 + tag 过滤**，再逐步拆。

---

## 6. SignalEvent 模型建议

```text
SignalEvent {
  signal_id: string          // provider 或具体规则 id
  symbol: string
  timeframe: string          // 信号所属周期，非当前图表周期
  timestamp: string          // 日键或 ISO；对齐 bar time
  type: string               // 冰|买|强|趋|… 或枚举 code
  strength: number | null    // 可选评分
  metadata: object           // rsi, ma20, reason, bar_index, …
}
```

**可支撑：**

| 用途 | 方式 |
| --- | --- |
| K 线 Overlay | `timeframe` 匹配当前图 → markers |
| 回测 | 事件序列 + 价格路径 |
| TradePlan 关联 | metadata / 后续 link `plan_id`（实现阶段再定，本设计不加库字段） |
| Alpha 研究 | 统一导出事件流 |

由 `computeFullSignals` 的 index 列表 **适配器映射** 为 Event，算法本体不动。

---

## 7. Signal Timeframe Contract 设计

```text
SignalConfig.timeframe / SignalEvent.timeframe
        ×
ChartSession.timeframe (klt→规范名)
        →
compatible ? render : filter(+可选提示)
```

### 推荐规则

| 信号声明 | 图表周期 | 行为 |
| --- | --- | --- |
| `timeframe: "1d"`（冰点默认） | 日 K | 展示 |
| 同上 | 5/30 分 | **过滤**；提示「仅日K」 |
| 同上 | 周/月 | **默认过滤**；可选「投影到周 bar」（实验，默认关） |
| 未来 `["1d","1w"]` | 周 K | 仅当 Provider **按周 bar 计算** 或显式投影开启 |

**禁止：** 无声明时默认「所有 DAILY_LIKE 都算日频逻辑」（修正 17.2 债务的产品规则）。

---

## 8. K 线展示层关系

```text
StockKlineModal
  └─ ChartSessionState（17.2 建议）
        ├─ timeframe
        ├─ overlays.signal_ids[] / visible
        └─ StockLightweightKlineChart（受控）
              ├─ KLine Layer: bars
              ├─ Indicator Layer: MA/MACD…（非 Signal）
              ├─ Signal Layer: 订阅 SignalEvent[] → markers
              └─ Position Layer: 成本/现价线
```

**原则：** Chart **不 import 冰点业务名** 作为唯一路径；只认 Provider 注册表 + Event。  
冰点按钮 →「信号」总开关或「启用的 SignalConfig 列表」。

---

## 9. 设置页面设计

### 9.1 现状

设置已有 **「K线信号参数」** + 多「选股策略」参数包 + `SignalSettingsPanel` 细则。  
问题是叙事像「选股策略阈值」，而非「可插拔 K 线 Signal」。

### 9.2 建议信息架构（不实现）

```text
设置
 ├── 数据参数
 ├── 风控参数
 ├── K线信号参数          ← 强化为 Signal 中心
 │     ├── Signal 列表（表格）
 │     └── 参数编辑（抽屉/子页）
 └── …
```

**列表示意：**

| 信号 | 开启 | 周期 | 参数 |
| --- | --- | --- | --- |
| 冰点反转族 (`ice_full_v1`) | 开 | 日K | 配置 → 现有 Panel |
| 趋势突破（未来） | 关 | 日K/周K | 配置 |
| 均线信号（未来） | 关 | … | … |
| MACD（未来） | 关 | … | … |

- 「配置」进入现有分组表单（common/buy/strong/…），**算法不变**。  
- 原「新建策略」可降级文案为 **「参数预设 / Profile」**，避免与交易 Strategy 混淆。

---

## 10. Strategy 关系设计

**严格分层，避免混淆：**

```text
Signal (Provider)
  发现/标注市场状态 → SignalEvent[]

Strategy (决策)
  输入: 多 SignalEvent + 持仓/风控/偏好
  输出: 意图（加仓/减仓/做T/观望）

TradePlan
  可执行计划（草稿→批准→冻结→成交）
```

| 现网叫法 | 应归层 |
| --- | --- |
| screenStrategies / K线信号参数 | **Signal 参数 Profile** |
| 选股扫描快照 | Signal 批量产出 / 研究 |
| 交易计划 / t-sell | **TradePlan** |
| 未来组合决策引擎 | **Strategy** |

Signal **不**直接下单；Strategy **不**替代 K 线 Overlay 计算（可订阅同一 Event）。

---

## 11. 持仓做 T 扩展分析

做 T 可能需要的信号类型：超跌/冰点、趋势反转、放量突破、短线强弱等。

| 需求 | 当前是否支持 |
| --- | --- |
| 冰点/反转类标记 | **有**（ice 族） |
| 突破类 | **有**（突，同族内） |
| 成本/现价/可卖 | **有**（Position 层 / SellDraft） |
| T 回合事件点 | **无**统一 Event |
| 多 Provider 并行（冰点+分钟突破） | **弱** |
| 周期正确的短线 Signal | **弱**（分钟关掉冰点，无替代 Provider） |

**结论：** 配置体系按 Provider 扩展后，做 T 可「订阅」多个 SignalConfig；**不需要**把做 T 写进冰点算法。T 计划点属 Trade/Position Overlay，与 Signal 并列。

---

## 12. 推荐实施顺序

| 序 | 内容 | 约束 |
| --- | --- | --- |
| **1** | 文档/命名治理：设置「策略」→「信号参数预设」；明确 Signal≠Strategy | 不改算法 |
| **2** | Timeframe Contract：冰点默认仅日 K；周/月过滤（可配置实验投影） | 不改计算公式 |
| **3** | `computeFullSignals` → Event 适配器 + Chart Signal Layer 只吃 Event | 算法黑盒包裹 |
| **4** | ChartSession + 最大化不丢信号开关（衔接 17.2） | — |
| **5** | SignalConfig 列表 UI（仍写入现有 `SignalParams` JSON） | **不新增表字段** |
| **6** | 注册第二 Provider（如均线）验证插件化 | 新文件，不动冰点源码逻辑 |
| **7** | Strategy 决策层（若产品需要）消费 Event → TradePlan | 另阶段 |

**本阶段明确禁止：** 改 `icePointSignals` 公式、加 DB 列、实现配置页重构。

---

## 13. 审计合规确认

| 要求 | 状态 |
| --- | --- |
| 未修改代码 | **是** |
| 未新增字段 | **是** |
| 未改变现有运行 | **是** |
| 冰点算法保持现状 | **是**（仅设计包裹） |
| 只完成架构设计 | **是** |

---

## 附录 — 涉及文件列表

| 文件 | 角色 |
| --- | --- |
| `frontend/src/utils/icePointSignals.js` | 冰点族计算 |
| `frontend/src/utils/signalSettings.js` | 默认参数 / merge / buildSignalOptions / screenStrategies |
| `frontend/src/utils/signalSettingsStore.js` | 运行时配置 ref + GetConfig 同步 |
| `frontend/src/components/SignalSettingsPanel.vue` | 参数表单 UI |
| `frontend/src/components/settings.vue` | 「K线信号参数」卡片与保存 |
| `frontend/src/components/StockLightweightKlineChart.vue` | 开关 / markers / 周期门闩 |
| `frontend/src/components/StockKlineModal.vue` | Modal 壳 |
| `backend/data/settings_api.go` | `SignalParams` JSON 持久化 |
| `backend/models/signal_scan_snapshot.go` | 扫描快照携带 params 副本 |
| 关联：`addPositionSignals.js` / `costAwareSellSignals.js` / `watchlistSignalScan.js` | 同源消费 |

---

## 一句话结论

**当前并非「纯硬编码冰点」——设置侧已有丰富 `signalParams`；真正缺口是 Provider/Event/Timeframe 契约与产品分层。下一步应把冰点定为 `ice_full_v1` Signal Provider，统一 SignalEvent，收紧周期，再扩展列表化配置与其它 Provider，且不改动现有冰点计算公式。**

---

*Phase17.3 KLine Signal Config 只读设计结束。*
