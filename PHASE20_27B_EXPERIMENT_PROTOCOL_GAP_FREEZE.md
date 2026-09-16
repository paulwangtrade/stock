# Phase 20.27B Experiment Protocol Gap Freeze

**日期：** 2026-09-15  
**性质：** 只设计冻结。不改 `experiment.v1`，不实现 Experiment，不连接 Strategy  
**已读：** [PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md](./PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md) · [PHASE20_26B_RESEARCH_ENGINE_OUTPUT_DESIGN_FREEZE.md](./PHASE20_26B_RESEARCH_ENGINE_OUTPUT_DESIGN_FREEZE.md) · `research_finding.v1` · `experiment.v1` · Research Proposal · Strategy Version · ResearchReport 白名单

## Status

**FROZEN**

Experiment 属于 Research Layer。它验证人对发现写下的假设。关联用引用，不复制正文。本阶段不改协议。

## Finding Reference Decision

**应该让 `experiment.v1` 引用 `research_finding.v1`，形式是 `finding_id`。**

采用引用，不复制：

| | Research Finding | Experiment |
|--|------------------|------------|
| 是什么 | 研究发现：分组后的路径描述 | 验证该发现的过程：人写下的假设卡片 |
| 身份 | `finding_id`（如 `rf:…`） | `experiment_id`（如 `rexp:…`） |
| 数字 | 路径分布留在发现上 | 实验不嵌 `statistics`、不抄均值 |

禁止把发现正文、研究行或观察路径嵌进实验。复制会变成第二真源，并诱使实验重算。

发现不会自动变成实验。人写 `hypothesis`，并带上要验证的 `finding_id`。没有句子就不是实验。

现卡片仍绑报告与回测身份。那是现状，不是目标形态。目标关联是 finding 引用。报告 / 回测身份若保留，只作可选回声，不得代替 `finding_id`，也不得打开 Fill。

## Experiment Version Decision

**保持 `experiment.v1`。不新增 `experiment.v1.1`。本阶段不改字段。**

| 选项 | 决定 |
|------|------|
| 现在改 `experiment.v1` | 否 |
| 现在开 `experiment.v1.1` | 否 |
| 保持现版本名 | **是** |

只补关联引用、不改已有字段含义时，将来实现可在同一 `experiment.v1` 上做兼容扩展：可选 `finding_id` 字符串，只引用，不嵌发现体。那不会改 `recorded` 的含义，也不引入评分。

若将来把创建条件从「报告 + 回测引用」改成「必须以 finding 为主锚」，那是创建语义变化，必须另有实现指令再动。本冻结不授权那次改动，也不为此预开 `v1.1`。

不因缺口另起 `ExperimentResult` 或第二张实验卡。

## Input Boundary

Experiment **可以引用**：

- `research_finding.v1` 的 `finding_id`
- 人写的 `hypothesis`

**不得直接读取**：

- SignalEvent
- Market Data / 日 K
- OutcomeObservation
- TradePlan
- `research_event_record.v1` 整行并重算

原因：实验验证假设，不重新计算研究事实。读行情或观察，就会变成第二套引擎。

## Output Boundary

Experiment 输出留在 Research Layer。

`experiment.v1` + `status = recorded` 足够表达「假设已记下」。不新增 Experiment Result。

实验上不写路径均值、胜率、通过 / 失败。那些摘要留在发现或报告结论。实验卡片一有结果摘要，就容易被当成自动通过线。

## State Model

**不采用** `draft → running → recorded → reviewed` 作为研究实验状态机。

| 拟议状态 | 决定 |
|----------|------|
| `draft` / `running` | 不要。像执行或回测跑批，会把实验拖向引擎 |
| `recorded` | **保留。** 假设已记下。这是现唯一状态 |
| `reviewed` | 不要放在 Experiment 上。审阅属于 Research Proposal |

状态只描述实验过程：卡片是否已记录。不是策略批准，不是上线，不是 `approved`。

将来若要「人已看过实验」，用 Proposal 的 `unreviewed` / `confirmed` / `declined`，不要在 Experiment 上再开一套审阅态。

## Proposal Boundary

```text
research_finding.v1
        ↓ 人写假设，引用 finding_id
Experiment                 experiment.v1 · recorded
        ↓ 人决定留档。实验函数不创建提案
Research Proposal          值不值得进入策略讨论
```

Experiment：验证假设。  
Proposal：整理是否值得进入策略讨论。  

二者职责不同。实验不会自动产生提案。提案不是 Strategy Version。

## Strategy Boundary

Strategy Version **只能**来自：

- `strategy_proposal.v1`
- `status = approved`
- 带有 change delta（`proposed_changes`）
- 人工批准

不是 Experiment 输出。不是 `research_finding.v1`。不是 Research Proposal 的 `confirmed`。

### confirmed / approved

| 词 | 层 | 含义 |
|----|----|------|
| `confirmed` | Research | 研究结果确认：值得留档讨论 |
| `approved` | Strategy Proposal | 策略批准：允许登记版本 |

禁止 `confirmed` 自动转换成 `approved`。禁止 Experiment 创建或修改 Strategy Version。

## Report Boundary

ResearchReport 继续只读取研究结论。白名单不变。

不读取：

- Experiment 过程 / `experiment.v1` 原始卡片作为新节
- Research Dataset / `research_event_record.v1`
- OutcomeObservation
- `research_finding.v1` 数组

报告里已有的 `sections.experiments` 仍是策略侧观察切片，不是 `rexp:` 假设卡，也不因本冻结改写。

## Protocol Impact

Code Change:
NONE

Schema Change:
NONE

Migration:
NONE

本冻结到此停止。不改 `experiment.v1`，不新增字段实现，不建表，不新增 API，不创建 Experiment Engine，不连接 Strategy。
