# PHASE17_C1 Position Evaluation Infrastructure — 前置审计

**日期：** 2026-09-07  
**阶段：** Phase17 C1（只读）  
**目标：** 为「模拟持仓自动评估是否卖出」「持仓做 T」「持仓健康监控」建立数据基础认知。  
**禁止项遵守：** **未改代码 / 未改库 / 未新增 API / 未改前端**

**关联：** Phase17.1 KLine Freshness · Phase17.2/17.3/17.4 Signal 边界 · 既有 HoldingEval / ExitReview / Provenance

---

## 0. 总判

| 问题 | 结论 |
| --- | --- |
| 持仓账本是否够用？ | **部分够** — `paper_sim_positions` + fills 可支撑成本/数量/市价/PnL 派生 |
| 能否回答「该不该卖 / 健不健康」？ | **部分能** — HoldingEval + ExitEval + Outcome 已有只读链 |
| 能否回答「为何持有 / 适合做 T」？ | **弱 / 基本不能** — 缺持续 thesis 与 T 适合度模型 |
| C1 是否要改库？ | **本阶段不建议** — 优先投影/桥接；真 `position_lots` 表仍属长期可选 |

---

## 一、持仓数据链路审计

### 1.1 真源与投影

```text
paper_sim_accounts
paper_sim_positions     ← 净仓账本真源（无 lifecycle/来源字段）
paper_sim_orders
paper_sim_fills         ← 成交真源（含 plan_id）
        │
        ├─ PositionAttribution / PositionLotDTO   （fills 投影，无独立 position_lots 表）
        ├─ portfolio.Snapshot / readmodel         （权益用持久 mark_price）
        ├─ positionstate.PositionStateView        （T+1 / holding_days / can_sell）
        ├─ HoldingEvaluation / ExitEvaluation     （观测）
        ├─ portfolio/provenance.View              （持仓↔成交↔TradePlanOrigin）
        └─ TradePlanOrigin / InvestmentNarrative  （信号/策略叙事）
```

| 名称 | 实体 | 说明 |
| --- | --- | --- |
| portfolio positions | Snapshot / Dashboard / ReadModel | 读模型，非第二套账本 |
| position_lots | **无物理表** | 运行时由 fills 投影为 Lot DTO |
| paper_sim_positions | `PaperSimPosition` | 净仓 |
| paper_sim_fills | `PaperSimFill` | 买卖成交 |
| settlement | `SettlementJob` + 日报告 | EOD 刷新 mark / 冻结日快照 |
| 遗留 | `paper_positions` | 与 paper_sim 隔离，主路径不用 |

### 1.2 `paper_sim_positions` 已存字段

`backend/papertrading/models.go` → `PaperSimPosition`：

| 字段 | 含义 |
| --- | --- |
| `account_id` / `stock_code` / `stock_name` | 身份 |
| `total_volume` / `available_volume` / `locked_volume` | 数量与 T+1 |
| `avg_cost` | 成本价 |
| `mark_price` | 账本市价 |
| `updated_at` | 更新时间 |

### 1.3 `paper_sim_fills` 关键字段

| 字段 | 含义 |
| --- | --- |
| `plan_id` / `plan_item_id` / `order_id` | TradePlan / 订单关联 |
| `side` / `price` / `volume` / `fee` | 成交 |
| `fill_reason` / `filled_at` | 开盘/收盘等、时间 |

### 1.4 mark_price 来源

| 路径 | 行为 |
| --- | --- |
| 买入/卖出成交 | `broker.fillBuy/fillSell` 写入成交价为 mark |
| EOD Settlement | `MarkPricer` 或开盘价 fallback → 批量更新 |
| 读模型 overlay | `display_price`（行情）**只展示，不回写账本权益** |
| 遗留 API | `SetPaperMarkPrice`（非主路径） |

### 1.5 能力对照表

| 需求字段 | 是否支持 | 落点 |
| --- | --- | --- |
| 成本价 | **是** | `avg_cost`；lot 级 `fill_price` 投影 |
| 当前价 | **是（分两层）** | 账本 `mark_price`；UI 可用 display quote |
| 盈亏 | **是（派生）** | `(mark−avg)×qty`；账户 `unrealized_pnl`；仓位表**不持久** PnL |
| 持仓时间 | **是（投影）** | `PositionState.holding_days` / HoldingEval；**净仓表无** |
| 买入来源 | **弱** | 经 fills→plan；无显式 `buy_source` |
| 信号来源 | **弱（只读桥）** | TradePlanOrigin / Provenance / Snapshot hits；不在仓位行上 |
| TradePlan 来源 | **是（间接）** | fill.`plan_id`；Attribution lots |
| 当前状态 | **部分** | 订单 status；PositionState S0–S4 / can_sell；**净仓无统一 lifecycle 枚举** |

