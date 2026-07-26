# Phase6.7-G 股票筛选性能优化 — 第一阶段实现报告

> 日期：2026-07-26  
> 设计：`PHASE6_7_G_IMPLEMENTATION_PLAN.md`  
> 范围：快照任务化 + 任务状态 UI + 信号搜索复用快照  
> **未改**：策略公式 / 信号 JS / 评分 / Candidate / TradePlan / Risk / Paper / Execution / 行情源

---

## 1. 修改文件

| 文件 | 变更 |
|---|---|
| `PHASE6_7_G_IMPLEMENTATION_PLAN.md` | 设计（先于实现） |
| `backend/data/signal_scan_task.go` | **新** 异步任务注册表 + `StartFullMarketSnapshotAsync` |
| `backend/data/signal_scan_task_test.go` | **新** 任务注册表单测 |
| `app_signal_scan.go` | `StartSignalScanSnapshot` / `GetSignalScanTask` / `GetLatestSignalScanTask`；保留同步 `RunSignalScanSnapshot`（兼容/测试） |
| `frontend/src/components/allStockList.vue` | 生成快照改异步；任务状态展示；带信号搜索禁止全市场重扫 |
| `frontend/wailsjs/go/main/App.js` | 绑定 Start/GetTask |
| `frontend/wailsjs/go/main/App.d.ts` | 类型声明 |
| `PHASE6_7_G_STOCK_SCREEN_PERFORMANCE_IMPLEMENTATION_REPORT.md` | 本报告 |

**未改：** `signal_scan_runner.go` / `signal_scan_bundle.js` / `icePointSignals` / TradePlan / PaperTrading。

---

## 2. 架构变化

### 2.1 生成快照（任务化）

```text
之前：
  UI await RunSignalScanSnapshot → 阻塞 20～40 分钟 → 返回

现在：
  UI StartSignalScanSnapshot → 立即返回 task
       ↓
  goroutine: 仍调用 RunFullMarketSnapshot（计算路径不变）
       ↓
  Events: signalScanProgress / signalScanDone
  查询: GetLatestSignalScanTask / GetSignalScanTask
```

任务状态：`pending | running | completed | failed`  
字段：`taskId, startTime, endTime, durationMs, snapshotId, hitTotal, message, error, phase/done/total`

进程内单飞：继续使用既有 `signalScanRunning`。

### 2.2 搜索（复用快照）

```text
带信号标签：
  ensureSnapshotForSignalFilter()
    → 当前/历史/最新 done 快照 payload
    → 本地 applySnapshotPayload 过滤
  无快照 → warning「需要先生成快照…」；禁止 loadWithSignalFilter 全市场重算

无信号标签：
  仍 GetAllStocks 单页 + 可选页内 scanPageSignals（轻量）
```

### 2.3 UI

- 「生成状态：未运行/运行中/已完成/失败」
- 开始时间、耗时、命中数摘要
- 「实时扫描」改为「退出快照」（不再触发全市场信号重算）
- 进度条仍监听 `signalScanProgress`

---

## 3. 性能提升（预期 / 机制）

| 场景 | 之前 | 之后 |
|---|---|---|
| 点击生成快照 | UI 阻塞 20～40 分钟 | **立即返回**；后台继续算 |
| 带信号标签搜索（已有快照） | 再全市场拉 K + 算 | **本地过滤，秒级** |
| 带信号标签搜索（无快照） | 自动全市场扫描 | **提示先生成，不自动扫** |
| 策略结果 | — | **同** `RunFullMarketSnapshot` |

全市场扫描墙钟时间仍受东财 K 线扇出限制（本阶段不改行情源）；优化点是 **交互等待** 与 **重复计算消除**。

---

## 4. 测试结果

| # | 项 | 结果 |
|---|---|---|
| 1 | `go test ./backend/data -run TestSignalScanTaskRegistry` | **PASS** |
| 2 | `go build`（主包，含新 Bind） | **PASS**（产物可编译） |
| 3 | 策略/计算路径 | Worker **直接调用**既有 `RunFullMarketSnapshot`，未改 goja/bundle |
| 4 | Trade Plan / Paper Trading | **无文件改动**于相关包 |
| 5 | 一致性 | 同一 `RunFullMarketSnapshot` → `tag` / `sortRank` / `statusText` 与优化前同路径一致 |

手工建议（运行时）：

1. 打开「股票筛选」页  
2. 点「生成快照」→ 应立刻提示任务已创建，状态「运行中」  
3. 完成后状态「已完成」、历史下拉出现新快照  
4. 勾选信号标签点搜索 → 应走快照本地过滤；无快照时仅提示  

---

## 5. 是否影响交易链

| 链路 | 影响 |
|---|---|
| Candidate Pool | 无 |
| Trade Plan / Approve / Freeze | 无 |
| Risk / Position Sizing | 无 |
| Paper Trading / Execution | 无 |
| 行情源 / K 线 API | 无（仍东财；仅调用时机改为后台） |

---

## 6. 兼容说明

- `RunSignalScanSnapshot` **保留**（同步），供旧脚本/测试；UI 已改用 `StartSignalScanSnapshot`。  
- 快照表结构未改；仍写 `status=done` 的 `signal_scan_snapshots`。  
- 任务状态默认 **进程内存**（重启丢失 pending/running）；完成后的快照仍在 DB。

---

## 7. 停止条件

第一阶段完成。不扩展 Market Intelligence Layer；不修改策略与交易逻辑。
