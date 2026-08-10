# PHASE13-V.1 — Release Dependency Closure Report

> **切片：** Phase13-V.1  
> **日期：** 2026-08-10  
> **Branch：** `release/v0.1.0-beta`  
> **Base tip（闭包前）：** `19cd8e6`  
> **闭包 tip：** `f014ac7`  
> **禁止：** push / tag / 改 Broker·Gateway·Execution·Fill·Settlement

---

## 0. Verdict

| 项 | 结果 |
|----|------|
| 依赖审计 | `PHASE13_V1_RELEASE_DEPENDENCY_AUDIT.md` |
| 最小闭包入库 | **完成**（多 commit，仅编译必需） |
| Clean frontend build | **PASS** |
| Clean `go build` / `wails build` | **PASS** |
| Startup smoke（空目录） | **PASS**（schema READY → starting） |
| Push / Tag | **未执行** |

**一句话：** `v0.1.0-beta` 锚点 `19cd8e6` 不可 clean 编译；闭包至 **`f014ac7`** 后 clean worktree 可构建可启动。既有 tag **未**移动。

---

## 1. Commits（`19cd8e6..f014ac7`）

```text
f014ac7  decodeJSONBody helper
fd61cd6  ApproveDraftGate
054da34  TradePlanRepo morning writers
177a44c  morning materialize + tradingconfig + ExecutionSummaryView type
b92eb95  TradePlan model fields + riskreport sources degraded
2aaf64b  approvegate/readiness/qualitygate/marketstate/strategysnapshot + tradeplans handlers + audit
```

---

## 2. 纳入 vs 排除

### 纳入（Beta 运行 / 编译必需）

- `approvegate` · `readiness` · `qualitygate` · `marketstate` · `strategysnapshot`
- `tradeplans_{approve,freeze,readiness}.go` · `http_json.go`
- `tradingconfig`（morning position 只读 Risk）
- `morning_price/position_materialize`（tip `morning_intent` 已引用）
- `models.TradePlan` Intent/Pricing 字段
- `data/trade_plan_repo_{morning,approve_gate}.go`（companion，避开大 diff）
- `papertrading/execution_summary_view.go`（**仅类型**）
- `riskreport/sources.go` **降级**（不拉 Phase10 Observation Builders）

### 排除

- Phase10 Observation / ExecutionRead 大包、脏 UI、Broker/Gateway/Execution 改动
- `approvegate/data|logs`、未审查功能、`git add .`

---

## 3. Clean 验证

| 步骤 | 环境 | 结果 |
|------|------|------|
| Worktree | `D:\stock_wt_v1_closure` @ `f014ac7` | 干净 |
| `npm run build` | frontend | **PASS** (`FE_EXIT=0`) |
| `go build ./backend/...` | | **PASS** |
| `go build -o …/go-stock-closure.exe .` | | **PASS** |
| `wails build -skipbindings -s` | | **PASS** → `build/bin/go-stock.exe` |
| Smoke | 启动日志至 `schema validation READY` + `starting...` | **PASS** |

已知降级：Risk Report live sources = `degraded_beta_tip`（无 Holding/Exit/Exec Observation 聚合）。

---

## 4. Tag / 发布说明

| 项 | 状态 |
|----|------|
| tag `v0.1.0-beta` | 仍指向 **`19cd8e6`**（本切片 **未** 移动） |
| 可分发构建 | 应以 **`f014ac7`**（或其后）clean checkout 为准 |
| 下一步（另令） | 移动/新建 tag、push、发测包 |

---

## 5. 约束核对

| 约束 | |
|------|--|
| 未改 Broker/Gateway/Execution/Fill/Settlement | 遵守 |
| 未 push / 未 tag | 遵守 |
| 未 `git add .` | 遵守 |
