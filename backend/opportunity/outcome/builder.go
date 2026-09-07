package outcome

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/opportunity/projection"
)

func buildOutcomeFromLeg(leg matchedLeg, ctx *fillContext, asOf time.Time) OutcomeProjection {
	code := normalizeCode(leg.BuyFill.StockCode)
	buyItem := ctx.items[leg.BuyFill.PlanItemID]
	buyPlan := ctx.plans[leg.BuyFill.PlanID]
	poolItem, inPool := ctx.poolItems[code]

	out := OutcomeProjection{
		StockCode:     code,
		StockName:     strings.TrimSpace(leg.BuyFill.StockName),
		OutcomeStatus: leg.Status,
		Metadata: MetadataBlock{
			SourceType: SourceTypeRoundTripFIFO,
			FIFOPolicy: FIFOPolicyAccountV1,
			AsOf:       asOf,
			Missing:    []string{},
		},
		Entry: EntryBlock{
			Present:       true,
			BuyFillID:     leg.BuyFill.ID,
			BuyPlanID:     leg.BuyFill.PlanID,
			BuyPlanItemID: leg.BuyFill.PlanItemID,
			EntryPrice:    leg.BuyFill.Price,
			EntryQty:      leg.Qty,
			EntryFee:      prorateFee(leg.BuyFill.Fee, leg.BuyFill.Volume, leg.Qty),
			EntryDate:     ctx.entryDate(leg.BuyFill),
		},
		Exit: ExitBlock{Present: false},
	}

	if out.StockName == "" {
		out.StockName = strings.TrimSpace(buyItem.StockName)
	}

	out.Signal, out.Opportunity, out.Decision = buildAnchorBlocks(ctx, code, buyPlan, &buyItem, poolItem, inPool, leg.BuyFill.PlanItemID)

	entryDate := out.Entry.EntryDate
	if leg.Status == OutcomeStatusClosed && leg.SellFill != nil {
		sell := *leg.SellFill
		sellItem := ctx.items[sell.PlanItemID]
		sellPlan := ctx.plans[sell.PlanID]
		out.Exit = ExitBlock{
			Present:        true,
			SellFillID:     sell.ID,
			SellPlanID:     sell.PlanID,
			SellPlanItemID: sell.PlanItemID,
			ExitPrice:      sell.Price,
			ExitQty:        leg.Qty,
			ExitFee:        prorateFee(sell.Fee, sell.Volume, leg.Qty),
			ExitDate:       ctx.exitDate(sell),
			ExitChannel:    exitChannel(sellPlan),
			ExitReasonText: strings.TrimSpace(sellItem.Reason),
		}
		exitPrice := sell.Price
		out.Performance = computePerformance(
			leg.BuyFill.Price, exitPrice,
			out.Entry.EntryFee, out.Exit.ExitFee,
			leg.Qty, entryDate, out.Exit.ExitDate, asOf, true,
		)
		out.OutcomeID = fmt.Sprintf("out:%s:rt:%d:%d", code, leg.BuyFill.ID, sell.ID)
	} else {
		out.Performance = computePerformance(
			leg.BuyFill.Price, 0, out.Entry.EntryFee, 0,
			leg.Qty, entryDate, "", asOf, false,
		)
		out.OutcomeID = fmt.Sprintf("out:%s:fill:%d", code, leg.BuyFill.ID)
	}

	tradeDate := entryDate
	if buyPlan != nil && strings.TrimSpace(buyPlan.TradeDate) != "" {
		tradeDate = strings.TrimSpace(buyPlan.TradeDate)
	}
	out.OpportunityID = buildOpportunityID(tradeDate, code, poolItem, inPool)
	out.Metadata.Quality = assessOutcomeQuality(out)
	return out
}

