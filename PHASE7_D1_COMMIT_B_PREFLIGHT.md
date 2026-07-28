# Phase7-D1 Commit B Preflight

> 性质：最终只读检查 + 选择性提交前确认  
> 前置：Commit A = `fdf706c`  
> 目标：Paper Observation UI explainability（C-OBS-001 / 002 / 003）

---

## 1. 白名单核对

### 1.1 允许纳入（整文件）

| 文件 | 状态 | 结论 |
|---|---|---|
| `frontend/src/components/PaperTradingObservation.vue` | `??` | **允许** — C-OBS-001/002/003 展示 |
| `frontend/src/api/paperObservation.ts` | `??` | **允许** — 只读 HTTP mapping |
| `PHASE7_D1_STEP2_PLAN.md` | `??` | **允许** |
| `PHASE7_D1_STEP2_IMPLEMENTATION_REPORT.md` | `??` | **允许** |
| `PHASE7_D1_STEP3_IMPLEMENTATION_REPORT.md` | `??` | **允许** |
| `PHASE7_D1_STEP4_IMPLEMENTATION_REPORT.md` | `??` | **允许** |

### 1.2 允许纳入（必须局部 patch）

| 文件 | 工作区混入 | D1 可取片段 | 结论 |
|---|---|---|---|
| `frontend/src/router/router.js` | 4 条新路由 | 仅 `/paper-observation` | **可安全执行（需局部）** |
| `frontend/src/App.vue` | 4 个新菜单 + `GitCompareOutline` + 扩展 `directKeys` | 仅「模拟盘观察」菜单 + `directKeys` 加 `paperObservation` | **可安全执行（需局部）** |
| `frontend/components.d.ts` | 多组件自动注册 | 仅 `PaperTradingObservation` 一行 | **可安全执行（需局部）** |

### 1.3 禁止纳入（已确认出现在工作区 diff，不得 stage）

| 项 | 出现位置 |
|---|---|
| `/production-readiness` / `/recovery-readiness` / `/broker-reconcile` | `router.js` |
| 「生产就绪 / 恢复就绪 / Broker 对账」菜单 | `App.vue` |
| `GitCompareOutline` import | `App.vue` |
| BrokerReconcile / CandidatePool / ProductionReadiness / RealOrders / RecoveryReadiness / ResearchCandidatePool / TradePlanUpcoming / WatchlistStockGrid | `components.d.ts` |
| QuantTradingDashboard / TradePlan / Risk / Execution / Broker / Settlement 业务逻辑 | **本白名单候选中未包含这些逻辑文件的交易改动** |

---

## 2. git diff 检查摘要

### 2.1 整文件新增（无交易逻辑）

- `PaperTradingObservation.vue`：展示层（名称兜底、T+1 标签、价源标签/tooltip）；调用只读 API。
- `paperObservation.ts`：`fetch` + DTO map；无下单/风控/结算。

### 2.2 混合文件（不可整文件 add）

```text
router.js     +4 routes（仅 1 条属 D1）
App.vue       +3 非 D1 菜单 + 1 条 D1 菜单 + icon/directKeys 污染
components.d.ts  多行非 D1 注册 + 1 行 Observation
```

### 2.3 交易逻辑修改

| 禁止域 | 本预检结果 |
|---|---|
| QuantTradingDashboard | 未纳入候选 |
| TradePlan / Risk | 未纳入候选 |
| Execution / Broker / Settlement | 未纳入候选 |
| Observation 计算 / QuoteService | 未改（仅 UI 解释） |

**结论：不存在交易逻辑修改进入 Commit B 白名单。**

---

## 3. 功能覆盖确认（代码级）

| ID | 内容 | 位置信号 |
|---|---|---|
| C-OBS-002 | `stockName \|\| stockCode \|\| '—'` | `PaperTradingObservation.vue` L94–96 |
| C-OBS-003 | `hasFullyLockedT1` + T+1 标签/tooltip | L43+ / L402+ |
| C-OBS-001 | `quoteSourceLabel` + 价源 tooltip / overlay tag | L79+ / L147+ |

---

## 4. 预检结论

| 项 | 判定 |
|---|---|
| 白名单是否可安全执行 | **是**（整文件直接 add + 三混合文件局部采纳） |
| 能否 `git add` 整个 `router.js`/`App.vue`/`components.d.ts` | **否** |
| 推荐操作 | 构造 D1-only 暂存内容后 commit；工作区保留非 D1 dirty |
| 下一步 | 执行选择性 `git add` + commit |

```text
Preflight
[x] 白名单文件识别完成
[x] 混合文件污染已识别
[x] 无交易逻辑进入白名单
[x] 可继续 Commit B（局部 stage）
```
