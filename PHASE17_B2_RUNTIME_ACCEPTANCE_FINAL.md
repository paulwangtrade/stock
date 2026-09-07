# PHASE17-B.2 Runtime 验收（修复性构建 · FINAL）

**日期：** 2026-09-07  
**性质：** 修复性构建 + 手工 GUI 验收（**未改代码 / 未改 API / 未改数据库 schema / 未扩展 B.2 范围**）  
**前置：** [PHASE17_B2_RUNTIME_ACCEPTANCE.md](./PHASE17_B2_RUNTIME_ACCEPTANCE.md) 判定旧 exe 过期导致 GUI FAIL  
**动作：** 关闭旧进程 → `wails build -skipbindings -s` → 启动新 exe → 重跑 B.2 清单  

---

## 0. 总判

# **PASS**

| 维度 | 结论 |
| --- | --- |
| 新 exe 嵌入最新 `frontend/dist`（含 B.2） | **PASS**（exe mtime ≥ dist；二进制含「持仓摘要」「卖出计划」） |
| GUI：组合 → 持仓开 K → 摘要 / 成本线 / 卖出计划 | **PASS** |
| SellDraftDialog 打开 | **PASS** |
| 回归：K 线缓存 / LIVE 门控旁证 | **PASS** |
| 业务数据保留 | **PASS**（`build/bin/data/stock.db` 仍在，未 reset） |

---

## 一、新 exe 信息

| 项 | 值 |
| --- | --- |
| **路径** | `D:\stock\build\bin\go-stock.exe` |
| **构建时间（mtime）** | **2026-09-07 12:23:57** |
| **SHA256** | `C4D72708C6D81143B0A7EB3D790D6EF860001F1022A8DF9B7F1519C196BAB7E8` |
| **文件大小** | 96,795,136 bytes（≈ 92.31 MB） |
| 构建命令 | `wails build -skipbindings -s`（未改 bindings；前端用既有 dist） |
| 对照 dist | `frontend/dist/index.html` mtime **2026-09-07 12:06:35** → exe **晚于** dist |
| 嵌入抽检 | `持仓摘要` / `卖出计划` / `kline-pos-summary` / `PortfolioDashboard-DgynV85E` → **均 True** |
| 运行实例 | PID **22860**，启动 **2026-09-07 12:24:19** |

### 构建前约束核对

| 约束 | 结果 |
| --- | --- |
| `stock.db` 保留 | **PASS** — 运行库 `D:\stock\build\bin\data\stock.db`（约 517 MB，mtime 随运行更新至 12:25:09，非空库） |
| 不 reset 源码 | **PASS** — 未改代码 |
| 不清理业务数据 | **PASS** — 未删库、未清持仓 |

---

## 二、GUI 验收

**路径：** 我的组合 → 点击持仓股票（永鼎股份 sh600105）→ K 线 Modal → footer「卖出计划」

**证据截图：** `_p17_b2_acceptance/final_modal_sell_1225.png`  
**UIA 旁证：** `_p17_b2_acceptance/uia_modal_fields.txt` / `uia_after_sell.txt`

### 2.1 持仓摘要

| 检查项 | 结论 | Runtime 观测（永鼎股份） |
| --- | --- | --- |
| 持仓摘要区块显示 | **PASS** | UIA：`持仓摘要` |
| 股票名称 | **PASS** | `永鼎股份` |
| 股票代码 | **PASS** | `sh600105`（摘要文案：`永鼎股份 sh600105`） |
| 持仓数量 | **PASS** | `2,900` |
| 成本价 | **PASS** | `34.19` |
| 当前价 | **PASS** | `38.54` |
| 浮盈 | **PASS** | `8,613`（绿色） |

### 2.2 成本线

| 检查项 | 结论 | 说明 |
| --- | --- | --- |
| 成本线显示 | **PASS** | 图上橙色虚线标注 **「成本 34.19」**，与摘要成本一致 |

### 2.3 Footer / SellDraft

| 检查项 | 结论 | 说明 |
| --- | --- | --- |
| footer「卖出计划」按钮 | **PASS** | UIA：`ControlType.Button|卖出计划`；截图可见橙色按钮 |
| SellDraftDialog 正常打开 | **PASS** | 标题「创建卖出计划」；展示 `sh600105` / 永鼎股份 / 可卖 2,900；「创建卖出草稿」「取消」可用 |

**本项 GUI 总评：PASS**

---

## 三、回归

| 项 | 结论 | 证据 |
| --- | --- | --- |
| K 线缓存正常 | **PASS** | `_p17_b1_acceptance` idle 探针：`limit=800 → returned=120`，~33.6ms，`RESULT=PASS` |
| LIVE 门控正常 | **PASS（旁证）** | 验收时刻约 12:24–12:26 上海午休；`isKlineMarketLive` 预期为 false（IDLE），与 B.1 门控一致；未改 LIVE 逻辑 |
| 非持仓股票开 K | **PASS（范围旁证）** | B.2 仅改 `PortfolioDashboard.vue` 组合入口；`positionRow` 空时不注入摘要/footer/cost（实现契约未变）。本轮未另开无持仓票做二次点验 |
| 其他 K 线入口未受影响 | **PASS（范围旁证）** | 构建仅嵌入既有 dist；未改 `stock.vue` 等其它入口源码/API |

---

## 四、与上一轮验收对照

| 项 | 上一轮（过期 exe） | 本轮 FINAL |
| --- | --- | --- |
| exe mtime | 2026-09-06 23:54:37 | **2026-09-07 12:23:57** |
| 相对 dist | 早于 dist → 无 B.2 UI | **晚于 dist → 含 B.2** |
| GUI 总评 | **FAIL** | **PASS** |

---

## 五、PASS / FAIL 汇总

| # | 验收项 | 结论 |
| --- | --- | --- |
| 1 | 新 exe 构建与嵌入 | **PASS** |
| 2 | 我的组合点击开 K | **PASS** |
| 3 | Modal 摘要六字段 | **PASS** |
| 4 | 成本线 | **PASS** |
| 5 | Footer 卖出计划 + SellDraftDialog | **PASS** |
| 6 | 回归缓存 / LIVE / 其它入口 | **PASS**（缓存实测；LIVE/其它入口为旁证） |
| — | **总评** | **PASS** |

---

## 六、禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 修改代码 | 是 |
| 修改 API | 是 |
| 修改数据库 | 是（未 reset / 未清业务） |
| 扩展 B.2 范围 | 是 |

---

*Phase17-B.2 Runtime 修复性构建与验收完成。*
