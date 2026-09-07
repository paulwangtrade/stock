# PHASE17-B.1 K 线加载性能优化 — 实现前方案冻结

**日期：** 2026-09-06  
**性质：** **设计冻结**（本阶段不改代码）  
**依据：** [KLINE_LOAD_PERFORMANCE_AUDIT.md](./KLINE_LOAD_PERFORMANCE_AUDIT.md)  
**目标：** 只调整**加载 / 缓存 / 轮询策略**，不重构 K 线架构。  

---

## 0. 冻结结论（先读）

| 项 | 决定 |
| --- | --- |
| 第三层缓存 | **禁止** |
| DB schema / 行情接口 / K 线模型 / 指标 / 执行链 | **禁止改** |
| Phase17-B.1 编码范围 | **P1 数据加载策略 only** |
| 图表 keep-alive / 实例复用 | **P2 暂缓**（本阶段不实施） |
| **可否进入编码** | **可以**（确认本文件后开实现切片） |

**必须做（B.1）：** 市场状态门控轮询 · 后端 latest 命中条件修正（barCount）· 分时段 TTL  
**可选（同切片若成本低）：** 前端休市延长内存 TTL；指数日K 走既有 `getOrFetch`  
**暂缓：** Modal keep-alive、降低默认 limit、新建缓存介质  

---

## 1. 当前加载链路与耗时点

```text
点击股票 / StockLink
    ↓
StockKlineModal（show → 挂载；关 → v-if 销毁图表）
    ↓
StockLightweightKlineChart
    ├─ ensureChart()                    【耗时·中】图表初始化
    ├─ loadData(klt)
    │    ├─ 前端 klineCache.getOrFetch   【快】命中则跳过 IPC
    │    │         miss ↓
    │    ├─ Wails GetStockEastMoneyKLine 【耗时·中】IPC + JSON
    │    │         ↓
    │    ├─ SQLite kline_cache           【快】命中则跳过 HTTP
    │    │         miss / 假 miss ↓
    │    └─ 东财 HTTP latest             【耗时·高】主慢点
    ├─ setData / 指标 / 可选信号         【耗时·中】渲染
    └─ setupPoll(60s)                    【休市仍请求】持续慢点
```

| 标记 | 位置 | 说明 |
| --- | --- | --- |
| ★1 | HTTP latest | TTL 60s + `BarCount < limit` 假 miss |
| ★2 | 60s poll | 无市场状态门控 |
| ★3 | 弹窗销毁重建 | 每次打开付图表 init（B.1 不修） |
| ★4 | 前端 5min miss | 关 App / 超时后必打后端 |

---

## 2. 市场状态判断方案

复用既有能力，**不新造日历库**：

| 层 | 建议复用 |
| --- | --- |
| 前端 | `tradingSession.js`（连续竞价 09:30–11:30、13:00–15:00）+ 周末判断（可对齐 `planContext.isTradingDay` / 上海时区） |
| 后端 | `tradingcalendar.IsTradingDay`（含节假日若已配置）+ 本地钟点；`IsWeekdayLocal` 仅作 fallback |

### 2.1 状态定义（A 股 · 上海）

| 状态 | 条件（建议） |
| --- | --- |
| **SESSION_OPEN** | 交易日且时刻 ∈ [09:30,11:30) ∪ [13:00,15:00) |
| **SESSION_BREAK** | 交易日且 ∈ [11:30,13:00)（午休） |
| **SESSION_PRE** | 交易日且 &lt; 09:30（含集合竞价前；B.1 可并入「非连续竞价」） |
| **SESSION_POST** | 交易日且 ≥ 15:00 |
| **SESSION_CLOSED_DAY** | 周末 / 非交易日（节假日） |

B.1 实现可先折叠为两档：**LIVE**（SESSION_OPEN）vs **IDLE**（其余全部），降低复杂度；午休与盘后同 IDLE。

### 2.2 分状态行为

