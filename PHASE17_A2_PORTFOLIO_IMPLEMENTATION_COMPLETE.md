# PHASE17-A.2 Portfolio Dashboard 字段语义增强 — 实现完成

**日期：** 2026-09-06  
**依据：** [PHASE17_A2_PORTFOLIO_DASHBOARD_AUDIT.md](./PHASE17_A2_PORTFOLIO_DASHBOARD_AUDIT.md)  
**方案：** **A（纯前端展示）** — 不改交易链 / 持仓模型 / Go 计算公式 / 不新增 API  

---

## 1. 修改文件

| 文件 | 变更 |
| --- | --- |
| `frontend/src/components/PortfolioDashboard.vue` | 账户/持仓盈亏改名 + tooltip；`daily_pnl_basis` 标签；页眉/持仓区 `as_of` |
| `frontend/src/api/portfolioSnapshot.ts` | 映射已有 `as_of` / `updated_at`（只读） |
| `frontend/src/api/portfolioDashboard.ts` | 映射已有 `as_of`（只读） |
| `frontend/src/components/InvestmentHome.vue` | 「我的资产」补「账户今日盈亏」+ tooltip（审计 P1） |

**未改：** Go portfolio 计算、mark 写入、schema、卖出/成交链、provenance API。

---

## 2. 语义落地对照

### 2.1 账户层

| 项 | 实现 |
| --- | --- |
| 文案 | 「今日盈亏」→ **账户今日盈亏** |
| Tooltip | 相对上一交易日结算权益变化；含成交/日终估值；**不是**盘中持仓浮盈；**≠** 持仓今日浮盈之和 |
| Basis | 展示已有 `daily_pnl_basis`：`日报差` / `暂无上一日报` |
| 绑定 | 仍 `dashboard.summary.daily_pnl`（无重算） |

### 2.2 持仓层

| 列 | 文案 | 口径（未改公式） |
| --- | --- | --- |
| `today_pnl` | **持仓今日浮盈** | (行情 − 昨收) × 数量 |
| `pnl` | **累计浮盈** | (mark_price − 成本) × 数量 |

### 2.3 时间语义

| 展示 | 来源 |
| --- | --- |
| 页眉「数据截至 …」 | snapshot `as_of` → `updated_at` → dashboard `as_of` |
| 持仓区旁注「截至 …」 | 同上 |

说明：`as_of` 为快照组装时刻，**不是**逐行 Quote 时间戳（方案 B 未做）。

### 2.4 首页

「我的资产」增加 **账户今日盈亏**（既有 `portfolio.dailyPnl`）+ 口径副文案，避免与组合页语义分叉。

---

## 3. 验收

| # | 项 | 结果 |
| --- | --- | --- |
| 1 | 字段语义正确 | **PASS**（命名/tooltip/basis/as_of 按审计方案 A） |
| 2 | 无 API 500 | **逻辑 PASS**（仅读既有字段；未改后端） |
| 3 | 不影响模拟交易 | **PASS**（无卖出/成交/mark 写入改动） |
| 4 | build | **PASS** — `npm run build` exit 0（~2m 5s） |

```text
Set-Location D:\stock\frontend; npm run build
→ ✓ built in ~2m 5s
```

---

## 4. 停止边界（已遵守）

- 未新建 portfolio 接口  
- 未改持仓模型 / DB  
- 未改账户 `daily_pnl` 或持仓 `today_pnl` / `pnl` 计算  
- 未用行情 overlay 改总权益  

---

## 5. 后续可选（非本切片）

- 方案 B：透出 Quote `FetchedAt` 做「行情截至」  
- P2：盈亏家数、仓位比、行级行情/估值 chip 强化  

**Phase17-A.2 实现完成。**
