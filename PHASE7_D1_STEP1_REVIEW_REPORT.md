# Phase7-D1 Step1 Review Report

> 性质：只读检查  
> 范围：C-OBS-002（Observation 持仓名称 enrich）运行链路复核  
> 约束：未修改代码 / 未 git add / 未 commit

---

## 1) `backend/api/stock_name_enrich.go` 复核

### 1.1 是否会产生 N+1 lookup

结论：**不会产生典型 N+1 DB 查询**（单次请求内是批量查一次）。

- `enrichObservationPositionNames()` 先收集所有空 `StockName` 且有 `StockCode` 的行到 `need`。
- 然后只调用一次 `lookup(need)`。
- `defaultStockNameLookup()` 使用一条 `WHERE symbol IN ?` 批量查询 `tushare_stock_basic`。
- 同一次调用中还有 `seenSym` 去重，避免重复 symbol 放大查询集合。

### 1.2 `defaultStockNameLookup` 缓存 / 降级机制

结论：

- **缓存**：当前实现**无跨请求缓存**（每次调用即算即查）。
- **降级**：有，且是静默降级：
  - `db.Dao == nil` 或 `codes` 为空：直接返回空 map。
  - `NormalizeStockCode` 失败：跳过该 code。
  - DB 查询失败：返回空 map。
  - enrich 阶段拿不到 name：保持原值（空），不抛错、不阻塞页面。

---

## 2) API 输出字段确认（`GetDashboardPositions`）

链路：

`handleDashboardPositions`  
→ `papertrading.GetDashboardPositions()`  
→ `enrichObservationPositionNames(view, nil)`  
→ `writeJSON(..., "positions": view)`

字段确认（来自 `DashboardPositionRow` JSON tag）：

- `stockCode`（`json:"stockCode"`）
- `stockName`（`json:"stockName"`）

结论：**JSON 字段名正确且稳定**，是 `stockCode` / `stockName`，不是 `name`。

---

## 3) Frontend 接收与展示确认

### 3.1 `paperObservation.ts`

已接收并映射：

- `DashboardPositionRow.stockName: string`
- `mapPosition()` 中 `stockName: str(raw?.stockName)`

### 3.2 `PaperTradingObservation.vue`

已展示：

- 持仓表列定义：`{ title: '名称', key: 'stockName', ... }`
- 数据源：`positions.positions`（来自 `getPaperDashboardPositions()`）

结论：前端**已接收并展示** `stockName`，当前不缺接线。

---

## 4) 需要修改的位置（仅在“未接收/未展示”时）

本次检查结果为“已接收且已展示”，因此**本项无必改位置**。

---

## 5) 回归校验（只读执行）

- 执行：`go test ./backend/api/... -run EnrichObservationPositionNames -count=1`
- 结果：`ok   go-stock/backend/api`

---

## 6) 结论

- C-OBS-002 Step1 的 API 层 enrich 链路已生效。
- 无 N+1 查询问题（请求内单次批量 lookup）。
- 有降级，无缓存（这是当前实现特征，不是本次缺陷）。
- 前端已完成 `stockName` 接收与展示链路。
