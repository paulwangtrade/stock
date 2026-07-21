# Phase6-A5.2.2 Implementation Report

> 基线 HEAD：`9631718` — Phase6-A5.2.1 add morning frozen plan adopt wrapper  
> 依据：`PHASE6A5_2_MORNING_ADOPT_DESIGN.md`（A5.2.2：9:20 cron 改调）  
> 状态：已实现并测试；**未执行 `git add` / `git commit`**

## Goal

仅切换 9:20 cron 调用入口：

```text
InitPaperOpenBuyJobs
  paper_daily_plan @ 0 20 9 * * 1-5
    旧: RunDailyCandidateAndPlan("")
    新: RunMorningPlanPreparation("")
```

## Implementation

### `app_paper_open_buy.go`

| 项 | 值 |
|---|---|
| Spec 常量 | `paperDailyPlanCronSpec = "0 20 9 * * 1-5"`（时间不变） |
| Job | `runPaperDailyPlanCronJob` → `strategy.RunMorningPlanPreparation("")` |
| Preflight | 仍 `TradingPreflightCheck()`（不变） |
| 9:25 / 9:30 | 不变 |

可注入 `morningPlanPreparationFn` 供单测捕获 mode。

### 保持不变

| 目标 | 状态 |
|---|---|
| `RunDailyCandidateAndPlan` 实现 | **未改**（strategy） |
| TradePlan 模型 / DB | **未改** |
| Execution / Trading Gate | **未改** |
| API / frontend | **未改** |
| Wails 调试 `App.RunDailyCandidateAndPlan` | 仍可用；注释标明 cron 已切换 |

## Tests

```text
go test -count=1 . -run "TestInitPaperOpenBuyJobs_DailyPlanCronSpec920|TestRunPaperDailyPlanCronJob|TestPaperDailyPlanCron_SourceBoundary" -v
→ ok
```

| 要求 | 用例 | 结果 |
|---|---|---|
| cron 仍为 `0 20 9 * * 1-5` | `DailyPlanCronSpec920` | PASS |
| Frozen → `adopt_frozen` | `AdoptFrozenMode` | PASS |
| 无 Frozen → `build_morning` | `BuildMorningMode` | PASS |
| 边界 | `SourceBoundary` | PASS — 走 Preparation；Preflight/9:25/9:30 仍在 |

## Files

| 路径 | 变更 |
|---|---|
| `app_paper_open_buy.go` | 9:20 改调 + job helper |
| `app_paper_open_buy_cron_test.go` | 增补 A5.2.2 测试 |
| `PHASE6A5_2_2_IMPLEMENTATION_REPORT.md` | 本报告 |

## Result

Phase6-A5.2.2 目标已实现。HEAD 仍为 `9631718`；未提交。

建议 commit message：

```text
Phase6-A5.2.2 switch 9:20 cron to morning plan preparation
```

提交时注意：`app_paper_open_buy.go` 若工作区另有脏 hunk，须精确 stage 本切片改动。
