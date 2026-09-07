# RELEASE_PACKAGE_CHECK — go-stock 外部体验版

**日期：** 2026-09-08  
**包路径：** `D:\stock\release\go-stock-v0.1.0\`  
**性质：** 仅打包发布目录（**未修改业务代码**）  
**构建来源：** `D:\stock\build\bin\go-stock.exe`（wails production，mtime 2026-09-08 00:04:41）

---

## 1. wails build 输出核对

| 项 | 路径 / 值 |
| --- | --- |
| 主程序 | `build\bin\go-stock.exe`（≈92.4 MB） |
| 运行时数据目录 | `build\bin\data\`（含 `stock.db` ≈530 MB） |
| 日志 | `build\bin\logs\` |
| 运行时元数据 | `build\bin\runtime\profile.json` |
| 前端 | 已嵌入 exe（无需目标机 Node / npm） |
| 基础行情字典 | 已 `go:embed` 进二进制（无需拷贝 `build\stock_basic.json`） |

---

## 2. 发布目录结构

```text
release/go-stock-v0.1.0/
  go-stock.exe
  START.bat
  README.txt
  README-zh.txt
  data/
    stock.db
    stock.db-wal          （与主库一致快照；若存在则一并拷贝）
    stock.db-shm
    paper_trading_mvp.json
    paper_open_buy.json
    after_close_plan.json
    trading_automation.json
    release.json          （本包渠道戳：external-experience）
    version.json
    strategy_intents.json
    strategy_schemas.json
    paper_observation_history.json
    observation/          （空目录占位）
    crash_reports/        （空目录占位）
  logs/
    .keep
    README.txt
  runtime/
    profile.json          （模板；启动后可由程序刷新）
```

**包体约：** 647 MB（主要是 `stock.db` + wal）  
**未包含：** 源代码、`node_modules`、`.git`、`tmp`、测试、`PHASE*.md` 开发文档、`stock.db.backup.*` 历史备份、开发机大日志。

---

## 3. 复制文件清单与用途

| 相对路径 | 来源 | 用途 |
| --- | --- | --- |
| `go-stock.exe` | `build/bin/go-stock.exe` | 桌面主程序（内嵌前端 + Go 后端） |
| `START.bat` | 打包生成 | **将 cwd 设为包根目录**后启动，避免空库 |
| `README.txt` / `README-zh.txt` | 打包生成 | 目标机操作说明（中英；非开发文档） |
| `data/stock.db` | `build/bin/data/stock.db` | 体验库（持仓/计划/行情缓存等） |
| `data/stock.db-wal` | 同目录（若有） | SQLite WAL，与主库一致拷贝 |
| `data/stock.db-shm` | 同目录（若有） | SQLite shared memory |
| `data/paper_trading_mvp.json` | 仓库 `data/` | 开启 paper 模拟盘 |
| `data/paper_open_buy.json` | 仓库 `data/` | 开盘买入开关（默认关） |
| `data/after_close_plan.json` | 仓库 `data/` | 盘后计划开关 |
| `data/trading_automation.json` | 仓库 `data/` | 自动化模式 MANUAL 等 |
| `data/release.json` | **本包重写** | 外部体验渠道版本戳 |
| `data/version.json` | 仓库 `data/` | 版本展示/更新提示元数据 |
| `data/strategy_intents.json` | 仓库 `data/` | 策略意图默认数据 |
| `data/strategy_schemas.json` | 仓库 `data/` | 策略 schema 默认数据 |
| `data/paper_observation_history.json` | 仓库 `data/` | 观察历史样例 |
| `data/observation/` | 新建空目录 | 观察产物目录占位 |
| `data/crash_reports/` | 新建空目录 | 崩溃报告目录占位 |
| `logs/.keep` + `logs/README.txt` | 打包生成 | 日志目录；运行后写 info/error |
| `runtime/profile.json` | 打包模板 | 运行环境说明；启动后可被覆盖 |

---

## 4. 明确排除项

| 排除 | 原因 |
| --- | --- |
| `backend/` `frontend/` `main.go` 等源码 | 外部体验不需要编译 |
| `node_modules/` | 前端已嵌入 |
| `.git/` | 非运行依赖 |
| `tmp/` `tmp_*` | 开发探测 |
| `*_test.go` / 测试夹具 | 非运行依赖 |
| `PHASE*.md` 等开发审计文档 | 非运行依赖 |
| `data/stock.db.backup.*` | 体积大且非必需 |
| `build/bin/logs/*.log` 开发日志 | 避免带出本机隐私/噪声 |
| Go / Wails / npm 工具链 | 目标机不需要 |

---

## 5. 另一台电脑运行前注意事项

1. **整目录拷贝**  
   必须保留 `go-stock.exe` 与同级的 `data\`、`logs\`、`runtime\`。只拷 exe 会新建空库，功能异常。

2. **用 `START.bat` 启动（强烈推荐）**  
   Beta 要求 **进程 cwd = exe 所在目录**。从资源管理器进入该文件夹再双击 exe 也可以；桌面快捷方式须把「起始位置」设为本包根目录。

3. **WebView2**  
   需已安装 [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。Windows 10/11 多数已预装。缺失时窗口可能白屏或无法打开。

4. **先关掉旧进程**  
   若目标机已有 `go-stock.exe` 在跑，先结束进程再覆盖/启动，避免文件占用。

5. **杀毒 / SmartScreen**  
   未签名 exe 可能被拦截；需允许本地运行。

6. **网络**  
   行情/扫描依赖外网；离线可打开 UI，但实时数据会失败。

7. **磁盘**  
   预留 ≥1 GB（库 + WAL + 日志增长）。

8. **体验范围**  
   本包为 **模拟盘 / 观察体验**（`enablePaperTrading=true`，自动化 MANUAL）。**不是**实盘券商交易包。

9. **数据隐私**  
   `stock.db` 来自当前开发机快照，含本地业务数据；若发给他人，请确认可接受，或日后改为空库模板包。

10. **不要改业务代码后只换配置**  
    本检查仅描述打包结果；升级请重新 `wails build` 再打新包。

---

## 6. 目标机建议验收（30 秒）

| 检查 | 通过标准 |
| --- | --- |
| 启动 | `START.bat` 后窗口出现，loading 结束 |
| 导航 | 底部菜单可见（组合 / 机会 / 交易计划等） |
| 数据 | 「我的组合」可打开且非因空库全空异常（视快照内容） |
| 日志 | `logs\` 下出现运行日志（可选） |

---

## 7. 结论

| 项 | 状态 |
| --- | --- |
| 外部体验目录已生成 | ✅ `release/go-stock-v0.1.0/` |
| 含 exe + data + 默认配置 + logs + runtime | ✅ |
| 排除源码/开发依赖 | ✅ |
| 业务代码未改 | ✅ |

**分发方式：** 将整个 `go-stock-v0.1.0` 文件夹压缩（zip）拷贝到另一台 Windows 即可。

---

*RELEASE_PACKAGE_CHECK.md*