| 状态 | 打开时是否允许打远端 latest | 弹窗轮询 | 缓存 TTL 档 |
| --- | --- | --- | --- |
| SESSION_OPEN | 允许（miss 时） | **开**，默认 60s | 短（现网量级） |
| SESSION_BREAK | 优先本地；miss 可偶发远端 | **关**或 ≥15min | 中长 |
| SESSION_POST | 优先本地；首次可增量/全量一次 | **关** | 长 |
| SESSION_CLOSED_DAY | 优先本地；仅冷缓存 miss 才远端 | **关** | 很长 |

**原则：** IDLE 下「有可用本地 bars → 禁止为刷新而重复打 latest」；仅真正无行 / 损坏 / 用户强制刷新时才 HTTP。

---

## 3. 缓存命中优化方案（barCount）

### 3.1 现状问题

`klineCacheGet`：

```text
if row.BarCount < limit → return nil   // 假 miss
if len(bars) < limit → return nil
```

例：请求 800，库中 500（股票上市不足 800 日）→ **永假 miss** → 每次 HTTP。

### 3.2 目标命中条件（冻结）

对 **latest** 且 payload 非空：

```text
命中当且仅当：
  1) 未过 TTL（见 §4）
  2) len(bars) > 0
  3) 返回 trim(bars, limit)：
       - 若缓存根数 ≥ limit → 取最近 limit 根
       - 若缓存根数 < limit → 返回全部缓存根（视为「上游已给满历史」）
```

**示例：** 请求 250、缓存 500 → 返回最近 250，**直接命中**。  
请求 800、缓存 500 → 返回 500，**命中**，不因不足 800 打全量。

### 3.3 完整性保障

| 风险 | 对策 |
| --- | --- |
| 缓存过短导致「其实还有更早历史」 | **左拖分页**仍走 `GetStockEastMoneyKLinePage`（end≠latest），不依赖本次 latest 命中；`hasMoreOlder` 逻辑保持「能继续向左请求」 |
| 盘中最新一根未进缓存 | LIVE 短 TTL + 现有 **stale+增量合并**（日/周/月等已支持）保留 |
| 空缓存 | 仍不命中；空数组不写入（前后端既有约定） |
| 非 latest（历史 endKey） | 可保持「bars 足够覆盖请求窗口」的更严规则，或同样允许「不足则返回已有」；B.1 **优先只改 latest 路径**，降低回归面 |

**冻结：B.1 只放宽 `end_key=latest` 的 barCount 条件；历史分页语义不变。**

---

## 4. TTL 调整方案

### 4.1 后端 `klineCacheTTL`（latest）

| klt 类 | LIVE（连续竞价） | IDLE（午休/盘后/非交易日） |
| --- | --- | --- |
| 日/周/月（101–103） | **60s**（保持现网实时感） | **6h**（盘后）；**至下一交易日 09:15** 或 **24h**（周末/节假日，取实现更简单者） |
| 季/年（104/106） | 5min | 24h |
| 分钟（1/5/…/60） | **30s**（保持） | **2h**（盘后分钟线不再变）；周末 **24h** |

非 latest（带历史 end）：维持现有 24h / 30min，**本阶段不改**。

### 4.2 前端内存 TTL（`klineCache.js`）

| 状态 | TTL |
| --- | --- |
| LIVE | **5min**（保持） |
| IDLE | **30min**（同进程反复打开同股同周期秒开） |

仍：**禁止**持久化第三层；空数组不缓存。

### 4.3 兼顾实时性与速度

- LIVE：短 TTL + 允许 poll → 盘中不牺牲最新一根。  
- IDLE：长 TTL + 停 poll → 休市打开吃满 SQLite/内存。  
- 强制刷新（若 UI 已有/后加）：绕过 TTL 一次，不作为默认路径。

---

## 5. 轮询策略

### 5.1 现状

`realtimeIntervalMs` 默认 **60000**；`setupPoll` **无**市场状态判断。

### 5.2 冻结行为

| 状态 | 轮询 |
| --- | --- |
| LIVE | **启用**，间隔保持 60s（或沿用 props） |
| IDLE | **`clearPoll` / 不启动**（等价 interval=0） |

