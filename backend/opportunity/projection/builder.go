package projection

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/portfolio"
	"go-stock/backend/research"
)

type loadedContext struct {
	tradeDate string
	asOf      time.Time

	pool      *models.CandidatePool
	poolByCode map[string]models.CandidatePoolItem

	plan      *models.TradePlan
	planByCode map[string]models.TradePlanItem

	snapshotByID map[uint]*models.SignalScanSnapshot
	hitsBySnap   map[uint][]models.SignalScanHit

	holdings map[string]portfolio.Position

	researchByCode map[string]research.Candidate
}

func (c *loadedContext) poolItem(code string) (models.CandidatePoolItem, bool) {
	if c == nil || c.poolByCode == nil {
		return models.CandidatePoolItem{}, false
	}
	it, ok := c.poolByCode[code]
	return it, ok
}

func (c *loadedContext) planItem(code string) (*models.TradePlan, *models.TradePlanItem, bool) {
	if c == nil || c.plan == nil {
		return nil, nil, false
	}
	it, ok := c.planByCode[code]
	if !ok {
		return c.plan, nil, false
	}
	return c.plan, &it, true
}

func (c *loadedContext) hitForPoolItem(item models.CandidatePoolItem, code string) *models.SignalScanHit {
	repo := data.NewSignalSnapshotRepo()
	if item.SignalSnapshotID > 0 {
		if snap := snapshotForID(c, item.SignalSnapshotID); snap != nil {
			c.snapshotByID[item.SignalSnapshotID] = snap
		}
		hits, ok := c.hitsBySnap[item.SignalSnapshotID]
		if !ok {
			hits = loadSnapshotHits(item.SignalSnapshotID, repo)
			c.hitsBySnap[item.SignalSnapshotID] = hits
		}
		if h := findHitForStock(hits, code); h != nil {
			return h
		}
	}
	return nil
}

func (c *loadedContext) hitForResearch(code string, tradeDate string) *models.SignalScanHit {
	repo := data.NewSignalSnapshotRepo()
	if snap := c.snapshotForTradeDate(tradeDate); snap != nil {
		hits := c.hitsForSnapshot(snap, repo)
		if h := findHitForStock(hits, code); h != nil {
			return h
		}
	}
	snap, _ := repo.GetLatestCloseSignalSnapshot(tradeDate)
	if snap == nil {
		return nil
	}
	hits := c.hitsForSnapshot(snap, repo)
	return findHitForStock(hits, code)
}

func (c *loadedContext) snapshotForTradeDate(tradeDate string) *models.SignalScanSnapshot {
	if c == nil || tradeDate == "" {
		return nil
	}
	for _, snap := range c.snapshotByID {
		if snap != nil && strings.TrimSpace(snap.TradeDate) == tradeDate {
			return snap
		}
	}
	snap := fetchSnapshotByTradeDate(tradeDate)
	if snap != nil {
		c.snapshotByID[snap.ID] = snap
	}
	return snap
}

func (c *loadedContext) hitsForSnapshot(snap *models.SignalScanSnapshot, repo *data.SignalSnapshotRepo) []models.SignalScanHit {
	if snap == nil || snap.ID == 0 {
		return nil
	}
	if hits, ok := c.hitsBySnap[snap.ID]; ok {
		return hits
	}
	hits := repo.ParseSnapshotHits(snap)
	c.hitsBySnap[snap.ID] = hits
	c.snapshotByID[snap.ID] = snap
	return hits
}

