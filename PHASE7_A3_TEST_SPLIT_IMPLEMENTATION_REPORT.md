# Phase7-A3 Observation 测试边界拆分实施报告

> 性质：实施完成（未 `git add` / 未 commit）  
> 日期：2026-07-27  
> 约束：仅修改测试文件；未修改任何生产代码

---

## 1. 修改文件列表

本次实际修改：

- `backend/papertrading/observation_test.go`

本次未新增文件：

- `backend/papertrading/observation_fixture_test.go`（评估后无需新增）

---

## 2. 删除的跨模块测试依赖

已从 `observation_test.go` 删除对以下跨模块依赖的调用：

- `setupTestDB`
- `enablePaperTrading`
- `seedFrozenPlan`
- `buyItem`
- `PaperTradingJob`

说明：

- 以上 helper/function 均来自/耦合 `broker_test.go` + Paper Engine 路径
- 已移除集成型用例 `TestGetDashboardPositions_QuoteOverlayDoesNotWriteDB`
- `observation_test.go` 现仅保留纯内存、纯计算、fake QuoteService 的 Unit 测试

---

## 3. 新测试覆盖范围（Observation Unit）

当前 `observation_test.go` 覆盖：

- Quote Overlay 正常返回（live）
- Quote 失败 fallback（persisted）
- Open fallback（`Price=0` 时回退 Open）
- DisplayPrice 计算正确
- MarketValue 计算正确
- UnrealizedPnL 计算正确
- ReturnRate 计算正确
- 只读不写语义：`BuildObservationPositionRows` 不修改输入 `PaperSimPosition`（内存 fixture 前后一致）

---

## 4. 测试结果

执行命令：

```text
go test ./backend/papertrading/ -count=1
```

执行结果：

```text
ok  	go-stock/backend/papertrading	22.054s
```

---

## 5. 约束遵守确认

- 未修改生产代码：
  - `observation.go`
  - `dashboard.go`
  - `models.go`
  - `query.go`
- 未修改禁改模块：
  - broker / settlement / realtime_price / job / execution
- 未执行：
  - `git add`
  - `git commit`

