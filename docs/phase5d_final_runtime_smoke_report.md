# Phase5-D Final Runtime Smoke Report

Date: 2026-07-22

Plan: `PHASE5D_FINAL_RUNTIME_SMOKE_PLAN.md`  
HEAD / build commit: **`8dee4c4`** — Phase5-D runtime: wire schema migrate and trading gate

## Build

| Item | Value |
|---|---|
| Source | `git worktree` detached at `8dee4c4` → `D:\stock\build\phase5d_final_wt` |
| Worktree dirty files | **0**（未编入主仓未提交改动） |
| Frontend embed | 自主仓复制已有 `frontend/dist`（仅资产嵌入，非业务源码） |
| Command | `wails build -skipbindings -s` |
| Output | `D:\stock\build\phase5d_final_wt\build\bin\go-stock.exe` |
| Smoke copy | `D:\stock\build\phase5d_final_run\ready\go-stock.exe`（及 blocked 同二进制） |
| Size | 98,368,512 bytes |
| Built at | 2026-07-22 00:10:01 |
| Embedded version/commit | blank（启动日志 `version:` / `commit:` 空） |
| Build log | `D:\stock\build\phase5d_final_build.log` |

确认二进制源码含 Wiring：`applyApplicationMigrations` + `InitAnalyzeSentiment`；**无** AssetServer。

## Database

| Item | Path |
|---|---|
| Production source (copy-from) | `D:\stock\build\bin\data\stock.db` |
| Pre-smoke backup | `D:\stock\build\phase5d_final_run\stock.db.phase5d_final.pre.bak` |
| READY session DB | `D:\stock\build\phase5d_final_run\ready\data\stock.db` |
| BLOCKED session DB | `D:\stock\build\phase5d_final_run\blocked\data\stock.db`（隔离副本；`DROP TABLE trade_plans`；version 仍为 2） |

生产库未被破坏；BLOCKED 仅改隔离副本。

---

## Migration Registry

**status: PASS**

READY / BLOCKED 启动均出现：

```text
2026-07-22 00:11:04.968 schema migration registry: from=2 to=2 applied=[] skipped=[1 2]
2026-07-22 00:14:38.427 schema migration registry: from=2 to=2 applied=[] skipped=[1 2]
```

`schema_migrations`（READY 会话只读查询）：

| version | name | checksum | status | applied_at |
|--------:|------|----------|--------|------------|
| 1 | *(empty)* | *(empty)* | applied | 2026-07-20 23:08:35 +08:00 |
| 2 | `add_candidate_decision_id` | `7f2656c5217b44b6ceb80a331dc201286d62b433f04f860d964c3a081d235137` | applied | 2026-07-21 12:52:28 +08:00 |

**current version: 2**

---

## Schema

### READY path — PASS

```text
2026-07-22 00:11:04.973 startup schema validation READY version=2
```

### BLOCKED path — PASS

隔离副本删除 `trade_plans` 后：

```text
2026-07-22 00:14:38.432 startup schema validation BLOCKED_SCHEMA_INVALID: version=2/2 missingTables=[trade_plans] missingColumns=[] missingIndexes=[] errors=[]
```

---

## Trading Gate

### READY — cron registered — PASS

```text
2026-07-22 00:11:06.433 paper open buy cron registered (9:20 plan / 9:25 prepare / 9:30 execute / reconcile 9:35+intraday)
```

同会话 **无** `TRADING_START_BLOCKED`。

### BLOCKED — TRADING_START_BLOCKED — PASS

```text
2026-07-22 00:14:40.117 TRADING_START_BLOCKED status=BLOCKED_SCHEMA_INVALID reason=schema invalid: version=2/2 missingTables=[trade_plans] missingColumns=[] missingIndexes=[] errors=[]
```

同会话 **无** `paper open buy cron registered`（对 info+error 全文扫描确认）。

`TradingPreflightCheck` 运行态由 Gate 日志间接证明（status=`BLOCKED_SCHEMA_INVALID`）。

### Automated supplement

`go test -count=1 -run TestTradingPreflightCheck_BlocksInvalidSchemaButAppContinues .`（干净 worktree）→ **`ok go-stock 30.536s`**（补充证据；以 C1 运行态为准）。

---

## Cron evidence（汇总）

| 会话 | `paper open buy cron registered` |
|---|---|
| READY | **是**（上引 INFO） |
| BLOCKED | **否** |

---

## UI Smoke（相对 D.1 不回归）

| 检查 | READY | BLOCKED |
|---|---|---|
| 进程 Responding | True（PID 39736） | True（PID 37648） |
| `http://127.0.0.1:18888/` | **200** | **200** |
| App 在 schema BLOCKED 时仍启动 | — | **是**（Gate 不杀进程） |

日志摘录（READY）：词典加载成功、`ai-assistant-web started at: :18888`、Daily Check 输出正常。  
本轮未做交互式 K 线截图；进程/助手端口与启动完成信号与 D.1「Application continues」一致。

---

## Final Decision

| 项 | 结果 |
|---|---|
| 干净 `8dee4c4` 构建 | PASS |
| Migration Registry | PASS（current=2） |
| Schema READY | PASS |
| Schema BLOCKED（隔离副本） | PASS |
| Trading Gate READY→cron | PASS |
| Trading Gate BLOCKED→`TRADING_START_BLOCKED` 且无 cron | PASS |
| UI 不回归 / BLOCKED 下 App 继续 | PASS |

### Verdict: **PASS**

Phase5-D Runtime Wiring 在真实运行态签收完成。  
代码历史冻结（`PHASE5D_FINAL_FREEZE_REVIEW.md`）+ 本报告 → **运行态最终签收：PASS**。