实现落点（设计）：`StockLightweightKlineChart.setupPoll` 内根据市场状态决定；状态变化（跨 15:00）可用已有 `watch` / 轻量定时重评（例如每分钟检查一次是否应启停 poll），**不必**新建行情订阅。

### 5.3 如何避免影响「真·实时」

- 连续竞价内行为与现网一致（60s 拉 latest K）。  
- Quote 行情条 / 组合 overlay **不在本切片范围**，互不替代。  
- K 线 poll 停的是「历史 K 线 latest 重拉」，不是关掉全站行情。  
- 用户盘中打开弹窗仍立即 `loadData` 一次（可 miss 打远端）。

---

## 6. 图表生命周期（评估）

| 方案 | 收益 | 成本 | B.1 |
| --- | --- | --- | --- |
| keep-alive 最近 N 只 | 再开免 createChart | Modal/多页挂载复杂、内存 | **暂缓 P2** |
| 全局单例图表 | 最大复用 | 架构级，越界 | **不做** |
| 延迟 dispose | 短时再开更快 | 易泄漏 | **暂缓** |

**优先级冻结：**

- **P1：数据加载优化**（状态 · TTL · barCount · poll）← **本阶段**  
- **P2：图表实例复用** ← **不进入 B.1 编码**

---

## 7. 最终实现范围

### 必须做（Phase17-B.1）

1. **后端** `klineCacheGet`（latest）：放宽 barCount / len 条件，不足 limit 仍返回已有 bars。  
2. **后端** `klineCacheTTL(latest)`：按 LIVE/IDLE 分档（§4.1）。  
3. **前端** `setupPoll`：IDLE 停轮询；LIVE 保持。  
4. **前端**（建议同切片）：IDLE 延长 `getOrFetch` TTL（§4.2）。  
5. 单测：假 miss 回归（500 bars + limit 800 → hit）；TTL 分档；poll 门控。

### 可选

- `ensureIndexMa20ByDay` 走 `getOrFetch`（减重复指数请求）。  
- 市场状态工具抽到前后端共用的小函数（仍用现有日历/时段，不新建服务）。

### 暂缓

- 第三层缓存、改 schema、改东财 URL/字段、改指标、改默认 limit、Modal keep-alive、执行链。

### 允许改动的文件面（编码时）

| 允许 | 禁止 |
| --- | --- |
| `backend/data/kline_cache.go`（及现有 test） | 新表 / 新 API 路径 |
| `EastMoney` 取数路径中 **仅** cache get/TTL 调用侧 | 改 parse / 字段模型 |
| `StockLightweightKlineChart.vue`（poll / TTL 入参） | 大拆组件 / 换图表库 |
| `klineCache.js`（TTL 策略） | 持久化 IndexedDB 等第三层 |
| `tradingSession.js` 等复用 | 交易 / Settlement / Portfolio 写链 |

---

## 8. 验收标准（实现后）

| # | 场景 | 期望 |
| --- | --- | --- |
| 1 | 周末夜盘，同股日K 第二次打开（进程内） | 前端 hit 或后端 hit，**无**东财全量（或仅无缓存时一次） |
| 2 | 缓存 500 根、请求 800 | **命中**返回 500，不假 miss |
| 3 | 缓存 500、请求 250 | 返回最近 250 |
| 4 | 交易日 10:00 打开弹窗 | poll 仍约 60s |
| 5 | 交易日 16:00 / 周末打开弹窗 | **无** 60s poll |
| 6 | 左拖更早历史 | 行为与现网一致（Page API） |

---

## 9. 是否可以进入编码

**可以。**  

下一切片名称建议：`Phase17-B.1 Implementation`（严格按 §7 必须项）。  
编码前无需再开审计；若实现中发现节假日日历与前端周末不一致，以 `tradingcalendar` 为准并在实现报告中注明。

---

## 10. 停止边界

- 本文 **只冻结设计，不改代码**。  
- 不扩大到 Quote 实时条、组合 mark、K 线 UI 视觉重构。
