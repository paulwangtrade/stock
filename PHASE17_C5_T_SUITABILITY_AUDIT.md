# PHASE17_C5 T-Suitability Evaluation — 前置审计（只读）

**日期：** 2026-09-07  
**阶段：** Phase17 C5（**仅审计，未改代码**）  
**目标：** 评估「持仓做 T 辅助」是否已有数据基础；给出最小实现方案与顺序。  
**依据：** C1 Position Model · C2 Explanation · C3 HealthScore · C4 Portfolio Health Display · Phase17.1 KLine Freshness · Phase17.1 持仓做T菜单审计  

**禁止项遵守：** **未修改代码 / 未新增 API / 未改库 / 未改交易链**

---

## 0. 总判

| 问题 | 结论 |
| --- | --- |
| 是否具备做 T **数据基础**？ | **部分具备** — 行情（5m/30m/日K+量）、仓位（成本/可卖/T+1）、评价（Health/Signal/Risk）大多已有 |
| 是否已有 **T-Suitability 评分**？ | **否** — 无后端/统一产品评分；HoldingT 为观察占位 |
| 能否产品化回答「适不适合做 T」？ | **尚不能正式回答** — 缺规则合成层 + 价源/波动门闩接线 |
| 是否需要新表？ | **C5 实现阶段不需要** — 投影 + 规则即可 |
| 与下单关系 | Suitability 必须是 **观察建议**；真正 T 卖仍走已有 `SellDraftDialog` / `t_sell`（人工） |

---

## 一、行情数据

### 1.1 能力对照

| 需求 | 是否支持 | 落点 / 说明 |
| --- | --- | --- |
| 实时价格 | **部分** | Snapshot `display_price` overlay；Follow 实时列表；账本 `mark_price`（≠ live） |
| 5 分钟 K | **是** | 东财 `klt=5`；主图 Lightweight + HoldingT SVG 均用 `GetStockEastMoneyKLine` |
| 30 分钟 K | **是** | 主图可选 `klt=30`；HoldingT **未用** |
| 日线趋势 | **是（弱绑定评价）** | `klt=101`；冰点族 Signal **仅日K**（17.4）；HoldingEval `trend_state` 常 **UNKNOWN** |
| 成交量 | **是** | K 线 `Volume` 字段；评价链 **未**消费量能做 T 门闩 |
| 分时波动 | **弱** | 迷你分时火花线；无统一「分时振幅/波动率」评价 DTO；可由 5m OHLC **派生** |

### 1.2 KLine Freshness（Phase17.1）

| 点 | 含义（对做 T） |
| --- | --- |
| 日 K calendar freshness | 避免「停在上周五」误判趋势；**收盘后/盘前**做 T 辅助更可信 |
| 分钟 K LIVE TTL ~30s | 盘中 5m/30m 相对及时；IDLE 拉长 TTL |
| HoldingT 旁路 | 同 Wails API + FE `klineCache`；**不经**主 Modal，但 freshness 门闩对 BE cache 仍生效 |
| 评价注意 | T 适合度应声明用 **live quote / 分钟K**，勿单独用过期 `mark_price` |

### 1.3 已有前端日内启发式（未产品化）

- `frontend/src/utils/intradayTSignals.js`：基于当日 bar 的 high/low、RSI、区间位置等 **前端启发式**  
- HoldingT：`buildIntradayChartModel` + 空 markers；文案「等待信号模型」  
- **未**接入 PositionState / HealthScore / 不可自动下单  

---

## 二、持仓数据

| 需求 | 是否支持 | 落点 |
| --- | --- | --- |
| 成本 | **是** | `avg_cost` / lot `fill_price`；Follow `costPrice` |
| 当前仓位 | **是** | `total_volume`；Snapshot positions |
| 可卖数量 | **是** | `available_volume` / PositionState `available_qty` |
| T+ 规则 | **是** | `locked_volume`；晨间 unlock；`can_sell = available > 0` |
| 当日买卖记录 | **是（账户级）** | Dashboard `trades.fills`（当日成交摘要）；Attribution lots / Provenance 历史 |

**缺口（相对做 T）：**

| 缺口 | 说明 |
| --- | --- |
| HoldingT 数据源分裂 | HoldingT 用 **Follow 成本仓**，组合做 T 用 **paper_sim**；适合度应对齐 **paper_sim + PositionState** |
| 无「今日已 T 次数 / 剩余可卖」合成视图 | 数据可从 fills + available 推，缺统一 DTO |
| 仓位权重 | Snapshot 有市值，前端可算占比；未进评价 |

---

## 三、评价数据

### 3.1 可结合的现有能力

| 能力 | 状态 | 做 T 用途 |
| --- | --- | --- |
| HoldingHealthScore (C3) | **有** | 质量背景：D/高风险时倾向 **不适合** 或 caution |
| Explanation / Signal (C2) | **有** | `SIGNAL_ACTIVE` / `EXPIRED`、`PRICE_STALE` |
| Trend | **弱** | `TREND_SUPPORT` 仅当 `trend_state` UP/SIDEWAY；多数 UNKNOWN |
| Risk | **有** | `LOSS_CONTROL`、ExitEval WATCH/REVIEW、RiskState |
| ExitEval | **有** | 复评状态；**不得**因 Exit 直接等于「做 T」 |
| PositionState | **有** | **硬门闩**：`can_sell` / available |

### 3.2 什么情况下适合做 T？（产品规则草案，非实现）

**适合（suitable）— 建议同时满足：**

