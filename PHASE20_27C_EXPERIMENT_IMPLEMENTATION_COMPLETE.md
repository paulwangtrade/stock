# PHASE20.27C-B Minimal Experiment Reference 实现完成

**日期：** 2026-09-15  
**前置：** [PHASE20_27C_EXPERIMENT_IMPLEMENTATION_AUDIT.md](./PHASE20_27C_EXPERIMENT_IMPLEMENTATION_AUDIT.md) PASS  
**范围：** Research Layer 最小 Experiment：引用 `finding_id`、记下假设、`recorded`、会话查询与展示

## Files Changed

| 文件 | 动作 |
|------|------|
| `frontend/src/utils/researchExperimentProjection.js` | 扩展：可选 `finding_id`；假设+finding 可成卡；会话 create/list/get |
| `frontend/src/utils/researchExperimentProjection.test.mjs` | 补 finding 路径与 store 测试 |
| `frontend/src/components/ResearchExperimentFoundationPanel.vue` | **新增**。创建 / 列表展示 |
| `frontend/src/components/researchIndex.vue` | 挂载「研究实验」页签 |

未改：`researchReportProjection.js`、`strategyVersion.js`、`strategyProposal.js`、`researchBatchEngine.js`、任何 Go / migration。

## Schema Changed

`experiment.v1` 名不变。兼容增加可空字段 `finding_id`（只存引用字符串）。不开 `v1.1`。不嵌发现 `statistics`。

## Migration

NONE

## Tests

```powershell
Set-Location D:\stock\frontend
node --test src/utils/researchExperimentProjection.test.mjs
```

结果：9 passed / 0 failed。

覆盖：空输入拒绝；旧报告+回测路径；假设+finding；发现对象只取 id；会话 create/list/get；不抄指标；不调用策略 / 执行符号。

## Build Verification

未跑 `wails build`（仅前端内存投影与面板）。单元测试已通过。若要桌面包，需另行完整编译。

## Regression Check

| 项 | 结果 |
|----|------|
| ResearchReport 白名单 | 未改 |
| Strategy Version | 未改 |
| Research Engine | 未调用 `runResearchBatch` |
| 自动回测 / 调参 / 选股 / 交易 | 未实现 |

## 行为摘要

```text
createResearchExperiment({ hypothesis, finding_id })
        ↓
experiment.v1
  finding_id · hypothesis · status=recorded · experiment_id=rexp:…
        ↓ 会话内存
listResearchExperiments / getResearchExperiment
        ↓
研究实验面板展示
```

旧路径（假设 + 报告 + 回测身份）仍可用；`finding_id` 可为 null。同 `experiment_id` 再创建会替换会话内同一条。不落库、不建 API、不接 Proposal、不接 Strategy Version。
