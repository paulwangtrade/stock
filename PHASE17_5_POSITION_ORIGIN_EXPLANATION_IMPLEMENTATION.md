# PHASE17.5 Portfolio Position Provenance Explanation — Implementation

**日期：** 2026-09-07  
**范围：** 「我的组合」单票「为什么买入」解释增强（展示层 only）

---

## 做了什么

| 项 | 说明 |
| --- | --- |
| `PositionOriginDrawer.vue` | 新抽屉：来源 chip（Strategy/Watchlist/Manual）、SignalPrice/SignalTime、TradePlan `source_session` |
| `positionOriginDisplay.js` | 纯展示投影；复用 Provenance DTO + 可选 plan session |
| `PortfolioDashboard.vue` | 来源 chip /「为什么买入」打开 Origin 抽屉；「完整溯源」仍进 `PortfolioProvenanceDrawer` |

**数据：** 仍 `GET /api/portfolio/positions/{code}/provenance`；Plan 来源用既有 `getTradePlanById` → `source_session`（不扩 provenance 模型）。

---

## 未改

- Provenance API / DTO  
- 交易能力、选股、Broker  
- `PortfolioProvenanceDrawer` 完整溯源 + Outcome Tab（保留）

---

## 验证

```text
node frontend/src/utils/positionOriginDisplay.test.mjs  → ok
```

---

*Phase17.5 完成。*
