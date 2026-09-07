# PHASE17 Release Freeze Report

**日期：** 2026-09-08  
**操作：** Release Freeze（commit + tag + push）  
**分支：** `release/v0.1.0-beta`

**权威定位：** `git rev-parse phase17-release`（本报告不内嵌可变 commit 自哈希，避免 amend 漂移）

---

## 1. Commit

| 项 | 值 |
| --- | --- |
| **Message** | `feat: complete Phase17 holding intelligence release` |
| **定位** | tag `phase17-release` ≡ 本冻结 commit |
| **Files** | 30 changed（源码 + 测试 + Phase17 报告 + 本文件） |

### 纳入范围

| 能力 | 源码 / 测试 / 报告 |
| --- | --- |
| C2 Explanation | checkpoint 已有；本 commit 经 Exit / Observation 接线延续 |
| C3 HealthScore | 同上 |
| C4 Portfolio Health Display | `PortfolioDashboard.vue` 等更新 |
| **17.1 T-Suitability** | `holding_t_suitability.go`(+test) · Drawer · display · Exit 挂载 · 设计/实现报告 |
| **17.2 Data Truth** | 读模型已在 checkpoint；组合页说明/展示延续 |
| **17.5 Position Origin** | 已在 checkpoint；组合入口延续 |
| **17.6 T Signal** | `holding_t_signal.go`(+test) · FE mirror · `HoldingTPanel` · 审计/实现报告 |
| 发布整理 | Menu visibility · Final Acceptance/Verification · Release build · Git checkpoint · `RELEASE_PACKAGE_CHECK.md` · Signal Identity 设计 · **本冻结报告** |

### 暂存文件（30）

- 报告：`PHASE17_1_T_SUITABILITY_*` · `PHASE17_4_1_*` · `PHASE17_6_*` · `PHASE17_FINAL_*` · `PHASE17_GIT_*` · `PHASE17_MENU_*` · `PHASE17_RELEASE_*` · `PHASE17_TEST_COVERAGE_IMPROVEMENT_*` · `RELEASE_PACKAGE_CHECK.md` · 本文件  
- 后端：`holding_t_suitability*` · `holding_t_signal*` · `exit_evaluation.go` · `holding_evaluation_observation.go`  
- 前端：`HoldingTSuitabilityDrawer` · `holdingTSuitabilityDisplay` · `holdingTSignal*` · `HoldingTPanel` · `PortfolioDashboard` · `paperObservation.ts` · `productMenu.js` · `App.vue` · `components.d.ts`

---

## 2. 排除项（未进入 commit）

| 排除 | 说明 |
| --- | --- |
| `*.exe` / `build/bin/**` | 编译产物 |
| `release/go-stock-v0.1.0/**` | 含 exe + `stock.db` 的体验包 |
| `tmp/` · `tmp_*` · `--` | 临时探测 |
| `*.db` / WAL / 备份 | 数据库运行缓存 |
| `node_modules` / `frontend/dist` | 构建缓存/依赖 |
| `PHASE10*`–`PHASE16*` / `PHASE18*` 等未选文档 | 非本冻结范围 |
| 其余大量未跟踪历史报告 | 刻意不 `git add .` |

---

## 3. Tag

| 项 | 值 |
| --- | --- |
| **Tag** | `phase17-release` |
| **先前 checkpoint** | `phase17-checkpoint` → `6fee00c`（仍保留） |

---

## 4. Push

| 目标 | 命令 | 结果 |
| --- | --- | --- |
| 分支 | `git push origin release/v0.1.0-beta` | 见执行记录（本会话） |
| Tag | `git push origin phase17-release` | 见执行记录（本会话） |

远程核对：

```text
git ls-remote origin refs/heads/release/v0.1.0-beta
git ls-remote origin refs/tags/phase17-release
```

---

## 5. 冻结声明

Phase17 Holding Intelligence 发布态（C2/C3/C4 · 17.1 · 17.2 · 17.5 · 17.6）已以 **`phase17-release`** 冻结于分支 **`release/v0.1.0-beta`**。

**注意：** 可运行 `go-stock.exe` 需另打 wails 包；本冻结为**源码与文档**冻结，不含二进制。

---

*PHASE17_RELEASE_FREEZE_REPORT.md*