### 1.6 缺失字段（相对评价基础设施）

| 缺口 | 说明 | C1 建议 |
| --- | --- | --- |
| 净仓无 `first_buy_at` / holding_days | 每次从 fills 算 | 继续投影，**不必立刻加列** |
| 无 `buy_source` / `signal_id` 落仓位 | 靠 provenance join | 评价层快照复制即可 |
| 无持久 lot 表 | 复杂仓位/部分卖归因靠内存投影 | 长期可选；非 C1 阻塞 |
| mark 可能 stale | 盘中依赖 settlement/成交；display≠ledger | 评价应声明用 mark 还是 live quote |
| Trend / T-suitability | 评价分类里 Trend 常 UNKNOWN | 后续能力，非仓位表字段 |

**本节产出文件名：** 即本文（`PHASE17_C1_POSITION_MODEL_AUDIT.md`）。

---

## 二、持仓评价能力审计

### 2.1 已有能力地图

| 能力 | 组件 | 回答什么 |
| --- | --- | --- |
| Holding Evaluation | `holding_evaluation*.go` | 浮盈亏、风险/盈亏/持有期标签 |
| Exit Policy | `exit_policy.go` | 天数/亏损阈值（**观测**，非卖单） |
| Exit Evaluation | `exit_evaluation.go` | NORMAL / WATCH / REVIEW_REQUIRED |
| Exit Context | `exit_evaluation_context.go` | 买入 reason、策略名、计划状态 |
| ExitReviewOutcome | `exit_review_outcome.go` | 用户 HOLD / WATCH / CREATE_SELL_PLAN |
| Holding Decision | `holdingdecision` | 默认闸门偏关；观测建议 |
| Sell Suggestion | `sellsuggestion` | 默认 `Enabled=false` |
| T-sell Draft | TradePlan `t_sell` / `exit_review` | 人工卖出意图 |
| Position Provenance | `portfolio/provenance` | 持仓↔成交↔ origin 信号块 |
| TradePlanOrigin | `tradeplanorigin` | 计划项 ← CandidatePool / SignalSnapshot |
| SignalPrice | Snapshot hit 内嵌 | 冻结信号价（≠ 现价） |
| SignalEvent | **未独立落库** | 17.5 Adapter 设计中；ExitReview 未挂信号事件 |
| Opportunity tracking | opportunity WATCH + OutcomeProjection | 机会侧闭环；与持仓复评弱耦合 |
| 持仓做 T UI | `HoldingTPanel` + 前端日内启发式 | 观察台；**无适合度评分** |

### 2.2 连接关系（简图）

```text
Positions + Fills
  → Attribution lots (plan_id)
  → HoldingEval → ExitEval (+ ExitContext from TradePlan)
  → ExitReviewOutcome（用户）
  →（可选）SellDraft / t-sell TradePlan
并行：
  Snapshot hits (SignalPrice) → Origin → Provenance / Narrative
  PositionState (can_sell, holding_days)
```

### 2.3 四个产品问题

| 问题 | 现状 | 说明 |
| --- | --- | --- |
| 为什么应该继续持有？ | **弱** | 有用户 HOLD 结论 + entry_reason；缺「仍符合原 thesis」自动论证 |
| 为什么应该卖出？ | **部分能** | 系统：TIME/LOSS/PLAN_REVIEW + 风险标签；用户 Outcome；**默认不自动卖** |
| 当前持仓是否健康？ | **部分能** | RiskState + ExitEval；Trend 弱；mark 时效影响 |
| 是否适合做 T？ | **基本不能** | 仅有 HoldingT 观察 UI；无后端适合度、未接 can_sell/波动门闩 |

### 2.4 最小新增能力（设计，不实现）

1. **Holding↔Signal 只读桥** — ExitContext/评价卡片复制 `signal_price/time/tag`（来自既有 Origin，不新建事件表亦可先行）。  
2. **持有 Explain 卡片** — 合成 entry_reason + ExitEval + 最新 Outcome + vs_signal（Narrative 已有雏形）。  
3. **T-suitability v0** — 输入：`can_sell`、可用量、日内振幅/波动、仓位权重 → `suitable|caution|unsuitable`。  
4. **评价用价源声明** — 明确 ledger `mark_price` vs live quote（衔接 Freshness / Settlement）。  
5. （可选）打开 HoldingDecision / SellSuggestion **观测开关**（仍 suggest_only）。

**不做：** 改库加列、自动卖出、SignalEvent 大表（留给 17.5+）、改 ExitPolicy 为执行策略。

---

## 三、数据刷新及时性审计

### 3.1 评价所需行情维度

