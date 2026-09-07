# PHASE17.2 Portfolio Data Truth Layer — Implementation Report

**日期：** 2026-09-07  
**范围：** 组合页展示语义澄清（display projection only）  
**审计：** `PHASE17_2_PORTFOLIO_DATA_TRUTH_AUDIT.md`

---

## 做了什么

### 后端 `portfolio/readmodel`

新增 PositionView 展示字段（不写账本）：

| 字段 | JSON | 含义 |
| --- | --- | --- |
| `QuotePrice` | `quote_price` | 仅 live/open 有值；**不**用 mark 冒充 |
| `QuoteTimestamp` | `quote_timestamp` | 优先 `Quote.FetchedAt` |
| `PriceFreshness` | `price_freshness` | `FRESH` / `STALE` / `UNKNOWN`（相对 `as_of`，阈值 5min） |

行为约束：

- `mark_price` / `market_value` / `equity` / `pnl` 仍只由账本 mark 计算。
- `today_pnl` = `(行情价 − 昨收) × qty`，且仅在 live/open 时计算（展示用）。
- Quote overlay **永不**回写 mark。

### 前端

- `portfolioSnapshot.ts`：映射 `quotePrice` / `quoteTimestamp` / `priceFreshness`
- `portfolioQuoteDisplay.js`：tooltip 含行情时间与新鲜度；`resolveQuotePriceOnly`
- `PortfolioDashboard.vue`：行情价列展示价格 + 时间（及陈旧提示）

---

## 验证

```text
go test ./backend/portfolio/readmodel/ -count=1  → ok
```

覆盖：

1. 有行情时 equity/mark 与 quote 无关（原有 + 扩展）
2. 行情刷新只更新 `quote_price` / `today_pnl`，mark/equity 不变
3. FetchedAt 过旧 → `STALE`；无行情 → `UNKNOWN` 且 `quote_price` 为空
4. Settlement / 日报路径未改（本阶段未触碰 settlement / account daily report 代码）

---

## 未改动（硬边界）

- Portfolio Snapshot 账本模型语义
- Settlement / `mark_price` 写入
- Paper Broker / Trade 执行
- 历史日报计算

---

## 用户可见语义（落地后）

| 列 / 指标 | 含义 |
| --- | --- |
| 估值价 | 账本 `mark_price`（Settlement / 成交写入） |
| 行情价 + 时间 | `quote_price` + `quote_timestamp`（参考） |
| 累计浮盈 | `(mark − cost) × qty` |
| 持仓今日浮盈 | `(行情 − 昨收) × qty` |
| 账户今日盈亏 | 仍为日报权益差（≠ 持仓今日浮盈合计） |

---

*Phase17.2 完成。*