func buildProjection(ctx *loadedContext, code string) OpportunityProjection {
	code = normalizeCode(code)
	out := OpportunityProjection{
		TradeDate: ctx.tradeDate,
		StockCode: code,
		Metadata: MetadataBlock{
			UpdatedAt: ctx.asOf,
			Missing:   []string{},
		},
		Portfolio: PortfolioBlock{HoldingStatus: HoldingStatusNotHeld},
		Decision: DecisionBlock{
			DecisionStatus:  DecisionStatusUnknown,
			CandidateStatus: CandidateStatusNotInPool,
		},
	}

	poolItem, inPool := ctx.poolItem(code)
	if inPool {
		out.StockName = strings.TrimSpace(poolItem.StockName)
		out.Opportunity = opportunityFromPoolItem(ctx.pool, poolItem)
		out.Metadata.SourceType = SourceTypePool
	} else {
		out.Metadata.Missing = append(out.Metadata.Missing, "candidate_pool_item")
	}

	plan, planItem, hasPlanItem := ctx.planItem(code)
	if hasPlanItem {
		out.Decision = decisionFromPlan(plan, planItem, inPool)
		out.TradePlan = tradePlanFromPlan(plan, planItem)
		if out.StockName == "" {
			out.StockName = strings.TrimSpace(planItem.StockName)
		}
	} else if inPool {
		out.Decision = DecisionBlock{
			DecisionStatus:  DecisionStatusWatch,
			CandidateStatus: CandidateStatusInPool,
		}
		if plan != nil {
			out.TradePlan = TradePlanBlock{
				Present:    true,
				PlanID:     plan.ID,
				PlanStatus: plan.Status,
				TradeDate:  plan.TradeDate,
				Frozen:     plan.IsFrozen(),
			}
		}
	}

	var hit *models.SignalScanHit
	if inPool {
		hit = ctx.hitForPoolItem(poolItem, code)
		if hit == nil && strings.TrimSpace(poolItem.SignalTag) != "" {
			out.Signal = signalFromPoolTag(poolItem)
		}
	}
	if hit == nil {
		hit = ctx.hitForResearch(code, ctx.tradeDate)
	}
	if hit != nil {
		out.Signal = signalFromHit(hit, ctx.snapshotByID)
		if inPool && poolItem.SignalSnapshotID > 0 {
			if snap := snapshotForID(ctx, poolItem.SignalSnapshotID); snap != nil {
				enrichSignalSnapshotMeta(&out.Signal, snap)
			}
		}
		if out.Metadata.SourceType == "" {
			out.Metadata.SourceType = SourceTypeSignal
		}
	} else if !out.Signal.Present {
		out.Metadata.Missing = append(out.Metadata.Missing, "signal_snapshot")
	}

	if pos, ok := ctx.holdings[code]; ok && pos.Volume > 0 {
		out.Portfolio = PortfolioBlock{
			HoldingStatus: HoldingStatusHeld,
			PositionQty:   float64(pos.Volume),
		}
		if out.StockName == "" {
			out.StockName = strings.TrimSpace(pos.StockName)
		}
	}

	if rc, ok := ctx.researchByCode[code]; ok {
		out.Research = researchFromCandidate(rc)
		if out.Metadata.SourceType == "" {
			out.Metadata.SourceType = SourceTypeResearch
		}
	}

	out.OpportunityID = buildOpportunityID(ctx, code, hit, poolItem, inPool)
	out.Metadata.Quality = assessQuality(out)
	return out
}

func opportunityFromPoolItem(pool *models.CandidatePool, item models.CandidatePoolItem) OpportunityBlock {
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
	return OpportunityBlock{
		Present:        true,
		PoolID:         item.PoolID,
		Score:          item.Score,
		Rank:           item.Rank,
		StrategySource: src,
		StrategyName:   strings.TrimSpace(item.StrategyName),
		PoolSource:     poolSource,
	}
}

func signalFromHit(hit *models.SignalScanHit, snaps map[uint]*models.SignalScanSnapshot) SignalBlock {
	if hit == nil {
		return SignalBlock{}
	}
	block := SignalBlock{
		Present:       true,
		SignalTag:     strings.TrimSpace(hit.Tag),
		SignalPrice:   hit.SignalPrice,
		SignalTime:    strings.TrimSpace(hit.SignalTime),
		SignalStatus:  strings.TrimSpace(hit.SignalPriceStatus),
		TriggerReason: strings.TrimSpace(hit.StatusText),
		SchemaVersion: strings.TrimSpace(hit.SchemaVersion),
	}
	if block.TriggerReason == "" && block.SignalTag != "" {
		block.TriggerReason = block.SignalTag
	}
	// snapshot meta filled by caller when known via pool item id — optional here
	_ = snaps
	return block
}

func signalFromPoolTag(item models.CandidatePoolItem) SignalBlock {
	tag := strings.TrimSpace(item.SignalTag)
	if tag == "" {
		return SignalBlock{}
	}
	return SignalBlock{
		Present:       true,
		SnapshotID:    item.SignalSnapshotID,
		SignalTag:     tag,
		TriggerReason: tag,
	}
}

