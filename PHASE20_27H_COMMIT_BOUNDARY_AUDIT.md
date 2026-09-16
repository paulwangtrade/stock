# PHASE20.27H Commit Boundary Audit

**日期：** 2026-09-16  
**性质：** 只读提交边界审计。不执行 git add / commit / 删除  
**参考：** [PHASE20_27G_WORKSPACE_STABILIZATION_AUDIT.md](./PHASE20_27G_WORKSPACE_STABILIZATION_AUDIT.md)  
**HEAD：** `4382b86`

## Status

**NEED REVIEW**

Commit B（文档）边界清晰，可独立提交。  
Commit A（实现）在**不含**整份 `researchIndex.vue` 时边界清晰；若整文件入 A，会因未跟踪的其他研究 / 策略面板 import **破坏可编译性**，并混入 Proposal / Strategy UI。

## Commit A

**主题：** Experiment Implementation（MVP）

### Files（允许提交）

| 文件 | 角色 |
|------|------|
| `frontend/src/utils/researchExperimentProjection.js` | `experiment.v1`；`finding_id` 引用；create / list / get |
| `frontend/src/utils/researchExperimentProjection.test.mjs` | 相关测试 |
| `frontend/src/components/ResearchExperimentFoundationPanel.vue` | Research Experiment UI |

### 明确排除

| 排除 | 原因 |
|------|------|
| `frontend/src/components/researchIndex.vue` | 见下节；整文件不可进 A |
| `StrategyProposal*` / `strategyProposal*` | Proposal |
| `StrategyVersion*` / `strategyVersion*` | Strategy Version |
| `StrategyExperiment*` / `ExperimentTemplate*` | 策略实验 / 模板，非 27 Research Experiment |
| Execution / TradePlan / paper 相关 | 交易执行 |
| `PHASE20_27*.md` | 归 Commit B |
| `build/` / `dist/` / `*.db` / `*.exe` | 产物与运行数据 |

### researchIndex.vue 审计

相对 HEAD 的 diff 中：

**属于 20.27 Experiment 的片段（仅此）**

- `import('./ResearchExperimentFoundationPanel.vue')`
- `<n-tab-pane name="研究实验">` + `<ResearchExperimentFoundationPanel />`

**属于其他研究 UI / WIP（不得进 Commit A）**

- 研究总览、信号研究 / 比较 / 归因 / 生命周期、事件研究
- 回测数据集 / 数据集版本、因素观察、绩效、数据质量、市场环境
- 默认页签改为「研究总览」、`回测数据集版本` → `数据集版本` 别名
- 策略实验、实验模板、策略提案、策略版本、策略评价、策略证据、研究报告页签

**风险（高）**

当前工作区里，上述多数面板文件为 **`??` 未跟踪**。若把整份已改 `researchIndex.vue` 打进 Commit A 而不带这些面板：

1. 检出该 commit 后前端 **import 断裂**；  
2. 同时把 Strategy Proposal / Version **页签挂载**写进历史，违反 A 的排除边界。

**结论：** Commit A **不要**包含 `researchIndex.vue`。挂载「研究实验」页签须另一步：只改这两处 import/tab（或后续 `git add -p`），且不引入其他面板。本阶段不改代码、不执行 git。

## Commit B

**主题：** Phase20.27 Documentation

可独立提交（无代码依赖）：

| 文件 |
|------|
| `PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md` |
| `PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md` |
| `PHASE20_27C_EXPERIMENT_IMPLEMENTATION_AUDIT.md` |
| `PHASE20_27C_EXPERIMENT_IMPLEMENTATION_COMPLETE.md` |
| `PHASE20_27D_EXPERIMENT_ACCEPTANCE_FREEZE.md` |
| `PHASE20_27E_BUILD_READINESS_AUDIT.md` |
| `PHASE20_27F_RUNTIME_ACCEPTANCE.md` |
| `PHASE20_27G_WORKSPACE_STABILIZATION_AUDIT.md` |
| `PHASE20_27H_COMMIT_BOUNDARY_AUDIT.md`（本文） |

A–G（及 H）可以独立成 Commit B。不要混入 `PHASE20_23`–`26`（可选第三笔 docs）、不要混入 Phase10/17 文档海。

## Deferred

| 分类 | 文件 / 范围 |
|------|-------------|
| 延后 · UI 挂载 | `researchIndex.vue`（待拆成仅 Experiment 两处改动后再提） |
| 延后 · 上游设计文档 | `PHASE20_23` … `PHASE20_26B` |
| 延后 · 策略链未跟踪 | `strategyProposal*`、`strategyVersion*`、对应 Foundation 面板与测试 |
| 延后 · 其他研究面板未跟踪 | Dashboard / Signal* / Backtest* / Alpha* / … |
| 延后 · 业务 WIP | backend opportunity / papertrading、持仓与自选等已修改文件 |
| 延后 · 历史文档 | Phase10/17/19 大量 `??` / `M` md |
| 删除候选（勿删，仅标记） | 根目录 `?? --`（异常名；人工打开后再定） |
| 永不提交 | `build/bin/**`、`frontend/dist/**`、`*.exe`、`*.db`、logs、runtime profile |

## Risk

1. **`researchIndex.vue` 整文件入 A → 高风险**（范围污染 + 可能无法编译）。  
2. **`git add .` → 禁止**（~1999 dirty）。  
3. Strategy / Proposal 未跟踪集与 Experiment 文件相邻，选择性 add 时易误选。  
4. Commit A 不含页签挂载时：仓库内有面板组件，但 Research 导航尚无入口，直到拆分提交 `researchIndex`。

## Next Action

1. **先 Commit B**（仅上表 27 文档）——边界 READY，风险最低。  
2. **再 Commit A**（仅三件套：projection + test + FoundationPanel）。  
3. **另开准备步（需用户明确允许改代码或 `add -p`）：** 将 `researchIndex.vue` 收敛为只增加「研究实验」两处，再单独小提交。  
4. 全程禁止 `git add .`；本审计不执行任何 git 操作。

## Protocol Impact

Code Change: NONE（本审计）  
Schema: NONE  
Migration: NONE  
Git: NONE（未 add / 未 commit）
