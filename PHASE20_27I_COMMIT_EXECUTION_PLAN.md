# PHASE20.27I Commit Execution Plan

**日期：** 2026-09-16  
**性质：** 只输出执行计划。禁止 git add / commit / reset / 删除  
**依据：** [PHASE20_27H_COMMIT_BOUNDARY_AUDIT.md](./PHASE20_27H_COMMIT_BOUNDARY_AUDIT.md)  
**HEAD（计划时）：** `4382b86`

## Status

**READY**

提交顺序与文件清单已按 27H 边界定稿。执行时须**按路径精确 add**，禁止 `git add .`。本文件本身不执行任何 git 操作。

## Commit Order

```text
1. Commit 1 — Phase20.27 Documentation
2. Commit 2 — Experiment MVP（仅三件套）
3. （延后）researchIndex 仅「研究实验」挂载 — 方案 A
```

先文档后代码：文档无依赖；代码不依赖文档；挂载单独处理以免污染 MVP。

---

## Commit Files

### Commit 1 — Phase20.27 Documentation

**建议说明：** `docs: Phase20.27 Experiment audit and acceptance chain`

| # | 文件 |
|---|------|
| 1 | `PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md` |
| 2 | `PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md` |
| 3 | `PHASE20_27C_EXPERIMENT_IMPLEMENTATION_AUDIT.md` |
| 4 | `PHASE20_27C_EXPERIMENT_IMPLEMENTATION_COMPLETE.md` |
| 5 | `PHASE20_27D_EXPERIMENT_ACCEPTANCE_FREEZE.md` |
| 6 | `PHASE20_27E_BUILD_READINESS_AUDIT.md` |
| 7 | `PHASE20_27F_RUNTIME_ACCEPTANCE.md` |
| 8 | `PHASE20_27G_WORKSPACE_STABILIZATION_AUDIT.md` |
| 9 | `PHASE20_27H_COMMIT_BOUNDARY_AUDIT.md` |
| 10 | `PHASE20_27I_COMMIT_EXECUTION_PLAN.md`（本文） |

**确认：** 仅 A–I 文档链。不含 `PHASE20_23`–`26`、不含 Phase10/17、不含任何 `.js` / `.vue`。

**执行时（仅当用户下令）：** 逐文件 `git add <path>`，再 commit。不要 `git add PHASE20_*.md`。

---

### Commit 2 — Experiment MVP

**建议说明：** `feat(research): Phase20.27 minimal Experiment reference (finding_id)`

| # | 文件 |
|---|------|
| 1 | `frontend/src/utils/researchExperimentProjection.js` |
| 2 | `frontend/src/utils/researchExperimentProjection.test.mjs` |
| 3 | `frontend/src/components/ResearchExperimentFoundationPanel.vue` |

**确认不含：**

| 禁止混入 | 核对 |
|----------|------|
| Proposal | 无 `strategyProposal*` / StrategyProposal 面板 |
| Strategy | 无 `strategyVersion*` / StrategyExperiment / 策略页签 |
| Execution | 无 TradePlan / paper / 下单相关 |
| `researchIndex.vue` | **不进本 commit** |
| 文档 | 已在 Commit 1 |

**能力覆盖：** `experiment.v1`、`finding_id` 引用、create/list/get、Foundation UI、测试。Research 导航挂载留到延后步。

---

## researchIndex.vue

**采用方案 A：后续拆分后提交。**

不选 B（整文件单独提交），原因：

1. 当前 diff 含研究总览、信号链、回测、策略提案/版本等大量非 27 页签。  
2. 那些面板多为未跟踪文件；整文件提交会导致检出后 **import 断裂**。  
3. 会把 Proposal / Strategy **挂载**写进历史，违反 Commit 2 边界。

**方案 A 后续动作（需另一次明确指令）：**

- 只保留 / 只暂存：`ResearchExperimentFoundationPanel` 的 import +「研究实验」`n-tab-pane`；  
- 或先把工作区 `researchIndex.vue` 收敛成相对 HEAD 仅上述两处，再单独小提交；  
- 可用 `git add -p`（仅当用户允许 git 操作时）。

在完成方案 A 之前，仓库内已有 Experiment 组件，但官方 Research 页签入口未入库；本地脏工作区仍可展示（27F 已验）。

---

## Deferred Files

| 项 | 处置 |
|----|------|
| `researchIndex.vue` | 方案 A 延后 |
| `PHASE20_23` … `PHASE20_26B` | 可选后续 docs commit |
| Strategy Proposal / Version 及面板 | 延后；永不进 Commit 1/2 |
| 其他研究 Foundation 面板 `??` | 延后 |
| backend / 持仓 / 自选 WIP | 延后 |
| Phase10/17 文档海 | 延后 |
| `build/` `dist/` `*.exe` `*.db` | 永不提交 |

### 未跟踪特别项：`?? --`

| 分类 | 判定 |
|------|------|
| 保留 | 否 |
| 延后 | 否（不必进任何 27 commit） |
| **删除候选** | **是** |

抽查内容像是 `main.go` 片段副本（`package main`、embed、go-stock imports），文件名非法/异常（`--`），日期 2026-07-21。  
**本计划不执行删除**；清理须用户另下指令。

其他 `??`：除 Commit 1/2 清单外，一律 **延后**，不保留进 v0.20.27 两笔提交。

---

## Risk

| 风险 | 缓解 |
|------|------|
| `git add .` | 禁止；只 add 上表路径 |
| Commit 2 误加 `researchIndex.vue` | 清单写死三件套；add 前 `git status` / `git diff --cached` |
| Commit 1 误加 PHASE10 海 | 只 add `PHASE20_27*.md` 中上表十个文件名 |
| 本地有入口、干净检出无入口 | 已知；靠方案 A 补挂载 |
| 误删 `--` | 本阶段不删 |

## Next Step

1. 用户明确下令「按 27I 执行 Commit 1」→ 仅 add 文档十文件 → commit。  
2. 用户明确下令「按 27I 执行 Commit 2」→ 仅 add 三件套 → commit。  
3. 另开任务处理 `researchIndex` 方案 A（允许改文件或 `add -p` 时再做）。  
4. `?? --` 删除候选：仅在用户明确要求清理时处理。

## Protocol Impact

Git: NONE（本计划未执行）  
Code Change: NONE  
Schema: NONE  
Migration: NONE
