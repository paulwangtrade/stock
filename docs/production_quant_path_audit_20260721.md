# Production Quant Trading Path Audit — 2026-07-21

> 类型：**只读审计**（不改代码 / 不改参数 / 不改逻辑 / 不修 bug / 不接新链）
> 交易日：2026-07-21（周二，工作日，交易日门禁通过）
> 运行实例：`D:\stock\build\bin\go-stock.exe`
> 证据来源：`build\bin\logs\info.log`、`build\bin\logs\error.log`、`build\bin\data\paper_open_buy.json`、`build\bin\data\stock.db`

---

## Summary

**结论：今日全程 0 笔自动量化买卖。根因不在策略/风控/开关，而在数据库 schema：`candidate_pool_items` 表缺少 `decision_id` 列，导致 9:20 建候选池落库直接 SQL 失败，后续链路全部空转。**

一句话链路结果：

```
9:20 BuildCandidatePool → INSERT 失败(缺列 decision_id) → 无 CandidatePool
      → 无 TradePlan → 9:25 prepare「no ready」→ 9:30 execute「no ready」→ 0 订单
```

关键证据（`error.log` 2360–2361）：

```
2026-07-21 09:20:00.008 [ERROR] [strategy/build_trade_plan.go:112]
  [PaperPlan] BuildCandidatePool failed: SQL logic error:
  table candidate_pool_items has no column named decision_id (1)
2026-07-21 09:20:00.009 [ERROR] [stock/app_paper_open_buy.go:162]
  RunDailyCandidateAndPlan: SQL logic error:
  table candidate_pool_items has no column named decision_id (1)
```

自动交易开关本身是**开启且正常**的（`enable=true amount=100000`），所以这不是「关了开关」的问题，而是一次**未随模型变更递增 schema 版本**导致的迁移缺列事故。

---

## Candidate Stage

| 项 | 结果 |
|----|------|
| 9:20 cron 是否触发 | **是**（`0 20 9 * * 1-5`，工作日门禁通过） |
| Universe 是否采集 | 是（signal enhancer 已运行：`snapshot_id=2 hits=56 matched=0`，09:20:00.007） |
| 内存排序/打分 | 已在内存完成（enhance→sort→rank） |
| **落库 CandidatePool** | **失败**：`CreatePoolWithItems` INSERT `candidate_pool_items` 报缺列 `decision_id` |
| 今日持久化 Candidate 数量 | **0** |
| 股票代码 | **无**（未落库） |
| 时间 | 失败发生于 `2026-07-21 09:20:00.008` |

启动自检（`info.log` 9391–9404，02:36:26）也已预警当日为空：

```
Paper Trading Daily Check
EnablePaperOpenBuy: true
CandidatePool: EMPTY
TradePlan: EMPTY
WARNING: CandidatePool empty, 09:30 will skip execution
ExecutionReady: false
```

---

## Rank Stage

| 项 | 结果 |
|----|------|
| rank score 计算 | 内存中已计算（`strategyScoreFromRank` + `ComposeCandidateScore`），但因池未落库无法核对 |
| threshold | 选股链无「分数阈值淘汰」；截断为 `defaultMaxCandidates=30`（Top-N 截断，非拒绝） |
| rejected reason | **N/A**：Rank 不产生 reject；唯一失败点是排序后的**落库 INSERT** |

> 说明：Rank 本身未阻断。它在 `BuildCandidatePool` 内部完成排序后交给 `CreatePoolWithItems`，正是这一步 INSERT 撞上缺列。

---

## PlanFilter Stage

| 项 | 结果 |
|----|------|
| 是否进入 PlanFilter | **否**（上游 pool 构建失败，`RunDailyCandidateAndPlan` 提前 return err） |
| passed | 0 |
| rejected | 0 |
| reject reason | **N/A**（从未调用 `FilterPoolForTradePlan` / `risk.PlanFilter`） |

`error.log` 显示错误在 `build_trade_plan.go:112`（即 `RunDailyCandidateAndPlan` 里 `BuildCandidatePool` 返回后立即 return），PlanFilter/BuildTradePlan 代码路径今日**未执行**。

