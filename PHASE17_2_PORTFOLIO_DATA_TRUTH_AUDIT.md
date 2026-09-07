# PHASE17.2 Portfolio Data Truth Layer — 实现前审计

**日期：** 2026-09-07  
**性质：** 只读审计（随后进入设计/实现；本文件固定审计结论）  
**目标：** 澄清「我的组合」当前价 / 今日盈亏 / 浮盈语义，指导展示层修复。  

**禁止改动：** Snapshot 账本模型语义、Settlement、mark_price 写入、Paper Broker、Trade 执行。

---

## 1. Snapshot 字段来源

| 字段 | 含义 | 真源 |
| --- | --- | --- |
| `mark_price` | 账本估值价 | `paper_sim_positions`；成交写入 + EOD Settlement |
| `avg_cost` | 成本 | 仓位摊薄成本 |
| `equity` / `market_value` | 权益 / 市值 | cash + Σ(mark×qty)；**不含** Quote overlay |
| `pnl` | 累计浮盈（相对成本·mark） | (mark − cost)×qty |
| `display_price` | GET 时行情 overlay | QuoteService；失败则回退展示逻辑 |
| `today_pnl` | 持仓今日浮盈 | (display − pre_close)×qty；缺价则 null |
| `daily_pnl`（Dashboard） | 账户今日盈亏 | 权益 − 上一日报；**≠** Σ today_pnl |

## 2. 行情

| 项 | 现状 |
| --- | --- |
| realtime | `marketdata.Quote` via `IncludeDisplay=1` |
| refresh | 仅进页 / 手动刷新；无推送 |
| `FetchedAt` | Quote 已有；**未透出到 PositionView**（本阶段缺口） |
| freshness | 行级 **未**有 `price_freshness`；仅有 view `as_of` |

## 3. 前端用户看到什么

| UI | 用户可能以为 | 实际 |
| --- | --- | --- |
| 账户今日盈亏 | 盘中涨跌 | 日报权益差 |
| 估值价 / 行情价 | 已分列（A2） | 正确方向；缺行情时刻 |
| 累计浮盈 | 「今天赚了」 | 相对成本·mark |
| 持仓今日浮盈 | 单票今日 | 相对昨收·行情；正确但缺时间戳 |

**误解风险：** 中高（时间戳缺失 + 账户今日≠盘中）。

## 4. 审计裁定

- 账本链正确；问题在 **展示真相不足**。  
- 最小修复：透出 `quote_price` / `quote_timestamp` / `price_freshness`；强化文案；**禁止** quote 写 mark。

---

## 5. 设计（展示层 only）

| UI 概念 | 字段 | 规则 |
| --- | --- | --- |
| 账本估值价 | `mark_price` | 不变；驱动 equity / 累计浮盈 |
| 行情参考价 | `quote_price`（及既有 `display_price`） | 仅 live/open；缺失为 null，**不**回退 mark |
| 行情时间 | `quote_timestamp` | 优先 `Quote.FetchedAt` |
| 新鲜度 | `price_freshness` | FRESH / STALE / UNKNOWN（相对 view `as_of`，默认 5min） |
| 持仓今日浮盈 | `today_pnl` | `(quote − pre_close)×qty`；仅展示；≠ 账户日报差 |

**硬约束：** quote 永不写入 mark；Settlement / 日报 / Broker / Trade 路径不动。

---

*审计 + 设计完成 → 实现。*
