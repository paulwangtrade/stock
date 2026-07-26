# Phase7-A1 Commit Plan

> 性质：**提交准备文档** — 本文件不执行 `git add` / `git commit`  
> 依据：`PHASE7_A1_IMPLEMENTATION_REPORT.md`、`PHASE7_A1_BOUNDARY_REVIEW.md`  
> 基线：`v0.6.7-snapshot-freeze` @ `f1951b75b6ef172dac4d923906438d200d64df13`  
> 分支：`dev`

---

## 1. 是否满足 commit 条件

**是 — 满足选择性 milestone commit 条件。**

| 门禁 | 状态 |
|---|---|
| A1 仅为新增文件，零已跟踪文件改动 | Pass |
| 边界无交易域污染 | Pass（见 Boundary Review） |
| `go test ./backend/marketdata/...` | Pass |
| `go vet ./backend/marketdata/...` | Pass |
| `go build ./backend/...` | Pass |
| 无调用方迁移 | Pass |
| 无 schema / DB 改动 | Pass |
| 可用路径级 add，避免 dirty workspace 污染 | Pass（禁止 `git add -A`） |

注意：工作区另有约 591 条非 A1 dirty；**不得**一并提交。

---

## 2. 推荐 Commit

### Message

```text
Phase7-A1: add market data service interfaces and legacy adapters
```

### 包含文件（仅此清单）

```text
backend/marketdata/interfaces.go
backend/marketdata/kline_service.go
backend/marketdata/quote_service.go
backend/marketdata/marketdata_test.go
backend/marketdata/adapter/eastmoney_kline_adapter.go
backend/marketdata/adapter/legacy_quote_adapter.go
PHASE7_A1_IMPLEMENTATION_REPORT.md
PHASE7_A1_BOUNDARY_REVIEW.md
PHASE7_A1_COMMIT_PLAN.md
```

等价路径级添加：

```powershell
Set-Location D:\stock
git add -- `
  backend/marketdata `
  PHASE7_A1_IMPLEMENTATION_REPORT.md `
  PHASE7_A1_BOUNDARY_REVIEW.md `
  PHASE7_A1_COMMIT_PLAN.md
git diff --cached --stat
git diff --cached --name-only
# 人工确认仅上述路径后：
# git commit -m "Phase7-A1: add market data service interfaces and legacy adapters"
```

### 提交前自检（必须）

`git diff --cached --name-only` **不得**出现：

- `backend/data/**`（除 marketdata 外）
- `backend/strategy/**`
- `backend/execution/**`
- `backend/models/trade_plan.go` 等 trade 相关
- `paper_*` / risk / position 业务文件
- `frontend/**`
- `app.go` / `main.go` / `main_schema_levels*`
- `tmp_*` / schema / migration
- 任何其它 dirty 文件

`git diff --cached` 中不得出现对已跟踪文件的修改（应为纯 `new file`）。

---

## 3. 禁止包含

| 类别 | 示例 | 原因 |
|---|---|---|
| 交易链 | TradePlan / Risk / Paper / Execution | A1 范围外；工作区有预先 dirty |
| 旧行情实现 | `eastmoney_kline_api.go`、`stock_data_api.go` | A1 未改；若出现在 staged 则为污染 |
| 前端 / App | `frontend/**`、`app.go` | 非 A1 |
| Schema | `main_schema_levels*`、migration、golden schema | 非 A1 |
| 临时脚本 | `tmp_diag_upcoming/` 等 | 会破坏 `go build ./...` |
| 其它 Phase 文档 / 未归档代码 | 工作区其余 ~590 条目 | 选择性提交纪律 |

---

## 4. 边界污染结论

**未发现 Phase7-A1 边界污染。**

- 模型与接口无 Trade/Position/Order/Risk/Plan/Execution 字段  
- Adapter 为旧 API wrapper，未改旧行为  
- 生产侧零调用方、零反向依赖  

工作区其它 dirty **不是** A1 污染，但若误用 `git add -A` 会变成提交污染。

---

## 5. 是否可以进入 Phase7-A2

**可以，在 A1 milestone commit 完成后进入。**

建议顺序（不变）：

1. **A2-0** 调用点 inventory  
2. **A2-1** 非交易只读路径试点 + golden  
3. Quote 按安全顺序迁移；Paper/Execution 最后  
4. 缓存/超集切片单独 PR；禁止改信号公式与 schema  

---

## 6. 本文件状态

| 项 | 值 |
|---|---|
| 是否已执行 git add | **否** |
| 是否已执行 commit | **否** |
| 下一步 | 用户确认后按 §2 路径级提交 |
