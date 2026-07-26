# Phase6.7-G Runtime Verification Report

> 日期：2026-07-26  
> 范围：**运行时验证 + 最新 build 核对**（股票筛选性能优化：异步快照任务 + 搜索复用）  
> 性质：**仅验证** — 未改策略公式 / Candidate / TradePlan / Risk / Paper Trading / Execution  
> 设计：`PHASE6_7_G_IMPLEMENTATION_PLAN.md`  
> 实现：`PHASE6_7_G_STOCK_SCREEN_PERFORMANCE_IMPLEMENTATION_REPORT.md`  
> 审计基线：`PHASE6_7_F_STOCK_SCREENER_PERFORMANCE_AUDIT.md`

---

## 1. Build 状态

| 项 | 值 |
|---|---|
| 命令 | `wails build`（完整，含 bindings） |
| 产物 | `D:\stock\build\bin\go-stock.exe` |
| 大小 | **99,052,032** bytes（≈94.5 MB） |
| 编译时间 | **2026-07-26 22:27:05**（+08:00） |
| Git HEAD（构建时） | `a02cff0f`（Phase6.5.4.1…） |
| Phase6.7-G 源码状态 | **工作区未 commit**（`signal_scan_task.go` 等为新增/修改，已编入本次 exe） |
| 符号抽查 | exe 含 `StartSignalScanSnapshot` / `GetSignalScanTask` / `GetLatestSignalScanTask` ✅ |
| 前端绑定 | `frontend/wailsjs/go/main/App.js` / `App.d.ts` 已导出上述 API ✅ |

结论：**验证所用 exe 为当日最新完整编译产物，已包含 Phase6.7-G 异步任务 API。**

运行库路径（真相源）：`D:\stock\build\bin\data\stock.db`

---

## 2. 验证方法说明

| 路径 | 结果 |
|---|---|
| 桌面 WebView UI 自动化 | **未完成**：进程可启动，但 Native WebView `MainWindowHandle=0`，无法可靠点击「股票筛选」页 |
| 运行库 + Go 运行时探针 | **采用**：对 `StartFullMarketSnapshotAsync` / 快照查询 / `kline_cache` 侧效应做实测 |
| 单测 | `TestSignalScanTaskRegistry*`、`TestPhase67GReadLatestVerification` **PASS** |

说明：UI 点击路径以源码审查 + 绑定进 exe 作为替代证据；交互语义与 `allStockList.vue` 中 `StartSignalScanSnapshot` / `ensureSnapshotForSignalFilter` 一致。

---

## 3. 运行时检查结果

### 3.1 异步快照：立即返回

| 检查 | 结果 | 证据 |
|---|---|---|
| `StartFullMarketSnapshotAsync` 返回耗时 | **PASS** `return_ms=0`（≪ 2s） | `task_id=0ffd850c-…` status=`pending` |
| 短时进入 `running` | **PASS**（≈200ms） | 同 task，`VERIFY_ASYNC_RUNNING` |
| 进程内单飞 / 任务视图 | **PASS**（单测 + 运行时） | `TestSignalScanTaskRegistry*` |

此前完整等待轮次（最长约 55 分钟）也观察到：创建后立即返回、`pending→running`、fetch 进度推进至约 `5446/5531`；因东财 K 线大量 EOF / 腾讯回退导致墙钟极长，验证进程未能等到 `completed` 落库。

### 3.2 全市场扫描完成 → 新快照落库

| 检查 | 结果 |
|---|---|
| 本轮验证产生 **新** `signal_scan_snapshots` 行 | **未观察到** |
| 最新快照仍为 | **id=4** / trade_date=**2026-07-24** / scanned=5445 / hit=126 / duration_ms=**2,400,155**（≈40.0 分钟） |

现有历史快照（优化前墙钟基线，未变）：

| id | trade_date | scanned | hit | duration_ms |
|---|---|---|---|---|
| 4 | 2026-07-24 | 5445 | 126 | 2,400,155（≈40.0 min） |
| 3 | 2026-07-22 | 5444 | 54 | 1,930,369（≈32.2 min） |
| 2 | 2026-07-20 | 5442 | 56 | 1,205,806（≈20.1 min） |
| 1 | 2026-07-17 | 5443 | 83 | 1,507,088（≈25.1 min） |

结论：**异步「不阻塞调用方」已证实；「后台跑完全市场并写入新快照」在本验证窗口内未完成（行情扇出墙钟仍为 20～40 分钟量级，与 Phase6.7-F 一致，本阶段未改行情源）。**

### 3.3 带信号搜索复用快照（无重扫）

对运行库最新快照 id=4 做本地过滤，并核对 `kline_cache`：

| 检查 | 结果 |
|---|---|
| 读取最新快照 | **PASS** `query_ms≈5`（`TestPhase67GReadLatestVerification`） |
| 按信号标签本地过滤 | **PASS** tag=`买` → 18 条；`elapsed_ms=0` |
| 过滤期间 `kline_cache` 行数 / `MAX(fetched_at)` | **不变**（`kline_unchanged=true`） |
| 过滤期间 `IsRunning()` | **false**（搜索本身未启动全市场扫） |
| 样本策略字段 | 保留：如 `300020.SZ` / signal=`买` / score=9 / `今日出冰点买点 · RSI …` |