---

## TradePlan Stage

| 项 | 结果 |
|----|------|
| `BuildTradePlan` 调用次数 | **0**（被上游错误短路） |
| plan count | **0** |
| ready plan | 无 |
| skipped reason | 上游 `candidate pool` 构建失败，链路未到 BuildTradePlan |

9:25 prepare 佐证（`info.log` 10094）：

```
2026-07-21 09:25:00.001 [WARN] [data/paper_open_buy.go:286]
  paper open buy PREPARE prepare: no ready TradePlan for 2026-07-21 (record not found)
```

---

## Execution Stage

| 项 | 结果 |
|----|------|
| 9:30 cron 是否触发 | **是**（`0 30 9 * * 1-5`） |
| `RunPaperOpenBuyOnce(requireEnabled=true)` | 执行，但 `GetReadyByTradeDate` 未命中 |
| **execution attempts** | **0**（未进入 `runPaperOpenBuyExecuting`） |
| `ExecutePlanItem` 调用 | **0** |
| Broker submit | **0** |
| **orders created** | **0** |
| **orders rejected** | **0**（谈不上拒绝，根本没生成订单） |

9:30 执行佐证（`info.log` 10098）：

```
2026-07-21 09:30:00.000 [WARN] [data/paper_open_buy.go:327]
  paper open buy: no ready TradePlan for 2026-07-21
```

> 注：`RunPaperOpenBuyOnce` 在 `GetReadyByTradeDate` 失败时直接 return，`EnablePaperOpenBuy=true` 也无从生效——因为根本没有可执行的计划。

---

## 自动交易开关核对（全部非阻断）

| 开关 / 条件 | 今日实际值 | 是否阻断 | 依据 |
|-------------|-----------|----------|------|
| market session（交易日 Mon-Fri） | 周二，通过 | 否 | 三个 cron 均在 9:20/9:25/9:30 触发 |
| `EnablePaperOpenBuy` | **true** | 否 | `paper open buy config: enable=true`（info 9389） |
| `openBuyAmountPerStock` | 100000 | 否 | 同上 |
| `EnableRiskFilter` | true（配置无该键→取默认 true） | 否（今日未到风控） | `build\bin\data\paper_open_buy.json` 无 `enableRiskFilter` 键 |
| `PlanMarketLevel` | 默认 3 | 否 | 同上（<=0 取默认 3） |
| `BlockNewEntriesOnDefense` | 默认 true（Lv1-2 才阻断） | 否 | 未到 PlanFilter |
| paper / real mode | Paper（`RunPaperOpenBuyOnce` 走 PaperBroker） | 否 | 执行入口为 paper open buy |
| max position / MaxNames | `defaultMaxPlanNames=5` | 否 | 未到 PlanFilter |
| cash / margin | PlanFilter 才校验 | 否 | 未到 PlanFilter |
| cooldown / duplicate protection | `TryBeginExecute` CAS（ready→executing） | 否 | 未到 CAS（无 ready plan） |
| **schema version 门禁** | **CurrentSchemaVersion=1，已判定「最新」跳过迁移** | **是（根因）** | `info.log` 9375 |

**唯一阻断项 = schema 迁移被跳过导致 `candidate_pool_items` 缺 `decision_id` 列。**

---

## Blocking Reasons

### 主根因：模型新增列未递增 schema 版本 → 迁移被跳过 → 缺列

1. `models.CandidatePoolItem` 已新增字段（Phase3-A）：

```55:55:backend/models/candidate_pool.go
	DecisionID string    `json:"decisionId" gorm:"column:decision_id;size:191;index"`
```

2. 但 `CurrentSchemaVersion` 仍为 **1**，未随该列变更递增：

```9:11:backend/db/schema_migrate.go
// CurrentSchemaVersion 当前代码期望的 schema 版本。
// 新增/变更表结构时必须递增本常量，否则已有库会跳过迁移。
const CurrentSchemaVersion = 1
```

3. 启动时因旧库 applied version 已 ≥1，`IsSchemaCurrent(1)=true`，AutoMigrate 被整体跳过（`info.log` 9375）：

