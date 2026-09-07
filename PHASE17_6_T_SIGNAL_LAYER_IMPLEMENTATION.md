# PHASE17.6 Holding T Signal Layer — Implementation Report

**日期：** 2026-09-08  
**性质：** 观察信号层 v0 实现（只读）  
**上游审计：** [PHASE17_6_T_SIGNAL_LAYER_AUDIT.md](./PHASE17_6_T_SIGNAL_LAYER_AUDIT.md)  

**约束遵守：**

- ✅ 只读评价 / 观察层  
- ✅ 不自动交易  
- ✅ 不修改 Broker / Settlement  
- ✅ 不生成 TradePlan  
- ✅ 不影响 ExitEval state（未改 `exit_evaluation.go` 管道）  
- ✅ 不改主 K 线冰点 / `StockLightweightKlineChart` 信号系统  

---

## 1. 交付物

| 文件 | 作用 |
| --- | --- |
| `backend/papertrading/holding_t_signal.go` | `HoldingTSignal` DTO + `BuildHoldingTSignal` v0 |
| `backend/papertrading/holding_t_signal_test.go` | Go 单测（正常 / 不可卖 / 过期 / 无5m / 空仓） |
| `frontend/src/utils/holdingTSignal.js` | 前端镜像规则 + ChartMarker 映射 |
| `frontend/src/utils/holdingTSignalDisplay.js` | 中文标签 |
| `frontend/src/utils/holdingTSignal.test.mjs` | 前端规则冒烟 |
| `frontend/src/components/HoldingTPanel.vue` | T买/T卖观察展示 + SVG markers |

---

## 2. DTO

```text
HoldingTSignal
  stock_code
  position_id
  signal_type   ∈ { T_BUY_WATCH, T_SELL_WATCH }
  signal_time
  signal_price
  level         ∈ { active, soft }
  reasons[]
  confidence    ∈ [0,1]
```

另：`HoldingTSignalResult{ signals, notes }` — 门闩失败时 `notes` 解释为何无信号。

---

## 3. BuildHoldingTSignal v0 规则

**硬门闩（直接无信号）：** 无持仓 · 行情 `STALE` · 5m bars &lt; 6 · 成本价无效。  

**T_BUY_WATCH**（需 `can_sell`）：至少命中 2 项，且未大幅远离成本上方  

- 短线回撤（相对窗口高点）  
- 反弹/恢复迹象  
- 接近成本区域  

**T_SELL_WATCH**（需持仓）：至少命中 2 项  

- 5m 上涨  
- 偏离成本上方  
- 短线动能减弱  

Suitability / Health 仅降置信或附加 reason，**不改 ExitEval**。

数据来源（设计）：HoldingTSuitability / PositionState / HealthScore / 5m / 成本 — **Go 输入结构已预留**。  
HoldingTPanel v0：**Follow 仓 + 5m K**；`freshness` 默认 FRESH；suitability/health 未接线（后续可 enrich）。

---

## 4. UI

- 「T 操作观察区」：T买观察 / T卖观察 · level · 置信 · 原因说明  
- 5m SVG：复用 `chartMarkers.js`（`BUY`/`SELL` → T买/T卖）；`<title>` 含 reason  
- **未**改 Lightweight 主图冰点系统  

---

## 5. 测试

| 场景 | Go | FE |
| --- | --- | --- |
| 正常持仓 → BUY/SELL watch | ✅ | ✅ |
| 不可卖 → 无 BUY | ✅ | ✅ |
| 行情过期 STALE | ✅ | ✅ |
| 无 5m 数据 | ✅ | ✅ |
| 空 position | ✅ | ✅ |

```text
go test ./backend/papertrading/ -run BuildHoldingTSignal   → ok
node frontend/src/utils/holdingTSignal.test.mjs            → ok
```

---

## 6. 已知限制（v0）

1. HoldingTPanel 仍用 **Follow** 成本仓，未强制 paper_sim PositionState。  
2. Panel 侧未接 C2 freshness / C3 Health / 17.1 Suitability 实时 enrich。  
3. 未挂 ExitEval / 组合「做 T」Drawer（刻意隔离）。  
4. `position_id` 在 Follow 列表常为空。  

---

## 7. 停止

Phase17.6 Holding T Signal Layer v0 **实现完成**。按指令停止。

---

*报告：`PHASE17_6_T_SIGNAL_LAYER_IMPLEMENTATION.md`*
