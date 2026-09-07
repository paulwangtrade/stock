# PHASE17_C4 Portfolio Health Display — 实现前审计

**日期：** 2026-09-07  
**阶段：** Phase17 C4（只展示）  
**依据：** PHASE17_C3_HOLDING_HEALTH_SCORE_IMPLEMENTATION.md  

---

## 1. 当前「我的组合」能力

| 表面 | 实现 | 已有字段 |
| --- | --- | --- |
| 持仓列表 | `PortfolioDashboard.vue` ← `GET /api/portfolio/snapshot` | `stock_code`、数量、成本、mark、行情、市值、累计浮盈、收益率、今日浮盈、持仓状态 |
| Dashboard 摘要 | `GET /api/portfolio/dashboard` | 权益、现金、成交、风险标签 |
| 来源 | 前端 chip + `provenance` / TradePlan | Strategy / Watchlist / Manual（浅标签） |
| Provenance | `GET /api/portfolio/positions/{code}/provenance` + Drawer | 仓位 / 成交 / Origin 信号块 |
| 评价（后端已有，组合页未接） | `GET /api/papertrading/observation/holdings/exit-evaluation` | Exit state + **C2 Explanation** + **C3 HealthScore** |

**缺口：** 组合持仓表 **未展示** `health_score` / 解释摘要；ExitReviewDrawer 带复评/卖出草稿，**不宜**直接作为本阶段 Health 入口（禁止自动卖出按钮）。

---

## 2. 可复用、不新增表

| 数据 | 复用方式 |
| --- | --- |
| `HoldingHealthScore` | Exit-evaluation JSON：`grade` / `score` / `supporting_factors` / `risk_factors` |
| `PositionEvaluationExplanation` | 同上 `explanation` / `health_score.explanation` |
| Explanation UI kit | `ExplanationHeader` 等只读组件 |
| Provenance Drawer | **保留**「详情」入口；Health 用独立只读 Drawer，不混卖出 |

**不改：** portfolio dashboard/snapshot DTO、交易执行、TradePlan、DB schema。

---

## 3. C4 展示方案

1. 前端并行拉取 exit-evaluation（best-effort）。  
2. 按 `stock_code` 合并到持仓行。  
3. 列：**健康等级** + **原因摘要**。  
4. 点击等级 → **HoldingHealthDrawer**（只读：支持因素 / 风险 / 信号 / 评价时间）。  

---

*审计结束 — 进入实现。*
