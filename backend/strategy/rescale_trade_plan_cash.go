package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
)

const (
	// minRescaleAmountPerStock is the floor for a single buy line after cash rescale (CNY).
	// Below this we drop names rather than emit an unexecutable tiny plan line.
	minRescaleAmountPerStock = 1_000.0

	cashRescaleSourceHint = "cash_rescale"
)

// CashRescaleRequest selects the source plan and optional cash override (tests).
type CashRescaleRequest struct {
	PlanID        uint
	AvailableCash float64 // if >0, skip Snapshot and use this
	Actor         string
	MinPerStock   float64 // optional; default minRescaleAmountPerStock
}

// CashRescaleResult is the outcome of RescaleTradePlanForCash.
type CashRescaleResult struct {
	Changed           bool
	Mode              string // none | scale_down | trim_names
	SourcePlanID      uint
	SourcePlanVersion int
	NewPlanID         uint
	NewPlanVersion    int
	AvailableCash     float64
	RequiredBefore    float64
	RequiredAfter     float64
	NamesBefore       int
	NamesAfter        int
	AmountPerStockOld float64
	AmountPerStockNew float64
	Message           string
	NewPlan           *models.TradePlan
	SourcePlan        *models.TradePlan
}

// AllocateCashToPlanItems is a pure allocator: does not touch DB / selection / scores.
// pending items should already be buy-intent rows with TargetAmount set.
// Returns new per-stock amount, kept indices into pending (priority order), and mode.
func AllocateCashToPlanItems(pending []models.TradePlanItem, availableCash, minPerStock float64) (
	amountPerStock float64, keepIdx []int, mode string, err error,
) {
	if minPerStock <= 0 {
		minPerStock = minRescaleAmountPerStock
	}
	if availableCash < 0 {
		availableCash = 0
	}
	n := len(pending)
	if n == 0 {
		return 0, nil, "", fmt.Errorf("no pending buy items to allocate")
	}

	required := 0.0
	for i := range pending {
		a := pending[i].TargetAmount
		if a <= 0 {
			a = 0
		}
		required += a
	}
	if required <= availableCash+1e-6 {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		// Uniform amount for header: max of kept target amounts (or average).
		amt := pending[0].TargetAmount
		for i := 1; i < n; i++ {
			if pending[i].TargetAmount > amt {
				amt = pending[i].TargetAmount
			}
		}
		return amt, idx, "none", nil
	}

	if availableCash < minPerStock {
		return 0, nil, "", fmt.Errorf(
			"available_cash=%.2f below min_per_stock=%.2f; cannot create executable plan",
			availableCash, minPerStock,
		)
	}

	// Try keep all names with equal scaled amount.
	for k := n; k >= 1; k-- {
		raw := availableCash / float64(k)
		amt := math.Floor(raw*100) / 100 // fen precision
		if amt < minPerStock {
			continue
		}
		// Ensure k * amt <= cash (floor may leave remainder).
		if amt*float64(k) > availableCash+1e-6 {
			amt = math.Floor((availableCash/float64(k))*100) / 100
		}
		if amt < minPerStock {
			continue
		}
		idx := make([]int, k)
		for i := 0; i < k; i++ {
			idx[i] = i // pending already sorted by priority
		}
		mode = "scale_down"
		if k < n {
			mode = "trim_names"
		}
		return amt, idx, mode, nil
	}

	return 0, nil, "", fmt.Errorf(
		"cannot fit any executable names into available_cash=%.2f (min_per_stock=%.2f, names=%d)",
		availableCash, minPerStock, n,
	)
}

