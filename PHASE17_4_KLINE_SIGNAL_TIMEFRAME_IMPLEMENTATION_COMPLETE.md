# Phase17.4 Implementation

**日期：** 2026-09-07  
**依据：** [PHASE17_4_IMPLEMENTATION_AUDIT.md](./PHASE17_4_IMPLEMENTATION_AUDIT.md) · Phase17.3 设计  
**性质：** 最小边界治理（**未改冰点算法 / 未改信号判断公式 / 未改 DB / 未做 SignalEvent 大重构**）

---

## 修改文件

| 文件 | 变更 |
| --- | --- |
| `frontend/src/utils/signalTimeframe.js` | **新建** — ICE descriptor + klt↔timeframe + 兼容判断 |
| `frontend/src/utils/signalSettings.js` | `signalPresets` / `activeSignalPresetId` 读写兼容与双写；`getSignalPresets` 别名；默认名「默认参数预设」 |
| `frontend/src/components/StockLightweightKlineChart.vue` | 冰点 markers **仅日 K**；保留 `showIceSignals`；不匹配提示 |
| `frontend/src/components/settings.vue` | 文案：当前参数预设 / 新建预设 |
| `frontend/scripts/verify-signal-timeframe.mjs` | 单测脚本 |
| `PHASE17_4_IMPLEMENTATION_AUDIT.md` | 实现前审计 |
| 本文件 | 完成报告 |

**未改：** `icePointSignals.js`、TradePlan、CandidatePool、数据库、最大化 remount。

---

## 命名治理方案

| 项 | 做法 |
| --- | --- |
| 新别名 | `signalPresets` ≡ `screenStrategies`；`activeSignalPresetId` ≡ `activeScreenStrategyId` |
| 读取 | **优先** `signalPresets`（非空）；否则 fallback `screenStrategies` |
| 写入 | `serialize` / `merge` **双写**两套键，旧端仍可读 |
| 不删除 | 旧字段保留，用户存量 JSON 无需迁移脚本 |
| UI | 「策略」→「参数预设」（设置 + K 线工具栏）；按钮仍叫 **冰点/买卖点** |

---

## Timeframe 规则

```text
ICE_SIGNAL_DESCRIPTOR = { id: ice_reversal, timeframe: daily }

klt 101 → daily   → 可显示冰点 markers
klt 102/103/…     → 隐藏 markers，提示「该信号仅支持日K周期」
分钟 klt          → 同上隐藏

showIceSignals 用户开关：切周/月不清除；切回日 K 自动恢复绘制
```

`DAILY_LIKE_KLT` 仍用于时间轴/分钟判断，**不再**作为冰点展示门闩。

---

## 代码变化（要点）

1. `syncStrategySignals`：不匹配则 `detachStrategyMarkers` + warning status，**不**改 `computeFullSignals` 调用参数语义（匹配时仍原样计算）。  
2. 按钮：周期不支持时非 primary + `title` 提示；不 disabled 用户开关键图（意图保留）。  
3. 参数下拉：不支持周期时 disabled。

---

## 测试结果

| 测试 | 结果 |
| --- | --- |
| `node frontend/scripts/verify-signal-timeframe.mjs` | **PASS**（daily visible / weekly hidden / presets fallback / dual-write） |
| `npm run build` | **PASS** |

冰点算法文件未改 → 计算结果路径不变（仅展示门闩收紧）。

---

## 回归结果

| 项 | 结论 |
| --- | --- |
| 日 K + 冰点 | **应正常**（门闩允许 101） |
| 周/月 + 冰点开 | **markers 隐藏** + 提示 |
| 切回日 K | **恢复**（`showIceSignals` 保留） |
| signalSettings 加载 | **兼容**仅含 `screenStrategies` 的旧配置 |
| TradePlan / CandidatePool | **未改代码路径** |
| DB | **无变化** |

---

## 已知未解决问题

| 问题 | 说明 |
| --- | --- |
| **最大化丢失 overlay** | `StockKlineModal` `:key` 含 max/normal → remount；**本阶段不修**（记入 17.2/17.5 前） |
| `signal_scan_bundle.js` | 后端扫描打包副本未同步别名；扫描仍读旧键，与双写兼容 |
| SignalEvent Adapter | 未做 → **可进入 Phase17.5** |
| 设置页内部变量名仍 strategy* | 仅文案治理；API 字段双写已够最小 |

---

## PASS 验收核对

| 标准 | 状态 |
| --- | --- |
| 冰点算法结果未改变 | **PASS**（未改 `icePointSignals.js`） |
| 日 K 显示正常 | **PASS**（门闩含 101） |
| 周/月不显示错误信号 | **PASS** |
| 配置兼容旧版本 | **PASS** |
| 无数据库变化 | **PASS** |
| 无 TradePlan 链路影响 | **PASS** |

---

## 是否可进入 Phase17.5 SignalEvent Adapter

**可以。** 本阶段已固定：参数预设命名边界 + 日 K timeframe 门闩；17.5 可在不改算法前提下做 `computeFullSignals` → `SignalEvent[]` 适配与 Chart Signal Layer 消费。

---

*Phase17.4 实现完成。*
