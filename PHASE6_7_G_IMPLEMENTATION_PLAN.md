# Phase6.7-G 股票筛选性能优化 — 实现设计（第一阶段）

> 日期：2026-07-26  
> 前置：`PHASE6_7_F_STOCK_SCREENER_PERFORMANCE_AUDIT.md`  
> 性质：**只读设计确认**；本文件不含代码改动说明以外的实现细节承诺  
> 约束：不改策略公式 / 信号计算 / 评分 / Candidate / TradePlan / Risk / Paper / Execution / 行情源

---

## 1. 代码审查结论

### 1.1 Signal Snapshot 数据结构

**表：** `signal_scan_snapshots`（`models.SignalScanSnapshot`）

| 字段 | 类型 | 用途 |
|---|---|---|
| `id` | PK | 快照 ID |
| `created_at` | time | 创建时间 |
| `trade_date` | string+index | 交易日 |
| `session` | midday\|close | 时段 |
| `scope` | default `all` | 范围 |
| `strategy_id` / `strategy_name` | string | 策略维度 |
| `signal_params_json` | text | 扫描时参数快照 |
| `scanned_total` / `hit_total` | int | 规模/命中 |
| `status` | string | 注释已写：`running \| done \| failed`（现状几乎只写 `done`） |
| `message` | string | 说明 |
| `result_json` | text | **完整命中列表 JSON**（`SignalScanResultPayload`） |
| `duration_ms` | int64 | 耗时 |

**Payload / Hit（`result_json` 内）：**

| 字段 | 说明 |
|---|---|
| `tag` | 信号标签（强/趋/转…）— 对应「signal」过滤 |
| `sortRank` | 排序秩 — 近似「score」展示用（非独立 score 列） |
| `SECUCODE` / `SECURITY_CODE` | 股票代码 |
| `rsi` / `statusText` / 行情字段 | 展示用 |
| `tradeDate` / `session` / `strategyId` | payload 元数据 |

**查询方式（已有）：**

| API | 行为 |
|---|---|
| `ListSignalScanSnapshots` | 按 trade_date/session/strategy 分页；**不含** ResultJSON |
| `GetSignalScanSnapshotDetail(id)` | 反序列化 ResultJSON → Payload |
| `GetLatestSignalScanSnapshot*` / Meta | 最新 done 快照 |
| `ParseSignalScanSnapshotPayload` | 内存解析 |

### 1.2 是否已具备「缓存结果」能力？

**是。** 一次全市场扫描落库后：

- 前端已有 `applySnapshotPayload`：按 `filterSignalTags` / 行业 / 市场板块 **本地过滤**；
- 无需再拉 K、再跑 goja。

缺口仅在于：

1. **生成**仍同步阻塞 UI（`await RunSignalScanSnapshot`）；
2. **带信号标签的搜索**走 `loadWithSignalFilter` **重新全市场实时扫描**，未强制复用快照。

### 1.3 生成快照能否拆成「任务创建 → 后台执行 → 状态查询」？

**可以，且风险可控。**

| 已有能力 | 说明 |
|---|---|
| `signalScanRunning` atomic | 全进程单飞，防并发双扫 |
| `status` 字段语义 | 已预留 running/done/failed |
| `EventsEmit(signalScanProgress/Done)` | 进度通道已存在 |
| `RunFullMarketSnapshot` | 纯后端计算入口，可被 goroutine 调用 |

建议 **不新建核心业务表**（满足「不改数据库核心模型」）：

- 用 **进程内 Task Registry**（`task_id` → 状态机）承载 pending/running/completed/failed；
- 任务成功后仍写入既有 `signal_scan_snapshots`（计算路径不变 → 结果一致）；
- `task_id` 可用 UUID；完成后关联 `snapshot_id`。

可选增强（非必须）：扫描开始时写一条 `status=running` 的快照行 — **本阶段可不落库 running 行**，避免半成品污染历史下拉；仅内存任务 + 完成后 done 快照。

### 1.4 同步 vs 可后台化

| 必须保留同步 | 原因 |
|---|---|
| `GetSignalScanSnapshotDetail` / List / Meta | 读缓存，应秒级 |
| 快照内本地过滤（标签/行业/关键字） | CPU 轻量 |
| `IsSignalScanRunning` / 任务状态查询 | 瞬时 |
| 单飞互斥 `CompareAndSwap` | 正确性 |

