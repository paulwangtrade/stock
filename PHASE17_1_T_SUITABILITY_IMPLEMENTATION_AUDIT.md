# PHASE17.1 Holding T-Suitability — 实现前审计（只读）

**日期：** 2026-09-07  
**阶段：** Phase17.1 第一阶段（**仅审计，暂停实现**）  
**目标：** 为实现「持仓做 T 适宜性」只读评价层确认数据源、复用点与交付边界。  
**约束回顾：** 不改交易执行 / 不自动卖单 / 不改 ExitEval state / 不新建表 / 不覆盖 mark_price；复用 C2 Explanation · C3 HealthScore · C4 组合展示 · KLine Freshness。  

**上游：**  
- [PHASE17_C5_T_SUITABILITY_AUDIT.md](./PHASE17_C5_T_SUITABILITY_AUDIT.md)（能力前置）  
- [PHASE17_C2_POSITION_EVALUATION_EXPLANATION_IMPLEMENTATION.md](./PHASE17_C2_POSITION_EVALUATION_EXPLANATION_IMPLEMENTATION.md)  
- [PHASE17_C3_HOLDING_HEALTH_SCORE_IMPLEMENTATION.md](./PHASE17_C3_HOLDING_HEALTH_SCORE_IMPLEMENTATION.md)  
- [PHASE17_C4_PORTFOLIO_HEALTH_DISPLAY_IMPLEMENTATION.md](./PHASE17_C4_PORTFOLIO_HEALTH_DISPLAY_IMPLEMENTATION.md)  

**本阶段动作：** 输出本审计后 **暂停**；第二阶段设计 / 实现另开。

---

## 0. 审计总判

| 问题 | 结论 |
| --- | --- |
| 数据是否够做 v0？ | **够** — paper_sim 仓位 + PositionState + Health/Explanation + 5m K（可选）可投影 |
| 是否必须新表？ | **否** |
| 是否必须新 API？ | **可选** — 优先 **纯投影函数** + 挂既有 exit-evaluation 或前端拼装；独立只读 GET 非阻塞 |
| ExitEval 关系 | **只可读** T-Suitability；**不得**因 level 改 NORMAL/WATCH/REVIEW |
| 与 HoldingT 页 | 评价应对齐 **paper_sim**；HoldingT Follow 源暂不作为权威输入 |
| 实现形态 | `HoldingTSuitability` DTO + `BuildHoldingTSuitability(...)` 规则引擎 |

---

## 1. 持仓数据来源

| 需求 | 来源 | 字段 / 路径 |
| --- | --- | --- |
| Position 结构 | `paper_sim_positions` → Snapshot / readmodel | `stock_code`, `total_volume`, `available_volume`, `locked_volume`, `avg_cost`, `mark_price` |
| 可卖 / can_sell | `portfolio/positionstate` | `available_qty`；`can_sell = available_qty > 0`（API：`GET /api/portfolio/position-state`） |
| T+1 状态 | 同上 + 晨间 unlock | `locked_qty`；状态机 S0–S4 / `is_new_position`（展示） |
| 持仓数量 | Snapshot / PositionState | `total_qty` / `total_volume` |
| 成本 | positions.`avg_cost`；lot `fill_price` | HoldingEval `avg_cost` / `cost_price` |

**组合页现状：** Snapshot 持仓行已带数量/成本/可卖相关；卖出按钮用 `canShowSellButton`（基于 available）。  
**T-Suitability 硬门闩：** 必须以 **PositionState.can_sell + available_qty**（paper_sim）为准，不用 Follow 成本仓。

---

## 2. 行情数据来源

| 需求 | 来源 | 说明 |
| --- | --- | --- |
| 实时行情 | Snapshot `include_display=1` → `display_price` / QuoteService | **展示 overlay**；不写 mark |
| 5min K | `GetStockEastMoneyKLine(..., klt=5)` + FE/BE cache | HoldingT / 主图可用；**波动派生首选** |
| 30min K | 同 API `klt=30` | 主图有；v0 **可选**，非必须 |
| 日 K | `klt=101` + calendar freshness（17.1） | 趋势背景；冰点 Signal 仅日 K |
| Freshness | C2 `EvaluationDataFreshness`；K 线 cache TTL/calendar | T 评价声明 **FRESH/STALE/UNKNOWN**；`PRICE_STALE` 标签可直接复用 |

**禁止：** 用 Suitability 结果回写 `mark_price` 或改权益。

**v0 波动：** 优先用调用方注入的 **5m bars 振幅启发式**（或 FE 已拉 K）；后端纯函数可收 `IntradayMetrics` 输入，**不强制**在投影内打东财 HTTP（避免执行链耦合）。

---

## 3. 已有评价复用

### 3.1 HoldingEval（可直接用）

| 字段 | T-Suitability 用途 |
| --- | --- |
| `holding_days` | 背景（过短/过长 → caution，非硬否） |
| `unrealized_return` / `profit_state` | 与 Health 一致；不作卖出 |
| `risk_state` | 配合 `LOSS_CONTROL` |
| `trend_state` | 多为 UNKNOWN；有 UP/SIDEWAY 作加分语境 |
| `quote_time` / quote source | Freshness 输入 |

### 3.2 Explanation（标签复用）

