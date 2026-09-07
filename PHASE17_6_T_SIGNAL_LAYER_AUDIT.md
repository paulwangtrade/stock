# PHASE17.6 Holding T Signal Layer — 前置审计

**日期：** 2026-09-08  
**性质：** **只读审计**（不改代码 / 不实现 / 不建表）  
**目标：** 为「持仓做 T 观察信号层」冻结能力边界与数据缺口，支撑后续 `HoldingTSignal` 设计。  

**上游：**  
- [PHASE17_1_T_SUITABILITY_DESIGN.md](./PHASE17_1_T_SUITABILITY_DESIGN.md) / Implementation  
- [PHASE17_C5_T_SUITABILITY_AUDIT.md](./PHASE17_C5_T_SUITABILITY_AUDIT.md)  
- [PHASE17_FINAL_ACCEPTANCE_REPORT.md](./PHASE17_FINAL_ACCEPTANCE_REPORT.md)  
- [PHASE17_3_HOLDING_EVALUATION_SNAPSHOT_DESIGN.md](./PHASE17_3_HOLDING_EVALUATION_SNAPSHOT_DESIGN.md)（`position_id` 缺口）  

**本阶段交付：** 本文档。完成后 **停止**。

---

## 0. 一句话结论

| 问题 | 结论 |
| --- | --- |
| Lightweight 主图能否画 Marker / Overlay / 信号？ | **能**（冰点 markers + 价位线 + 指标叠加）；**未**接做 T 观察信号 |
| HoldingT 观察台？ | **独立 SVG** + 空 `ChartMarker` 占位；文案「等待信号模型」 |
| 现有数据能否支撑 `T_BUY_WATCH` / `T_SELL_WATCH`？ | **原料部分够，产品规则未建** — 可做启发式观察；不可当交易指令 |
| 本层定位 | Holding Intelligence **观察信号**；**禁止**自动交易 / Broker / Settlement / TradePlan |

---

## 1. 审计：K 线能力（`StockLightweightKlineChart`）

### 1.1 Marker

| 能力 | 现状 | 与做 T 关系 |
| --- | --- | --- |
| `createSeriesMarkers` | ✅ 主图蜡烛系列挂插件 | 可复用渲染通道 |
| 冰点 / 买卖点 tags | ✅ `computeFullSignals` → markers | **日 K 门闩**（Phase17.4 `signalTimeframe`）；**非**做 T 信号 |
| `focusSignalTag` / 列表对齐 | ✅ | 机会列表高亮，非 T_WATCH |
| `positionAwareSignals` | ✅ 有持仓时在强/趋/突上叠「加」 | 自选加仓语义，**≠** `T_BUY_WATCH` |
| 外部注入任意 `HoldingTSignal[]` | ❌ **无**正式 props/API | 做 T 层若用主图，需后续最小接线（本审计不实现） |

### 1.2 Overlay

| 能力 | 现状 |
| --- | --- |
| 指标叠加 | ✅ MA / BOLL / MACD / KDJ / RSI / OBV 等（副图/主图系列） |
| 价位线 `createPriceLine` | ✅ 成本 / 开仓 / 止损 / 止盈 / MA20 支撑 |
| `costPrice` / `costVolume` props | ✅ 组合页可传入持仓成本 |
| 做 T 专用 overlay（如成本带、T 区） | ❌ 无独立 T overlay 模型 |

### 1.3 Signal 事件

| 形态 | 现状 |
| --- | --- |
| 日 K 冰点 SignalEvent（扫描 Snapshot 域） | 研究链已有；**默认日 K** |
| 主图内实时 `SignalEvent` 表 / 持久化 | ❌ 无；markers 为计算态 |
| 分钟级做 T SignalEvent | ❌ **不存在** |
| HoldingT 占位 `ChartMarker`（`BUY`/`SELL` + `reason`） | ✅ 类型在 `chartMarkers.js`；**运行时 markers 为空** |

### 1.4 双通道事实（重要）

