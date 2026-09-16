# PHASE20.27C Experiment 最小实现审计

**日期：** 2026-09-15  
**性质：** 只读审计。不改代码、不改 schema、不建表、不建 API  
**已读：** [PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md](./PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md) · [PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md](./PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md) · `experiment.v1` · `research_finding.v1` · `strategy_experiment.v1` · Phase20.19 Research Proposal · `strategy_version.v1` · `researchIndex.vue`

## Status

**PASS**

可以进入最小实现。范围必须锁死：内存投影补上 `finding_id` 引用，记下 `hypothesis` 与 `recorded`。不落库，不接 Proposal，不接 Strategy Version，不自动跑实验。

## Implementation Scope

### 1. experiment.v1 当前实现

| 项 | 现况 |
|----|------|
| schema 位置 | 前端常量 `RESEARCH_EXPERIMENT_SCHEMA = 'experiment.v1'`，文件 `researchExperimentProjection.js` |
| 存储位置 | **无。** 无 Go 表，无 migration，无持久化 |
| 创建入口 | `projectResearchExperiment({ hypothesis, researchReport, backtestResult })` |
| 查询入口 | **无。** 无列表 API，无按 id 查询 |
| 使用方 | 仅单元测试。研究页没有调用该投影的面板 |

**已有能力**

- 内存卡片：`experiment_id`（`rexp:…`）、`hypothesis`、报告 / 回测身份回声、`status = recorded`
- 缺假设或报告 / 回测模式不对 → `available: false`，不编造
- 与 `strategy_experiment.v1` 隔离；不创建 StrategyVersion

**缺失能力**

- 不引用 `finding_id`
- 创建仍强制报告 + 回测结果，与 27B「以 finding 为关联」不一致
- 无历史查询、无前端研究实验入口
- Research Proposal 投影未落地，无法把 `rexp:` 留档为研究确认

研究页已有的是 `StrategyExperimentFoundationPanel` 与 `ExperimentTemplateFoundationPanel`。那是策略侧 / 模板，不是 `experiment.v1`。

### 2. finding_id 关联影响

| 影响面 | 判断 |
|--------|------|
| schema | 可选 `finding_id` 字符串，只引用。可保持 `experiment.v1` 名（27B：兼容扩展，不开 `v1.1`） |
| migration | **无。** 没有实验表 |
| API | **无。** 没有实验 API；MVP 不新增 |
| 前端 | 可选：调用投影时传入 finding 引用。MVP 可不做面板 |
| 历史数据 | **无。** 无库内历史行可迁。旧内存卡片没有该字段，当作 null |

**可以 nullable 兼容新增。** 旧调用方仍可只传报告 + 回测；新调用方传 `finding_id` + `hypothesis`。不得把发现正文、`statistics` 或研究行嵌进卡片。

### 3. Research Finding 连接点

`research_finding.v1` 由 `runResearchBatch` 在内存产出。`finding_id` 形如 `rf:…`，稳定、无时钟。

访问方式：调用方持有发现数组。无 Finding 表，无按 id 的后端查询。

Experiment **只保存引用**即可。策略侧 `strategy_experiment.v1` 已有 `source.finding_ids` 先例，但那是 Strategy Layer，Research Experiment 不得并入该对象。

### 4. Proposal 连接检查

**现在不能把 `experiment.v1` 当作已实现的 Research Proposal 输入。** Phase20.19 只审计，未写投影。

`strategy_proposal.v1` 只吃策略实验，且要 `draft|reviewed|approved` 与旋钮差量。`recorded` 不在允许列表。禁止放宽去接 `rexp:`。

MVP **不接** Proposal，更不接 Strategy Version。`Proposal → Strategy Version` 边界不动。

### 5. 最小实现范围

MVP 只需要：

- 创建 Experiment（内存投影）
- 记录 `finding_id`（可空兼容）
- 记录 `hypothesis`
- `status = recorded`
- 同进程内查询 / 列出调用方已创建的卡片（可选；非持久）

不要：

- 自动实验执行、自动回测、参数优化、策略生成
- 读 SignalEvent / 行情 / OutcomeObservation
- 落库、新 API、改报告白名单
- 创建 Research Proposal 或 StrategyVersion

## Schema Impact

最小实现若只增加可选 `finding_id` 引用：仍用 `experiment.v1`。本审计阶段 **Schema Change: 未改**。进入编码时再动投影，不得开新 schema 名，不得嵌发现体。

## Migration Impact

NONE（无表）

## API Impact

NONE（MVP 不建接口）

## Frontend Impact

NONE 作为硬依赖。研究页可不改。若加入口，只调用投影，不拉行情，不触发策略面板。

## Risk

| 风险 | 说明 |
|------|------|
| 与策略实验混淆 | 同页已有 Strategy Experiment。`rexp:` 与 `sexp:` / `strategy_experiment` 不得混用 |
| 创建门仍绑回测 | 若 MVP 仍强制报告 + 回测，finding 关联形同未落地。最小实现应允许「假设 + finding_id」成卡，报告 / 回测身份保持可选回声 |
| 误接 Proposal / Version | 卡片一出现就被送进 `createStrategyVersion`。MVP 禁止任何自动下游 |
| 假查询 | 无库却声称「历史实验」。文档与 UI 只能写内存 / 会话列表 |

## 结论

**可以进入最小实现。**

条件：只扩展 `projectResearchExperiment` 一类内存能力；`finding_id` 可空引用；`hypothesis` + `recorded`；不落库；不建 API；不接 Research Proposal；不接 Strategy Version；不自动执行。

## Protocol Impact

Code Change:
NONE（本审计）

Schema Change:
NONE（本审计）

Migration:
NONE

API Change:
NONE
