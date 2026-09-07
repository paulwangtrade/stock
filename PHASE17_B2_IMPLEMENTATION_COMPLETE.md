# PHASE17-B.2 实现完成 — 组合持仓 → K 线工作台

**日期：** 2026-09-07  
**依据：** [PHASE17_B2_IMPLEMENTATION_AUDIT.md](./PHASE17_B2_IMPLEMENTATION_AUDIT.md)  
**范围：** B.2-A / B.2-B / B.2-C / B.2-D（最小前端接线）  

**原则遵守：** 未新增 API · 未改后端/模型/执行链 · 未改 `StockLightweightKlineChart` 核心 · 未复制 `SellDraftDialog`

---

## 1. 修改文件列表

| 文件 | 变更 |
| --- | --- |
| `frontend/src/components/PortfolioDashboard.vue` | 开 K 注入持仓上下文；Modal prepend 摘要；footer「卖出计划」；透传 `cost-price` / `cost-volume` / `position-aware-signals` |

**未改：** `StockKlineModal.vue`（插槽/attrs 已够）、图表引擎、backend、TradePlan、SellDraftDialog 实现。

---

## 2. 数据流变化

### 修改前

```text
Portfolio row
  → StockLink model (code/name only via adapt)
  → openStockKline(model)
  → klineModal { code, stockName }
  → StockKlineModal
  → Chart（无成本线 / 无摘要 / 无卖出）
```

### 修改后

```text
Portfolio row (snapshot 已有字段)
  → StockLink onOpen(model, row)
  → openStockKline(model, row)
  → klineModal {
        code, stockName,
        positionRow,
        costPrice ← avgCost,
        costVolume ← totalQty
      }
  → StockKlineModal
        ├─ #prepend  持仓摘要（名称/代码/数量/成本/现价/浮盈）
        ├─ attrs     cost-price / cost-volume / position-aware-signals
        │              → Chart 成本线（既有能力）
        └─ #footer   「卖出计划」→ openSellDialog(positionRow)
                       → 既有 SellDraftDialog
```

---

## 3. 功能对照

| 批次 | 实现 |
| --- | --- |
| B.2-A | `klineModal.positionRow` + cost/qty 自 snapshot row；无新请求 |
| B.2-B | `#prepend`：股票、数量、成本、现价（displayPrice）、浮盈（pnl） |
| B.2-C | `#footer`「卖出计划」→ `openSellDialog`；`canShowSellButton` 门控 |
| B.2-D | `:cost-price` / `:cost-volume` 透传既有图表 props；未重写 overlay |

---

## 4. 手工验收清单

| # | 步骤 | 预期 |
| --- | --- | --- |
| 1 | 我的组合 → 点击持仓股票 | K 线 Modal 打开（B.1 缓存下应快） |
| 2 | 看摘要区 | 显示数量、成本价、现价（有 overlay 时）、浮盈 |
| 3 | 看主图 | 成本线出现（成本&gt;0） |
| 4 | 可卖持仓 | footer「卖出计划」可点 → 既有 SellDraftDialog |
| 5 | 不可卖 / 锁定 | footer 不出现（或等同表列逻辑） |

---

## 5. 回归

| 项 | 结论 |
| --- | --- |
| K 线缓存 / LIVE 轮询 | **未改**图表加载与 `setupPoll` |
| 其他入口开 K（自选 `stock.vue` 等） | **未改**其 Modal 接线 |
| TradePlan / 执行链 | **未改**；卖出仍 `createTSellDraft` |
| Snapshot API | **未改** |

---

## 6. Build

```text
Set-Location D:\stock\frontend; npm run build
→ ✓ built（exit 0）
```

产物含 `PortfolioDashboard-*.js` 持仓摘要 / 卖出计划接线。
---

## 7. 暂缓（未做）

- Fill 成交点图标  
- 自动卖出评价  
- 新图表引擎 / Modal keep-alive  
- HoldingT 统一  

---

**Phase17-B.2 实现完成。**
