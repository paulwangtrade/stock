# PHASE20.27E Build Readiness Audit

**日期：** 2026-09-15  
**性质：** 编译前只读审计。不改代码、不改 schema、不改 migration  
**前置：** [PHASE20_27D_EXPERIMENT_ACCEPTANCE_FREEZE.md](./PHASE20_27D_EXPERIMENT_ACCEPTANCE_FREEZE.md) PASS

## Status

**READY**

Experiment 最小实现是纯前端内存能力，无新 API、无 migration。工作区可进入 `wails build`。工作区整体很脏，新 exe 会带上大量与 27C 无关的未提交改动。

## Workspace

| 项 | 结果 |
|----|------|
| Experiment 相关 | `researchExperimentProjection.js` / `.test.mjs`、`ResearchExperimentFoundationPanel.vue` 为未跟踪；`researchIndex.vue` 已修改（含「研究实验」页签） |
| 27 系列文档 | `PHASE20_27A`–`27D` 未跟踪 |
| 整体脏度 | 约 **1997** 条 porcelain 记录（大量 PHASE 文档、backend、frontend） |
| 未提交风险 | **高。** 新 exe 不是「仅 Experiment」增量包，而是整棵脏工作树 |
| 异常大文件 | Experiment 文件本身很小。现有 `build\bin\go-stock.exe` ≈ 97MB（2026-09-12），属既有产物 |
| 误修改 | Experiment 模块未见误接策略 / 行情。`researchIndex.vue` 相对 HEAD 还包含大量更早研究页签，不单是 27C |

`researchExperimentProjection.js` 在 HEAD 中不存在（磁盘有、仓库未入库）。这不阻塞编译，但说明提交前需单独整理。

## Frontend

| 项 | 结果 |
|----|------|
| `App.vue` / `main.js` | 入口完整；路由挂载正常 |
| router | `/research` → `researchIndex.vue`，hash 模式 |
| Research 页面 | `researchIndex.vue` 结构完整，script/template/style 闭合 |
| Experiment 页面 | `ResearchExperimentFoundationPanel.vue` 完整；import 指向同目录 utils |
| 截断 / import | 未见截断。`import('./researchExperimentProjection.js')` 运行时加载 OK |
| 编译错误（静态） | Experiment 相关无缺失符号。全仓前端正式 `npm run build` 本阶段未跑 |

## Backend

| 项 | 结果 |
|----|------|
| Go 入口 | `main.go` → `wails.Run`；`Bind: []{ app }` |
| Wails 配置 | `wails.json` → `outputfilename: go-stock`；前端 `npm run build` |
| API | Experiment **未**新增 Go 方法，无未注册绑定 |
| postBuild | `scripts/sync-beta-runtime-data.ps1` 同步 `data/*.json` → `build/bin/data/` |

## Database

Schema: **NONE**（相对 27D：仍为可空 `finding_id` 的 `experiment.v1` 内存字段）  
Migration: **NONE**  
数据库：**无需升级。** 实验不落库。

## Build Risk

| 风险 | 说明 |
|------|------|
| 运行中的 exe | 编译前需关闭已打开的 `go-stock.exe`，否则可能出现 bindings Access denied |
| 脏工作树 | 新产物会包含 ~2k 未提交变更，不仅是 Experiment |
| 输出路径 | `D:\stock\build\bin\go-stock.exe` |
| 运行时数据 | 继续 **exe-dir** 规则：`data/`、`logs/`、`runtime/` 相对可执行文件目录 |
| 旧 exe | 当前 bin 内 exe 日期早于 27C；需重新 `wails build` 才会带上研究实验页 |

## Test Result

```powershell
Set-Location D:\stock\frontend
node --test src/utils/researchExperimentProjection.test.mjs
```

**9 passed / 0 failed**（本审计复跑）。

明显回归风险：低（未改报告白名单、Strategy Version、观察生产、Wails Bind）。全仓其他脏文件的回归不在本阶段证明范围内。

## Recommendation

1. **可以编译。** 关闭运行中的 `go-stock.exe` 后执行：

```powershell
Set-Location D:\stock
wails build
```

2. 成功后启动新 `build\bin\go-stock.exe`，在研究中心打开「研究实验」页签做一次手工冒烟。  
3. 若需要「仅含 Experiment」的可追溯发布，先整理 / 提交工作区，再编；当前脏树不适合当作干净 release 基线。  
4. 本阶段不改任何业务文件；本审计文件为交付物。
