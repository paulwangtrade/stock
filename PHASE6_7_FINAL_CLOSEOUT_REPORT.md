# Phase6.7 Final Closeout Report

> 日期：2026-07-27  
> 性质：**阶段冻结与文档归档** — 仅生成本报告，不修改代码 / 数据库 / 配置，不进入 Phase7 实现  
> 依据：
> 1. `PHASE6_7_G_RUNTIME_VERIFICATION_REPORT.md`
> 2. `PHASE6_7_H_MARKET_DATA_LAYER_AUDIT.md`
> 3. `PHASE6_7_G_IMPLEMENTATION_PLAN.md`
> 4. `PHASE6_7_F_STOCK_SCREENER_PERFORMANCE_AUDIT.md`
> 5. （实现侧）`PHASE6_7_G_STOCK_SCREEN_PERFORMANCE_IMPLEMENTATION_REPORT.md`

---

# 1. Phase6.7 目标回顾

Phase6.7 围绕「股票筛选」交互与数据层可演进性展开，原始目标可归纳为：

| 目标 | 说明 |
|---|---|
| 股票筛选性能优化 | 解决「生成快照 / 带信号搜索」阻塞 UI 数十分钟的问题（Phase6.7-F 审计基线：全市场约 5400 只、墙钟 20～40 分钟） |
| 异步快照任务机制 | 将同步 `RunSignalScanSnapshot` 拆为「任务创建 → 后台执行 → 状态查询」 |
| Snapshot 复用 | 搜索在已有快照时本地过滤，不再重新拉 K + 重算信号 |
| 避免重复全市场扫描 | 无快照时提示先生成，禁止搜索路径自动全市场重扫 |
| 行情数据层审计 | 只读梳理入口 / 消费链 / 重复计算，为未来 Market Intelligence Layer 做准备（Phase6.7-H） |

**明确不在本阶段范围内：** 改策略公式、改行情源、压缩单次全市场 K 线扇出墙钟、改 Trade Plan / Risk / Paper Trading / Execution 语义、开始 Phase7 实现。

---

# 2. Phase6.7-G 完成情况

设计见 `PHASE6_7_G_IMPLEMENTATION_PLAN.md`；运行时验证见 `PHASE6_7_G_RUNTIME_VERIFICATION_REPORT.md`。

## 已完成

| 项 | 状态 | 说明 |
|---|---|---|
| 异步任务启动机制 | ✅ | 进程内 Task Registry：`pending → running → completed/failed`；全进程单飞 |
| `StartSignalScanSnapshot` API | ✅ | 立即返回 `task_id`，后台调用既有 `RunFullMarketSnapshot` |
| `GetSignalScanTask` / `GetLatestSignalScanTask` | ✅ | 状态查询；完成后关联 `snapshot_id` |
| 搜索复用 Signal Snapshot | ✅ | 有快照 → `applySnapshotPayload` 本地过滤 |
| 无自动全市场重扫 | ✅ | 无快照 → warning「需要先生成快照…」，禁止自动扫 |
| 最新 build 验证 | ✅ | `D:\stock\build\bin\go-stock.exe`，2026-07-26 22:27:05；含异步 API 符号 |
| Wails 绑定验证 | ✅ | `App.js` / `App.d.ts` 已导出 Start / GetTask / GetLatest |
| 交易链隔离验证 | ✅ | G 范围文件未触达 TradePlan / Risk / Paper / Execution 语义；plan#5 只读抽查正常 |

## 验证结果

### PASS

| 检查项 | 证据摘要 |
|---|---|
| 异步调用立即返回 | `return_ms=0`，`pending → running`（≪ 2s） |
| Snapshot 查询复用 | 最新快照 id=4 可读；`query_ms≈5` |
| 搜索本地过滤 | tag=`买` → 18 条，`elapsed_ms=0`；`kline_cache` 不变；未触发 `IsRunning()` |
| 未影响 TradePlan / Risk / Paper / Execution | G 改动范围仅限任务注册表 + 筛选 UI + 绑定；未改公式 / 行情源 / 下单路径 |

### Pending

| 检查项 | 说明 |
|---|---|
| 完整后台扫描 `completed` 快照落库验证 | 本验证窗口未观察到新 `signal_scan_snapshots` 行；最新仍为 id=4（2026-07-24，`duration_ms≈40min`）。墙钟仍为 20～40 分钟量级（与 Phase6.7-F 一致），属行情扇出耗时，非异步架构失败 |
| 桌面 UI 人工点击验证 | Native WebView `MainWindowHandle=0`，无法可靠自动化点选「股票筛选」；以源码路径 + 绑定进 exe 作为替代证据 |

