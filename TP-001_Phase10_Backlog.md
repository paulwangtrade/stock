# TP-001 — TradePlan Upcoming Selection Semantics

> **ID:** TP-001  
> **Priority:** P2（产品语义 / UX；不阻断后端物化与执行链）  
> **Status:** Accepted debt — tracked; **not fixed in this note**  
> **Target:** Phase10+ TradePlan UX / Upcoming 语义澄清或可配置偏好  
> **Nature:** Tracking only — no code / API / schema change in this document  
> **Date:** 2026-08-04  
> **Evidence:** 只读诊断（runtime DB `build/bin/data/stock.db`；`TradePlanUpcoming.vue` + `GetUpcomingTradePlan`）

---

## 1. Problem

Upcoming 默认选择规则与用户期待的「**当前操作计划**」（通常为最新生成 / 正在处理的 Draft）不一致。

### 1.1 Current server rule

`GET /api/tradeplans/upcoming` → `GetUpcomingTradePlan(today)`：

1. **Frozen 优先**（`status=ready AND freeze_at IS NOT NULL`，`trade_date >= today`）  
2. 否则 Draft：`trade_date >= today`  
3. 排序：

```text
ORDER BY trade_date ASC, plan_version DESC, id DESC
```

即：**最早即将到来的交易日**优先，同日再取高 version / 高 id。

### 1.2 Observed effect

| 期待 | 实际 |
|------|------|
| 冷启动看到刚生成的 **2026-08-05** Draft（如 #19） | 若今日仍是 **2026-08-04** 且存在当日 Draft（如 #15），Upcoming 命中 **08-04** |
| 「最新生成计划」= 当前操作对象 | 「最早 upcoming 日」的 Draft/Frozen |

旧日期 Draft **覆盖**新生成计划的可见性（默认查询无 `trade_date` / 无持久化偏好时）。

### 1.3 UI 侧叠加

- `preferredGeneratedPlanId` / `preferredGeneratedTradeDate` 仅在本次会话「生成明日计划」成功后写入内存  
- **无 localStorage / 无重启恢复**  
- 冷启动 `tradeDateInput=''` → 无 query → `today=本地日历日` → 走上述 ASC 规则  

生成后短时焦点修复（postGenerate helpers）**不改变** Upcoming 服务端语义，也不跨会话。

---

## 2. Impact

| Area | Effect |
|------|--------|
| TradePlan 页初始加载 | 易显示「旧日 Draft」，用户以为 08-05 未生成 |
| Approve / 早盘物化 / Freeze | 操作对象可能不是刚生成的计划（除非手改日期或本会话刚 generate） |
| 支持/排障 | 易误判为 generate-next 失败或 DB 未写入 |

**等级：P2** — 数据与 API 按既有设计工作；问题是 **选择语义 vs 操作意图**。

---

## 3. Non-goals（本记录）

- 不修改 `GetUpcomingTradePlan` / generate-next / 表结构  
- 不在本条实现「默认最新 generated_at」或持久化 preferred  
- 不自动 Approve / Freeze  

---

## 4. Candidate directions（Phase10+，仅设计备忘）

任选其一或组合，需单独立项，避免 silently 改 Upcoming 契约：

1. **文档化现状**：Upcoming =「下一可交易日视图」，操作最新计划须显式 `trade_date`  
2. **UI 持久化**：保存 `preferredGeneratedPlanId` + `trade_date`（重启恢复焦点；仍不改服务端 ASC）  
3. **双模式 API**：`mode=upcoming`（现语义）vs `mode=latest_draft`（`generated_at DESC` / 最新 trade_date）  
4. **默认 query = next_trading_day**：今日盘后生成次日计划时，冷启动更贴近「明日 Draft」（仍可能与多日未关闭 Draft 并存问题）

---

## 5. Related

| ID / Area | Relation |
|-----------|----------|
| Phase10-A 早盘物化 UI | 按钮/操作依附当前 Upcoming 命中计划；选错日则物化错对象 |
| postGenerate UI helpers | 会话内缓解；不消除 TP-001 |
| BUILD-001 等 | 无关；本条为产品选择语义 |

---

## 6. Registry line

```text
TP-001  P2  OPEN  — Upcoming ASC 旧日 Draft 盖住新生成计划；待 Phase10+ UX/语义立项
```
