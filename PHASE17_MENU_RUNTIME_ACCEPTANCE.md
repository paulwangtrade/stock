# PHASE17 菜单恢复 Runtime 验收

**日期：** 2026-09-07  
**依据：** [PHASE17_MENU_RESTORE_COMPLETE.md](./PHASE17_MENU_RESTORE_COMPLETE.md)  
**性质：** 仅构建 + GUI 验收（**未改代码 / 未扩展菜单 / 未调布局**）

---

## 0. 总判

# **PASS**

| 维度 | 结论 |
| --- | --- |
| 新 exe 嵌入最新菜单 | **PASS** |
| 底栏七项显示 | **PASS** |
| 逐项点击路由/页面 | **PASS** |
| 回归：组合 / K 线 / B.2 摘要 / SellDraft | **PASS** |
| stock.db / staged 源码保留 | **PASS** |

---

## 一、新 exe

| 项 | 值 |
| --- | --- |
| **路径** | `D:\stock\build\bin\go-stock.exe` |
| **构建时间（mtime）** | **2026-09-07 18:59:39** |
| **SHA256** | `387BF05FB46D64ADB4FAAFFEC8B19D69A99BDBBC13A3394535D128C698062FDF` |
| **文件大小** | 96,793,600 bytes（≈ 92.31 MB） |
| 构建命令 | `wails build -skipbindings -s` |
| 运行实例 | PID **17008**，启动 **2026-09-07 18:59:56** |

### 构建前约束

| 约束 | 结果 |
| --- | --- |
| 关闭旧 `go-stock.exe` | **PASS** |
| `stock.db` 保留 | **PASS** — `build/bin/data/stock.db`（≈544 MB，mtime 18:52:58 未因构建清空） |
| 源码 / staged 保留 | **PASS** — `App.vue` M；`productMenu.js` A（staged）；`router.js` M |

### 嵌入抽检（UTF-8）

`投资驾驶舱` / `我的组合` / `机会` / `交易计划` / `事件监控` / `自选股` / `设置` / `持仓摘要` / `卖出计划` → **均存在于 exe**。

---

## 二、GUI 验收

菜单位置：应用为**底部横向** `n-menu`（非左侧栏）；与恢复实现一致。

证据：`_p17_menu_acceptance/menu_home.png`、`uia_report.txt`、各 `click_*.png`。

### 2.1 菜单显示

| 菜单项 | UIA | 截图 |
| --- | --- | --- |
| 投资驾驶舱 | Menu/Pane 可见；默认落地页 | **PASS** |
| 我的组合 | MenuItem + Hyperlink | **PASS** |
| 机会 | 可见（页内亦有「发现机会」文案） | **PASS** |
| 交易计划 | MenuItem + Hyperlink | **PASS** |
| 事件监控 | MenuItem + Hyperlink | **PASS** |
| 自选股 | MenuItem + Hyperlink | **PASS** |
| 设置 | MenuItem + Hyperlink | **PASS** |

底栏顺序（左→右）：  
**投资驾驶舱 · 我的组合 · 机会 · 交易计划 · 事件监控 · 自选股 · 设置**

### 2.2 逐项点击

| 菜单 | 操作 | 页面证据 | 结论 |
| --- | --- | --- | --- |
| 投资驾驶舱 | INVOKED | 我的资产 / 今日流程 | **PASS** |
| 我的组合 | INVOKED | 持仓列表 / 持仓快照 / 账户今日盈亏 | **PASS** |
| 机会 | INVOKED | 机会 / 选股相关页内容 | **PASS** |
| 交易计划 | INVOKED | 交易计划 / 批准相关文案 | **PASS** |
| 事件监控 | INVOKED | 监控 / 事件（异动监控 Tab） | **PASS** |
| 自选股 | INVOKED | 自选 / 行情 | **PASS** |
| 设置 | INVOKED | 设置页 | **PASS** |

---

## 三、回归

| 项 | 结论 | 证据 |
| --- | --- | --- |
| 我的组合入口 | **PASS** | 底栏「我的组合」→ `持仓列表`/`持仓快照`（`on_portfolio.png`） |
| K 线入口 | **PASS** | 组合持仓点击「永鼎股份(sh600105)」→ 多周期 K 线 Modal |
| B.2 持仓摘要 | **PASS** | UIA：`持仓摘要` / `成本` / `浮盈` = True |
| SellDraftDialog | **PASS** | footer「卖出计划」→「创建卖出计划」/「创建卖出草稿」= True（`b2_final.png`） |

说明：首次自动回归曾误点驾驶舱「今日机会/计划」股票按钮（无持仓上下文），B.2 字段为 False；纠正为**组合持仓表**开 K 后全部为 True。菜单恢复未破坏 B.2。

---

## 四、PASS / FAIL 汇总

| # | 验收项 | 结论 |
| --- | --- | --- |
| 1 | 新 exe 构建与嵌入 | **PASS** |
| 2 | 七项菜单显示 | **PASS** |
| 3 | 七项点击路由/页面 | **PASS** |
| 4 | 组合 + K 线 + B.2 + SellDraft | **PASS** |
| — | **总评** | **PASS** |

---

## 五、禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 修改代码 | 是 |
| 扩展菜单 | 是 |
| 调整布局 | 是 |

---

*Phase17 菜单恢复 Runtime 验收完成。*
