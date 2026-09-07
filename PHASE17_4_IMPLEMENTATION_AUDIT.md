# PHASE17.4 Implementation Audit（实现前）

**日期：** 2026-09-07  
**性质：** 只读范围确认（随后进入最小实现）

---

## 1. Signal 配置入口

| 入口 | 文件 | 说明 |
| --- | --- | --- |
| 设置「K线信号参数」 | `settings.vue` + `SignalSettingsPanel.vue` | 读写 `screenStrategies` / `activeScreenStrategyId` |
| 运行时 Store | `signalSettingsStore.js` | `parseSignalParams` ← `GetConfig().signalParams` |
| 选股页策略选择 | `allStockList.vue` | `getScreenStrategies` / 快照携带 params |
| K 线工具栏下拉 | `StockLightweightKlineChart.vue` | `getScreenStrategyOptions()` |

持久化：后端 `settings.SignalParams` JSON（**不改表**）。

## 2. `screenStrategies` 引用

- `frontend/src/utils/signalSettings.js`（定义 / merge / serialize）
- `signalSettingsStore.js`、`settings.vue`、`allStockList.vue`、`StockLightweightKlineChart.vue`
- 打包副本：`backend/data/signal_scan_bundle.js`（扫描用；本阶段**不强制改**，避免扩大面；FE 配置双写即可）

## 3. 冰点计算入口

- `icePointSignals.computeFullSignals` ← Chart `syncStrategySignals`
- **本阶段不改**该文件算法

## 4. K 线周期字段

- Chart 内 `activeKlt`（东财：`101` 日 / `102` 周 / `103` 月 / 分钟…）
- `DAILY_LIKE_KLT = {101,102,103,104,106}` — 当前冰点门闩过宽（周月也会算）

## 5. Marker 生成

- `syncStrategySignals` → `createSeriesMarkers`
- 不匹配周期时应 `detachStrategyMarkers`，保留 `showIceSignals` 用户开关

## 6. 最小修改范围

| 改 | 不改 |
| --- | --- |
| `signalSettings.js`：`signalPresets` 读写兼容 + 双写 | `icePointSignals.js` 公式 |
| 新建小模块 timeframe 契约（或同文件导出） | DB / SignalEvent 大重构 |
| Chart：日 K 才显示冰点 markers + UI 提示 | 最大化 remount（只记录） |
| 设置/图表文案：策略→参数预设（轻量） | TradePlan / CandidatePool |
| 单测脚本 | `signal_scan_bundle.js` 全量同步（可选后续） |

---

*审计结束，进入实现。*