| 标签 | 建议映射到 Reasons |
| --- | --- |
| `PRICE_STALE` | → 倾向 **unsuitable** / caution（盘中做 T） |
| `NO_SOURCE_TRACE` | → caution（信息不完整） |
| `LOSS_CONTROL` | → caution（或 Health D 时 unsuitable） |
| `SIGNAL_EXPIRED` | → caution（非硬否） |
| `SIGNAL_ACTIVE` / `TREND_SUPPORT` / `PROFIT_PROTECTION` | → 正向 reasons（不单独决定 suitable） |

### 3.3 HealthScore（背景因素）

| Grade | 建议 |
| --- | --- |
| A / B | 允许进入 suitable（仍须 can_sell + freshness + 波动） |
| C | 默认 **caution** |
| D | 默认 **unsuitable**（或强 caution，产品二选一；审计建议 **unsuitable**） |

**Health 不替代 can_sell；ExitEval state 不读入决策公式（可读作展示旁注，v0 可不写入 Reasons）。**

---

## 4. 前端入口（我的组合）

| 项 | 现状 |
| --- | --- |
| 持仓列表 | `PortfolioDashboard.vue` 内 `n-data-table`（无独立 Holdings 组件文件） |
| 健康展示 | C4：`健康` / `健康摘要` 列 + `HoldingHealthDrawer` |
| Drawer 结构 | `HoldingHealthDrawer`、`PortfolioProvenanceDrawer`、`SellDraftDialog`（卖出 **人工**，保持独立） |
| 数据拼装 | Snapshot + Dashboard + exit-evaluation（Health）best-effort |
| HoldingT 菜单页 | 观察台；**非**本阶段主入口（宜组合列 + Drawer） |

**实现建议（设计阶段确认）：**  
- 列：「做 T」适宜性 chip（suitable/caution/unsuitable）  
- 点击 → 复用/轻扩 Drawer（只读 reasons；**禁止**在该 Drawer 加自动卖出）  
- 既有「卖出」列保持人工 Draft，与适宜性解耦  

---

## 5. 交付边界裁定

| 项 | 裁定 |
| --- | --- |
| 新增 DTO | **是** — `HoldingTSuitability`（内存/JSON；不落库） |
| 新增 DB 表 | **否** |
| 新增 API | **可选** — MVP 可将投影挂在 `exit-evaluation` 行字段 `t_suitability`，或前端用 PositionState+Health+本地 5m 拼装；独立 `GET .../t-suitability` 非必须 |
| Projection 计算 | **是** — `BuildHoldingTSuitability(input) → HoldingTSuitability` 纯函数 + 单测 |
| 改 ExitEval 阈值 | **否** |
| 改 mark / Broker | **否** |

### 5.1 建议 DTO（与任务对齐；实现阶段定稿）

```go
type HoldingTSuitability struct {
    StockCode          string
    Level              string   // suitable | caution | unsuitable
    Reasons            []string
    CanSell            bool
    Freshness          string   // FRESH | STALE | UNKNOWN
    IntradayVolatility string   // high | normal | low | unknown
    HealthGrade        string   // A|B|C|D|"" 
    EvaluatedAt        time.Time
}
```

### 5.2 推荐输入（投影，不读库亦可测）

```text
PositionState(can_sell, available_qty)
+ HealthScore.grade / Explanation.risk_hints|freshness
+ optional IntradayMetrics(amplitude_pct from 5m)
+ AsOf / session LIVE flag（可选）
```

### 5.3 规则骨架（设计阶段细化；此处仅审计共识）

1. `!can_sell` 或 `available < 1 手` → **unsuitable**（`T1_LOCKED` / `NO_SELLABLE`）  
2. `Freshness == STALE` 或 `PRICE_STALE` → **unsuitable** 或 **caution**（盘中建议 unsuitable）  
3. `HealthGrade == D` → **unsuitable**  
4. 振幅 `low` → **caution**（或 unsuitable）  
5. 否则 can_sell + FRESH + Health A/B + 振幅 normal/high → **suitable**  
6. 其余 → **caution**  

Reasons **仅解释码**，不含 BUY/SELL。

---

## 6. 风险与注意

| 风险 | 缓解 |
| --- | --- |
| 用户把 suitable 当成下单按钮 | Drawer/列文案：「适宜性评价 · 非交易指令」 |
| Follow vs paper_sim 混用 | 组合页只绑 paper_sim |
| 后端拉 K 拖慢 exit-evaluation | 波动作 **可选输入**；缺省 `IntradayVolatility=unknown` → caution |
| Exit 误改 state | 代码评审守门：Exit 投影不读 Level |

---

## 7. 第一阶段结论（暂停点）

1. **可以进入第二阶段设计 + 第三阶段实现**（纯投影 + 可选挂载 + 组合只读展示）。  
2. **不需要**新表；**DTO 需要**；**API 可选**。  
3. **硬依赖：** PositionState.can_sell + Health/Explanation freshness；**软依赖：** 5m 振幅。  
4. **本审计完成后暂停实现**，待设计确认 `HoldingTSuitability` 规则阈值与挂载点（exit-evaluation vs 独立 API vs 纯前端）。

---

## 8. 合规确认

| 要求 | 状态 |
| --- | --- |
| 只读审计 | **是** |
| 未改代码 / 未实现 | **是** |
| 审计后暂停 | **是** |

---

*Phase17.1 T-Suitability 实现前审计结束 — 等待设计阶段。*
