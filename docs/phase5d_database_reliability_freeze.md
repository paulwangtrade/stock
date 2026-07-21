# Phase5-D — Database Reliability Freeze Report

> 日期：2026-07-21  
> 状态：**Implemented / Frozen**  
> 范围：Schema Migration Registry、启动 Schema Validation、自动交易启动门禁  
> 生产交易语义：**未改变**

---

## 1. Summary

Phase5-D 将生产启动从“单一 `CurrentSchemaVersion` 命中后跳过整批
`AutoMigrate`”升级为逐版本、可追踪、可校验的 Migration Registry：

```text
DB Init
  → Migration Registry（逐版本事务执行）
  → Startup Schema Validation
  → TradingPreflightCheck
      ├─ READY → 注册 9:20 / 9:25 / 9:30 / reconcile 交易 cron
      └─ BLOCKED_SCHEMA_INVALID → App/UI/行情继续；交易 cron 不注册
```

生产启动不再使用 `CurrentSchemaVersion` 决定是否跳过全量迁移。该常量只为
尚未迁移的兼容调用保留；Registry 是唯一生产启动版本来源。

---

## 2. Migration Registry

实现位置：

- `backend/db/migration_registry.go`
- `backend/db/schema_migrate.go`
- `main_schema_levels.go`

### 2.1 Registry 契约

每个 migration 固定记录：

- `version`
- `name`
- `checksum`
- `status=applied`
- `duration_ms`
- `applied_at`

规则：

1. version 必须正数且严格递增；
2. name 唯一；
3. checksum 非空，已应用 checksum 漂移会阻断；
4. 每个 migration 在独立事务中执行；
5. `Up` 与成功版本记录处于同一事务；
6. 失败不写成功记录；
7. 重复启动只跳过已应用版本，不重复执行；
8. 数据库存在当前二进制未知的未来版本时阻断；
9. Phase5-D 前旧版 `schema_migrations(id, version, applied_at)` 自动补齐元数据列。

### 2.2 Migration 列表

| Version | Name | Checksum | 作用 |
|---------|------|----------|------|
| 1 | `baseline_schema` | `661da2df...92aad3` | 仅全新数据库建立当前完整 baseline |
| 2 | `add_candidate_decision_id` | `7f2656c5...235137` | 幂等增加 `candidate_pool_items.decision_id` 与索引 |

完整 checksum 由 `backend/db/testdata/schema_registry_golden.json` 冻结。

### 2.3 旧数据库恢复路径

生产事故旧库状态：

```text
schema_migrations: version=1
candidate_pool_items: 无 decision_id
```

下次启动：

```text
Registry 读取 version=1
  → 发现 version=2 缺失
  → add_candidate_decision_id
  → HasColumn=false 时 AddColumn
  → HasIndex=false 时 CreateIndex
  → 写入 version=2 applied
  → Schema Validation READY
  → 允许注册交易 cron
```

若列/索引已经存在，v2 不重复创建，执行仍成功。

---

## 3. Schema Validation

实现：`backend/db/schema_validation.go`

状态：

- `READY`
- `BLOCKED_SCHEMA_INVALID`

### 3.1 验证项

| Table | Required columns / indexes |
|-------|----------------------------|
| `candidate_pools` | `id, trade_date, status, item_count` |
| `candidate_pool_items` | `id, pool_id, stock_code, rank, score, decision_id` + `idx_candidate_pool_items_decision_id` |
| `trade_plans` | `id, trade_date, status, enable_execute` |
| `trade_plan_items` | `id, plan_id, status, order_id, fill_id` |
| `paper_orders` | `id, client_order_id, status` |
| `paper_fills` | `id, order_id` |

同时验证：

- Registry 所有版本均已应用；
- migration name/checksum/status 未漂移；
- 当前数据库不存在未知未来版本。

示例阻断：

```text
Status: BLOCKED_SCHEMA_INVALID
MissingColumns:
  - candidate_pool_items.decision_id
```

---

## 4. Trading Block Policy

实现：`app_trading_preflight.go`、`app_paper_open_buy.go`

### 4.1 READY

Schema Validation 为 `READY`：

- 正常注册 9:20 Candidate/Plan cron；
- 正常注册 9:25 prepare cron；
- 正常注册 9:30 Execution cron；
- 正常注册 TradePlan reconcile cron。

### 4.2 BLOCKED_SCHEMA_INVALID

允许：

- App 继续启动；
- UI、行情、诊断继续；
- 记录 `TRADING_START_BLOCKED` 与缺表/缺列/版本错误。

禁止：

- Candidate cron 注册；
- Plan cron 注册；
- Execution cron 注册；
- 交易 reconcile cron 注册。

门禁发生在 cron 注册之前，不采用“注册后每次拒绝”，不进入
`ExecutionService.PreTradeCheck`，不修改运行态交易业务规则。

---

## 5. Golden / Reliability Tests

Golden：

- `backend/db/testdata/schema_registry_golden.json`
- `main_schema_golden_test.go`

覆盖：

| Case | 结果 |
|------|------|
| Fresh database：v1→v2 全部应用 | PASS |
| Legacy v1：缺 `decision_id` 自动升级 | PASS |
| Registry 重复执行 | PASS（版本记录不重复，Up 不重跑） |
| Migration 事务失败 | PASS（DDL 回滚，不记录版本） |
| Checksum 漂移 / 未知未来版本 | PASS（阻断） |
| 旧版 migration 元数据表升级 | PASS |
| Schema invalid → TradingPreflightCheck | PASS（`BLOCKED_SCHEMA_INVALID`） |
| App 可构造、交易 cron 不注册 | PASS |
| Schema READY → cron 保持原行为 | PASS |

旧库测试使用内存 SQLite fixture；未修改
`build/bin/data/stock.db`。

---

## 6. Acceptance Results

```text
go test ./...   PASS
go build ./...  PASS
wails build     PASS
```

Windows 系统 Temp 首次执行出现 `unlinkat ... Access is denied`（所有 Go 包已
显示 `ok`）。将 `GOTMPDIR` 指向现有 `D:\stock\build` 后重跑：

- 全量测试 exit code 0；
- Wails frontend/assets/application 全部完成；
- 产物：`D:\stock\build\bin\go-stock.exe`。

---

## 7. 未修改模块确认

Phase5-D 未修改：

- Candidate Rank / Score / Pool 构建逻辑；
- Decision Schema / `deriveQuantAction`；
- TradePlan 模型与生命周期；
- Execution / Broker / PaperOrder；
- 策略参数、风控参数、资金规则、自动交易业务规则；
- Decision / Draft / Candidate / Proposal 新链路。

本阶段只改变数据库迁移可靠性、启动 schema 诊断和交易 cron 启动门禁。

---

## 8. Recovery 操作

1. 启动应用，Registry 自动补齐缺失 migration；
2. 日志确认：
   - `schema migration registry: ... applied=[2]`
   - `startup schema validation READY version=2`
3. `TradingPreflightCheck` 为 `READY` 后，交易 cron 才注册；
4. 若仍为 `BLOCKED_SCHEMA_INVALID`，根据日志中的
   `missingTables / missingColumns / missingIndexes / errors` 修复数据库；
5. 不应通过关闭门禁、直接注册 cron 或修改策略参数绕过 schema 错误。

