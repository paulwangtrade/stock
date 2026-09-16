# Phase 20.27D Experiment Acceptance Freeze

**日期：** 2026-09-15  
**性质：** 只验收冻结。不改代码、不改 schema、不建表、不接 Proposal / Strategy Version  
**对照：** [PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md](./PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md) · [PHASE20_27C_EXPERIMENT_IMPLEMENTATION_COMPLETE.md](./PHASE20_27C_EXPERIMENT_IMPLEMENTATION_COMPLETE.md) · `researchExperimentProjection.js` · `ResearchExperimentFoundationPanel.vue`

## Status

**PASS**

最小 Experiment 能力符合 27B 冻结边界。仍属 Research Layer。`finding_id` 只作引用。未接策略与执行。

## Implementation Summary

| 项 | 验收结果 |
|----|----------|
| `experiment.v1` | 内存投影。字段含 `experiment_id`、`finding_id`（可空）、`hypothesis`、`status=recorded`，以及可选报告 / 回测身份回声 |
| Finding 关联 | `readFindingIdRef` 只取 id 字符串；传入发现对象时不抄 `statistics` |
| 研究实验页 | `ResearchExperimentFoundationPanel`：人工填写 finding_id + 假设 → 会话列表。文案标明不跑回测、不生成版本 |
| ResearchReport 白名单 | 仍只抄 `batch_outcome_summary.v1` 与 `backtest_result.v1`。未纳入 `experiment.v1` / finding |
| Strategy Version | 实验模块未调用 `createStrategyVersion` / `strategyProposal` |

## Boundary Check

### 1. Research Layer 定位

**通过。** Experiment 验证人对发现写下的假设，并记录为 `recorded`。  
禁止项均未出现：不生成交易信号、不自动执行、不自动选股、不自动调参。

### 2. Finding 引用

**通过。** 卡片只存 `finding_id`。测试断言不复制 `event_success_rate` / 路径均值。不复制 Outcome、因子正文。

### 3. 输入边界

**通过。** 投影不读 SignalEvent、行情、OutcomeObservation、TradePlan。面板不调用 K 线接口。

### 4. 输出边界

**通过。** 输出仍是 `experiment.v1`。未新增 Experiment Result。

### 5. Strategy 隔离

**通过。** 未连接 Research Proposal、`strategy_proposal.v1`、Strategy Version、Execution。会话 store 不是下游门。

### 6. Observation

**通过。** 不修改 Observation，不调用观察生产。

## Compatibility Check

| 路径 | 结果 |
|------|------|
| 旧：`hypothesis` + report + backtest | 可用；`finding_id` 可为 null |
| 新：`hypothesis` + `finding_id` | 可用；`status=recorded`；可不传报告 / 回测 |

## Test Result

```powershell
Set-Location D:\stock\frontend
node --test src/utils/researchExperimentProjection.test.mjs
```

**9 passed / 0 failed**（验收当日复跑）。

**wails build：** 未执行（与 27C-B 完成报告一致）。桌面包需另行完整编译。

## Protocol Impact

Code Change:
确认（27C-B 已落地；本阶段无新增改动）

Schema:
确认（仍为 `experiment.v1`；可空 `finding_id`；无 `v1.1`）

Migration:
确认（NONE）

API:
确认（NONE）

本验收到此冻结。不增加功能，不接 Proposal，不接 Strategy Version。