```text
StockLightweightKlineChart   ← 产品主图（Lightweight Charts）
  Markers: 冰点/策略（日K）
  Overlay: 指标 + 成本价位线
  做 T 信号: 未接入

HoldingTPanel                ← 「持仓做T」菜单观察台
  5m SVG 折线（非 Lightweight）
  ChartMarker 框架已预留 ↑T买 / ↓T卖
  文案: 「等待信号模型接入」
```

**裁定：** 观察信号层 MVP **优先挂 HoldingTPanel + `chartMarkers.js`**（已预留）；主图 Lightweight 为增强通道，非前置阻塞。

---

## 2. 审计：数据是否够支撑 `T_BUY_WATCH` / `T_SELL_WATCH`

### 2.1 数据清单

| 数据 | 可用性 | 落点 | 对 T_WATCH 的作用 |
| --- | --- | --- | --- |
| **5m K 线** | ✅ | `GetStockEastMoneyKLine` `klt=5`；HoldingT 拉 120 根 + cache TTL 5min；主图可选分钟周期 | 价位/振幅/局部高低；**信号时刻与价**的候选锚 |
| **quote freshness** | ✅ | C2 `EvaluationDataFreshness`；C5/17.1 TSuitability `Freshness`；17.2 `price_freshness` | **门闩**：STALE → 不宜发高置信观察信号 |
| **HoldingTSuitability** | ✅（工作区；tag 可能未齐） | `suitable` / `caution` / `unsuitable` + Reasons | **环境门闩**：unsuitable 时应抑制或降级 T_WATCH |
| **HealthScore** | ✅ | grade A–D + factors | **软上下文**（D → 降置信 / 仅 caution 文案）；非方向真源 |
| **成本价** | ✅ | paper_sim `avg_cost`；Explanation `cost_price`；HoldingT Follow `costPrice`；主图 `costPrice` prop | 相对成本的「回踩 / 冲高」启发式输入 |

### 2.2 缺口（相对完整信号产品）

| 缺口 | 影响 |
| --- | --- |
| **无** `T_BUY_WATCH` / `T_SELL_WATCH` 规则引擎 | 不能从现有 API「直接取出」方向观察信号 |
| **`position_id` 未进评价 DTO** | 17.3 已记；identity 暂用 `stock_code`（+ account）可运行，正式字段需 lookup |
| HoldingT **Follow 成本仓** vs 组合 **paper_sim** | C5 已警告数据源分裂；信号层应对齐 paper_sim，否则观察与组合适宜性不一致 |
| 5m **未**统一 freshness 枚举进 HoldingTPanel | 有 cache TTL；与 C2 FRESH/STALE 未接线 |
| TSuitability 的 5m 振幅 | 后端可算，但 Exit 富集路径 **常缺** bars → `UNKNOWN` → caution |
| `ChartMarker.type` 仅为 `BUY`/`SELL` | 与拟议 `T_BUY_WATCH`/`T_SELL_WATCH` 需映射，避免与真实下单语义混淆 |
| 冰点 SignalEvent / Snapshot | **日 K 研究域**；不宜直接冒充分钟做 T 信号 |

### 2.3 充分性裁定

| 目标 | 裁定 |
| --- | --- |
| 支撑 **观察向** `T_BUY_WATCH` / `T_SELL_WATCH` MVP（启发式 + 门闩） | **条件充足** |
| 支撑 **可交易** 买卖信号 / 自动单 | **不充足且禁止** |
| 仅靠 HealthScore / TSuitability **单独**产出方向 | **不充足** — 二者是适宜性/质量，不是方向 |

**建议输入组合（设计冻结，非实现）：**

```text
IF TSuitability.level ∈ {suitable, caution}
AND freshness ≠ STALE
AND 5m bars 足够
THEN 允许计算 HoldingTSignal（相对 cost + 5m 结构）
ELSE 不产出 或 confidence 极低 + reason=门闩码
```

---

## 3. 设计草案：`HoldingTSignal`（本阶段只定模型）

