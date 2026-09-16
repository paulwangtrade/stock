# PHASE20.27F Build And Runtime Acceptance

**日期：** 2026-09-15  
**前置：** [PHASE20_27E_BUILD_READINESS_AUDIT.md](./PHASE20_27E_BUILD_READINESS_AUDIT.md) READY  
**性质：** 编译 + 运行验收。不新增功能，不进入 Proposal

## Final Status

**PASS**

新 `go-stock.exe` 已生成并启动。Experiment 新/旧创建路径与边界通过自动化核对；Research 页签已打进前端产物。Wails 桌面 UI 未做浏览器点击自动化。

## Build Result

| 项 | 值 |
|----|-----|
| 命令 | `wails build`（`D:\stock`） |
| BUILD_START | 2026-09-15（日志墙钟约 23:11） |
| BUILD_END | 2026-09-15 23:13:51 |
| Wails 报告耗时 | 2m9.061s |
| 墙钟约 | 131s |
| 结果 | Compiling frontend / application / post build hook：**Done** |
| 输出 | `D:\stock\build\bin\go-stock.exe` |
| 大小 | 97,278,976 bytes |
| LastWriteTime | 2026-09-15 23:13:49 |
| HEAD | `4382b86` |
| Workspace | 约 **1998** 条 dirty（与 27E 一致；非仅 Experiment） |

前端产物含：

- `frontend/dist/assets/ResearchExperimentFoundationPanel-Dhew__e-.js`
- `researchIndex` chunk 异步加载该面板（「研究实验」页签）

## Runtime Result

| 项 | 结果 |
|----|------|
| 启动 | `Start-Process` 工作目录 `D:\stock\build\bin` |
| 进程 | `go-stock` PID 14168，StartTime 2026-09-15 23:14:51 |
| Runtime profile | `build\bin\runtime\profile.json` 已写入 |
| `exe_dir` | `D:\stock\build\bin` |
| `exe_dir_matches_cwd` | **true** |
| `database_exists_at_exe_dir` | **true** |
| `database_path` | `D:\stock\build\bin\data\stock.db` |
| DB 文件 | 存在；本次启动后 mtime 更新至 23:15:29（约 572MB） |
| 首页 / 存活 | 进程持续运行；无启动 panic |
| 附带噪音 | `error.log` 有东财 K 线 HTTP EOF / 新闻超时——外网行情噪声，非 Experiment，未导致退出 |

继续遵循 **exe-dir** 运行时规则。

## Feature Verification

### Experiment

| 路径 | 结果 |
|------|------|
| 新：`finding_id` + `hypothesis` → `status=recorded` | **通过**（`createResearchExperiment` 实测） |
| 旧：`hypothesis` + report + backtest → `recorded` | **通过**（`projectResearchExperiment` 实测；`finding_id` 可为 null） |
| 单元测试 | `node --test …researchExperimentProjection.test.mjs` → **9 passed** |

### Research

| 项 | 结果 |
|----|------|
| Research 页面进包 | **通过**：`researchIndex` dist 挂载 `ResearchExperimentFoundationPanel` |
| research_finding 展示 | Experiment 面板以 **finding_id 输入** 引用发现；不复制 finding 正文。无独立 Finding 展柜要求于 27C MVP |
| 桌面内点击 Research 各页签 | 未做 Wails UI 自动化；靠进程 + 产物证明页签已嵌入 |

### 边界

| 禁止项 | 结果 |
|--------|------|
| 交易入口 | 面板无 TradePlan / 下单控件 |
| 自动执行 | 无；仅「记录实验」 |
| 策略生成 | 文案写明不生成 Strategy Version；投影不调用 `createStrategyVersion` / `runResearchBatch` |
| Proposal | 未接入 |

## Regression Check

| 项 | 结果 |
|----|------|
| 程序启动 | 通过 |
| 数据库加载（exe-dir） | 通过 |
| Schema / Migration | 无变更；无需升级 |
| ResearchReport 白名单 | 本阶段未改 |
| Strategy Version | 本阶段未接 |
| 外网 K 线 EOF | 记录为环境噪声，不判 Experiment 失败 |

## Notes

1. 新 exe 已在运行。若需再编，先关闭该进程。  
2. 工作区仍很脏；本包不是「仅 27C」干净 release。  
3. 本验收未进入 Proposal，未新增功能。
