# Phase20.27G Workspace Stabilization Audit

**日期：** 2026-09-15  
**性质：** 只审计与稳定化设计。禁止 `git add .`、commit、删除、改代码 / schema / migration  
**前置：** [PHASE20_27F_RUNTIME_ACCEPTANCE.md](./PHASE20_27F_RUNTIME_ACCEPTANCE.md) PASS  
**HEAD：** `4382b86`  
**Dirty：** 约 **1999** 条

## Status

**NEED REVIEW**

Build / Runtime / Experiment 均已 PASS，但工作区远未干净。可以按下列 Keep / Ignore / 提交建议稳定化；**不得**一次 `git add .`。`researchIndex.vue` 混有 27C 与更早研究页签，提交前需人工核对。

## Workspace Summary

| 项 | 值 |
|----|-----|
| HEAD | `4382b86` |
| Dirty 约数 | 1999 |
| Build | `build/bin/go-stock.exe` 已生成且被 ignore |
| Runtime DB | `build/bin/data/stock.db`（~572MB，运行数据） |
| Repo `data/stock.db` | ignore（`*.db`）；另有一份较旧副本 |
| 异常根文件 | `?? --`（约 14KB，名称异常） |

按顶层粗分（约数，有重叠归类）：

| 桶 | 约数 | 含义 |
|----|------|------|
| PHASE10/17/19 等历史文档 | ~366+ | 大量遗留 md |
| PHASE20 其他文档 | ~107 | 非 23–27 的 Phase20 文档 |
| PHASE20.23–26 文档 | 8 | 观察 / Dataset / Engine 设计链 |
| PHASE20.27 文档 | 7（本审计写入后为 8） | Experiment 验收链 |
| Experiment 代码+测试 | 3 | 投影 / 面板 / 测试 |
| Strategy Proposal/Version 等 | 12 | **非 27 Experiment 范围** |
| `researchIndex.vue` | 1 | 含研究实验页签，也含大量其他页签 |
| 其他 frontend | ~163 | 持仓 / 机会 / 行情等 |
| backend | ~16 | opportunity / papertrading 等 |
| build/dist 进 status | 0 | 已被 `.gitignore` 挡住 |

## Dirty Classification

### A. 必须保留（20.27 Experiment 基线候选）

**代码 / 测试**

- `frontend/src/utils/researchExperimentProjection.js`
- `frontend/src/utils/researchExperimentProjection.test.mjs`
- `frontend/src/components/ResearchExperimentFoundationPanel.vue`
- `frontend/src/components/researchIndex.vue`（**仅**「研究实验」相关挂载；整文件还含其他研究页签 → 见 Risk）

**文档（20.27）**

- `PHASE20_27A_EXPERIMENT_LAYER_DESIGN_AUDIT.md`
- `PHASE20_27B_EXPERIMENT_PROTOCOL_GAP_FREEZE.md`
- `PHASE20_27C_EXPERIMENT_IMPLEMENTATION_AUDIT.md`
- `PHASE20_27C_EXPERIMENT_IMPLEMENTATION_COMPLETE.md`
- `PHASE20_27D_EXPERIMENT_ACCEPTANCE_FREEZE.md`
- `PHASE20_27E_BUILD_READINESS_AUDIT.md`
- `PHASE20_27F_RUNTIME_ACCEPTANCE.md`
- `PHASE20_27G_WORKSPACE_STABILIZATION_AUDIT.md`（本文）

**可选同包文档（上游冻结，非运行代码）**

- `PHASE20_23` … `PHASE20_26B` 共 8 份（观察 / Dataset / Engine 设计）。可进独立 docs commit，勿与 Strategy 混提。

### B. 已生成产物（勿纳入版本管理）

| 路径 | 说明 |
|------|------|
| `build/bin/go-stock.exe` | ~97MB；`.gitignore` 已忽略 `*.exe`、`build/bin/` |
| `build/bin/data/stock.db` | 运行库；已 ignore |
| `build/bin/runtime/profile.json` | 运行时诊断 |
| `build/bin/logs/*` | 运行日志 |
| `frontend/dist/` | 前端构建产物；已 ignore |

**结论：exe / dist / 运行库不应 git add。**

### C. 历史遗留修改（不进 v0.20.27 Experiment 提交）

- 大量 `PHASE10_*` / `PHASE17_*` / `PHASE19_*` / 其他 `PHASE20_*`（非 23–27）未跟踪或已修改 md
- `backend/opportunity/*`、`backend/papertrading/*`、持仓 / 自选 / K 线相关 frontend 修改
- 未跟踪的 Strategy 侧：`strategyProposal*.js`、`strategyVersion*.js`、对应 Foundation 面板与测试  
  → 属于更早策略链，**不是** 27 Experiment；单独处置