**说明：** Pending 项**不是代码失败**，而是验证窗口长度、东财/腾讯行情限流与 EOF、以及桌面 WebView 环境限制导致。Phase6.7-G 优化的是**交互阻塞**与**重复全市场计算**，不是把单次全市场 K 线扇出压到秒级。

### 交互视角对比（摘自 G 验证报告）

| 场景 | 优化前（F） | 优化后（G） |
|---|---|---|
| 点击生成快照（调用方等待） | 阻塞 20～40 分钟 | 立即返回；后台仍跑全市场 |
| 带信号搜索（已有快照） | 可能再次全市场拉 K + 计算 | 本地过滤秒级 |
| 带信号搜索（无快照） | 自动全市场扫描 | 提示先生成，不自动扫 |
| 单次全市场墙钟 | ≈20～40 分钟 | 预期同量级（未改行情路径） |

---

# 3. Phase6.7-H 数据层审计总结

依据：`PHASE6_7_H_MARKET_DATA_LAYER_AUDIT.md`（只读，未改任何实现）。

## 当前问题

### K 线

- 多入口最终汇聚到 `EastMoneyKLineApi.GetKLineData` / `GetKLineDataBefore`（全市场快照、前端筛选现算、图表/回测、Agent 工具等）
- 不同 `limit`（如 120 vs 260）导致 `kline_cache` 无法充分复用（`BarCount < limit` 即未命中）
- `kline_cache`（热路径 TTL 缓存）与 `StockKLineRepo`（规范化持久层）**并存**，后者未接入热路径

### 实时行情

- 多模块直接调用 `GetStockCodeRealTimeData`（自选监控、papertrading、开盘买入、AI 推荐等）
- 缺少统一 Quote Service
- 缺少批量行情共享缓存：`FollowRealtimePriceCache`（约 5s）仅覆盖自选批量路径；papertrading `OpenQuote` / `MarkPrice` 逐票且各请求一次

### 信号计算

- 后端 goja 执行 `signal_scan_bundle.js`（全市场快照）
- 前端 JS（`icePointSignals.js` 等）在自选/筛选现算/图表再次计算
- **双运行时、同源算法**；Phase6.7-G 后搜索已收敛到快照，但主动扫描与自选信号仍可浏览器重算

### 指标

- 指数 MA20：后端 `buildIndexCloseMap` 与前端 `ensureIndexMa20ByDay` / `buildIndexMa20ByDay` **重复计算**
- SMA 等在 Go（`GetKLineWithMA`）与 JS 信号引擎内分别实现

### 审计结论要点

上述问题属于**架构演进需求**，不是当前交易功能 Bug。P0 建议收敛统一 K 线服务与统一批量实时行情服务；交易域保持隔离、仅只读取价。

---

# 4. 架构影响评估

Phase6.7-F/G/H 暴露的问题：

- **不是**当前功能正确性 Bug（筛选可用、快照可落库、搜索可复用、交易计划可冻结执行）
- **属于**系统规模扩大后（全市场 ~5400、多消费方、前后端双算）的**架构演进需求**

## 交易链隔离（保持完好）

```text
Market Data
    ↓
Signal Snapshot
    ↓
Strategy
    ↓
Candidate
    ↓
Trade Plan
    ↓
Risk
    ↓
Paper Trading
    ↓
Execution
```

| 断言 | 结论 |
|---|---|
| Phase6.7-G 未改策略公式 / 信号 bundle / Candidate / Approve-Freeze / Risk / Paper Broker / Execution | ✅ |
| Phase6.7-H 为只读审计，零实现改动 | ✅ |
| 运行库 plan#5（2026-07-27 / ready / 已 approve+freeze）验证期间未被改写 | ✅ |
| Frozen Spec 仍为下单唯一价量来源；实时行情对 Paper 多为观测/盯市 | ✅ |

**结论：Phase6.7 未破坏交易闭环。** 行情层问题与交易语义解耦，可在后续 Phase 独立演进。

---

# 5. 后续规划建议

## Future Phase7-A: Market Data Intelligence Layer（行情智能层）

> **仅规划，本阶段不实现。**

目标：将分散的行情抓取、缓存与指标/信号计算收敛为平台级只读服务，供 PC / 未来 Mobile / Web / AI 助手统一消费。