| 数据 | 用途 | 现网可得性 |
| --- | --- | --- |
| 最新价格 | PnL / 健康 / 相对成本 | mark（账本）+ display quote；K 线 latest（17.1 freshness） |
| 分时涨跌 | 盘中情绪 / 做 T | 有限（报价 overlay）；非完整分时评价管线 |
| 5m / 30m K | 做 T / 短线趋势 | 同东财 K 链；HoldingT 用 5m |
| 日线趋势 | Exit / 健康 | 日 K + 冰点族 Signal（仅日 K 展示，17.4） |
| 成交量 | 突破/异常 | K 线 volume |
| 资金流 | 增强项 | 有独立模块，**未**接入 HoldingEval |

### 3.2 三场景需求

| 场景 | 评价重点 | 价格/K 线策略 | 风险 |
| --- | --- | --- | --- |
| **盘前** | 隔夜缺口、是否仍触发 REVIEW | 昨收/mark；日 K expected=昨收（freshness） | mark 未更新到开盘 |
| **盘中** | 实时健康、做 T、是否升级 REVIEW | live quote + 分钟 K；日 K≥09:30 追当日 | IDLE 午休曾导致 K 线陈旧（已修 freshness）；评价需避免误用过期 mark |
| **收盘后** | 日终健康、Exit 复评主场 | Settlement 刷新 mark；日 K 完整 | 应以 settled mark + 日 K 为准做报表 |

### 3.3 与 KLine Freshness 的关系

- **日线趋势评价**可依赖 17.1 门闩，减少「停在上周五」的错误趋势判断。  
- **持仓 PnL（账本）**仍跟 `mark_price` / Settlement，**不自动**等于 K 线最新收盘。  
- **做 T**更依赖分钟 K + LIVE TTL，与日线 ice Signal 分离（17.4 已禁止周月错配）。

---

## 四、设计建议（不实现）

### 4.1 当前能力地图（一页）

```text
[账本] positions + fills + settlement(mark)
[状态] PositionState (T+1, holding_days, can_sell)
[评价] HoldingEval → ExitEval → Outcome
[溯源] Attribution → Origin → Provenance / Narrative
[交易] t-sell / exit_review Draft（人工）
[图表] K 线 freshness + Signal Provider（日K冰点）
[做T] HoldingT 观察 UI（弱）
```

### 4.2 缺口列表（优先级）

| P | 缺口 |
| --- | --- |
| P0 | 评价价源语义（mark vs live）写清并在观测 API/UI 标注 |
| P0 | Holding↔Signal 只读桥（为何买 / vs 信号价） |
| P1 | 「为何仍持有」Explain 合成（无新引擎） |
| P1 | T-suitability 只读评分 v0 |
| P2 | TrendState 真实化（接日/分钟趋势 Provider） |
| P2 | HoldingT 并入统一 ChartSession / 工作台 |
| P3 | 可选物理 `position_lots` 表（非阻塞） |
| P3 | SignalEvent 持久化（Phase17.5+） |

### 4.3 推荐 Phase17 后续顺序

| 序 | 阶段 | 内容 |
| --- | --- | --- |
| 已完成 | 17.1–17.4 | K 线 freshness · 多周期审计 · Signal 配置设计 · 命名/Timeframe 门闩 |
| **C1（本文）** | 前置审计 | 持仓评价数据基础 |
| **建议 C2** | Position Evaluation 读模型加固 | 价源标注 + Eval 卡片接 provenance/signal 快照（**不改表**） |
| **建议 C3** | Hold/Sell Explain v0 | 只读叙事；接 ExitReview UI |
| **建议 C4** | T-suitability v0 | 只读评分挂 HoldingT/组合 |
| 并行 | 17.5 | SignalEvent Adapter（图表/事件统一，服务 C2 桥接） |
| 其后 | 工作台统一 | ChartSession + HoldingT 收敛 · 可选自动评估闸门（默认关） |

**原则：** 先 **投影与解释**，再 **评分**，最后才考虑 **自动动作**；全程默认不改 `paper_sim_positions` schema。

---

## 五、合规确认

| 要求 | 状态 |
| --- | --- |
| 未修改数据库 | **是** |
| 未新增 API | **是** |
| 未改前端 | **是** |
| 只做审计 | **是** |

---

## 附录 — 关键路径

- `backend/papertrading/models.go`
- `backend/papertrading/settlement.go` / `broker.go`
- `backend/papertrading/holding_evaluation*.go` / `exit_*.go` / `exit_review_outcome.go`
- `backend/papertrading/attribution.go`
- `backend/portfolio/positionstate/` / `provenance/` / `readmodel/`
- `backend/tradeplanorigin/` / `investmentnarrative/` / `holdingdecision/` / `sellsuggestion/`
- `frontend/.../ExitReviewDrawer.vue` / `HoldingTPanel.vue` / `PortfolioDashboard.vue`

---

*Phase17 C1 Position Evaluation Infrastructure 前置审计结束。*
