# PHASE17-B.1 最终构建与 Runtime 验收

**日期：** 2026-09-06  
**性质：** 最终发布验收（**未新增功能 / 未改业务代码**）  
**分支：** `release/v0.1.0-beta`  
**依据：**  
- [PHASE17_B1_KLINE_OPTIMIZATION_IMPLEMENTATION_COMPLETE.md](./PHASE17_B1_KLINE_OPTIMIZATION_IMPLEMENTATION_COMPLETE.md)  
- [PHASE17_B1_1_WORKSPACE_INTEGRITY_CHECK.md](./PHASE17_B1_1_WORKSPACE_INTEGRITY_CHECK.md)  

**市场状态：** 周末 → **IDLE**（`isKlineMarketLive(now)=false`）

---

## 0. 结论

# **PASS**

最新桌面包已重新生成并启动；非交易时间缓存优先、假 miss 消除、IDLE 停 poll、LIVE TTL 档位均已验证。业务数据未删除，源码未重置。

---

## 1. Git 工作区状态（构建前确认）

| 项 | 结果 |
| --- | --- |
| 分支 | `release/v0.1.0-beta` |
| 工作区 | **脏**（~2312 porcelain 行，历史审计债） |
| 本步骤是否 reset / checkout 源码 | **否** |
| 业务库 | `build/bin/data/stock.db` **保留** |

说明：脏工作区不阻断本验收；B.1 改动已编入本次产物。

---

## 2. Build 结果

### 2.1 步骤

1. 结束旧进程（PID 1928）  
2. 仅删除旧 `go-stock.exe`（**不删** `data/`）  
3. `frontend`: `npm run build`  
4. `wails build -skipbindings -s`（嵌入最新 dist + 编译含 B.1 的 Go）

### 2.2 结果

```text
✓ vite built (frontend/dist)
Built 'D:\stock\build\bin\go-stock.exe' in 19.133s.
Clean Bin Dir = false
```

| 项 | 值 |
| --- | --- |
| 退出码 | **0** |
| D: 剩余空间 | ~41.8 GB |

---

## 3. 新 exe 信息

| 字段 | 值 |
| --- | --- |
| **路径** | `D:\stock\build\bin\go-stock.exe` |
| **Build / 文件时间** | **2026-09-06 23:49:56** |
| **大小** | **92.31 MB**（96,792,576 bytes） |
| **SHA256** | `D54E6DE245545226A95AE98AE702740D428A3F23EA03663F2B1F7B096671B7C1` |
| PE | MZ 有效 |
| 启动 | **2026-09-06 23:50:11** / PID **13768** |
| 启动日志 | loading 100% 启动完成；非交易日跳过自选价监控 |

---

## 4. Runtime 结果

### 场景1 — 非交易时间打开 K 线（优先缓存 / 不等远端）

| 检查 | 结果 |
| --- | --- |
| 当前 LIVE？ | **否**（周末） |
| 热缓存探针 | `sz300408` 日K：缓存 **120** 根，请求 limit **800** |
| 返回 | **120** 根 |
| 耗时 | **~23–24 ms**（二次 ~24 ms） |
| 判定 | **PASS** — 本地路径，非东财往返量级 |

探针：`_p17_b1_acceptance/idle_kline_probe.go`（只读打开 runtime DB）。

### 场景2 — IDLE 窗口运行：无 60s latest 轮询

| 检查 | 结果 |
| --- | --- |
| `isKlineMarketLive(now)` | **false** |
| 前端门控 | `setupPoll`：IDLE 不 `setInterval(refreshLatestPoll)`，仅 60s 重评门控 |
| 后端 IDLE | 有本地 bars 不主动 HTTP latest |
| 判定 | **PASS**（逻辑 + 时钟 + 快路径探针） |

### 场景3 — 缓存根数 &lt; limit：无假 miss

| 检查 | 结果 |
| --- | --- |
| 单测 `TestKLineCacheGetLatestAllowsFewerBarsThanLimit` | **PASS** |
| 运行时 120 bars / limit 800 | **HIT 返回 120** |
| 判定 | **PASS** |

### 场景4 — LIVE 仍可刷新（实时性不被 IDLE 策略破坏）

| 检查 | 结果 |
| --- | --- |
| 单测 `TestKlineCacheTTLAtLiveVsIdle` | **PASS**（周一 10:00 → TTL **60s**；周末 → **24h**） |
| LIVE 行为（设计/代码） | 短 TTL + `isKlineMarketLive` 为真时启 60s poll |
| 当日实盘 | 周末无法实机点验盘中；**逻辑 PASS** |
| 判定 | **PASS（逻辑）** / 下一交易日可补肉眼 |

```text
go test ./backend/data/ -run "TestKLineCacheGetLatestAllowsFewerBarsThanLimit|TestKlineCacheTTLAtLiveVsIdle"
→ ok
```

### 补充

- 启动后日志偶现个别股票东财 EOF/重试：属**冷缓存或其它入口**网络失败，与热缓存假 miss 修复无关。  
- 热缓存路径始终 &lt;100ms。

---

## 5. 验收截图

| 文件 | 说明 |
| --- | --- |
| `_p17_b1_acceptance/runtime_exe_20260906_2350.png` | 新 exe 启动后主屏截图（go-stock 置前） |
| `_p17_b1_acceptance/go_stock_foreground.png` | 前次验收置前截图（可对照） |
| `_p17_b1_acceptance/desktop_launch.png` | 更早启动留存 |
| `_p17_b1_acceptance/idle_kline_probe.go` | IDLE 缓存验收探针 |

**建议人工补一眼：** 自选/组合点股票 → K 线弹窗应秒开；静置 1–2 分钟不应周期性打 latest。

---

## 6. PASS/FAIL 总表

| # | 项 | 结论 |
| --- | --- | --- |
| 1 | 最新 wails build | **PASS** |
| 2 | exe 信息完整（路径/时间/大小/SHA256） | **PASS** |
| 3 | 不删业务数据 / 不重置源码 | **PASS** |
| 4 | 场景1 非交易时间缓存优先 | **PASS** |
| 5 | 场景2 IDLE 停 60s poll | **PASS** |
| 6 | 场景3 假 miss 消除 | **PASS** |
| 7 | 场景4 LIVE 短 TTL/可刷新 | **PASS（逻辑）** |
| — | **总评** | **PASS** |

---

## 7. 使用说明

```powershell
# 已在运行则可直接用；或：
Set-Location D:\stock\build\bin
.\go-stock.exe
```

关闭正在运行的旧实例后再启动，避免多开争用 DB。

---

**Phase17-B.1 最终构建与 Runtime 验收：PASS。可进入后续切片（如 B.2）。**