前端源码对应行为：`ensureSnapshotForSignalFilter()` — 有快照则 `applySnapshotPayload`；无快照则 warning「需要先生成快照…」，**禁止**自动全市场重扫。

### 3.4 「股票筛选」页（UI）

| 检查 | 结果 |
|---|---|
| UI 自动化实点「生成快照」 | **未执行**（无可用窗口句柄） |
| 代码路径 | `allStockList.vue` → `StartSignalScanSnapshot` → 成功提示「快照任务已创建…」+ `startScanTaskPoll` |
| 绑定进最新 exe | **是** |

---

## 4. 前后性能对比（交互视角）

| 场景 | 优化前（Phase6.7-F） | 优化后（本轮实测） |
|---|---|---|
| 点击生成快照（调用方等待） | 阻塞整次扫描 **20～40 分钟** | **立即返回**（实测 `return_ms=0`）；后台仍跑全市场 |
| 全市场扫描墙钟 | ≈20～40 分钟（DB `duration_ms`） | **未改计算/行情路径**；墙钟预期仍同量级（本轮未测到新 completed） |
| 带信号标签搜索（已有快照） | 可能再次全市场拉 K + 计算 | **本地过滤秒级**（实测 `elapsed_ms=0`，无 `kline_cache` 写入） |
| 带信号标签搜索（无快照） | 自动全市场扫描 | **提示先生成，不自动扫**（源码） |

要点：Phase6.7-G 优化的是 **交互阻塞** 与 **重复全市场计算**；不是把单次全市场 K 线扇出从 40 分钟压到秒级。

---

## 5. 交易链回归（只读）

### 5.1 Phase6.7-G 文件触达范围

| 文件 | 与交易链关系 |
|---|---|
| `backend/data/signal_scan_task.go`（新） | 仅任务注册表 + 调用既有 `RunFullMarketSnapshot` |
| `app_signal_scan.go` | 新增 Start/GetTask；无 TradePlan/Paper/Risk 引用 |
| `frontend/src/components/allStockList.vue` | 筛选页异步 UI + 搜索复用 |
| Wails `App.js` / `App.d.ts` | 绑定导出 |

**未改：** `signal_scan_runner` / 信号 JS bundle / icePoint 公式、Candidate、TradePlan Approve/Freeze、Risk、Paper Broker、Execution、行情源实现。

> 注：工作区另有大量与 Phase6.7-G **无关** 的脏文件（含 trade/paper 等）；本报告只断言 **G 范围文件** 未改交易语义。验证未对这些无关改动做全面回归。

### 5.2 运行库交易状态抽查（未因本次验证改写）

| 项 | 值 |
|---|---|
| `trade_plans` id=5 | trade_date=**2026-07-27** status=**ready** |
| approved_at | 2026-07-26 18:48:07 |
| freeze_at | 2026-07-26 18:48:07 |
| plan_version | 3 |

与 Phase6.7-E 激活后的计划状态一致；本次验证探针 **未** 调用 Generate / Approve / Freeze / Paper cron。

---

## 6. 已知问题 / 限制

1. **全市场后台完成未在本窗口落库**：HTTP 限流/EOF 下墙钟仍长；验证 harness 主动中止或超时，未得到新 `snapshot_id`。  
2. **桌面 UI 未点通**：WebView 句柄为 0，无法用浏览器 MCP 代替原生点选。  
3. **任务状态在进程内存**：重启后 pending/running 丢失；已完成快照仍在 DB（设计如此）。  
4. **行情源未优化**：单次全市场扫描仍可能 20～40 分钟（Phase6.7-F）。  
5. 验证产物：`backend/data/phase67g_runtime_verification_test.go`（仅验证用，未改生产路径）。

---

## 7. 总评

| 目标 | 判定 |
|---|---|
| 最新源码完整 `wails build` | **PASS**（22:27:05 exe） |
| 异步任务立即返回 | **PASS** |
| 搜索复用快照、无自动全市场重扫 | **PASS**（运行库过滤 + 源码） |
| 新快照 completed 落库（端到端） | **未在本轮证实**（墙钟/网络） |
| 桌面筛选页实点 | **未完成**（环境限制） |
| 交易链未被 G 改动破坏 | **PASS**（代码范围 + plan#5 只读抽查） |

**总体：Phase6.7-G 的核心交互目标（不阻塞 UI、搜索走快照）在运行时探针下成立；全市场扫描墙钟与 UI 实点仍待一次完整后台 completed 或人工点选补证。**

---

## 8. 建议的后续补证（非本阶段实现）

1. 交易日盘后人工：打开最新 exe →「股票筛选」→「生成快照」→ 确认状态机与完成后历史下拉出现新 id。  
2. 同一进程内等到 `GetSignalScanTask`=`completed` 后，再跑一遍 `TestPhase67GRuntimeVerification` 的落库断言。  
3. 勿在验证窗口对交易链做 Generate/Approve（避免污染 2026-07-27 ready 计划）。