func buildNoTradeOutcome(code string, tradeDate string, asOf time.Time) (OutcomeProjection, error) {
	p, err := projection.ProjectOne(code, projection.ProjectOptions{TradeDate: tradeDate})
	if err != nil {
		return OutcomeProjection{}, err
	}
	out := OutcomeProjection{
		OutcomeID:     fmt.Sprintf("out:%s:noop:%s", normalizeCode(code), tradeDate),
		OpportunityID: p.OpportunityID,
		StockCode:     p.StockCode,
		StockName:     p.StockName,
		OutcomeStatus: OutcomeStatusNoTrade,
		Signal:        p.Signal,
		Opportunity:   p.Opportunity,
		Decision:      p.Decision,
		Entry:         EntryBlock{Present: false},
		Exit:          ExitBlock{Present: false},
		Performance:   PerformanceBlock{},
		Metadata: MetadataBlock{
			SourceType: SourceTypeNoTrade,
			Quality:    p.Metadata.Quality,
			Missing:    append([]string{}, p.Metadata.Missing...),
			AsOf:       asOf,
		},
	}
	if !p.Signal.Present && !p.Opportunity.Present {
		out.Metadata.Quality = QualityPartial
	}
	return out, nil
}

func buildAnchorBlocks(
	ctx *fillContext,
	code string,
	plan *models.TradePlan,
	item *models.TradePlanItem,
	poolItem models.CandidatePoolItem,
	inPool bool,
	planItemID uint,
) (projection.SignalBlock, projection.OpportunityBlock, projection.DecisionBlock) {
	var (
		sig  projection.SignalBlock
		opp  projection.OpportunityBlock
		dec  projection.DecisionBlock
	)
	if inPool {
		pool := ctx.pools[poolItem.PoolID]
		opp = opportunityFromPoolItem(pool, poolItem)
		sig = signalFromPoolItem(ctx, poolItem, code)
	}
	if item != nil && item.ID == planItemID {
		dec = decisionFromPlanItem(plan, item, inPool)
		if strings.TrimSpace(item.StrategyName) != "" && !opp.Present {
			opp = projection.OpportunityBlock{
				Present:      true,
				StrategyName: strings.TrimSpace(item.StrategyName),
				Score:        item.Score,
			}
		}
	} else if inPool {
		dec = projection.DecisionBlock{
			DecisionStatus:  projection.DecisionStatusWatch,
			CandidateStatus: projection.CandidateStatusInPool,
		}
	} else {
		dec = projection.DecisionBlock{
			DecisionStatus:  projection.DecisionStatusUnknown,
			CandidateStatus: projection.CandidateStatusNotInPool,
		}
	}
	if plan != nil && item != nil {
		dec.PlanID = plan.ID
		dec.PlanItemID = item.ID
		dec.ItemStatus = item.Status
		dec.RiskCode = item.RiskCode
		dec.TargetAmount = item.TargetAmount
		dec.DecisionProvider = plan.DecisionProvider
	}
	return sig, opp, dec
}

func opportunityFromPoolItem(pool *models.CandidatePool, item models.CandidatePoolItem) projection.OpportunityBlock {
	src := strings.TrimSpace(item.StrategyName)
	if v := strings.TrimSpace(item.StrategyVersion); v != "" {
		if src != "" {
			src = src + "@" + v
		} else {
			src = v
		}
	}
	poolSource := ""
	if pool != nil {
		poolSource = strings.TrimSpace(pool.Source)
	}
	return projection.OpportunityBlock{
		Present:        true,
		PoolID:         item.PoolID,
		Score:          item.Score,
		Rank:           item.Rank,
		StrategySource: src,
		StrategyName:   strings.TrimSpace(item.StrategyName),
		PoolSource:     poolSource,
	}
}

func signalFromPoolItem(ctx *fillContext, item models.CandidatePoolItem, code string) projection.SignalBlock {
	if item.SignalSnapshotID > 0 {
		hit := loadHitForSnapshot(item.SignalSnapshotID, code)
		if hit != nil {
			block := signalFromHit(hit)
			if snap := loadSnapshot(item.SignalSnapshotID); snap != nil {
				block.SnapshotID = snap.ID
				block.Session = strings.TrimSpace(snap.Session)
				block.StrategyID = strings.TrimSpace(snap.StrategyID)
			}
			return block
		}
	}
	if strings.TrimSpace(item.SignalTag) != "" {
		return projection.SignalBlock{
			Present:   true,
			SignalTag: strings.TrimSpace(item.SignalTag),
		}
	}
	return projection.SignalBlock{}
}

func signalFromHit(hit *models.SignalScanHit) projection.SignalBlock {
	if hit == nil {
		return projection.SignalBlock{}
	}
	return projection.SignalBlock{
		Present:       true,
		SignalTag:     strings.TrimSpace(hit.Tag),
		SignalPrice:   hit.SignalPrice,
		SignalTime:    strings.TrimSpace(hit.SignalTime),
		SignalStatus:  strings.TrimSpace(hit.SignalPriceStatus),
		TriggerReason: strings.TrimSpace(hit.StatusText),
		SchemaVersion: strings.TrimSpace(hit.SchemaVersion),
	}
}

