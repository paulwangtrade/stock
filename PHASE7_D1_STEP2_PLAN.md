# Phase7-D1 Step2 Plan

> 性质：只读实施计划（本阶段不写代码）  
> 目标：仅处理 `C-OBS-002`（Paper Observation 股票名称展示）  
> 约束：不改 `paper_sim_positions` / Fill / Broker / Settlement / T+1 计算 / TradePlan / Risk

---

## 1. 只读检查结论（按指定文件）

### 1.1 `paper_observation.go`

结论：
- 名称 enrich 已在 API 响应层调用：`handleDashboardPositions` 中 `enrichObservationPositionNames(view, nil)`。
- enrich 发生在 `GetDashboardPositions()` 之后、`writeJSON()` 之前，符合“响应前处理、只读”要求。
- 路由与只读边界不变（仅 `/status`、`/dashboard/*`）。

### 1.2 `stock_name_enrich.go`

结论：
- 名称来源链路：`stockCode` → `NormalizeStockCode` → `tushare_stock_basic(symbol,name)` 批量查询。
- 只对 `StockName` 为空的行补全，已有名称不覆盖。
- 失败降级：DB 不可用/查询失败/无法归一时返回空 map，不阻塞接口。
- 无写库逻辑，满足 C-OBS-002 约束。

### 1.3 `PaperTradingObservation.vue`

结论：
- 前端已展示名称列：`positionColumns` 中 `key: 'stockName'`。
- 数据来源是 `getPaperDashboardPositions()` 返回的 `positions.positions`。
- 当前未做前端 fallback（例如 `stockName || stockCode`）；空名时 UI 仍可能空白（仅在 enrich 未命中时出现）。

---

## 2. 名称来源 / DTO 字段 / 前端展示链路确认

### 2.1 名称来源

1. 主来源：`paper_sim_positions.stock_name`（由历史写入决定）  
2. 响应层补充来源：`tushare_stock_basic.name`（通过 `stock_name_enrich.go` 批量 lookup）  
3. 当前前端兜底：无（未写 `stockCode` 兜底显示）

### 2.2 DTO 字段

- 行级字段：`stockCode`、`stockName`（`DashboardPositionRow`）
- JSON 输出：`positions.positions[].stockCode` / `positions.positions[].stockName`

### 2.3 前端展示链路

`GET /api/papertrading/dashboard/positions`  
→ `paperObservation.ts` `mapPosition(raw)`  
→ `DashboardPositionRow.stockName`  
→ `PaperTradingObservation.vue` 名称列渲染

---

## 3. Step2 实施计划（仅 C-OBS-002）

> 说明：Step1 后端 enrich 已就位；Step2 聚焦“展示稳定性与验收收口”，不触碰交易逻辑。

### 3.1 计划项 A：展示兜底（前端）

目的：当 lookup 未命中时，避免名称列空白。

- 建议改动点：`PaperTradingObservation.vue` 的名称列 `render`
- 目标行为：优先显示 `stockName`，为空则显示 `stockCode`
- 影响范围：仅 Observation 展示层

### 3.2 计划项 B：可观测验证（不改后端逻辑）

- 使用现有接口回包，验证三类样本：
  1) DB 有名（应保持原值）
  2) DB 空名但 lookup 命中（应显示补全名）
  3) DB 空名且 lookup 失败（应显示代码兜底）

### 3.3 计划项 C：测试补充（展示层）

- 后端：沿用 Step1 单测（无需改业务逻辑）
- 前端：补 1 个最小渲染测试或手工验收脚本（名称列兜底）

---

## 4. 变更边界（Step2 执行时）

允许：
- `frontend/src/components/PaperTradingObservation.vue`（名称列渲染兜底）
- 可选：`frontend/src/api/paperObservation.ts`（若需要小型 helper，不改字段契约）

禁止：
- `backend/papertrading/*` 核心逻辑
- `paper_sim_positions` 数据结构/写入逻辑
- Fill/Broker/Settlement/Execution/TradePlan/Risk
- T+1 计算规则

---

## 5. 验收标准（C-OBS-002）

1. 名称非空时：显示原始 `stockName`  
2. 名称为空且 enrich 命中：显示补全名称  
3. 名称为空且 enrich 未命中：显示 `stockCode`（不空白）  
4. 不引入任何写库行为  
5. 不影响 C-OBS-001/C-OBS-003 的既有行为

---

## 6. 当前状态

```text
[x] 只读检查完成
[x] 名称来源/DTO/展示链路已确认
[x] Step2 计划已输出
[ ] 代码实施（后续单独执行）
[ ] git add / commit（本任务明确不执行）
```
