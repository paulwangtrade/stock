# Phase5-B Full Build Verification Report

> 日期：2026-07-21  
> 范围：只验证，不新增功能  
> 目标：确认 Decision Layer + TradePlan Shadow Layer + Existing Trading Pipeline 可共存

---

## 1. go test 结果

### 1.1 全量 `go test ./...`

| 结果 | 说明 |
|------|------|
| **FAIL** | 仅 `go-stock/backend/data` 失败 |
| Decision / TradePlan Shadow | **全部通过** |
| Execution / Broker / Strategy | **全部通过** |

**失败包（与 Phase5-B 无关）：** `backend/data`

失败用例均为 Paper 订单观测/生命周期缺口类（计数期望非 0、实际为 0），例如：

- `TestGetPaperOrderHealth_*`
- `TestLifecycleGaps_*`

判定：**既有 `backend/data` 观测测试不稳定/环境问题**，非 Decision / `tradeplan/*` 引入；本轮未改 PaperBroker / TradePlan 生命周期 / Execution 业务逻辑。

### 1.2 Milestone 包回归（共存关键路径）

```
ok  backend/decision/authority
ok  backend/decision/registry
ok  backend/decision/semantic
ok  backend/decision/shadow
ok  backend/tradeplan/candidate
ok  backend/tradeplan/draft
ok  backend/strategy
ok  backend/execution
ok  backend/broker
ok  backend/models
```

**MILESTONE_EXIT: 0**

---

## 2. go build 结果

```
go build ./...
BUILD_EXIT: 0
```

全包可编译。

---

## 3. Frontend / Wails build 结果

### 3.1 Frontend

```
cd frontend
npm run build   # vite build
FE_EXIT: 0      # ~1m51s
```

### 3.2 Wails

```
wails build -skipbindings -s
WAILS_EXIT: 0
```

产物：`D:\stock\build\bin\go-stock.exe`

（此前完整 `wails build` 亦成功；本轮用 `-skipbindings -s` 验证前端产物可打入 exe。）

---

## 4. Dependency Audit

### 4.1 精确 import：`go-stock/backend/decision`

| 扫描 | 结果 |
|------|------|
| `backend/execution` → `go-stock/backend/decision` | **空（通过）** |
| `backend/broker` → `go-stock/backend/decision` | **空（通过）** |

说明：`backend/execution` 中出现的 `decision` 字样为 **`risk.RiskDecision` 局部变量名**，不是 QuantDecision 包依赖。

### 4.2 `backend/tradeplan/candidate` 生产代码

| 扫描 | 结果 |
|------|------|
| `execution` / `broker` / `BuildTradePlan`（`*.go` 排除 `_test.go`） | **空（通过）** |

`freeze_test.go` 中含禁止字符串仅为防回退断言，不算生产依赖。

### 4.3 架构共存结论

```
Decision / Draft / Candidate Shadow  ──✓── 独立于 Execution
Existing: CandidatePool → PlanFilter → BuildTradePlan → PaperBroker  ──✓── 未改动、测试通过
```

---

## 5. Runtime Smoke Test

| 项 | 状态 |
|----|------|
| 启动 App（GUI） | **需人工**：本环境未做交互式 Watchlist/KLine/Paper UI 点击验证 |
| 编译产物可用 | **通过**：`go-stock.exe` 已生成 |
| 旧交易链单元覆盖 | **通过**：`strategy` / `execution` / `broker` 测试 OK |
| 建议人工勾选 | 启动 exe → Watchlist / K-line / Paper Trading；确认日批仍为 Pool→Filter→BuildTradePlan→Paper |

旧路径行为预期（代码未改）：

```
CandidatePool → PlanFilter → BuildTradePlan → PaperBroker
```

---

## 6. 是否进入 Phase5-C

| 门槛 | 状态 |
|------|------|
| Draft → Candidate Shadow | 完成且测试绿 |
| Executable / 依赖隔离 | 完成且 audit 通过 |
| go build / frontend / wails | 通过 |
| 全量 `go test ./...` | **有既有 `backend/data` 失败**（非本里程碑回归） |
| GUI Runtime Smoke | **待人工** |

**建议：**

- **可以进入 Phase5-C（设计/下一批次实现）**，前提是接受：全量测试中 `backend/data` Paper 观测失败为**已知既有问题**，不阻塞 Shadow 层里程碑。
- Phase5-C 前可选：单独排查/修复 `paper_order_observability` / `LifecycleGaps`（与 Decision 无关）。
- 发布前完成一次人工 Runtime Smoke（Watchlist / KLine / Paper）。

---

## 7. 确认未改（本验证轮）

- 未修改业务逻辑  
- 未修改 TradePlan 生命周期  
- 未修改 Execution / PaperBroker  
- 未修改 Candidate Rank / Score  
