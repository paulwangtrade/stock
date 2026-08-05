# PHASE10_B.0 — Acceptance Report

> **切片：** Phase10-B.0 AnchorProvider Implementation  
> **验收日期：** 2026-08-05  
> **基线远程 HEAD：** `5beafb82897de1210c36a57e8f089cc8880ce163`（`origin/dev`）  
> **性质：** 只读验收 — **未修改业务代码、未 commit**  
> **依据：** `PHASE10_B_ANCHOR_LAYER_DESIGN.md`、`PHASE10_B0_ANCHOR_PROVIDER_INTERFACE_DESIGN.md`  

---

## 0. 验收结论

| 项 | 结果 |
|----|------|
| B.0 目标文件范围 | **PASS**（仅 Anchor 抽象 + populate 接线） |
| 无 Kline / Candidate Snapshot 扩展 | **PASS** |
| 无 Intent schema / Readiness / Materialize 业务改动（B.0 文件内） | **PASS** |
| `backend/marketdata/anchor` 单测 | **PASS** |
| `backend/strategy` 单测 | **PASS** |
| `npm run build` | **PASS**（~6m48s） |
| `wails build -skipbindings -s` | **PASS** → `D:\stock\build\bin\go-stock.exe` |
| `go test ./backend/...` 全量 | **FAIL / 不完整**（环境与既有失败；非 B.0 引入，见 §4.3） |

**总评：** Phase10-B.0 抽象与接线质量 **可接受**；全量 backend 测试不能作为本切片否决依据。

---

## 1. 修改范围（Git / 工作区）

### 1.1 B.0 验收范围文件（全部为未跟踪新增）

| 路径 | 角色 |
|------|------|
| `backend/marketdata/anchor/doc.go` | 包边界 |
| `backend/marketdata/anchor/types.go` | `Context` / `Result` / RefSource 常量 |
| `backend/marketdata/anchor/provider.go` | `Provider`、`Chain`、`DefaultProvider` |
| `backend/marketdata/anchor/followed.go` | `FollowedStockAnchorProvider` |
| `backend/marketdata/anchor/followed_test.go` | Followed / Chain / 禁 Open 守卫 |
| `backend/strategy/after_close_intent_populate.go` | populate 经 Provider |
| `backend/strategy/after_close_intent_populate_test.go` | 钩子迁移 + 对等/soft-fail |

相关设计文档（非运行时代码，同属本阶段产物，未 commit）：

- `PHASE10_B_ANCHOR_LAYER_DESIGN.md`
- `PHASE10_B0_ANCHOR_PROVIDER_INTERFACE_DESIGN.md`
- 本文 `PHASE10_B0_ACCEPTANCE_REPORT.md`

### 1.2 范围确认

在 B.0 源文件内扫描：

| 禁止项 | 结果 |
|--------|------|
| `KlineService` / `GetBars` / `stock_kline` / `kline_cache` | 未出现 |
| Candidate Snapshot 实现 | 未出现 |
| `EvaluateExecutionIntentReadiness` | 未出现 |
| `MaterializeMorning*` 调用/修改 | 未出现 |
| Open→ref（生产路径） | 未出现；测试中仅作 **禁止字符串守卫** |

`followed.go` 行为：`FollowPrice > 0` 优先，否则 `Price > 0`，否则 `ok=false`；标签仍为 `prev_close` / `strategy_snapshot`。

### 1.3 工作区噪声（**不属于** B.0）

当前工作区相对 `origin/dev` 存在 **大量** 既有 dirty / untracked（Phase6.5+ 本地残留等），例如：

- 已修改：`app.go`、`backend/data/*`、`frontend/*`、`go.mod` 等  
- 未跟踪：`backend/readiness/`、大量 `PHASE6_*` 文档、`tmp_*` 等  
- `backend/models/trade_plan.go` / `main_schema_levels.go` 等有 M，**非本次 B.0 编辑**

**验收口径：** 只审 B.0 文件集；不把整棵 dirty tree 算作本切片 diff。

### 1.4 与 HEAD 的重要事实

| 事实 | 说明 |
|------|------|
| `after_close_intent_populate.go` **不在** `5beafb8` 树中 | 该文件在验收时为 **untracked 新增**（历史本地意图填充 + B.0 Provider 接线） |
| HEAD 中 `build_draft_trade_plan.go` **无** `populateAfterCloseExecutionIntent` 调用 | 工作区副本有调用（既有 dirty）；B.0 未改 schema/readiness/materialize 实现体 |
| Phase10-A 已入仓 | `morning_intent_materialize.go` 等仍在 HEAD，B.0 未改 |

因此：B.0 把「盘后 Intent 锚点」正式落到 `marketdata/anchor`，并提供可注入 Provider；若后续单独 commit，需明确是否一并纳入此前未入仓的 populate 模块与 draft 接线。

---

## 2. 调用链