func decisionFromPlan(plan *models.TradePlan, item *models.TradePlanItem, inPool bool) DecisionBlock {
	if plan == nil || item == nil {
		return DecisionBlock{DecisionStatus: DecisionStatusUnknown}
	}
	return DecisionBlock{
		DecisionStatus:   mapDecisionStatus(item, inPool),
		CandidateStatus:  mapCandidateStatus(inPool, item),
		PlanID:           plan.ID,
		PlanItemID:       item.ID,
		ItemStatus:       item.Status,
		RiskCode:         strings.TrimSpace(item.RiskCode),
		TargetAmount:     item.TargetAmount,
		DecisionProvider: strings.TrimSpace(plan.DecisionProvider),
	}
}

func tradePlanFromPlan(plan *models.TradePlan, item *models.TradePlanItem) TradePlanBlock {
	if plan == nil {
		return TradePlanBlock{}
	}
	block := TradePlanBlock{
		Present:    true,
		PlanID:     plan.ID,
		PlanStatus: plan.Status,
		TradeDate:  plan.TradeDate,
		Frozen:     plan.IsFrozen(),
	}
	if item != nil {
		block.ItemStatus = item.Status
	}
	return block
}

func researchFromCandidate(c research.Candidate) *ResearchBlock {
	block := &ResearchBlock{
		ResearchID: c.ID,
		Status:     c.Status,
		Tags:       append([]string(nil), c.Tags...),
		ExplainSummary: strings.TrimSpace(c.ExplainSummary),
	}
	if c.SignalScore != nil {
		v := *c.SignalScore
		block.SignalScore = &v
	}
	return block
}

func buildOpportunityID(ctx *loadedContext, code string, hit *models.SignalScanHit, poolItem models.CandidatePoolItem, inPool bool) string {
	var snapID uint
	sess := models.SignalScanSessionClose
	strategyID := ""
	if inPool && poolItem.SignalSnapshotID > 0 {
		snapID = poolItem.SignalSnapshotID
	}
	if hit != nil {
		secucode := strings.TrimSpace(hit.SECUCODE)
		tag := strings.TrimSpace(hit.Tag)
		signalTime := strings.TrimSpace(hit.SignalTime)
		batchKey := opportunity.BatchKeyFromSnapshot(snapID, ctx.tradeDate, sess, strategyID)
		if secucode != "" {
			return opportunity.BuildOpportunityID(batchKey, secucode, signalTime, tag)
		}
	}
	if inPool {
		batchKey := opportunity.BatchKeyFromSnapshot(snapID, ctx.tradeDate, sess, strategyID)
		return opportunity.BuildOpportunityID(batchKey, code, "", strings.TrimSpace(poolItem.SignalTag))
	}
	batchKey := opportunity.BatchKeyFromSnapshot(0, ctx.tradeDate, sess, strategyID)
	return opportunity.BuildOpportunityID(batchKey, code, "", "")
}

func assessQuality(p OpportunityProjection) string {
	hasSignal := p.Signal.Present
	hasOpp := p.Opportunity.Present
	hasDecision := p.Decision.DecisionStatus != DecisionStatusUnknown || p.TradePlan.Present
	if hasSignal && hasOpp && hasDecision {
		return QualityComplete
	}
	if hasSignal || hasOpp || p.Research != nil {
		return QualityPartial
	}
	return QualityPartial
}

func normalizeCode(raw string) string {
	return opportunity.NormalizeReadStockCode(raw)
}

func snapshotForID(ctx *loadedContext, snapshotID uint) *models.SignalScanSnapshot {
	if ctx == nil || snapshotID == 0 {
		return nil
	}
	if snap, ok := ctx.snapshotByID[snapshotID]; ok {
		return snap
	}
	snap := fetchSnapshot(snapshotID)
	if snap != nil {
		ctx.snapshotByID[snapshotID] = snap
	}
	return snap
}

func loadSnapshotHits(snapshotID uint, repo *data.SignalSnapshotRepo) []models.SignalScanHit {
	snap := fetchSnapshot(snapshotID)
	if snap == nil {
		return nil
	}
	return repo.ParseSnapshotHits(snap)
}

func findHitForStock(hits []models.SignalScanHit, code string) *models.SignalScanHit {
	for i := range hits {
		if hitMatchesStock(hits[i], code) {
			return &hits[i]
		}
	}
	return nil
}

func hitMatchesStock(hit models.SignalScanHit, code string) bool {
	hitCode := opportunity.SecucodeToStockCode(hit.SECUCODE)
	if hitCode == "" {
		hitCode = strings.ToLower(strings.TrimSpace(hit.SECURITY_CODE))
	}
	return hitCode == code
}