```
2026-07-21 02:36:24.943 [INFO] [stock/main.go:281] schema version 1 已是最新，跳过表迁移
```

`main.go` 门控逻辑：

```279:289:main.go
func AutoMigrate() {
	if db.IsSchemaCurrent(db.CurrentSchemaVersion) {
		log.SugaredLogger.Infof("schema version %d 已是最新，跳过表迁移", db.CurrentSchemaVersion)
	} else {
		if err := runCoreSchemaMigrations(); err != nil {
```

4. `candidate_pool_items` 仅在 **extended** 迁移组内 `AutoMigrate`（`main_schema_levels.go:62`），迁移被跳过后该表结构停留在旧版本，缺 `decision_id` 列。

5. 9:20 `CreatePoolWithItems` 的 INSERT 带 `decision_id` 字段 → SQLite 报 `no column named decision_id` → 建池失败 → 全链空转。

### 次要观察（非今日阻断，但值得记录）

- `signal enhancer ... hits=56 matched=0`：信号快照匹配到 0，只影响信号加分，不影响候选生成本身（strategyScore 仍在）。今日即使无缺列问题，也应能建池；此项不是阻断根因。
- 生产配置文件 `build\bin\data\paper_open_buy.json` 只写了 4 个键（enable/codes/amount），Phase1.2 风控键缺失走默认值——属兼容路径，非阻断。

---

## Recommended Fix（仅建议，未执行）

> 以下为建议，本次审计**不做任何修改**。

### 1. 首选：递增 schema 版本，触发一次 AutoMigrate 补列

- 将 `backend/db/schema_migrate.go` 的 `CurrentSchemaVersion` 由 `1` 递增为 `2`。
- 下次启动 `IsSchemaCurrent(2)=false` → 重新执行 core+extended AutoMigrate → GORM 自动为 `candidate_pool_items` 补 `decision_id` 列 → `finalizeExtendedSchemaMigrations` 成功后 `MarkSchemaVersion(2)`。
- 风险低：AutoMigrate 只加列不删列；与代码注释既定约定一致（“新增/变更表结构时必须递增本常量”）。

### 2. 备选：手工补列（若不便发版）

对 `build\bin\data\stock.db` 执行（离线、先备份）：

```sql
ALTER TABLE candidate_pool_items ADD COLUMN decision_id varchar(191);
CREATE INDEX IF NOT EXISTS idx_candidate_pool_items_decision_id ON candidate_pool_items(decision_id);
```

- 仅应急；治本仍是方案 1（否则下一次模型变更会重蹈覆辙）。

### 3. 流程加固（防复发）

- **迁移守卫**：新增/改动任一 `extendedSchemaModels()` / `coreSchemaModels()` 列时，强制递增 `CurrentSchemaVersion`（可加 CI 校验：模型字段哈希 vs 版本号）。
- **建池失败告警**：`RunDailyCandidateAndPlan` 失败时，除 error 日志外增加显式启动/9:20 失败通知（当前仅落 error.log，容易被忽略）。
- **启动自检升级**：`RunPaperTradingDailyCheck` 可增加“对 `candidate_pool_items` 做一次 schema 探测/试插滚回”，在 9:20 前就暴露缺列。

### 4. 复核验证（修复后）

- 手动触发 `App.RunDailyCandidateAndPlan("")` 或等次日 9:20，确认 `info.log` 出现：
  - `candidate pool: total=N ... poolId=...`
  - `[PaperPlan] ... planId=... risk=...`
  - 9:30 `Paper Open Buy Execution Start/End`。

---

## 是否具备继续自动交易条件

| 条件 | 状态 |
|------|------|
| 自动交易开关/cron/交易日门禁 | **正常** |
| 策略/风控/资金逻辑 | 今日未触达，无证据表明有问题 |
| 数据库 schema（candidate_pool_items.decision_id） | **缺列，阻断建池** |
| 结论 | **修复缺列（建议方案 1）后即可恢复自动量化链路**；在此之前每个交易日都会在 9:20 建池处失败、0 单。 |
