# Phase7-A3.1 HTTP Split Implementation Report

> 基线：`825cb3b`；设计：`PHASE7_A3_1_READONLY_HTTP_DESIGN.md`  
> 未 `git add` / 未 commit

---

## 1. 修改文件

| 文件 | 动作 |
|---|---|
| `backend/api/paper_observation.go` | **新增** — 只读 handler（status + dashboard/*） |
| `backend/api/paper_observation_test.go` | **新增** — 路径注册 / GET-only / `/run` 不捕获 |
| `backend/api/papertrading.go` | **修改** — Dashboard/status 移出；`/run` + `reports/daily` 保留；middleware 分派观察路径 |

未改：`observation.go` / `dashboard.go` / Broker / Job / Settlement / realtime_price / OpenQuote / Execution / TradePlan / DB / 前端 API URL。

---

## 2. 调用链变化

### 之前
```text
PaperTradingAssetMiddleware
  → PaperTradingHandler（混有 /run + dashboard）
```

### 之后
```text
PaperTradingAssetMiddleware
  ├─ observation paths → PaperObservationHandler
  │     GET status / dashboard/today|positions|runs
  │     → GetTodayStatus / GetDashboardToday / GetDashboardPositions / ListRuns
  │     → Overlay（825cb3b，未改）
  └─ 其它 /api/papertrading/* → PaperTradingHandler
        POST /run → PaperTradingJob（原位保留，未改逻辑）
        GET reports/daily → ListDailyReports（仍在原文件）
```

前端 URL **不变**：

- `/api/papertrading/dashboard/today`
- `/api/papertrading/dashboard/positions`
- `/api/papertrading/dashboard/runs`

DTO 信封不变：`{ code, ok, today|positions|runs }`。

---

## 3. 测试结果

```text
go test ./backend/api/ -count=1 -run "PaperObservation|PaperTradingDashboard"
→ ok   go-stock/backend/api   18.356s

go build ./backend/api/
→ ok
```

覆盖：

- 只读路径注册；POST → 405
- Observation middleware **不**捕获 `/run`（fall-through）
- 既有 `TestPaperTradingDashboard_GETOnly` 仍通过（`RegisterPaperTradingRoutes` 内部调用 `RegisterPaperObservationRoutes`）

前端 URL 核对：`paperObservation.ts` 中 dashboard 路径未改。

---

## 4. 边界确认

| 检查 | 结果 |
|---|---|
| `paper_observation.go` 无 `PaperTradingJob` / Broker / Settlement / OpenQuote | ✅ |
| POST `/run` 仍在 `papertrading.go`，逻辑未改 | ✅ |
| `GetDashboardPositions` / Overlay 未改 | ✅ |
| 前端 URL / DTO 形状保持 | ✅ |
| `reports/daily` 未并入只读文件（仍依赖 Engine 侧 daily_report） | ✅ |
| git add / commit | **未执行** |
