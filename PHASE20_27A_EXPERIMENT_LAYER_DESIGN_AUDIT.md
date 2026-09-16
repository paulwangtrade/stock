# Phase 20.27A Experiment Layer Design Audit

**日期：** 2026-09-15  
**性质：** 只读设计审计。不实现 Experiment Engine，不改代码、schema、白名单  
**已读：** [PHASE20_26B_RESEARCH_ENGINE_OUTPUT_DESIGN_FREEZE.md](./PHASE20_26B_RESEARCH_ENGINE_OUTPUT_DESIGN_FREEZE.md) · [PHASE20_26A_RESEARCH_ENGINE_INPUT_AUDIT.md](./PHASE20_26A_RESEARCH_ENGINE_INPUT_AUDIT.md) · `research_finding.v1` · `experiment.v1` · [PHASE20_19_RESEARCH_PROPOSAL_AUDIT.md](./PHASE20_19_RESEARCH_PROPOSAL_AUDIT.md) · `strategy_proposal.v1` · `strategy_version.v1` · ResearchReport 白名单

## Status

**NEED REVIEW**

Experiment 的层位和职责已经清楚。它属于 Research，只验证人写下的假设。现有 `experiment.v1` 还没接上 `finding_id`，也没有 Research Proposal 投影。缺的是引用接线，不是新协议。

## Experiment Position

**Experiment 属于 Research Layer。**

| | Research Experiment `experiment.v1` | 策略侧 `strategy_experiment.v1` |
|--|-------------------------------------|--------------------------------|
| 层 | Research | Strategy |
| 问什么 | 人要验证哪一句假设 | 改哪些旋钮、参数臂如何并列 |
| 状态 | `recorded` | `draft` / `reviewed` / `approved` 一类 |
| 下游 | 人另开 Research Proposal | 可进 StrategyProposal |

核心职责是**验证研究假设**。不是自动交易，不是自动参数优化，不是自动调仓，不是修改历史 Observation。

已有投影 `projectResearchExperiment` 只记下调用方句子，并抄报告上的身份标签。它不评分，不调参，不创建 StrategyVersion。

## Input Boundary

当前**不存在**自动链路：

```text
research_finding.v1  →  Experiment
```

引擎到发现为止。实验由人另写假设创建。现卡片输入是：

- `hypothesis`（人句子）
- 已建好的 `research_report_projection.v1`（只取身份）
- `backtest_result.v1` 引用（只保留 schema 与 evaluator 身份）

它**不读** `finding_id`。这是缺口。

设计上 Experiment **应该引用** `finding_id`，而不是复制发现正文、研究行或路径数字。复制会变成第二真源，并诱使实验重算。

| 候选输入 | 是否允许 | 说明 |
|----------|----------|------|
| `finding_id` 引用 | **应允许（设计）** | 只引用，不嵌统计块 |
| 人写的 `hypothesis` | 已有 | 没有句子就不是实验 |
| 报告 / 回测身份字符串 | 已有 | 不打开 Fill，不抄五个指标 |
| `research_event_record.v1` 整行 | **禁止直接读** | 那是引擎输入。实验再读就会自己做分析 |
| Market Data / SignalEvent / OutcomeObservation | **禁止** | 会重算研究，并改观察 |

Experiment 不得调用观察生产，不得改 `outcome_observation.v1`，不得按行情重算均值。

## Output Boundary

**不需要新的 Experiment Result / Experiment Record schema。**

已有 `experiment.v1` 就是实验记录：

| 拟议 | 现况 | 判断 |
|------|------|------|
| `experiment_id` | `rexp:…` | 够用 |
| `hypothesis` | 有 | 够用 |
| `input finding` | **没有** | 将来只加引用字段的设计空间；本阶段不加 schema |
| `evaluation method` | 没有 | 现在不要。加上易变成评分器 |
| `result summary` | 没有路径均值 | 正确。摘要留在引擎发现或报告结论 |
| `status` | 固定 `recorded` | 够用。不是通过 / 失败 |

不要另起 `ExperimentResult.v1`。结果摘要若出现在实验上，会被当成自动通过线。

## Proposal Boundary

正确链路：

```text
research_finding.v1
        ↓ 人写假设，引用 finding_id（设计；现未接）
Experiment                 experiment.v1 · recorded
        ↓ 人决定留档。实验函数不创建提案
Research Proposal          值得讨论 · confirmed ≠ approved
```

Experiment 结果不是 Strategy。Proposal 是人整理后的「值不值得继续做策略研究」的确认单，不是上线单。

Research Proposal 投影按 Phase20.19 仍未落地。本阶段不实现。不要把 `experiment.v1` 塞进 `strategy_proposal.v1`。

报告里的 `sections.experiments` 是策略侧观察切片，文案是「描述性路径，非优选」。它不是 `rexp:` 假设卡片，也不是下一跳门。

## Strategy Boundary

Strategy Version **只能**来自：

- `strategy_proposal.v1`
- `status = approved`
- 且带有 `proposed_changes`

不是来自 Experiment，不是来自 `research_finding.v1`，不是来自 Research Proposal 的 `confirmed`。

`createStrategyVersion` 已按此拒绝。本阶段不改它。

## Status Transition

| 状态 | 对象 | 含义 | 不是 |
|------|------|------|------|
| `recorded` | `experiment.v1` | 假设已记下 | 不是验证通过 |
| `unreviewed` / `confirmed` / `declined` | Research Proposal（设计） | 研究确认：看过并留档 / 不留档 | `confirmed` ≠ 建版本 |
| `draft` / `reviewed` / `approved` | `strategy_proposal.v1` | 策略侧送审与批准 | 不是研究确认的别名 |
| 版本登记后的草稿 | `strategy_version.v1` | 登记而已 | 不是启用、不是报单 |

**禁止 `confirmed` 自动进入 `approved`。** 两词不在同一层。研究确认单没有旋钮差量，过不了版本门。不要为接链去补差量。

报告白名单不变。报告只抄已有结论对象（如 `batch_outcome_summary.v1`、`backtest_result.v1`）。不读 Experiment 原始过程、Research Dataset、OutcomeObservation、引擎发现数组。

## Protocol Impact

Code Change:
NONE

Schema Change:
NONE

Migration:
NONE

本阶段不实现 Experiment Engine，不建表，不新增 API，不修改 Strategy Version，不连接交易执行。
