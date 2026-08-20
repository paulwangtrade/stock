# Phase12-D2-B OpportunityCard 实现报告

**日期**：2026-08-20  
**依据**：`PHASE12_D2_B_OPPORTUNITY_CARD_AUDIT.md`、`PHASE12_D2_A_IMPLEMENTATION_REPORT.md`  
**目标**：首页「今日机会」主对象改为候选，持仓只作对比，分数沿用现有 `opportunity_attention`。

未改 CandidatePool、Score Engine、TradePlan、数据库、后端 API。未新增 HTTP 接口。

---

## 1. 做了什么

首页不再用 `daily_attention.items[].stockCode` 当机会股。

`mapHome` 读取已有 `decision_summary.opportunity_attention`，经 `toOpportunityCards` 生成：

- `candidate_stock`：`code` / `name` / `display`（D2-A `toStockDisplay`）
- `holding_stock`：同上
- `candidate_score` / `holding_score` / `score_gap`（候选分 − 持仓分，不解析 reason）

一条 highlight 一张卡，最多 3 张。无 highlight 不出对比卡。

上屏：候选 `StockLink` + 候选评分 + 「超过当前持仓」+ 持仓 `StockLink` + 持仓评分 + 机会优势。点击复用首页已有 `StockKlineModal`。

缺名：`未知名称` + `000021.SZ`。禁止把持仓码或内部码当名称。名称仅从同一 home 包的 `position_attention` / attention `stock_name` 侧连。

---

## 2. 测试

前端：

```
node frontend/scripts/verify-opportunity-card.mjs
node frontend/scripts/verify-stock-display.mjs
```

均为 `ok`。语义用例：attention 的 `sz000021` 是持仓；卡片主对象为候选 `sz000858`，比较对象为 `sz000021`。

后端（本切片相关，未改 Go 源码）：

```
go test ./backend/portfolio/decision ./backend/portfolio/attention ./backend/portfolio/home
go test ./backend/api -run "TestInvestmentHome|TestDecisionSummaryAPI|TestPortfolioMiddleware_RoutesDecisionSummary"
```

均为 `ok`。

全量 `go test ./backend/api` 中有既有失败（`TestPortfolioObservationAPI_NormalBookNoMutation`、`TestPaperObservation_AssetMiddleware_DoesNotCaptureRun`），与本切片无关。

---

## 3. 修改文件

| 文件 | 说明 |
| --- | --- |
| `frontend/src/utils/opportunityCard.js` | Adapter |
| `frontend/src/api/investmentHome.ts` | 保留 opportunity_attention → `opportunityCards` |
| `frontend/src/components/InvestmentHome.vue` | 今日机会改读卡片 |
| `frontend/scripts/verify-opportunity-card.mjs` | 对象语义测试 |
| `PHASE12_D2_B_IMPLEMENTATION_REPORT.md` | 本报告 |

未改：`backend/**`、K 线组件、TradePlan API、评分包。

---

## 4. 未做（范围外）

- D2-C 计划 15:00 上下文
- 后端给 `candidate_name` / `holding_name`（缺名仍「未知名称」）
- 把 attention 的 `StockCode` 改成候选（前端已绕开）

---

## 5. 结论

「今日机会」对象语义已按审计纠正：候选为主、持仓为比较、分数来自现有 quality score 字段。可以进入 D2-C。
