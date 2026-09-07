# PHASE17 Release Build Report

**日期：** 2026-09-08（本地）  
**分支：** `release/v0.1.0-beta`  
**性质：** 菜单可见性审计后的最小修复 + 发布构建  
**禁止范围遵守：** 未改 Broker / Settlement / 交易逻辑 / DB

---

## 1. 修复列表

| # | 问题 | 修复 | 文件 |
| --- | --- | --- | --- |
| 1 | 「持仓做T」路由存在但菜单 `show: false` | 「我的组合」改为子菜单：组合总览 + **持仓做T** | `frontend/src/navigation/productMenu.js` |
| 2 | 路由高亮未含 holdingT | `activeKey` 增加 `holdingT` | `frontend/src/App.vue` |
| 3 | 组合页说明未点明 Health / 做T / Origin | 页脚说明补一句可达提示 | `frontend/src/components/PortfolioDashboard.vue` |

其余 Phase17 能力（Health / T-Suitability / Origin / Data Truth / TradePlan Origin / 机会信号）审计为 **已可达**，无额外改动。

审计文档：`PHASE17_MENU_VISIBILITY_AUDIT.md`

---

## 2. Capability Matrix（修复后）

| 功能 | 后端 | 前端 | 菜单/入口 | 状态 |
| --- | --- | --- | --- | --- |
| Holding Health | OK | OK | 我的组合 → 健康列 → Drawer | **PASS** |
| 持仓做T 适宜性 | OK | OK | 我的组合 → 做 T 列 → Drawer | **PASS** |
| 持仓做T 观察台 | OK | OK | 我的组合 → 持仓做T | **PASS**（已修复） |
| Position Origin / Provenance | OK | OK | 来源列 / 为什么买入 → Drawer | **PASS** |
| Portfolio Data Truth | OK | OK | 估值价 / 行情价+时间 / 今日浮盈 | **PASS** |
| TradePlan Origin | OK | OK | 交易计划页内 Origin Panel | **PASS** |
| Opportunity / Signal | OK | OK | 机会 → 机会列表 | **PASS** |
| 风险（一级菜单） | N/A | 页内标签/摘要 | 无一级（不扩功能） | **N/A** |
| 选股（同名菜单） | N/A | 机会列表 | 机会 → 机会列表 | **PASS**（别名） |

### 用户路径（明日验收）

```
打开 go-stock.exe
→ 底部菜单「我的组合」→「组合总览」→ 健康 / 做 T / 为什么买入 / 行情列
→ 「我的组合」→「持仓做T」→ HoldingT 观察台
→「机会」→「机会列表」
→「交易计划」→ Origin 面板
```

---

## 3. 测试结果

| 项 | 命令 | 结果 |
| --- | --- | --- |
| Frontend | `npm run build`（frontend） | **PASS**（~2m 28s） |
| Phase17 相关 Go | `go test ./backend/papertrading/ ./backend/portfolio/... ./backend/tradeplanorigin/` | **PASS** |
| 全量 `./backend/...` | `go test ./backend/...` | **FAIL**（既有债，与本次无关） |

**既有失败（未修）：**  
`backend/strategy` · `TestBuildDraftTradePlan_UsesPositionSizerForAmount`  
断言源码应含 `resolvePlanAmountViaSizerForDraft`，属 PositionSizer 接线债，**不在本阶段修复范围**。

`go test ./...` 另会因 `tmp/` 多 main、以及根包 embed 与 dist hash 竞态失败——发布验证不以该命令为准。

---

## 4. Build 时间与产物

| 步骤 | 耗时 |
| --- | --- |
| `npm run build` | ~2m 28s |
| `wails build -skipbindings -s` | **38.15s**（Wails 报告） / ~38.6s 墙钟 |
| **合计（本轮构建）** | ~3 分钟量级（前端已先编） |

| 产物 | 路径 | 大小 | 写入时间 |
| --- | --- | --- | --- |
| 可执行文件 | `D:\stock\build\bin\go-stock.exe` | 96 884 224 bytes（~92.4 MB） | 2026-09-08 00:04:41 |

构建参数：`windows/amd64` · production · Skip Bindings · Skip Frontend（已用最新 Vite dist）

---

## 5. 版本状态

| 项 | 值 |
| --- | --- |
| Git 分支 | `release/v0.1.0-beta` |
| 检查点 commit（构建前 HEAD） | `6fee00c`（checkpoint: Phase17 holding intelligence…） |
| 本轮未提交改动 | 菜单/入口修复 + 审计/本报告（以及工作区其它既有未提交文件，**未 commit**） |
| 标签 | `phase17-checkpoint`（此前已打） |
| 版本定位 | **发布前整理构建**：已完成能力入口齐备；非新功能版本 |

---

## 6. 使用说明

1. **先关闭**正在运行的 `go-stock.exe`。  
2. 启动：`D:\stock\build\bin\go-stock.exe`  
3. 按上文 Capability Matrix 路径点一遍 Health / 做T / Origin / 机会 / 交易计划。

---

*Phase17 Menu Capability Visibility Audit & Release Preparation — 完成。*