- `PHASE20_18_EXPERIMENT_*`、`outcomeObservationProduction.*`、`PHASE20_21/22` 等：上游或并行工作，勿塞进「仅 27 Experiment」一笔

### D. 风险文件

| 文件 | 风险 |
|------|------|
| `frontend/src/components/researchIndex.vue` | 相对 HEAD +83 行：研究总览及大量 foundation 页签 + 研究实验。一次整文件提交会混入非 27 范围 UI |
| `?? --` | 异常文件名；内容未知；勿盲目 add |
| Strategy Proposal / Version 未跟踪集 | 与 Experiment 同屏出现在 status 过滤里，易被误装进同一 commit |
| `build/bin/data/stock.db` | ~572MB 运行数据；启动后 mtime 更新属正常；**勿提交** |
| 全量 dirty ~1999 | `git add .` 会把遗留文档与业务 WIP 全部打进基线 |

## Keep List

进入 **v0.20.27 Experiment** 最小基线时保留：

1. 上述 A 类 Experiment 三文件 + 测试  
2. `researchIndex.vue` 中与「研究实验」页签相关的变更（或接受整文件但须在提交说明中写明混入范围）  
3. `PHASE20_27A`–`PHASE20_27G` 文档  

可选第二笔：`PHASE20_23`–`PHASE20_26B` 设计文档。

## Ignore List

- `build/bin/**`、`*.exe`、`frontend/dist/**`
- `*.db`（含 `data/stock.db`、`build/bin/data/stock.db`）
- `build/bin/logs/**`、`build/bin/runtime/**`
- 根目录 `?? --`（先人工打开看内容，默认不进库）
- Strategy Proposal / Version 全套（直到单独 Phase）
- Phase10/17 海量遗留 md（直到专门文档整理）

## Risk Files

见上表 D。稳定化前最大操作风险是：**对脏工作区执行 `git add .`**。

## 20.27 实现范围核对

| 要求 | 结果 |
|------|------|
| `experiment.v1` + `finding_id` | 在 `researchExperimentProjection.js` |
| Research Experiment 页面 | `ResearchExperimentFoundationPanel.vue` + `researchIndex` 页签 |
| 测试 | `researchExperimentProjection.test.mjs` |
| 未混入 Proposal 实现进 Experiment 模块 | **通过**（投影不调用 Proposal） |
| 未混入 Strategy Version / Execution 进 Experiment 模块 | **通过** |
| 工作区是否另有 Proposal/Version 未跟踪文件 | **有**——须排除在 Experiment commit 之外 |

## Build 产物检查

| 项 | 结果 |
|----|------|
| 路径 | `D:\stock\build\bin\go-stock.exe` |
| 大小 | ~97.3MB（正常桌面产物量级） |
| 是否纳入版本管理 | **否**（已 ignore） |
| 异常大文件进 git status | **否**（bin/db 未出现在 porcelain） |

## 数据文件检查

| 项 | 结果 |
|----|------|
| `build/bin/data/stock.db` | 运行数据；27F 启动后被打开属预期；**不要 migration** |
| 是否意外「逻辑迁移」 | 无 schema/migration 变更 |
| 是否需要备份 | 若担心运行损坏，可在仓外复制备份；**不**进 git |
| `data/stock.db` | 较旧、已 ignore；与 exe-dir 运行库分离 |

## Recommended Commit Plan

**只建议，不执行。**

```text
commit A — feat(research): Phase20.27 minimal Experiment reference
  - researchExperimentProjection.js / .test.mjs
  - ResearchExperimentFoundationPanel.vue
  - researchIndex.vue（仅研究实验挂载；若无法拆分则注明含其他研究页签）
  - 可选：不带 md

commit B — docs: Phase20.27 Experiment audit chain
  - PHASE20_27A … PHASE20_27G

commit C（可选）— docs: Phase20.23–26 observation/engine freezes
  - PHASE20_23 … PHASE20_26B

以后另开：
  - Strategy Proposal / Version 未跟踪集
  - backend opportunity / papertrading WIP
  - Phase10/17 文档海
```

禁止：单笔塞进 Strategy + Experiment + 全部 PHASE md。

## Next Step

1. 人工打开 `?? --` 与 `researchIndex.vue` diff，决定 A 类是否整文件入 commit A。  
2. 用户明确下令后，再按 Recommended Commit Plan **选择性** add/commit（仍禁止 `git add .`）。  
3. 提交后可用新 HEAD 作为 **go-stock v0.20.27** 开发基线标签候选（本阶段不打 tag）。  
4. 继续忽略 build/dist/db；日常开发在干净基线上开分支。

本审计到此结束。未改代码，未提交，未清理。
