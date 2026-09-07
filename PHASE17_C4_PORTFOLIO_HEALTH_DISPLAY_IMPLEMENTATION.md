# PHASE17_C4 Portfolio Health Display — 实现报告

**日期：** 2026-09-07  
**阶段：** Phase17 C4  
**前置：** [PHASE17_C4_PORTFOLIO_HEALTH_DISPLAY_AUDIT.md](./PHASE17_C4_PORTFOLIO_HEALTH_DISPLAY_AUDIT.md) · C3 HealthScore  
**状态：** **完成（前端构建通过）**

---

## 1. 做了什么

在「我的组合」持仓列表只读展示 Holding Health：

| 列 | 内容 |
| --- | --- |
| **健康** | A 健康 / B 观察 / C 关注 / D 风险（可点击） |
| **健康摘要** | ✓ 支持因素 · ⚠ 风险因素（最多 3 行预览） |

点击等级或摘要 → **HoldingHealthDrawer**（只读）：

- 支持因素 / 风险因素  
- 信号来源（类型、价、时间、快照）  
- 评价时间 / 持仓天数  
- 明确文案：**非卖出建议 · 不生成买卖单**

**无**自动卖出按钮、**无**自动交易建议、**不**改持仓。

---

## 2. 数据流（不改交易链）

```text
GET /api/portfolio/snapshot     → 持仓事实
GET /api/portfolio/dashboard    → 摘要/成交（不变）
GET .../exit-evaluation         → HealthScore + Explanation（C2/C3，best-effort 合并）
```

- 未改 snapshot / dashboard / provenance DTO  
- 未改 Broker / TradePlan 执行  
- 未做 DB 迁移  

---

## 3. 主要文件

| 文件 | 作用 |
| --- | --- |
| `frontend/src/utils/holdingHealthDisplay.js` | 等级文案 / 摘要行 |
| `frontend/src/components/HoldingHealthDrawer.vue` | 只读解释抽屉（复用 ExplanationHeader） |
| `frontend/src/components/PortfolioDashboard.vue` | 列 + 合并 + 抽屉入口 |
| `frontend/src/api/paperObservation.ts` | 映射 `health_score` / `explanation` |

---

## 4. 验证

```text
node src/utils/holdingHealthDisplay.test.mjs  → ok
npm run build (frontend)                      → ok (exit 0)
```

---

## 5. 对交易链影响

**无影响。** 仅前端展示既有评价 API；既有人工「卖出」入口保留且与 Health 抽屉分离。

---

*Phase17 C4 Portfolio Health Display 实现结束。*