| 可后台化 | 原因 |
|---|---|
| 全市场名单拉取 | I/O 长 |
| 逐股日 K | **主耗时** |
| goja 批算 | CPU 中等 |
| 快照落库 | 尾部 |

| 本阶段明确不做 | 原因 |
|---|---|
| 改 `RunSignalScanBatchJS` / icePointSignals | 策略一致性 |
| 改 K 线源 / TTL | 行情源约束 |
| 拆 ResultJSON 到行表 | 属核心模型扩展，留给后续 |
| Market Intelligence Layer | 范围外 |

---

## 2. 第一阶段目标架构

```text
【生成】
UI「生成快照」
  → StartSignalScanSnapshotAsync(...)   // 立即返回 task
  → 显示 pending/running + 进度事件
  → 后台 goroutine: 现有 RunFullMarketSnapshot（逻辑不变）
  → completed → 刷新快照列表 / 可选自动选中最新

【搜索·带信号标签】
UI「搜索」/ 筛选变化
  → 若无有效 done 快照：提示「请先生成快照」，禁止 loadWithSignalFilter
  → 若有：加载/复用 payload → 本地过滤（与现 applySnapshotPayload 一致）

【搜索·无信号标签】
  → 保持现有 GetAllStocks 单页 + 可选页内 scan（轻量），不强制快照
```

### 2.A 快照生成任务化

新增（命名可微调）：

| Bind API | 行为 |
|---|---|
| `StartSignalScanSnapshot(session, params, strategyId, name)` | 创建 task，`go` 跑扫描，立即返回 `{taskId, status}` |
| `GetSignalScanTask(taskId)` / `GetLatestSignalScanTask()` | 返回 status、start/end、duration、error、snapshotId、hitTotal… |

状态机：`pending → running → completed | failed`

兼容：保留 `RunSignalScanSnapshot` 可改为内部调用 Start+wait（**本阶段建议前端只走 Start**，旧同步 API 标记为兼容或内部给测试用）。

### 2.B 页面任务状态显示

在 `allStockList.vue` 快照工具条增加只读状态区：

- 未运行 / 运行中 / 已完成 / 失败  
- 开始时间、耗时、命中数（完成后）  
- 保留现有进度条与结果表展示逻辑  

### 2.C 搜索优化

当 `hasSignalFilter === true`：

1. **禁止** `loadWithSignalFilter` 全市场重算；  
2. 优先使用当前选中或最新 `done` 快照的 payload；  
3. 无快照 → `message.warning('需要先生成快照')`，不自动扫描。

无信号标签：行为不变（列表查询）。

### 2.D 结果一致性

- Worker **直接调用现有** `RunFullMarketSnapshot`（不改 prepareStockBars / RunSignalScanBatchJS）；  
- 验证：同 session/strategy/params 下，随机抽 hit 对比 `tag` / `sortRank` / `statusText`（异步前后或与历史快照对比）。

---

## 3. 风险与门禁

| 风险 | 缓解 |
|---|---|
| 进程退出中断任务 | 状态 failed/丢失可接受；重启后可重新生成（第一阶段不要求持久化 pending） |
| 双重点击 | 沿用 `signalScanRunning` + UI loading |
| 交易链误伤 | 不触及 TradePlan/Paper/Execution 包 |
| 策略漂移 | 不改 JS bundle / 评分公式 |
| 无快照时信号筛选不可用 | **有意为之**（产品提示先生成） |

**设计确认：无阻断性风险，可进入第一阶段实现。**

---

## 4. 实现文件预估（下一步）

| 区域 | 文件 |
|---|---|
| Backend task | `backend/data/signal_scan_task.go`（新） |
| Bind | `app_signal_scan.go` |
| Frontend | `allStockList.vue`（+ 必要时 bindings 再生） |
| 报告 | `PHASE6_7_G_STOCK_SCREEN_PERFORMANCE_IMPLEMENTATION_REPORT.md` |

**不做：** Market Intelligence、策略改写、行情源改造、结果行级拆表。

---

## 5. 测试计划（实现阶段）

1. 股票筛选页正常打开  
2. 生成快照：Start 立即返回  
3. 后台完成 → 状态 completed + 快照可选  
4. 带信号标签搜索：秒级本地过滤；无快照时提示且不扫描  
5. 冒烟：Trade Plan / Paper Trading 入口无改动（diff 范围审查）  
6. 抽样对比 hit 字段一致性  

---

**本设计文档结束。下一步：按本节实现第一阶段并输出实现报告。**
