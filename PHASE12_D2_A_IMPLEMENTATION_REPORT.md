# Phase12-D2-A StockDisplay 基础展示层实现报告

**日期**：2026-08-20  
**依据**：`PHASE12_D1_DASHBOARD_DISPLAY_ARCHITECTURE_DESIGN.md`  
**范围**：只实现统一股票展示能力，并接到首页「我的计划」做最小验证。

未改交易逻辑、数据库、TradePlan API；未新增 K 线页面 / `/stock/chart`；未改 `StockKlineModal` / `StockLightweightKlineChart`。

---

## 1. 现有能力审计（只读）

### 1.1 SECURITY_SHORT_NAME / 名称 Resolver

| 位置 | 作用 | D2-A 是否改动 |
| --- | --- | --- |
| `backend/strategy/universe.go` | 策略入池时读 `SECURITY_SHORT_NAME` | 否 |
| `backend/stockname.ResolveWithDB` | 服务端代码 → 短名（计划/池/行情等） | 否 |
| `backend/api/stock_name_enrich.go` `enrichUpcomingItemNames` | upcoming item 补 `stock_name` | 否 |

前端展示层**不直连** resolver。D2-A 只消费调用方已有的 `name` / `stock_name`（计划预览走 upcoming 已 enrich 的名称）。缺名时显示「未知名称」，禁止把 `sz000021` 当名称。

### 1.2 symbol formatter

`frontend/src/utils/stockCode.js` 的 `toEastMoneyCode`：`sz000021` → `000021.SZ`。`StockDisplay` 复用该函数，不复制一套市场推断。

### 1.3 StockKlineModal

现成弹窗：`title` + attr `code`（东财码）+ `stockName`，内部仍是 `StockLightweightKlineChart`。`HotStockList.openKline` 已是正确样板。D2-A 只在首页挂**一份**同样的 Modal，不改组件本身。

---

## 2. 实现契约

### 输入

- 规范：`{ market, symbol, name? }`
- 兼容：`{ stock_code, stock_name? }`（新浪码 / 东财码 / 6 位数字）

### 输出（`toStockDisplay`）

| 字段 | 示例 |
| --- | --- |
| `display_name` | 深科技（空则「未知名称」） |
| `display_code` | `000021.SZ` |
| `click_action.type` | `kline_modal` |
| `click_action.chart_code` | `000021.SZ`（给现有图表） |
| `click_action.title` | `深科技 000021.SZ — 日K` |
| `stock_code` | `sz000021`（不上屏） |

组件：

- `StockDisplay.vue`：只展示 name + code  
- `StockLink.vue`：点击 `emit('open', model)`  
- 页面 host：`applyStockClickAction` → 已有 `StockKlineModal`

---

## 3. 最小验证页

接入 **首页「我的计划」**，不用「今日机会」。

原因：今日机会的 `daily_attention.stock_code` 仍是**持仓码**（D1 已定性）。若 D2-A 把它包成 `StockDisplay`，会把错误对象显示得更像「机会股」。该问题留给 **D2-B OpportunityCard**。

计划侧 upcoming 已有 `stock_name`，把 `sz001232、sz301717` 换成「名称 + `001232.SZ`」，点击打开现有 K 线弹窗。查询仍是 `getUpcomingTradePlan(tradeDate)`，DTO 不变。

---

## 4. 测试

```
node frontend/scripts/verify-stock-display.mjs
```

结果：`verify-stock-display: ok`

覆盖：新浪码 / market+symbol / 缺名 / 名称等于内部码时丢弃 / 东财码 / 6 位数字 / 非法输入 / `kline_modal` 写入弹窗 target。

---

## 5. 修改文件

| 文件 | 说明 |
| --- | --- |
| `frontend/src/utils/stockDisplay.js` | `toStockDisplay` / `applyStockClickAction` |
| `frontend/src/components/StockDisplay.vue` | 展示 |
| `frontend/src/components/StockLink.vue` | 点击 |
| `frontend/src/components/InvestmentHome.vue` | 计划预览 + 共享 Modal |
| `frontend/scripts/verify-stock-display.mjs` | 纯函数测试 |
| `PHASE12_D2_A_IMPLEMENTATION_REPORT.md` | 本报告 |

未改：`backend/**`、`tradeplans.go`、K 线组件、路由。

---

## 6. 后续接口（D2-B / D2-C）

### D2-B OpportunityCard

消费本层：`candidate_stock` / `holding_stock` 均为 `StockDisplayModel`。

首页应改读 `decision_summary.opportunity_attention`（`candidate_code` + `highlights[]`），**不要**把 attention 的 holding `stock_code` 当主对象。点击复用首页已挂的 `openStockKline`。

本切片冻结的字段：`candidate_stock`、`holding_stock`、`candidate_score`、`holding_score`、`score_gap`。

### D2-C Plan Context

不改 `GET /api/tradeplans/upcoming`。两次 upcoming 填 `current_plan` / `next_plan`，15:00 选 `active`。计划预览继续用 `StockLink` + `toStockDisplay({ stock_code, stock_name })`。D2-A 的 `planStocks` 只需把数据源从「单次 upcoming」换成 `display.plan.items`。

---

## 7. 结论

StockDisplay 基础层已落地，首页计划预览可验证名称 + `000021.SZ` + 现有 K 线弹窗。机会对比语义与收盘后计划切换未做，分别属 D2-B / D2-C。