func loadHitForSnapshot(snapshotID uint, code string) *models.SignalScanHit {
	repo := data.NewSignalSnapshotRepo()
	snap := loadSnapshot(snapshotID)
	if snap == nil {
		return nil
	}
	hits := repo.ParseSnapshotHits(snap)
	for i := range hits {
		hitCode := opportunity.SecucodeToStockCode(hits[i].SECUCODE)
		if hitCode == code {
			return &hits[i]
		}
	}
	return nil
}

func loadSnapshot(id uint) *models.SignalScanSnapshot {
	if db.Dao == nil || id == 0 {
		return nil
	}
	var snap models.SignalScanSnapshot
	if err := db.Dao.First(&snap, id).Error; err != nil {
		return nil
	}
	return &snap
}

func decisionFromPlanItem(plan *models.TradePlan, item *models.TradePlanItem, inPool bool) projection.DecisionBlock {
	status := mapDecisionStatus(item, inPool)
	candidate := mapCandidateStatus(inPool, item)
	return projection.DecisionBlock{
		DecisionStatus:  status,
		CandidateStatus: candidate,
		PlanID:          plan.ID,
		PlanItemID:      item.ID,
		ItemStatus:      item.Status,
		RiskCode:        item.RiskCode,
		TargetAmount:    item.TargetAmount,
		DecisionProvider: plan.DecisionProvider,
	}
}

func mapDecisionStatus(item *models.TradePlanItem, inPool bool) string {
	if item == nil {
		if inPool {
			return projection.DecisionStatusWatch
		}
		return projection.DecisionStatusUnknown
	}
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case models.TradePlanItemPending, models.TradePlanItemFilled:
		return projection.DecisionStatusBuyCandidate
	case models.TradePlanItemSkipped:
		if inPool {
			return projection.DecisionStatusWatch
		}
		return projection.DecisionStatusNotInPlan
	default:
		if inPool {
			return projection.DecisionStatusWatch
		}
		return projection.DecisionStatusUnknown
	}
}

func mapCandidateStatus(inPool bool, item *models.TradePlanItem) string {
	if item == nil {
		if inPool {
			return projection.CandidateStatusInPool
		}
		return projection.CandidateStatusNotInPool
	}
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case models.TradePlanItemPending:
		return projection.CandidateStatusPlanPending
	case models.TradePlanItemFilled:
		return projection.CandidateStatusPlanFilled
	case models.TradePlanItemSkipped:
		return projection.CandidateStatusPlanSkipped
	default:
		if inPool {
			return projection.CandidateStatusInPool
		}
		return projection.CandidateStatusNotInPool
	}
}

func exitChannel(plan *models.TradePlan) string {
	if plan == nil {
		return ""
	}
	switch strings.TrimSpace(plan.SourceSession) {
	case models.TradePlanSourceExitReview:
		return models.TradePlanSourceExitReview
	case models.TradePlanSourceTSell:
		return models.TradePlanSourceTSell
	default:
		return strings.TrimSpace(plan.SourceSession)
	}
}

func prorateFee(totalFee float64, totalVol, matchVol int64) float64 {
	if totalVol <= 0 || matchVol <= 0 || totalFee == 0 {
		return 0
	}
	return totalFee * float64(matchVol) / float64(totalVol)
}

func buildOpportunityID(tradeDate, code string, poolItem models.CandidatePoolItem, inPool bool) string {
	if inPool && poolItem.PoolID > 0 {
		return fmt.Sprintf("opp:%s:%s:pool:%d", code, tradeDate, poolItem.PoolID)
	}
	return fmt.Sprintf("opp:%s:%s", code, tradeDate)
}

func assessOutcomeQuality(o OutcomeProjection) string {
	missing := 0
	if !o.Signal.Present {
		missing++
	}
	if !o.Opportunity.Present {
		missing++
	}
	if !o.Entry.Present {
		missing++
	}
	if o.OutcomeStatus == OutcomeStatusClosed && !o.Exit.Present {
		missing++
	}
	if missing == 0 {
		return QualityComplete
	}
	return QualityPartial
}
