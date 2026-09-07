// Phase9-C.3 Daily Runtime Review — read-only report generator.
// Scans logs + sqlite + Observation ENTRY; writes markdown.
// Does not modify trading logic, strategy, config, or database.
//
// Usage (repo root):
//
//	go run ./scripts/phase9c3_daily_review -date 2026-08-03
//	go run ./scripts/phase9c3_daily_review -date 2026-08-03 -out D:\stock
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	date := flag.String("date", time.Now().Format("2006-01-02"), "trade date YYYY-MM-DD")
	repo := flag.String("repo", "", "repo root (default: cwd)")
	bin := flag.String("bin", "", "runtime bin dir (default: <repo>/build/bin)")
	outDir := flag.String("out", "", "report output dir (default: repo root)")
	jsonOnly := flag.Bool("json", false, "print DB snapshot JSON only (no markdown)")
	flag.Parse()

	root := *repo
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			root = filepath.Dir(cwd)
		}
	}
	binDir := *bin
	if binDir == "" {
		binDir = filepath.Join(root, "build", "bin")
	}
	out := *outDir
	if out == "" {
		out = root
	}

	dbPath := filepath.Join(binDir, "data", "stock.db")
	infoLog := filepath.Join(binDir, "logs", "info.log")
	errLog := filepath.Join(binDir, "logs", "error.log")
	exePath := filepath.Join(binDir, "go-stock.exe")

	snap := loadDBSnapshot(*date, dbPath)
	if *jsonOnly {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(snap)
		return
	}

	infoLines := filterDayLines(infoLog, *date)
	errLines := filterDayLines(errLog, *date)
	entryPath, entryText := loadEntry(root, *date)

	rep := buildReport(*date, root, binDir, exePath, dbPath, infoLog, errLog, infoLines, errLines, entryPath, entryText, snap)
	ymd := strings.ReplaceAll(*date, "-", "")
	outPath := filepath.Join(out, fmt.Sprintf("PHASE9_C3_DAILY_RUNTIME_REVIEW_%s.md", ymd))
	if err := os.WriteFile(outPath, []byte(rep), 0o644); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "[phase9c3_daily_review] wrote %s\n", outPath)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "phase9c3_daily_review: %v\n", err)
	os.Exit(1)
}

func filterDayLines(path, date string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	prefix := date
	var out []string
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 8*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, prefix) {
			out = append(out, line)
		}
	}
	return out
}

func firstMatch(lines []string, re *regexp.Regexp) string {
	for _, ln := range lines {
		if m := re.FindStringSubmatch(ln); len(m) > 1 {
			return m[1]
		}
	}
	return "n/a"
}

func findLines(lines []string, re *regexp.Regexp) []string {
	var out []string
	for _, ln := range lines {
		if re.MatchString(ln) {
			out = append(out, ln)
		}
	}
	return out
}

func countRe(lines []string, re *regexp.Regexp) int {
	n := 0
	for _, ln := range lines {
		if re.MatchString(ln) {
			n++
		}
	}
	return n
}

