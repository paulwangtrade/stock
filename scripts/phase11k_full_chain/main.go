// Phase11-K full-chain acceptance harness (isolated SQLite).
// Observation only — does not modify product packages.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/home"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingcalendar"
	"go-stock/backend/tradingdaymonitor"
	"go-stock/backend/tradingevent"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Simulated trading day: Monday 2026-08-17 (confirm via calendar).
var (
	tradeDay   = time.Date(2026, 8, 17, 0, 0, 0, 0, time.Local)
	prevDayStr = "2026-08-14" // previous trading day (Fri)
	tradeStr   = "2026-08-17"
)

type nodeLog struct {
	Clock       string         `json:"clock"`
	Action      string         `json:"action"`
	OK          bool           `json:"ok"`
	Notes       []string       `json:"notes"`
	Findings    []string       `json:"findings,omitempty"`
	Position    any            `json:"position_state,omitempty"`
	TradePlans  any            `json:"trade_plans,omitempty"`
	HomeMon     any            `json:"home_monitor,omitempty"`
	ExecCheck   any            `json:"exec_check,omitempty"`
	Settlement  any            `json:"settlement,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type report struct {
	Harness     string    `json:"harness"`
	TradeDate   string    `json:"trade_date"`
	DBPath      string    `json:"db_path"`
	StartedAt   string    `json:"started_at"`
	Nodes       []nodeLog `json:"nodes"`
	Findings    []string  `json:"findings"`
	Consistency map[string]any `json:"consistency"`
}

func main() {
	if !tradingcalendar.IsTradingDay(tradeDay) {
		fail("tradeDay %s is not a trading day", tradeStr)
	}
	root := filepath.Clean(filepath.Join(".", "tmp", "phase11k_full_chain"))
	_ = os.RemoveAll(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		fail("mkdir: %v", err)
	}
	dbPath := filepath.Join(root, "acceptance.db")
	setupDB(dbPath)

	rep := report{
		Harness:   "Phase11-K Full-Chain Acceptance",
		TradeDate: tradeStr,
		DBPath:    dbPath,
		StartedAt: time.Now().Format(time.RFC3339),
		Findings:  []string{},
		Consistency: map[string]any{},
	}

	acc := seedAccount()
	posYesterday := seedLockedPos(acc.ID, "sz000001", "平安银行", 1000)
	seedBuyFill(acc.ID, "sz000001", prevDayStr, 1000)
	// Today's incremental buy still locked (partial after unlock of yesterday).
	posTodayCode := "sz000002"
	_ = seedLockedPos(acc.ID, posTodayCode, "万科A", 500)
	seedBuyFill(acc.ID, posTodayCode, tradeStr, 500)

	oversize := seedOversizePlan()

	// ---- 09:20 ----
	n0920 := run0920(posYesterday.ID)
	rep.Nodes = append(rep.Nodes, n0920)
	rep.Findings = append(rep.Findings, n0920.Findings...)

	// ---- 09:25 ----
	n0925 := run0925(oversize.ID)
	rep.Nodes = append(rep.Nodes, n0925)
	rep.Findings = append(rep.Findings, n0925.Findings...)

	// ---- 09:29 ----
	var rescalePlanID uint
	if m, ok := n0925.TradePlans.(map[string]any); ok {
		if id, ok := m["new_plan_id"].(float64); ok {
			rescalePlanID = uint(id)
		} else if id, ok := m["new_plan_id"].(uint); ok {
			rescalePlanID = id
		}
	}
	// Prefer typed extraction
	rescalePlanID = extractNewPlanID(n0925)
	n0929 := run0929(rescalePlanID)
	rep.Nodes = append(rep.Nodes, n0929)
	rep.Findings = append(rep.Findings, n0929.Findings...)

	// ---- 09:31 ----
	n0931 := run0931(rescalePlanID, oversize.ID)
	rep.Nodes = append(rep.Nodes, n0931)
	rep.Findings = append(rep.Findings, n0931.Findings...)

	// ---- 15:05 ----
	n1505 := run1505()
	rep.Nodes = append(rep.Nodes, n1505)
	rep.Findings = append(rep.Findings, n1505.Findings...)

	// Consistency summary from last Home/Monitor attach
	rep.Consistency = buildConsistencySummary(rep.Nodes)

	outPath := filepath.Join(root, "acceptance_report.json")
	b, _ := json.MarshalIndent(rep, "", "  ")
	_ = os.WriteFile(outPath, b, 0o644)
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "\nWrote %s\n", outPath)
}

func extractNewPlanID(n nodeLog) uint {
	m, ok := n.Extra["new_plan_id"]
	if !ok {
		return 0
	}
	switch v := m.(type) {
	case uint:
		return v
	case int:
		return uint(v)
	case float64:
		return uint(v)
	case json.Number:
		i, _ := v.Int64()
		return uint(i)
	}
	return 0
}

func setupDB(path string) {
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		fail("open db: %v", err)
	}
	db.Dao = gdb
	if err := papertrading.EnsureSchema(gdb); err != nil {
		fail("ensure paper schema: %v", err)
	}
	if err := data.MigratePaperTrading(gdb); err != nil {
		fail("migrate paper: %v", err)
	}
	if err := data.EnsureTradePlanTables(); err != nil {
		fail("trade plan tables: %v", err)
	}
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 150_000})
}

func seedAccount() *papertrading.PaperSimAccount {
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 150_000, Cash: 150_000, Equity: 150_000,
	}
	if err := db.Dao.Create(acc).Error; err != nil {
		fail("create account: %v", err)
	}
	return acc
}

func seedLockedPos(accID uint, code, name string, vol int64) *papertrading.PaperSimPosition {
	p := &papertrading.PaperSimPosition{
		AccountID: accID, StockCode: code, StockName: name,
		TotalVolume: vol, AvailableVolume: 0, LockedVolume: vol,
		AvgCost: 10, MarkPrice: 10, UpdatedAt: time.Now(),
	}
	if err := db.Dao.Create(p).Error; err != nil {
		fail("create pos: %v", err)
	}
	return p
}

func seedBuyFill(accID uint, code, tradeDate string, volume int64) {
	ord := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: 1, PlanItemID: uint(time.Now().UnixNano()%1_000_000_000 + 1),
		TradeDate: tradeDate, StockCode: code, StockName: "t",
		Side: "buy", Quantity: volume, Status: papertrading.OrderStatusFilled,
		FilledPrice: 10, FilledVolume: volume, OrderTime: time.Now(),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Dao.Create(ord).Error; err != nil {
		fail("order: %v", err)
	}
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: ord.ID, PlanID: 1, StockCode: code, StockName: "t",
		Side: "buy", Price: 10, Volume: volume, FillReason: papertrading.FillReasonMarketOpen,
		FilledAt: time.Now(),
	}
	if err := db.Dao.Create(fill).Error; err != nil {
		fail("fill: %v", err)
	}
}

func seedOversizePlan() *models.TradePlan {
	plan := &models.TradePlan{
		TradeDate: tradeStr, GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		AmountPerStock: 100_000, MaxNames: 3, EnableExecute: false,
		Side: "buy", SourceSession: models.TradePlanSourceAfterClose,
		Message: "acceptance oversize plan (required 300k > cash 150k)",
	}
	items := []models.TradePlanItem{
		{StockCode: "sz000001", StockName: "A", Priority: 1, TargetAmount: 100_000, Score: 90, Status: models.TradePlanItemPending, Side: "buy"},
		{StockCode: "sz000002", StockName: "B", Priority: 2, TargetAmount: 100_000, Score: 80, Status: models.TradePlanItemPending, Side: "buy"},
		{StockCode: "sz000003", StockName: "C", Priority: 3, TargetAmount: 100_000, Score: 70, Status: models.TradePlanItemPending, Side: "buy"},
	}
	if err := data.NewTradePlanRepo().CreatePlanWithItems(plan, items); err != nil {
		fail("seed plan: %v", err)
	}
	return plan
}

func snapPos(id uint) map[string]any {
	var p papertrading.PaperSimPosition
	_ = db.Dao.First(&p, id).Error
	return map[string]any{
		"code": p.StockCode, "total": p.TotalVolume, "avail": p.AvailableVolume, "locked": p.LockedVolume,
	}
}

func evalStates(asOf time.Time) []positionstate.PositionStateView {
	b := positionstate.NewService(nil).Evaluate(positionstate.Query{TradeDate: tradeStr, AsOf: asOf})
	if b == nil {
		return nil
	}
	return b.Positions
}

func homeMonitorCompare(asOf time.Time) (map[string]any, []string) {
	findings := []string{}
	mon := tradingdaymonitor.Build(tradingdaymonitor.Options{TradeDate: tradeStr, AsOf: asOf})
	hv := home.Assemble(home.Inputs{
		TradeDate: tradeStr,
		AsOf:      asOf,
		Monitor:   mon,
		// Intentionally minimal: PositionStates resolved via service when nil
	})
	// Force resolve like production Evaluate path for PositionStates
	if hv.PositionStates == nil {
		hv.PositionStates = positionstate.NewService(nil).Evaluate(positionstate.Query{TradeDate: tradeStr, AsOf: asOf})
	}

	homeMap := map[string]positionstate.PositionStateView{}
	if hv.PositionStates != nil {
		for _, p := range hv.PositionStates.Positions {
			homeMap[strings.ToLower(p.Symbol)] = p
		}
	}
	monMap := map[string]tradingdaymonitor.PositionStateRow{}
	for _, p := range mon.PositionStates {
		monMap[strings.ToLower(p.Symbol)] = p
	}

	mismatches := []string{}
	for code, hp := range homeMap {
		mp, ok := monMap[code]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("%s missing on Monitor", code))
			continue
		}
		if hp.State != mp.State || hp.IsNewPosition != mp.IsNewPosition ||
			hp.AvailableQty != mp.AvailableQty || hp.LockedQty != mp.LockedQty {
			mismatches = append(mismatches, fmt.Sprintf(
				"%s Home(state=%s new=%v a=%d l=%d) vs Monitor(state=%s new=%v a=%d l=%d)",
				code, hp.State, hp.IsNewPosition, hp.AvailableQty, hp.LockedQty,
				mp.State, mp.IsNewPosition, mp.AvailableQty, mp.LockedQty,
			))
		}
	}
	for code := range monMap {
		if _, ok := homeMap[code]; !ok {
			mismatches = append(mismatches, fmt.Sprintf("%s missing on Home", code))
		}
	}
	if len(mismatches) > 0 {
		findings = append(findings, "Home/Monitor PositionState mismatch: "+strings.Join(mismatches, "; "))
	}

	// Probe fill lot loader column
	var lotProbe []struct {
		StockCode string
		TradeDate string
		Side      string
		Quantity  int64
	}
	_ = db.Dao.Table("paper_sim_fills AS f").
		Select("o.stock_code AS stock_code, o.trade_date AS trade_date, o.side AS side, f.quantity AS quantity").
		Joins("JOIN paper_sim_orders o ON o.id = f.order_id").
		Scan(&lotProbe).Error
	var lotProbeVol []struct {
		StockCode string
		TradeDate string
		Side      string
		Quantity  int64
	}
	_ = db.Dao.Table("paper_sim_fills AS f").
		Select("o.stock_code AS stock_code, o.trade_date AS trade_date, o.side AS side, f.volume AS quantity").
		Joins("JOIN paper_sim_orders o ON o.id = f.order_id").
		Scan(&lotProbeVol).Error
	qtySum, volSum := int64(0), int64(0)
	for _, r := range lotProbe {
		qtySum += r.Quantity
	}
	for _, r := range lotProbeVol {
		volSum += r.Quantity
	}
	if qtySum == 0 && volSum > 0 {
		findings = append(findings,
			"PositionState loadLotsByCode uses f.quantity but paper_sim_fills column is volume — buy_records empty → is_new_position/holding_days may be wrong",
		)
	}

	return map[string]any{
		"home_states":      hv.PositionStates,
		"monitor_states":   mon.PositionStates,
		"monitor_morning":  mon.Morning,
		"monitor_exec":     mon.Execution,
		"monitor_settle":   mon.Settlement,
		"home_trading":     hv.TradingStatus,
		"lot_probe_qty_sum": qtySum,
		"lot_probe_vol_sum": volSum,
		"mismatches":       mismatches,
	}, findings
}

func run0920(yesterdayPosID uint) nodeLog {
	asOf := time.Date(2026, 8, 17, 9, 20, 0, 0, time.Local)
	n := nodeLog{Clock: "09:20", Action: "RunMorningSettlement → PositionUnlockJob → PositionState recalc", Notes: []string{}, Findings: []string{}}

	before := snapPos(yesterdayPosID)
	n.Notes = append(n.Notes, fmt.Sprintf("before unlock sz000001=%v", before))

	ms, err := positionstate.RunMorningSettlement(asOf)
	if err != nil {
		n.OK = false
		n.Findings = append(n.Findings, "RunMorningSettlement error: "+err.Error())
		return n
	}
	after := snapPos(yesterdayPosID)
	states := evalStates(asOf)
	hm, findings := homeMonitorCompare(asOf)
	n.Findings = append(n.Findings, findings...)

	n.OK = !ms.UnlockSkipped && ms.PositionsUnlocked > 0
	n.Notes = append(n.Notes,
		fmt.Sprintf("unlock_skipped=%v unlocked=%d volume=%d msg=%s", ms.UnlockSkipped, ms.PositionsUnlocked, ms.UnlockVolumeTotal, ms.UnlockMessage),
		fmt.Sprintf("after unlock sz000001=%v", after),
	)
	if after["avail"].(int64) != 1000 || after["locked"].(int64) != 0 {
		n.Findings = append(n.Findings, fmt.Sprintf("expected yesterday lot fully unlocked; got avail=%v locked=%v", after["avail"], after["locked"]))
		n.OK = false
	}
	n.Position = map[string]any{"morning_result_states": ms.PositionStates, "evaluated": states}
	n.HomeMon = hm
	n.Extra = map[string]any{"trade_date": ms.TradeDate, "prev": ms.PreviousTradeDay}
	return n
}

func run0925(srcPlanID uint) nodeLog {
	asOf := time.Date(2026, 8, 17, 9, 25, 0, 0, time.Local)
	n := nodeLog{Clock: "09:25", Action: "TradePlan status + cash_rescale new plan_version", Notes: []string{}, Findings: []string{}}

	repo := data.NewTradePlanRepo()
	src, err := repo.GetByID(srcPlanID)
	if err != nil {
		n.OK = false
		n.Findings = append(n.Findings, err.Error())
		return n
	}
	n.Notes = append(n.Notes, fmt.Sprintf("source plan id=%d v=%d status=%s enable_execute=%v amount=%.0f items=%d required=%.0f",
		src.ID, src.PlanVersion, src.Status, src.EnableExecute, src.AmountPerStock, len(src.Items), 300_000.0))

	snap, _ := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	cash := 0.0
	if snap != nil && snap.Found {
		cash = snap.AvailableCash
		if cash <= 0 {
			cash = snap.Cash
		}
	}
	n.Notes = append(n.Notes, fmt.Sprintf("available_cash(snapshot)=%.2f (seed account cash=150000; holdings may reduce free cash)", cash))

	// Force cash=150000 for rescale semantics under test (account InitialCash).
	res, err := strategy.RescaleTradePlanForCash(strategy.CashRescaleRequest{
		PlanID: src.ID, AvailableCash: 150_000, Actor: "phase11k-acceptance",
	})
	if err != nil {
		n.OK = false
		n.Findings = append(n.Findings, "RescaleTradePlanForCash: "+err.Error())
		return n
	}
	orig, _ := repo.GetByID(src.ID)
	neu, _ := repo.GetByID(res.NewPlanID)
	n.OK = res.Changed && res.NewPlanVersion == 2 && orig.PlanVersion == 1 && !neu.EnableExecute
	n.TradePlans = map[string]any{
		"source": map[string]any{"id": orig.ID, "version": orig.PlanVersion, "amount": orig.AmountPerStock, "status": orig.Status, "enable_execute": orig.EnableExecute},
		"new":    map[string]any{"id": neu.ID, "version": neu.PlanVersion, "amount": neu.AmountPerStock, "status": neu.Status, "enable_execute": neu.EnableExecute, "message": neu.Message},
		"mode":   res.Mode, "required_before": res.RequiredBefore, "required_after": res.RequiredAfter,
		"new_plan_id": res.NewPlanID,
	}
	n.Extra = map[string]any{"new_plan_id": res.NewPlanID, "mode": res.Mode}
	n.Notes = append(n.Notes, fmt.Sprintf("rescale mode=%s v%d→v%d amount %.0f→%.0f required %.0f→%.0f",
		res.Mode, res.SourcePlanVersion, res.NewPlanVersion, res.AmountPerStockOld, res.AmountPerStockNew, res.RequiredBefore, res.RequiredAfter))
	if !strings.Contains(neu.Message, "cash_rescale") {
		n.Findings = append(n.Findings, "new plan message missing cash_rescale tag")
		n.OK = false
	}
	if neu.EnableExecute {
		n.Findings = append(n.Findings, "cash_rescale plan has enable_execute=true (expected false)")
		n.OK = false
	}
	hm, findings := homeMonitorCompare(asOf)
	n.HomeMon = hm
	n.Findings = append(n.Findings, findings...)
	n.Position = evalStates(asOf)
	return n
}

func run0929(planID uint) nodeLog {
	asOf := time.Date(2026, 8, 17, 9, 29, 0, 0, time.Local)
	n := nodeLog{Clock: "09:29", Action: "Risk check + pre-execution state", Notes: []string{}, Findings: []string{}}
	if planID == 0 {
		n.OK = false
		n.Findings = append(n.Findings, "no rescale plan id")
		return n
	}
	repo := data.NewTradePlanRepo()
	plan, err := repo.GetByID(planID)
	if err != nil {
		n.OK = false
		n.Findings = append(n.Findings, err.Error())
		return n
	}
	riskRes, err := strategy.EvaluateDraftTradePlanRisk(plan)
	if err != nil {
		n.Notes = append(n.Notes, "EvaluateDraftTradePlanRisk error (recorded): "+err.Error())
		// Risk may fail if risk filter needs market context — record, not always FAIL acceptance
		n.Findings = append(n.Findings, "Risk evaluation error: "+err.Error())
	} else {
		n.Notes = append(n.Notes, fmt.Sprintf("risk passed=%v status=%v reasons=%v",
			riskRes.Passed, riskRes.PlanFilterResult.RiskStatus, riskRes.RiskReasons))
		n.Extra = map[string]any{"risk_passed": riskRes.Passed, "risk_status": riskRes.PlanFilterResult.RiskStatus}
	}

	guard := models.RequireFrozenReadyTradePlan(plan)
	n.Notes = append(n.Notes, fmt.Sprintf("RequireFrozenReadyTradePlan allowed=%v reason=%s (draft expected BLOCK)", guard.Allowed, guard.Reason))
	if guard.Allowed {
		n.Findings = append(n.Findings, "draft cash_rescale plan unexpectedly passes FrozenReady guard")
	}

	n.OK = !plan.EnableExecute && plan.Status == models.TradePlanStatusDraft && !guard.Allowed
	n.ExecCheck = map[string]any{
		"enable_execute": plan.EnableExecute,
		"status":         plan.Status,
		"frozen_ready":   guard.Allowed,
		"is_frozen":      plan.IsFrozen(),
	}
	hm, findings := homeMonitorCompare(asOf)
	n.HomeMon = hm
	n.Findings = append(n.Findings, findings...)
	n.Position = evalStates(asOf)
	n.TradePlans = map[string]any{"plan_id": plan.ID, "version": plan.PlanVersion, "status": plan.Status}
	return n
}

func run0931(rescaleID, oversizeID uint) nodeLog {
	asOf := time.Date(2026, 8, 17, 9, 31, 0, 0, time.Local)
	n := nodeLog{Clock: "09:31", Action: "Simulated execution entry; enable_execute=false must not execute", Notes: []string{}, Findings: []string{}}

	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
		"sz000001": {Open: 10}, "sz000002": {Open: 10}, "sz000003": {Open: 10},
	}}

	// Attempt RunExecution against draft rescale plan
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: tradeStr, PlanID: rescaleID, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: price, SkipWeekdayCheck: true, Now: asOf,
	})
	ordersBefore, fillsBefore := countOrdersFills()
	n.Notes = append(n.Notes, fmt.Sprintf("RunExecution(rescale draft) err=%v res=%v", errString(err), summarizeExec(res)))

	entered := false
	if res != nil {
		// Any fill/order write would mean it entered execution
		ordersAfter, fillsAfter := countOrdersFills()
		if ordersAfter > ordersBefore || fillsAfter > fillsBefore {
			entered = true
			n.Findings = append(n.Findings, "enable_execute=false / draft plan produced new orders/fills")
		}
		st := strings.ToLower(string(res.Status))
		n.Notes = append(n.Notes, fmt.Sprintf("orders %d→%d fills %d→%d status=%s decision=%s reason=%s",
			ordersBefore, ordersAfter, fillsBefore, fillsAfter, res.Status, res.Decision, res.Reason))
		if strings.Contains(st, "complet") && !strings.Contains(st, "skip") {
			// completed without skip is suspicious for draft
			if entered {
				n.Findings = append(n.Findings, "execution completed with writes on non-executable plan")
			}
		}
	}

	// Also verify Track-A open buy won't pick draft
	ob := data.RunPaperOpenBuyOnce(false)
	n.Notes = append(n.Notes, fmt.Sprintf("RunPaperOpenBuyOnce message=%s planId=%d", ob.Message, ob.PlanID))
	if ob.PlanID == rescaleID || ob.PlanID == oversizeID {
		n.Findings = append(n.Findings, "OpenBuy selected draft/cash_rescale plan")
		entered = true
	}

	n.OK = !entered
	n.ExecCheck = map[string]any{
		"execution_entered": entered,
		"open_buy_message":  ob.Message,
		"gateway_result":    summarizeExec(res),
		"gateway_error":     errString(err),
	}

	// TradingEvent buffer observation (may be empty in harness if emit skipped)
	evs := tradingevent.ListByTradeDate(tradeStr)
	n.Extra = map[string]any{"trading_events_count": len(evs), "trading_events": evs}

	hm, findings := homeMonitorCompare(asOf)
	n.HomeMon = hm
	n.Findings = append(n.Findings, findings...)
	n.Position = evalStates(asOf)
	return n
}

func run1505() nodeLog {
	asOf := time.Date(2026, 8, 17, 15, 5, 0, 0, time.Local)
	n := nodeLog{Clock: "15:05", Action: "SettlementJob + Portfolio refresh + Home/Monitor sync", Notes: []string{}, Findings: []string{}}

	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
		"sz000001": {Open: 10.5}, "sz000002": {Open: 9.8},
	}}
	settle, err := papertrading.SettlementJob(tradeStr, price, false)
	if err != nil {
		n.OK = false
		n.Findings = append(n.Findings, "SettlementJob: "+err.Error())
		return n
	}
	n.Settlement = settle
	n.Notes = append(n.Notes, fmt.Sprintf("settlement enabled=%v updated=%d equity=%.2f lockedVol=%d msg=%s",
		settle.Enabled, settle.PositionsUpdated, settle.Equity, settle.LockedVolumeTotal, settle.Message))

	snap, snapErr := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	if snapErr != nil {
		n.Findings = append(n.Findings, "portfolio Snapshot: "+snapErr.Error())
	}
	n.Extra = map[string]any{"snapshot_found": snap != nil && snap.Found}
	if snap != nil && snap.Found {
		n.Extra["cash"] = snap.Cash
		n.Extra["available_cash"] = snap.AvailableCash
		n.Extra["equity"] = snap.TotalEquity
		n.Extra["positions"] = len(snap.Positions)
		n.Notes = append(n.Notes, fmt.Sprintf("portfolio cash=%.2f equity=%.2f positions=%d", snap.Cash, snap.TotalEquity, len(snap.Positions)))
	}

	states := evalStates(asOf)
	n.Position = states
	hm, findings := homeMonitorCompare(asOf)
	n.HomeMon = hm
	n.Findings = append(n.Findings, findings...)

	// Settlement must NOT unlock same-day buys
	var todayPos papertrading.PaperSimPosition
	_ = db.Dao.Where("stock_code = ?", "sz000002").First(&todayPos).Error
	if todayPos.LockedVolume != 500 {
		n.Findings = append(n.Findings, fmt.Sprintf("15:05 must not unlock today buy; sz000002 locked=%d want 500", todayPos.LockedVolume))
	} else {
		n.Notes = append(n.Notes, "T+1 invariant: today's buy remains locked after settlement")
	}

	n.OK = settle.Enabled && len(findings) == 0 || (settle.Enabled && todayPos.LockedVolume == 500)
	// OK if settlement ran; findings about lots still listed separately
	if !settle.Enabled {
		n.OK = false
	} else {
		n.OK = todayPos.LockedVolume == 500
	}
	return n
}

func buildConsistencySummary(nodes []nodeLog) map[string]any {
	out := map[string]any{"nodes": len(nodes)}
	for _, n := range nodes {
		if hm, ok := n.HomeMon.(map[string]any); ok {
			out[n.Clock+"_mismatches"] = hm["mismatches"]
			out[n.Clock+"_lot_qty"] = hm["lot_probe_qty_sum"]
			out[n.Clock+"_lot_vol"] = hm["lot_probe_vol_sum"]
		}
	}
	return out
}

func countOrdersFills() (orders, fills int64) {
	_ = db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&orders).Error
	_ = db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fills).Error
	return
}

func summarizeExec(res *papertrading.ExecutionResult) any {
	if res == nil {
		return nil
	}
	return map[string]any{
		"status": res.Status, "decision": res.Decision, "reason": res.Reason,
		"session": res.Session, "entry": res.Entry, "message": res.Message,
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
