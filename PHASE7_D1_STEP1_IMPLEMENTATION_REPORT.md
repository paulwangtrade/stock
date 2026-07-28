# Phase7-D.1 Step1 Implementation Report

> C-OBS-002：Paper Observation 持仓股票名称 API 层 enrich  
> 状态：**待审核**（未 git add / 未 commit）  
> 依据：`PHASE7_D1_IMPLEMENTATION_PLAN.md` Task 1

---

## 1. 目标与范围

| 项 | 说明 |
|---|---|
| 问题 | 持仓 API 返回 `stockName` 为空（DB `paper_sim_positions.stock_name` 常空） |
| 方案 | `GET /api/papertrading/dashboard/positions` 响应前只读 enrich |
| 允许 | `backend/api` 层 |
| 禁止 | 改 `papertrading` 模型/Overlay、写 DB、Broker/Settlement/Execution/TradePlan/Risk |

---

## 2. DTO `StockName` 来源确认（实施前）

| 环节 | 事实 |
|---|---|
| 字段 | `DashboardPositionRow.StockName`（`backend/papertrading/dashboard.go`） |
| 构建 | `BuildObservationPositionRows` 透传 `PaperSimPosition.StockName`（`observation.go` L122） |
| 落库 | fill 时自 `TradePlanItem.StockName` 写入；常为空 → 响应为空 |
| 结论 | **字段存在**；需在 API 响应层补展示名，不改 Observation 计算 |

---

## 3. 实现摘要

### 3.1 调用链（实施后）

```text
handleDashboardPositions
  → papertrading.GetDashboardPositions()     // 不变
  → enrichObservationPositionNames(view, nil) // 新增：仅 DTO
  → writeJSON(...)
       └─ defaultStockNameLookup
            └─ data.NormalizeStockCode + tushare_stock_basic（DB 只读）
```

### 3.2 变更文件

| 文件 | 变更 |
|---|---|
| `backend/api/stock_name_enrich.go` | 新增 `enrichObservationPositionNames`；import `papertrading` |
| `backend/api/paper_observation.go` | `handleDashboardPositions` 写 JSON 前调用 enrich |
| `backend/api/stock_name_enrich_test.go` | 新增 4 个单测 |

### 3.3 行为规则

| 规则 | 实现 |
|---|---|
| 有名保留 | `StockName` 非空跳过 |
| 空名 enrich | batch lookup 后填 DTO |
| 失败降级 | lookup 空 map → 保持空，不报错 |
| 不写库 | 仅改内存中 `view.Positions[i].StockName` |
| 无循环依赖 | enrich 在 `api` 包；`papertrading` 不 import `api` |

---

## 4. 测试

```text
go test ./backend/api/... -run EnrichObservation -count=1
→ ok  go-stock/backend/api  (~31s)
```

| 用例 | 结果 |
|---|---|
| 有名称时保持原值 | PASS |
| 无名称时 enrich | PASS |
| enrich 失败保持空 | PASS |
| nil view 不 panic | PASS |

**未改**：`backend/papertrading/observation_test.go`（Overlay 逻辑未动）。

---

## 5. 验收对照（C-OBS-002）

| 验收项 | 状态 |
|---|---|
| 响应 `positions[].stockName` 在 DB 空时可由 lookup 填充 | 已实现 |
| 不 UPDATE `paper_sim_positions` | 是 |
| 不调用外部行情 HTTP | 是（仅 `StockBasic` DB） |
| Handler 契约不变（路径/envelope） | 是 |
| 页面不阻塞（lookup 失败） | 是 |

---

## 6. 未实施（后续 Step）

| ID | 内容 |
|---|---|
| C-OBS-003 | T+1 展示文案（前端） |
| C-OBS-001 | quoteSource 用户可见化（前端） |
| 前端防御 | `stockName \|\| stockCode`（可选，Step2/3） |

---

## 7. 审核提示

- 仅 3 个 Go 文件变更；范围限于 Observation positions 只读 API。  
- 建议审核：enrich 挂点是否在 `handleDashboardPositions`；是否与 upcoming 共用 `defaultStockNameLookup` 行为一致。  
- **请勿在本步合并前 git add/commit**（按任务要求等待审核）。

```text
Step1 勾选
[x] StockName 来源已确认
[x] API 层只读 enrich
[x] 单测 4 例通过
[x] 未改 papertrading 核心 / DB / Broker 等
[ ] 审核通过 → Step2/3 或单独 commit
```