func snip(lines []string, n int) string {
	if len(lines) == 0 {
		return "- (none)"
	}
	if n > len(lines) {
		n = len(lines)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("- `")
		b.WriteString(lines[i])
		b.WriteString("`\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func bullets(items []string) string {
	if len(items) == 0 {
		return "- (none)"
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString("- ")
		b.WriteString(it)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func loadEntry(root, date string) (string, string) {
	exact := filepath.Join(root, fmt.Sprintf("PHASE9_C3_OBSERVATION_ENTRY_%s.md", date))
	if b, err := os.ReadFile(exact); err == nil {
		return exact, string(b)
	}
	matches, _ := filepath.Glob(filepath.Join(root, "PHASE9_C3_OBSERVATION_ENTRY*.md"))
	if len(matches) == 0 {
		return "none", ""
	}
	sort.Slice(matches, func(i, j int) bool {
		ii, _ := os.Stat(matches[i])
		jj, _ := os.Stat(matches[j])
		if ii == nil || jj == nil {
			return matches[i] > matches[j]
		}
		return ii.ModTime().After(jj.ModTime())
	})
	b, err := os.ReadFile(matches[0])
	if err != nil {
		return "none", ""
	}
	return matches[0], string(b)
}

func entryField(text, label string) string {
	if text == "" {
		return "n/a"
	}
	re := regexp.MustCompile(`(?mi)^` + regexp.QuoteMeta(label) + `\s*:\s*(.+)$`)
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	re2 := regexp.MustCompile(`(?mis)` + regexp.QuoteMeta(label) + `\s*:\s*\r?\n([^\r\n]+)`)
	if m := re2.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return "n/a"
}

type dbSnap struct {
	TradeDate          string           `json:"tradeDate"`
	DBPath             string           `json:"dbPath"`
	Generated          string           `json:"generated"`
	FollowedWithVolume int64            `json:"followedWithVolume"`
	Counts             map[string]int64 `json:"counts"`
	TradePlans         []map[string]any `json:"tradePlans"`
	CandidatePools     []map[string]any `json:"candidatePools"`
	PaperSimRuns       []map[string]any `json:"paperSimRuns"`
	PaperAccounts      []map[string]any `json:"paperAccounts"`
	PaperMarginAccts   []map[string]any `json:"paperMarginAccounts"`
	Error              string           `json:"error,omitempty"`
}

func loadDBSnapshot(date, dbPath string) dbSnap {
	out := dbSnap{
		TradeDate: date,
		DBPath:    dbPath,
		Generated: time.Now().Format(time.RFC3339),
		Counts:    map[string]int64{},
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		out.Error = err.Error()
		return out
	}
	var tables []string
	_ = db.Raw(`SELECT name FROM sqlite_master WHERE type='table'`).Scan(&tables)
	has := map[string]bool{}
	for _, t := range tables {
		has[t] = true
	}
	for _, t := range []string{
		"followed_stock", "trade_plans", "candidate_pools", "paper_orders",
		"paper_sim_runs", "paper_sim_orders", "paper_sim_fills", "paper_accounts", "paper_margin_accounts",
	} {
		if !has[t] {
			continue
		}
		var n int64
		_ = db.Table(t).Count(&n)
		out.Counts[t] = n
	}
	if has["followed_stock"] {
		_ = db.Table("followed_stock").Where("volume > 0").Count(&out.FollowedWithVolume)
	}
	out.TradePlans = qMaps(db, has, "trade_plans",
		`SELECT id, trade_date, status, pool_id, risk_status, enable_execute, approved_at, freeze_at, executed_at, message
		 FROM trade_plans WHERE trade_date = ? ORDER BY id`, date)
	out.CandidatePools = qMaps(db, has, "candidate_pools",
		`SELECT id, trade_date, status, item_count, source, message FROM candidate_pools WHERE trade_date = ? ORDER BY id`, date)
	out.PaperSimRuns = qMaps(db, has, "paper_sim_runs",
		`SELECT id, trade_date, status, plan_id, orders_total, filled_count, message, started_at, finished_at
		 FROM paper_sim_runs ORDER BY id DESC LIMIT 5`)
	out.PaperAccounts = qMaps(db, has, "paper_accounts", `SELECT id, name FROM paper_accounts`)
	out.PaperMarginAccts = qMaps(db, has, "paper_margin_accounts",
		`SELECT id, account_id, last_accrued_at, mode FROM paper_margin_accounts`)
	return out
}

func qMaps(db *gorm.DB, has map[string]bool, table, sql string, args ...any) []map[string]any {
	if !has[table] {
		return nil
	}
	var rows []map[string]any
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return []map[string]any{{"error": err.Error()}}
	}
	for _, r := range rows {
		for k, v := range r {
			if b, ok := v.([]byte); ok {
				r[k] = string(b)
			}
		}
	}
	return rows
}

func buildReport(date, root, binDir, exePath, dbPath, infoLog, errLog string, info, errs []string, entryPath, entryText string, snap dbSnap) string {
	now := time.Now().Format("2006-01-02 15:04:05")
	ymd := strings.ReplaceAll(date, "-", "")

	reFP := regexp.MustCompile(`Database Fingerprint:\s*(\S+)`)
	reSchema := regexp.MustCompile(`startup schema validation\s+(\S+)\s+version=(\d+)`)
	reMigr := regexp.MustCompile(`schema migration registry:\s*(.+)`)
	reWD := regexp.MustCompile(`Working Directory:\s*(.+)`)
	reOpenBuy := regexp.MustCompile(`EnablePaperOpenBuy:\s*(\S+)`)
	reACEnable := regexp.MustCompile(`after-close plan cron registered.+enable=(\w+)`)
	reExecReady := regexp.MustCompile(`ExecutionReady:\s*(.+)`)
	rePlan := regexp.MustCompile(`BuildTradePlan date=`)
	rePool := regexp.MustCompile(`candidate pool:`)
	rePrepare := regexp.MustCompile(`PREPARE blocked|paper open buy PREPARE`)
	reBuySkip := regexp.MustCompile(`paper open buy SKIP|EnablePaperOpenBuy=false, skip buy`)
	reMorning := regexp.MustCompile(`RunMorningPlanPreparation|BuildTradePlan date=`)
	reACSkip := regexp.MustCompile(`AfterCloseWorkflow cron skipped|after_close_plan_enabled=false`)
	reMarginAccrue := regexp.MustCompile(`paper margin accrue`)
	reMarginScan := regexp.MustCompile(`paper margin scan`)
	reEnhancer := regexp.MustCompile(`signal enhancer applied`)
	reEmptyWarn := regexp.MustCompile(`WARNING: TradePlan empty`)
	reEOF := regexp.MustCompile(`EOF|eastmoney|EastMoney|push2his\.eastmoney`)
	reTO := regexp.MustCompile(`(?i)timeout|deadline`)
	reDing := regexp.MustCompile(`dingding|unsupported protocol scheme`)
	reMarginErr := regexp.MustCompile(`paper margin`)
	reC3 := regexp.MustCompile(`controlledSwitch|holdingDecision|projectionSelected|riskAdvice_projection`)
	reTencent := regexp.MustCompile(`tCode=.*count=|腾讯`)
	reSina := regexp.MustCompile(`sCode=.*count=|新浪`)
	reRetry := regexp.MustCompile(`attempt=\d+ err=|请求重试`)

	fp := firstMatch(info, reFP)
	schema := "n/a"
	if m := findLines(info, reSchema); len(m) > 0 {
		sm := reSchema.FindStringSubmatch(m[0])
		if len(sm) > 2 {
			schema = sm[1] + " v" + sm[2]
		}
	}
	migr := firstMatch(info, reMigr)
	cwd := firstMatch(info, reWD)
	start := "n/a"
	if binds := findLines(info, regexp.MustCompile(`Runtime Data Binding:`)); len(binds) > 0 {
		if len(binds[0]) >= 19 {
			start = binds[0][:19]
		}
	}
	openBuy := firstMatch(info, reOpenBuy)
	acEnable := firstMatch(info, reACEnable)
	execReady := firstMatch(info, reExecReady)

	planLines := findLines(info, rePlan)
	poolLines := findLines(info, rePool)
	prepareLines := findLines(info, rePrepare)
	buySkip := findLines(info, reBuySkip)
	morning := findLines(info, reMorning)
	acSkip := findLines(info, reACSkip)
	marginAccrue := append(findLines(info, reMarginAccrue), findLines(errs, reMarginAccrue)...)
	marginScan := append(findLines(info, reMarginScan), findLines(errs, reMarginScan)...)
	enhancer := findLines(info, reEnhancer)
	emptyWarn := findLines(info, reEmptyWarn)

	planID, poolID, items, risk, poolTotal := "n/a", "n/a", "n/a", "n/a", "n/a"
	reBuildTP := regexp.MustCompile(`BuildTradePlan date=\S+ planId=(\d+) poolId=(\d+) items=(\d+).*risk=(\w+)`)
	reCandPool := regexp.MustCompile(`candidate pool:\s*total=(\d+).*poolId=(\d+)`)
	// Prefer morning BuildTradePlan (first of day); keep last cand pool as extra note.
	for _, ln := range planLines {
		if m := reBuildTP.FindStringSubmatch(ln); len(m) > 4 {
			if planID == "n/a" {
				planID, poolID, items, risk = m[1], m[2], m[3], m[4]
			}
		}
	}
	var laterPools []string
	for _, ln := range poolLines {
		if m := reCandPool.FindStringSubmatch(ln); len(m) > 2 {
			if poolTotal == "n/a" {
				poolTotal, poolID = m[1], m[2]
			} else if m[2] != poolID {
				laterPools = append(laterPools, fmt.Sprintf("poolId=%s total=%s", m[2], m[1]))
			}
		}
	}
	// Align poolId with morning plan when available.
	for _, ln := range planLines {
		if m := reBuildTP.FindStringSubmatch(ln); len(m) > 4 {
			planID, poolID, items, risk = m[1], m[2], m[3], m[4]
			break
		}
	}
	for _, ln := range poolLines {
		if m := reCandPool.FindStringSubmatch(ln); len(m) > 2 && m[2] == poolID {
			poolTotal = m[1]
			break
		}
	}
	prepareReason := "n/a"
	reReason := regexp.MustCompile(`reason=(\S+)`)
	for _, ln := range prepareLines {
		if m := reReason.FindStringSubmatch(ln); len(m) > 1 {
			prepareReason = m[1]
			break
		}
	}
	buyReason := "n/a (no skip line)"
	if len(buySkip) > 0 {
		buyReason = buySkip[0]
	}
	acStatus := "not seen in day log (before 15:30 or rotated)"
	if len(acSkip) > 0 {
		acStatus = "skipped or enable=false visible"
	}

	errTotal := len(errs)
	errEOF := countRe(errs, reEOF)
	errTO := countRe(errs, reTO)
	errDing := countRe(errs, reDing)
	errMargin := countRe(errs, reMarginErr)
	var errOther []string
	for _, ln := range errs {
		if !reEOF.MatchString(ln) {
			errOther = append(errOther, ln)
		}
	}
	tencentOK := countRe(info, reTencent)
	sinaOK := countRe(info, reSina)
	emRetry := countRe(info, reRetry)
	c3Hits := countRe(info, reC3)

	c3Samples := entryField(entryText, "Samples")
	c3Source := entryField(entryText, "Controlled Switch")
	c3Usage := entryField(entryText, "Projection Usage")
	c3Fallback := entryField(entryText, "Fallback")
	c3Unavail := entryField(entryText, "Unavailable")
	c3Shadow := entryField(entryText, "Shadow")
	c3Mismatch := entryField(entryText, "Mismatch Reason")
	c3Healthy := entryField(entryText, "Healthy Window")
	c3Reason := entryField(entryText, "Reason")

	plansSummary := "n/a"
	if len(snap.TradePlans) > 0 {
		var parts []string
		for _, p := range snap.TradePlans {
			parts = append(parts, fmt.Sprintf("id=%v status=%v approved=%v freeze=%v",
				p["id"], p["status"], p["approved_at"] != nil && fmt.Sprint(p["approved_at"]) != "", p["freeze_at"] != nil && fmt.Sprint(p["freeze_at"]) != ""))
		}
		plansSummary = strings.Join(parts, "; ")
	}
	simSummary := "n/a"
	if len(snap.PaperSimRuns) > 0 {
		r := snap.PaperSimRuns[0]
		simSummary = fmt.Sprintf("latest id=%v date=%v status=%v filled=%v", r["id"], r["trade_date"], r["status"], r["filled_count"])
	}
	paperOrders := "n/a"
	if v, ok := snap.Counts["paper_orders"]; ok {
		paperOrders = fmt.Sprintf("%d", v)
	}
	countsJSON, _ := json.Marshal(snap.Counts)

	exeMeta := "n/a"
	if st, err := os.Stat(exePath); err == nil {
		exeMeta = fmt.Sprintf("mtime=%s; size=%.1fMB", st.ModTime().Format("2006-01-02 15:04:05"), float64(st.Size())/1e6)
	}
	dbMeta := "n/a"
	if st, err := os.Stat(dbPath); err == nil {
		dbMeta = fmt.Sprintf("size=%.1fMB; mtime=%s", float64(st.Size())/1e6, st.ModTime().Format("2006-01-02 15:04:05"))
	}

	var why []string
	overall := "WARNING"
	if len(info) == 0 {
		overall = "BLOCKED"
		why = append(why, "no info.log lines for trade date")
	} else {
		if len(planLines) > 0 {
			why = append(why, "morning Candidate/TradePlan path observed")
		} else {
			why = append(why, "morning BuildTradePlan not found")
		}
		if len(buySkip) > 0 || strings.Contains(prepareReason, "PLAN_NOT_FROZEN") {
			why = append(why, "execution skipped by config/gate (observation-typical)")
		}
		if errEOF > 100 {
			why = append(why, fmt.Sprintf("EastMoney EOF noise count=%d (DATA-001)", errEOF))
		}
		if entryPath == "none" || strings.Contains(c3Healthy, "NOT COUNTED") {
			why = append(why, "C.3 HEALTHY window not counted or ENTRY thin")
		}
		if errMargin > 0 {
			why = append(why, "paper margin errors present")
		}
		if len(planLines) > 0 && errMargin == 0 && strings.Contains(c3Healthy, "HEALTHY") && !strings.Contains(c3Healthy, "NOT") {
			overall = "PASS"
		}
	}

	var p0, p1, p2, p3 []string
	if len(info) == 0 {
		p0 = append(p0, "No runtime info lines for trade date")
	}
	if errEOF > 50 {
		p2 = append(p2, fmt.Sprintf("DATA-001 EastMoney EOF/errors ~ %d", errEOF))
	}
	if errMargin > 0 {
		p1 = append(p1, "Paper margin account not found / accrue-scan errors")
	}
	if errDing > 0 {
		p2 = append(p2, "DingTalk webhook empty / protocol scheme error")
	}
	if len(emptyWarn) > 0 {
		p3 = append(p3, "Daily Check WARN TradePlan empty vs items present")
	}
	if strings.Contains(prepareReason, "PLAN_NOT_FROZEN") {
		p3 = append(p3, "PREPARE blocked PLAN_NOT_FROZEN")
	}
	if len(buySkip) > 0 {
		p3 = append(p3, "Open-buy SKIP EnablePaperOpenBuy=false")
	}
	for _, ln := range enhancer {
		if strings.Contains(ln, "matched=0") {
			p2 = append(p2, "Signal enhancer matched=0")
			break
		}
	}
	if entryPath == "none" {
		p1 = append(p1, fmt.Sprintf("Missing PHASE9_C3_OBSERVATION_ENTRY_%s.md", date))
	} else if strings.Contains(c3Healthy, "NOT COUNTED") {
		p3 = append(p3, "C.3 Healthy Window NOT COUNTED")
	}
	if c3Hits == 0 {
		p3 = append(p3, "No backend C.3 metrics (by design)")
	}

	poolCell := "not found"
	if poolID != "n/a" || poolTotal != "n/a" {
		poolCell = fmt.Sprintf("seen poolId=%s total=%s", poolID, poolTotal)
		if len(laterPools) > 0 {
			poolCell += "; later=" + strings.Join(laterPools, ",")
		}
	}
	planCell := "not found"
	if planID != "n/a" {
		planCell = fmt.Sprintf("seen planId=%s items=%s risk=%s", planID, items, risk)
	}
	backendC3 := "no structured C.3 metrics in backend logs (use GUI ENTRY)"
	if c3Hits > 0 {
		backendC3 = fmt.Sprintf("info keyword hits~=%d", c3Hits)
	}
	dbErr := ""
	if snap.Error != "" {
		dbErr = " DB probe error: " + snap.Error
	}

	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }
	w("# PHASE9_C3_DAILY_RUNTIME_REVIEW_" + ymd)
	w("")
	w("> **Nature:** read-only Daily Runtime Review — no code/DB/config/trading changes")
	w("> **Generator:** `go run ./scripts/phase9c3_daily_review`")
	w("> **TradeDate:** " + date)
	w("> **GeneratedAt:** " + now + " (local)")
	w("> **Evidence:** `" + infoLog + "` / `" + errLog + "`; DB `" + dbPath + "`; ENTRY `" + entryPath + "`")
	w("")
	w("---")
	w("")
	w("# Runtime Summary")
	w("")
	w("Runtime:")
	w("- **TradeDate:** " + date)
	w("- **Process start (log):** " + start)
	w("- **Working Directory:** " + cwd)
	w("- **Review generated:** " + now)
	w("")
	w("Build / Binding:")
	w("- **exe:** `" + exePath + "` (" + exeMeta + ")")
	w("- **DB:** " + dbMeta + dbErr)
	w("- **Fingerprint:** `" + fp + "`")
	w("- **Schema:** " + schema)
	w("- **Migration:** " + migr)
	w("")
	w("---")
	w("")
	w("## 1. Overall Status")
	w("")
	w("**" + overall + "**")
	w("")
	w("Reasons:")
	w(bullets(why))
	w("")
	w("---")
	w("")
	w("## 2. Trading Pipeline / Paper Lifecycle")
	w("")
	w("| Node | Status | Notes |")
	w("|------|--------|-------|")
	w("| Candidate Pool | " + poolCell + " | morning log |")
	w("| TradePlan Generate | " + planCell + " | BuildTradePlan |")
	w("| Approve / Freeze | " + plansSummary + " | sqlite read-only |")
	w("| Prepare 09:25 | reason=" + prepareReason + " | PREPARE |")
	w("| Buy 09:30 | see skip | " + buyReason + " |")
	w("| after_close 15:30 | " + acStatus + " | cron |")
	w("| ExecutionReady | " + execReady + " | Daily Check |")
	w("")
	w("Notes:")
	w("- EnablePaperOpenBuy=" + openBuy + "; after_close enable=" + acEnable)
	w("- paper_orders=" + paperOrders + "; sim=" + simSummary)
	w(fmt.Sprintf("- followed_stock volume>0 = %d", snap.FollowedWithVolume))
	w("")
	w("Evidence:")
	w(snip(planLines, 3))
	w(snip(prepareLines, 3))
	w(snip(buySkip, 3))
	w("")
	w("---")
	w("")
	w("## 3. Controlled Switch Observation")
	w("")
	w("| Field | Value |")
	w("|-------|-------|")
	w("| ENTRY file | " + entryPath + " |")
	w("| Samples | " + c3Samples + " |")
	w("| Controlled Switch | " + c3Source + " |")
	w("| Projection Usage | " + c3Usage + " |")
	w("| Fallback | " + c3Fallback + " |")
	w("| Unavailable | " + c3Unavail + " |")
	w("| Shadow | " + c3Shadow + " |")
	w("| Mismatch Reason | " + c3Mismatch + " |")
	w("| Healthy Window | " + c3Healthy + " |")
	w("| Reason | " + c3Reason + " |")
	w("| Backend log C.3 | " + backendC3 + " |")
	w(fmt.Sprintf("| Holdings volume>0 | %d |", snap.FollowedWithVolume))
	w("")
	w("Judgement:")
	w("- C.3 metrics authority is GUI same-process / ENTRY, not backend error.log.")
	w("- If Healthy NOT COUNTED: continue observation; do not claim READY_FOR_LEGACY_REVIEW.")
	w("")
	w("---")
	w("")
	w("## 4. Paper Trading")
	w("")
	w("| Item | Result |")
	w("|------|--------|")
	if len(buySkip) > 0 {
		w("| open-buy | SKIP (config/gate) |")
	} else {
		w("| open-buy | n/a |")
	}
	w("| Prepare block | " + prepareReason + " |")
	w("| paper_orders | " + paperOrders + " |")
	w("| latest paper_sim_run | " + simSummary + " |")
	w(fmt.Sprintf("| margin errors | ~%d |", errMargin))
	w("")
	w("Margin lines:")
	w(snip(append(marginAccrue, marginScan...), 5))
	w("")
	w("---")
	w("")
	w("## 5. Market Data")
	w("")
	w("| Metric | Count |")
	w("|--------|-------|")
	w(fmt.Sprintf("| error.log day lines | %d |", errTotal))
	w(fmt.Sprintf("| EOF / EastMoney-like | %d |", errEOF))
	w(fmt.Sprintf("| timeout / deadline | %d |", errTO))
	w(fmt.Sprintf("| DingTalk-like | %d |", errDing))
	w(fmt.Sprintf("| paper margin | %d |", errMargin))
	w(fmt.Sprintf("| info tencent-fallback-like | %d |", tencentOK))
	w(fmt.Sprintf("| info sina-fallback-like | %d |", sinaOK))
	w(fmt.Sprintf("| info eastmoney-retry-like | %d |", emRetry))
	w("")
	w("| Issue | Severity | Need fix now? |")
	w("|-------|----------|---------------|")
	w("| EastMoney EOF flood | medium | No (Phase9) — DATA-001 |")
	w("| Tencent/Sina fallback | low | No — mitigation path |")
	w("| DingTalk empty webhook | low | P2 config |")
	w("| margin account not found | medium | P1 suggested |")
	w("")
	w("Non-EOF error samples:")
	w(snip(errOther, 8))
	w("")
	w("---")
	w("")
	w("## 6. Scheduler Status")
	w("")
	w("| Job | Observation |")
	w("|-----|--------------|")
	obReg := "not in day log"
	if len(findLines(info, regexp.MustCompile(`paper open buy cron registered`))) > 0 {
		obReg = "registered"
	}
	acReg := "not in day log"
	if acEnable != "n/a" {
		acReg = "enable=" + acEnable
	}
	w("| paper open-buy register | " + obReg + " |")
	w("| after_close register | " + acReg + " |")
	w("| 09:20 morning | " + mapBool(len(morning) > 0, "fired", "not found") + " |")
	w("| 09:25 prepare | " + mapBool(len(prepareLines) > 0, "fired", "not found") + " |")
	w("| 09:30 buy | " + mapBool(len(buySkip) > 0, "skip seen", "not found") + " |")
	w("| 15:10 margin accrue | " + mapBool(len(marginAccrue) > 0, "seen", "not found") + " |")
	w("| 15:20 margin scan | " + mapBool(len(marginScan) > 0, "seen", "not found") + " |")
	w("| 15:30 after_close | " + acStatus + " |")
	w("")
	w("---")
	w("")
	w("## 7. Database (read-only)")
	w("")
	w("- Probe: embedded sqlite read via `scripts/phase9c3_daily_review`")
	w("- Plans: " + plansSummary)
	w(fmt.Sprintf("- followed volume>0: %d", snap.FollowedWithVolume))
	w("- counts: " + string(countsJSON))
	w("")
	w("> No DB tables for shadow metrics / holding decisions (GUI memory by design).")
	w("")
	w("---")
	w("")
	w("## 8. Issues Classification")
	w("")
	w("### P0 must fix")
	w(bullets(p0))
	w("")
	w("### P1 suggested")
	w(bullets(p1))
	w("")
	w("### P2 architecture debt")
	w(bullets(p2))
	w("")
	w("### P3 observe only")
	w(bullets(p3))
	w("")
	w("---")
	w("")
	w("## 9. Recommended Next Actions")
	w("")
	w(fmt.Sprintf("1. If ENTRY missing: after same-process scan, write `PHASE9_C3_OBSERVATION_ENTRY_%s.md`.", date))
	w("2. Keep EnablePaperOpenBuy / after_close off unless leaving observation mode.")
	w("3. DATA-001 / margin ghost account -> Phase10; this tool does not fix.")
	w("4. Re-run after close to capture 15:20 / 15:30 lines.")
	w("")
	w("---")
	w("")
	w("**End of read-only daily review.**")
	w("")
	_ = root
	_ = binDir
	return b.String()
}

func mapBool(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