### 1. Kline Service

| 能力 | 说明 |
|---|---|
| 统一 K 线入口 | 单一 `GetBars(code, period, adjust, limit, end)` |
| 缓存 | 统一 TTL / 增量合并；收敛 `kline_cache` 与 `StockKLineRepo` 职责 |
| 增量 | 延续现有 latest 增量合并能力 |
| 复权 | 统一复权参数语义 |
| 超集切片 | 大根数缓存切片满足小 `limit`，消除重复抓取 |

### 2. Quote Service

| 能力 | 说明 |
|---|---|
| 批量实时行情 | 所有消费方统一走批量接口 |
| 统一缓存 | 进程内共享 TTL，可配置 |
| 模块共享 | 自选、Paper、开盘买入、AI、监控 cron 共用；`OpenQuote`/`MarkPrice` 复用同批结果 |

### 3. Indicator Service

| 能力 | 说明 |
|---|---|
| MA / RSI / MACD / ATR 等 | 单一计算入口，消除前后端双算（含指数 MA20） |

### 4. Signal Service

| 能力 | 说明 |
|---|---|
| 单一信号计算入口 | 保持算法源单一，对外只暴露批量信号服务 |
| 多端统一消费 | PC / Mobile / Web 读同一结果，逐步下线浏览器现算路径 |

**隔离边界（规划原则）：** 行情智能层只提供只读数据；Trade Plan / Risk / Position / Paper Trading **不纳入**该层，通过只读接口取价，Frozen Spec 仍为下单唯一价量来源。

### 建议的补证（非实现）

1. 交易日盘后人工：最新 exe →「股票筛选」→「生成快照」→ 确认状态机与完成后历史出现新 id  
2. 同一进程内等到 `GetSignalScanTask=completed` 后，补跑落库断言  
3. 补证期间勿对交易链做 Generate/Approve，避免污染已 ready 计划

---

# 6. 与未来 Quant OS 产品化关联

Phase6.7 发现的问题，与未来产品化方向存在**直接关系**：

| 方向 | 关联 |
|---|---|
| 多用户 | 无统一行情层时，每用户/每会话重复拉 K、算信号，成本不可控 |
| Mobile 端 | 双运行时（goja + 浏览器 JS）无法直接复用；需要单一 Signal / Indicator 服务 |
| SaaS 化 | 批量 Quote / 超集 K 线缓存是租户级成本中心；缺少共享层则无法规模化 |
| AI 助手 | Agent 工具已分散调用 K 线 API；统一层可提供可观测、可限流的数据产品 |
| 策略生态 | 策略/回测/筛选共享同一 Market Data 契约，避免「每策略一套抓取逻辑」 |

**定位说明：** 当前架构正在从**个人交易工具**向**平台化系统（Quant OS）**演进。Phase6.7 完成了筛选侧的交互收敛与数据层问题地图；Phase7-A（规划）将是平台化的关键基础设施，而非局部修丁。

---

# 7. Phase6.7 最终状态

```text
Phase6.7 Status:
PASS WITH FOLLOW-UP VERIFICATION

Completed:
✅ 异步任务架构
✅ Snapshot 复用机制
✅ 股票筛选交互优化
✅ 数据层审计
✅ 交易链隔离

Follow-up:
1. 完整后台扫描 completed 验证
2. UI 人工验证

Future:
Phase7-A Market Data Intelligence Layer（规划）
```

### 归档索引

| 文档 | 角色 |
|---|---|
| `PHASE6_7_F_STOCK_SCREENER_PERFORMANCE_AUDIT.md` | 性能问题基线（只读） |
| `PHASE6_7_G_IMPLEMENTATION_PLAN.md` | G 设计 |
| `PHASE6_7_G_STOCK_SCREEN_PERFORMANCE_IMPLEMENTATION_REPORT.md` | G 实现 |
| `PHASE6_7_G_RUNTIME_VERIFICATION_REPORT.md` | G 运行时验证 |
| `PHASE6_7_H_MARKET_DATA_LAYER_AUDIT.md` | H 行情层审计（只读） |
| `PHASE6_7_FINAL_CLOSEOUT_REPORT.md` | **本文件：阶段冻结** |

---

> Phase6.7 文档归档到此结束。未修改任何代码、数据库或配置；未启动 Phase7 实现；未修复行情层问题（留给 Future Phase7-A 规划）。