1. `can_sell == true` 且 `available_qty` 足够最小交易单位  
2. 盘中 LIVE；价格 / 5m K **非 STALE**（对齐 freshness + Explanation `PRICE_STALE`）  
3. 日内有足够波动（例如 5m 派生振幅 ≥ 阈值，避免无波动空转）  
4. Health 非 D；无极端 `LOSS_CONTROL` 与「仅剩锁仓」冲突  
5. （加分）短线结构清晰：5m/30m 有可交易区间；非单边无回撤暴跌/暴涨中途追击  

**不适合（unsuitable）— 任一命中即否：**

1. `can_sell == false` 或可卖为 0（T+1 锁仓）  
2. 价格数据过期 / 非交易时段却假装实时 T  
3. Health **D** 或 Exit **REVIEW_REQUIRED** 且叙事为「应复评减仓」而非波段 T（产品可配置为 caution）  
4. 振幅过低（死水）或流动性/量能异常萎缩（v0 可用 5m volume 启发式）  
5. 无持仓或数据无法对账（reconcile 失败）  

**谨慎（caution）：** Health C、Signal 过期、Trend UNKNOWN、仅有 Follow 仓无 paper_sim 状态等。

> **定位：** 以上是 **辅助标签**，不是 BUY/SELL 指令；执行仍人工 Draft。

---

## 四、设计输出

### 4.1 当前能力地图

```text
[行情] 东财 K(5/30/101)+Volume + Freshness + display quote
[仓位] paper_sim + PositionState(can_sell, available, locked)
[成交] dashboard fills / attribution
[评价] Explanation + HealthScore + ExitEval（观察）
[观察 UI] HoldingTPanel（Follow+5m，无评分）
[执行 UI] SellDraftDialog → t_sell（人工，已存在）
[启发式] intradayTSignals.js（前端，未接线产品评分）
```

### 4.2 缺失字段 / 能力（相对 T-Suitability）

| P | 缺失 | 建议 |
| --- | --- | --- |
| P0 | 统一 **TSuitability** 投影（suitable/caution/unsuitable + reasons） | 纯规则，读已有输入 |
| P0 | 评价绑定 **paper_sim PositionState**（非仅 Follow） | 复用 position-state API |
| P1 | 日内振幅 / 波动派生指标 | 从 5m OHLC 计算，不落库 |
| P1 | 价源声明（live vs mark）与 STALE 门闩 | 复用 C2 Freshness 语义 |
| P2 | Trend 真实化 | 可选；UNKNOWN 时不阻断，标 caution |
| P2 | HoldingT 与主图/组合数据源统一 | 产品收敛，非评分阻塞 |
| P3 | 今日已 T 次数等 | 可选，fills 聚合 |

### 4.3 最小实现方案（设计 only）

**不改交易链；不自动下单；不新建表。**

```text
输入（只读）:
  PositionState(can_sell, available_qty)
  HoldingHealthScore + Explanation(freshness, risk tags)
  可选: 最近 5m bars → amplitude / volume heuristic
  可选: session LIVE/IDLE

输出:
  TSuitability {
    stock_code, as_of,
    status: suitable | caution | unsuitable,
    reasons: [...],          // 解释码，非 SELL
    inputs_note: "observation only",
    health_grade?, can_sell?, amplitude?
  }

挂载建议:
  HoldingT / 组合持仓旁路只读 chip
  点击看 reasons；卖出仍走既有 SellDraft（人工）
```

规则 v0 示例（与第三节一致）：  
`unsuitable` if !can_sell OR PRICE_STALE OR available==0；  
else if Health D OR 振幅过低 → `unsuitable`/`caution`；  
else if Health A/B + can_sell + 振幅够 → `suitable`。

### 4.4 是否需要新增数据模型

| 项 | 结论 |
| --- | --- |
| DB 新表 | **不需要** |
| 持久化评分 | **不需要**（每次投影） |
| 新 API | **可选** 只读 `.../t-suitability`；亦可前端拼装既有 API（更贴「最小」） |
| SignalEvent 大表 | **不需要**（C5） |

### 4.5 建议开发顺序

| 序 | 阶段 | 内容 |
| --- | --- | --- |
| 已完成 | C1–C4 | 仓位审计 · Explanation · HealthScore · 组合展示 |
| **C5（本文）** | 前置审计 | T-Suitability 数据基础 |
| **建议 C5.1** | 规则引擎 v0（纯函数 + 单测） | 输入 DTO → status/reasons；无交易副作用 |
| **建议 C5.2** | 接线 PositionState + Health + 5m 振幅 | 前端或轻量只读 API |
| **建议 C5.3** | UI chip（组合 / HoldingT） | 只展示；禁止自动卖出按钮 |
| 其后 | 观察台收敛 | HoldingT → paper_sim + 主图 Session；Trend 增强 |

**原则：** 先 **门闩（can_sell + freshness）**，再 **波动启发式**，最后 **UI**；全程 suggest_only。

---

## 五、合规确认

| 要求 | 状态 |
| --- | --- |
| 只读审计 | **是** |
| 未改代码 | **是** |
| 未实现评分/下单 | **是** |

---

## 附录 — 关键路径

- 行情：`backend/data/eastmoney_kline_api.go` · `kline_cache.go` · `StockLightweightKlineChart.vue` · `HoldingTPanel.vue`  
- Freshness：Phase17.1 报告 · `klineLatestCalendarFresh`  
- 仓位：`paper_sim_positions` · `portfolio/positionstate`  
- 评价：`holding_health_score.go` · `position_evaluation_explanation.go` · `exit_evaluation.go`  
- 做 T UI/链：`HoldingTPanel.vue` · `SellDraftDialog` · `tradePlansTSell` · `PHASE17_1_TRADE_T_MENU_AUDIT.md`  
- 启发式：`frontend/src/utils/intradayTSignals.js`  

---

*Phase17 C5 T-Suitability 前置审计结束。*