### 3.1 定位

```text
Holding Intelligence · 观察信号层
  ≠ Trading Signal（不下单）
  ≠ HoldingTSuitability（适宜性环境）
  ≠ ExitEval state
  ≠ Opportunity / 日K 冰点 SignalEvent
```

### 3.2 字段（按需求冻结）

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| **`signal_type`** | enum string | **`T_BUY_WATCH`** \| **`T_SELL_WATCH`** 仅此二值（MVP） |
| **`stock_code`** | string | 归一化代码 |
| **`position_id`** | string/uint | 可选；今日常空，后续从 `paper_sim_positions.id` lookup |
| **`signal_time`** | string/time | 建议对齐触发 5m bar 时间 |
| **`signal_price`** | float64 | 观察锚价（触发 bar close/low/high 约定需在实现设计写死）；**禁止**事后用现价回填改写历史 |
| **`reason`** | string \| string[] | 解释码或短文案（观察语言；禁止「立即买入/卖出」指令口吻） |
| **`confidence`** | number 或 enum | 建议 `0–1` 或 `low|medium|high`；受 freshness / suitability / health 降级 |

**建议可选元数据（非本审计必填）：**  
`suitability_level` · `health_grade` · `freshness` · `cost_price` · `timeframe=5m` · `evaluated_at` · `data_source_note`

### 3.3 与相邻模型边界

| 模型 | 关系 |
| --- | --- |
| HoldingTSuitability | **门闩 / 上下文**；不替代 `signal_type` |
| HealthScore | 降置信 / reason 附件 |
| ChartMarker | UI 投影：`T_BUY_WATCH`→展示「T买」类标记；保持 observation-only |
| SignalScanHit / 冰点 | **不同域**；禁止混用 `signal_id`（17.4.1）除非显式桥接文档 |
| TradePlan / Broker | **无边** |

### 3.4 UI 落点（预期，不实现）

1. **HoldingTPanel**「T 策略观察」占位 → 列表 + SVG markers  
2. 可选：组合「做 T」Drawer 只读摘要（非下单）  
3. 可选后期：Lightweight 5m + markers（需新接线）

---

## 4. 硬约束

1. **不自动交易** — 不发单、不改 Exit state、无「一键做 T」执行。  
2. **不改 Broker** — 无 fill/order API 副作用。  
3. **不改 Settlement** — 不写 `mark_price` / 日结。  
4. **不生成 TradePlan** — 不创建/改 plan、不 freeze、不 enable execute。  
5. 输出仅为 **WATCH 观察**；文案与 UI **禁止**伪装成委托指令。  
6. 不把本层结果回写 CandidatePool Score/Rank。

---

## 5. 阶段结论与下一步（仅指引）

| 项 | 结论 |
| --- | --- |
| K 线 Marker/Overlay/Signal | 主图具备通用能力；做 T 未接；HoldingT **已留空 markers** |
| 数据充分性 | **MVP 观察信号：条件够**；规则与数据源对齐仍待设计/实现阶段 |
| `HoldingTSignal` | 字段可冻结如上；**本阶段不实现** |
| 交付 | **本审计报告** |
| 动作 | **停止** — 等待授权再进设计细化或实现 |

---

## 附录 — 关键路径

- `frontend/src/components/StockLightweightKlineChart.vue` — markers / priceLine / cost props  
- `frontend/src/utils/signalTimeframe.js` — 冰点日 K 门闩  
- `frontend/src/components/HoldingTPanel.vue` — 5m + 信号占位  
- `frontend/src/utils/chartMarkers.js` — `BUY`/`SELL` ChartMarker 框架  
- `backend/papertrading/holding_t_suitability.go` — 适宜性门闩  
- `backend/papertrading/holding_health_score.go` — HealthGrade  
- `backend/papertrading/position_evaluation_explanation.go` — freshness / cost_price  

---

*PHASE17.6 Holding T Signal Layer 前置审计结束 · 停止。*