```text
BuildDraftTradePlanFromCandidatePool   (工作区接线；HEAD 尚无 populate 调用)
  → populateAfterCloseExecutionIntent(plan, items, pool)
       → afterCloseAnchorProvider.Resolve(Context)
            → anchor.Chain
                 → FollowedStockAnchorProvider
                      → followed_stock (FollowPrice → Price)
       → 写 item.ref_* / intent_status=selected
         或 soft-fail（不写 selected；limit/volume 仍为 0）

下游（未改）：
  MaterializeMorningLimitPrices   // 仍要求 selected && ref>0
  Readiness / Approve / Freeze    // 未放宽
```

生产默认：`afterCloseAnchorProvider = anchor.DefaultProvider()`（仅 Followed）。

---

## 3. 测试结果

### 3.1 目标包（B.0 验收主证据）

```text
go test ./backend/marketdata/anchor/... -count=1
  ok  go-stock/backend/marketdata/anchor   ~27s

go test ./backend/strategy/ -count=1
  ok  go-stock/backend/strategy            ~11s
```

覆盖要点：followed 命中、FollowPrice 优先、Price fallback、缺失 false、禁止 Open 作为 ref 源、populate soft-fail、DefaultProvider 对等。

### 3.2 全量 `go test ./backend/...`

| 观察 | 详情 |
|------|------|
| 退出码 | `1` |
| 早期失败 | `go-stock/backend/agent` — `database disk image is malformed (11)`（环境/本地 DB，与 Anchor 无关） |
| 其它 FAIL | Paper order health / lifecycle gaps 等（既有路径） |
| 中止 | `panic: test timed out after 10m0s`；日志被大量 TLS/网络栈淹没 |
| 结论 | **全量未绿**；**未证明** B.0 回归；亦 **未指向** B.0 文件为根因 |

B.0 不依赖修复 agent DB / 全量超时问题。

---

## 4. Build 结果

### 4.1 `npm run build`（`frontend/`）

| 项 | 值 |
|----|-----|
| 命令 | `npm run build`（vite） |
| 结果 | **成功** `exit_code=0` |
| 耗时 | ~6m 48s（`✓ built in 6m 48s`） |
| 备注 | chunk >500kB 警告（既有），非 B.0 |

### 4.2 `wails build -skipbindings -s`

| 项 | 值 |
|----|-----|
| 命令 | `wails build -skipbindings -s` |
| Skip Bindings / Skip Frontend | true / true |
| 结果 | **成功** `exit_code=0` |
| 产物 | `D:\stock\build\bin\go-stock.exe` |
| 耗时 | ~36.8s |
| 备注 | CLI v2.13.0 vs go.mod Wails 2.12.0 版本提示（既有） |

---

## 5. 已知限制

1. **DATA-003 未关闭：** `strategy_run` 候选若不在 `followed_stock`，仍无 `selected`（B.0 预期行为兼容）。  
2. **价源仍唯一：** 仅 Followed；无 Kline / Snapshot。  
3. **`ref_source` 标签遗留：** 仍写 `prev_close` / `strategy_snapshot`（价实为自选），未切换新枚举。  
4. **`confidence` 不落库。**  
5. **工作区脏：** 大量非 B.0 改动并存；提交时必须精确 `git add` B.0 路径。  
6. **HEAD 缺口：** populate 文件与 draft→populate 接线相对 `5beafb8` 可能需在 commit 计划中一并说明（本地先前已有意图填充，非 B.0 新发明 schema）。  
7. **全量测试环境不稳：** malformed DB + 10m timeout，不能代表 B.0 质量。

---

## 6. Phase10-B.1 前置条件

进入 **B.1（Kline Close 源）** 前建议满足：

| # | 前置 |
|---|------|
| 1 | B.0 已单独 commit（或明确纳入的 populate 接线）并可选 push |
| 2 | `Provider` / `Chain` 稳定；populate 仅依赖接口 |
| 3 | 约定 `source_date`（T）与 `Close(code,T)` 对齐规则 |
| 4 | 选择日线真相源：`StockKLineRepo` 灌库 和/或 `KlineService`/`kline_cache` 门面（只读 Close，禁止 Open→ref） |
| 5 | Chain 顺序：`KlineClose` →（仍保留）`Followed`；缺价 soft-fail 不变 |
| 6 | 单测：有 K 无自选 → selected；无 K 无自选 → miss；静态守卫继续禁止 Open→ref |
| 7 | **不做：** Candidate Snapshot（B.2）、EOD Store（B.3）、放宽 Readiness |

---

## 7. Registry

```text
PHASE10-B.0   ACCEPTANCE  — 本文；实现可接受，未 commit
DATA-003      OPEN        — Followed 唯一耦合仍在；B.1+ 缓解
PHASE10-A     FROZEN      — Materialize 路径未改
```

---

## 8. 边界声明

- 验收过程 **未修改业务代码**、**未执行 commit / push**  
- 仅新增本验收报告文档  
- 构建产物 `go-stock.exe` 已生成；使用前请先关闭正在运行的同名进程  