// RescaleTradePlanForCash clones the source plan into a new plan_version with amounts
// fitting available_cash. Does not re-run stock selection or scoring. Original plan retained.
func RescaleTradePlanForCash(req CashRescaleRequest) (*CashRescaleResult, error) {
	if req.PlanID == 0 {
		return nil, fmt.Errorf("plan_id is required")
	}
	repo := data.NewTradePlanRepo()
	src, err := repo.GetByID(req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("load plan: %w", err)
	}
	if src == nil {
		return nil, fmt.Errorf("plan %d not found", req.PlanID)
	}
	switch src.Status {
	case models.TradePlanStatusDraft, models.TradePlanStatusReady:
		// ok
	default:
		return nil, fmt.Errorf("plan %d status=%s cannot rescale (want draft|ready)", src.ID, src.Status)
	}
	if src.IsFrozen() {
		return nil, fmt.Errorf("plan %d is frozen; refuse cash rescale", src.ID)
	}
	if strings.TrimSpace(src.DecisionProvider) == models.TradePlanDecisionProviderPortfolio {
		return nil, fmt.Errorf("plan %d decision_provider=%s; refuse cash rescale (must re-run Alloc)", src.ID, src.DecisionProvider)
	}

	cash := req.AvailableCash
	if cash <= 0 {
		snap, snapErr := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{AsOf: time.Now()})
		if snapErr != nil {
			return nil, fmt.Errorf("load portfolio cash: %w", snapErr)
		}
		if snap == nil || !snap.Found {
			return nil, fmt.Errorf("paper portfolio not found; cannot resolve available_cash")
		}
		cash = snap.AvailableCash
		if cash <= 0 {
			cash = snap.Cash
		}
	}

	pending := collectPendingBuyItems(src.Items)
	sortPendingByPriority(pending)

	minPS := req.MinPerStock
	if minPS <= 0 {
		minPS = minRescaleAmountPerStock
	}

	requiredBefore := 0.0
	for _, it := range pending {
		requiredBefore += it.TargetAmount
	}

	out := &CashRescaleResult{
		SourcePlanID:      src.ID,
		SourcePlanVersion: src.PlanVersion,
		AvailableCash:     cash,
		RequiredBefore:    requiredBefore,
		NamesBefore:       len(pending),
		AmountPerStockOld: src.AmountPerStock,
		SourcePlan:        src,
	}

	amt, keepIdx, mode, allocErr := AllocateCashToPlanItems(pending, cash, minPS)
	if allocErr != nil {
		return out, allocErr
	}
	out.Mode = mode
	out.AmountPerStockNew = amt
	out.NamesAfter = len(keepIdx)

	requiredAfter := amt * float64(len(keepIdx))
	out.RequiredAfter = requiredAfter

	if mode == "none" {
		out.Changed = false
		out.Message = fmt.Sprintf(
			"cash sufficient: available=%.2f required=%.2f; no new plan_version",
			cash, requiredBefore,
		)
		logger.SugaredLogger.Infof("RescaleTradePlanForCash plan_id=%d %s", src.ID, out.Message)
		return out, nil
	}

	if requiredAfter > cash+1e-6 {
		return out, fmt.Errorf("internal allocator error: required_after=%.2f > cash=%.2f", requiredAfter, cash)
	}

	version, err := repo.NextPlanVersion(src.TradeDate)
	if err != nil {
		return out, err
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "system"
	}
	scaleRatio := 0.0
	if requiredBefore > 0 {
		scaleRatio = requiredAfter / requiredBefore
	}
	msg := fmt.Sprintf(
		"%s from_plan_id=%d v%d available_cash=%.2f required_before=%.2f required_after=%.2f mode=%s names=%d→%d amount=%.2f→%.2f scale_ratio=%.6f actor=%s",
		cashRescaleSourceHint, src.ID, src.PlanVersion, cash, requiredBefore, requiredAfter,
		mode, len(pending), len(keepIdx), src.AmountPerStock, amt, scaleRatio, actor,
	)

	newPlan := &models.TradePlan{
		TradeDate:          src.TradeDate,
		PoolID:             src.PoolID,
		GeneratedAt:        time.Now(),
		Status:             models.TradePlanStatusDraft,
		Side:               src.Side,
		AmountPerStock:     amt,
		MaxNames:           len(keepIdx),
		EnableExecute:      false,
		Message:            msg,
		PlanVersion:        version,
		FreezeAt:           nil,
		SourceSession:      models.TradePlanSourceCashRescale,
		SourceKind:         models.TradePlanSourceCashRescale,
		ParentPlanID:       src.ID,
		RescaleMode:        mode,
		AvailableCashUsed:  cash,
		RequiredCashBefore: requiredBefore,
		RequiredCashAfter:  requiredAfter,
		ScaleRatio:         scaleRatio,
		// G.13: copy source provider metadata (Legacy/empty only; portfolio rejected above).
		ProviderMode:      src.ProviderMode,
		DecisionProvider:  src.DecisionProvider,
		DecisionVersion:   src.DecisionVersion,
		AllocationVersion: src.AllocationVersion,
		RiskStatus:        src.RiskStatus,
		MarketLevel:       src.MarketLevel,
		RiskFilteredCount: src.RiskFilteredCount,
		RiskAcceptedCount: len(keepIdx),
		RiskSummary:       src.RiskSummary,
		RiskSnapshotJSON:  src.RiskSnapshotJSON,
		CheckedAt:         nil,
	}
	if newPlan.Side == "" {
		newPlan.Side = "buy"
	}

	newItems := make([]models.TradePlanItem, 0, len(keepIdx))
	for _, idx := range keepIdx {
		srcItem := pending[idx]
		newItems = append(newItems, models.TradePlanItem{
			TradeDate:       src.TradeDate,
			StockCode:       srcItem.StockCode,
			StockName:       srcItem.StockName,
			Side:            firstNonEmptySide(srcItem.Side, newPlan.Side),
			Priority:        srcItem.Priority,
			TargetAmount:    amt,
			TargetVolume:    0, // clear morning materialize residue
			LimitPrice:      0,
			Score:           srcItem.Score,
			Reason:          srcItem.Reason,
			StrategyName:    srcItem.StrategyName,
			StrategyVersion: srcItem.StrategyVersion,
			Status:          models.TradePlanItemPending,
			RiskCode:        srcItem.RiskCode,
			RiskMessage:     srcItem.RiskMessage,
			// clear execution / pricing
			RefPrice:     0,
			RefSource:    "",
			EntryRule:    "",
			IntentStatus: "",
		})
	}

	// Append skipped audit rows for dropped names (same codes/scores; status skipped).
	kept := map[string]bool{}
	for _, idx := range keepIdx {
		kept[pending[idx].StockCode] = true
	}
	for _, srcItem := range pending {
		if kept[srcItem.StockCode] {
			continue
		}
		newItems = append(newItems, models.TradePlanItem{
			TradeDate:            src.TradeDate,
			StockCode:            srcItem.StockCode,
			StockName:            srcItem.StockName,
			Side:                 firstNonEmptySide(srcItem.Side, newPlan.Side),
			Priority:             srcItem.Priority,
			TargetAmount:         0,
			OriginalTargetAmount: srcItem.TargetAmount,
			Score:                srcItem.Score,
			Reason:               srcItem.Reason,
			StrategyName:         srcItem.StrategyName,
			StrategyVersion:      srcItem.StrategyVersion,
			Status:               models.TradePlanItemSkipped,
			RiskCode:             "CASH_RESCALE_TRIMMED",
			RiskMessage:          fmt.Sprintf("trimmed by cash_rescale; available_cash=%.2f original_target_amount=%.2f", cash, srcItem.TargetAmount),
		})
	}
	// Also copy originally skipped items for audit continuity (unchanged).
	for _, it := range src.Items {
		if it.Status == models.TradePlanItemPending || it.Side == "sell" {
			continue
		}
		if it.Status == models.TradePlanItemSkipped {
			newItems = append(newItems, models.TradePlanItem{
				TradeDate:       src.TradeDate,
				StockCode:       it.StockCode,
				StockName:       it.StockName,
				Side:            firstNonEmptySide(it.Side, newPlan.Side),
				Priority:        it.Priority,
				TargetAmount:    it.TargetAmount,
				Score:           it.Score,
				Reason:          it.Reason,
				StrategyName:    it.StrategyName,
				StrategyVersion: it.StrategyVersion,
				Status:          models.TradePlanItemSkipped,
				RiskCode:        it.RiskCode,
				RiskMessage:     it.RiskMessage,
			})
		}
	}

	if err := repo.CreatePlanWithItems(newPlan, newItems); err != nil {
		return out, err
	}

	out.Changed = true
	out.NewPlanID = newPlan.ID
	out.NewPlanVersion = newPlan.PlanVersion
	out.NewPlan = newPlan
	out.Message = msg
	logger.SugaredLogger.Infof("RescaleTradePlanForCash ok %s new_plan_id=%d", msg, newPlan.ID)
	return out, nil
}

func collectPendingBuyItems(items []models.TradePlanItem) []models.TradePlanItem {
	out := make([]models.TradePlanItem, 0, len(items))
	for _, it := range items {
		side := strings.ToLower(strings.TrimSpace(it.Side))
		if side != "" && side != "buy" {
			continue
		}
		if it.Status != "" && it.Status != models.TradePlanItemPending {
			continue
		}
		if it.TargetAmount <= 0 && it.StockCode == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

func sortPendingByPriority(items []models.TradePlanItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority // lower priority number first
		}
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].StockCode < items[j].StockCode
	})
}

func firstNonEmptySide(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
