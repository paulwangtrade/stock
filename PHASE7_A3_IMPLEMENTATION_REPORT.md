# Phase7-A3 Paper Observation Quote Overlay Implementation Report

> 性质：实现完成（未提交）  
> 范围：仅 Dashboard 查询路径（只读）  
> 日期：2026-07-27

---

## 1. 实现目标与约束达成

本次实现满足目标链路：

```text
QuoteService
    ↓
Paper Observation Overlay
    ↓
Dashboard DTO
```

并满足硬约束：

- 仅作用于 `GetDashboardPositions` 查询路径
- 实时行情仅用于展示，不写数据库
- 失败自动 fallback：`Quote.Price -> Quote.Open -> Persisted Mark`
- 不修改 Fill / OpenQuote / Execution / Settlement / TradePlan / Risk

---

## 2. 代码落点（白名单实现文件）

后端（A 类核心）：

- `backend/papertrading/models.go`
- `backend/papertrading/config.go`
- `backend/papertrading/query.go`
- `backend/papertrading/dashboard.go`
- `backend/papertrading/observation.go`

前端适配（DTO 透传与展示兼容）：

- `frontend/src/api/paperObservation.ts`
- `frontend/src/components/PaperTradingObservation.vue`

---

## 3. 关键行为说明

### 3.1 Overlay 价格来源

在 `observation.go` 中，展示价解析为：

1. `Quote.Price > 0` -> `QuoteSourceLive`
2. 否则 `Quote.Open > 0` -> `QuoteSourceOpenFallback`
3. 否则 -> `PersistedMarkPrice` (`QuoteSourcePersisted`)

### 3.2 只读保障

- `BuildObservationPositionRows` 只构造 DTO 行数据，不执行任何 `UPDATE/INSERT`
- `GetDashboardPositions` 仅读取账户与持仓，再叠加 Overlay 计算
- `paper_sim_positions.mark_price` 保持持久化值，仅通过 `persistedMarkPrice` 透传

### 3.3 DTO 结果

`DashboardPositionRow` 已包含：

- `displayPrice`
- `persistedMarkPrice`
- `marketValue`
- `unrealizedPnl`
- `returnRate`
- `quoteUpdatedAt`（作为行情更新时间；与持仓 `updatedAt` 并存）

兼容字段：

- `markPrice` 继续保留，且设置为 `displayPrice`，前端“当前价”列无需改 key

---

## 4. 公式与结果口径

Overlay 行级计算：

- `MarketValue = DisplayPrice * TotalVolume`
- `UnrealizedPnL = (DisplayPrice - AvgCost) * TotalVolume`
- `ReturnRate = (DisplayPrice - AvgCost) / AvgCost`（`AvgCost > 0`）

账户级保持双口径：

- `marketValue/equity/unrealizedPnl`：库快照
- `observationMarketValue/observationEquity/observationUnrealizedPnl`：Overlay 汇总

---

## 5. 单元测试覆盖

已覆盖并通过：

- Quote 正常返回（live）
- Quote 失败 fallback（persisted）
- PnL 计算正确
- ReturnRate 计算正确
- DB Mark 不变化（Overlay 不写库）

执行命令：

```text
go test ./backend/papertrading/ -count=1 -run "TestBuildObservationPositionRows_|TestGetDashboardPositions_QuoteOverlayDoesNotWriteDB"
```

执行结果：

```text
ok  	go-stock/backend/papertrading	18.437s
```

---

## 6. git status 与暂存约束

本次检查结果（相关路径）：

```text
?? backend/api/papertrading.go
?? backend/papertrading/
?? frontend/src/api/paperObservation.ts
?? frontend/src/components/PaperTradingObservation.vue
```

约束确认：

- 不使用 `git add -A`
- 仅允许白名单逐文件暂存

推荐白名单（后续如需提交）：

```text
backend/papertrading/models.go
backend/papertrading/config.go
backend/papertrading/query.go
backend/papertrading/dashboard.go
backend/papertrading/observation.go
backend/papertrading/observation_test.go
frontend/src/api/paperObservation.ts
frontend/src/components/PaperTradingObservation.vue
PHASE7_A3_IMPLEMENTATION_REPORT.md
```

> 注意：`backend/api/papertrading.go` 当前仍与 `/run` 相关执行入口同文件，若坚持“仅 Dashboard 查询路径”边界，本轮不建议纳入提交，建议后续拆分 handler 后再独立入仓。

---

## 7. 结论

可以进入下一步（提交准备/分批提交）：

- Overlay 实现完整
- 读路径隔离满足要求
- 测试通过
- 无交易执行链路改动
- 无数据库写入副作用

