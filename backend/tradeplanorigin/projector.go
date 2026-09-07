package tradeplanorigin

import (
	"fmt"
	"strconv"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
)

// ProjectPlanOrigin builds read-only origin projections for all items in a trade plan.
func ProjectPlanOrigin(planID uint) ([]ItemOrigin, error) {
	if planID == 0 {
		return nil, fmt.Errorf("plan id required")
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	plan, err := data.NewTradePlanRepo().GetByID(planID)
	if err != nil {
		return nil, err
	}

	var pool *models.CandidatePool
	poolItemsByCode := map[string]models.CandidatePoolItem{}
	if plan.PoolID > 0 {
		if p, perr := data.NewCandidatePoolRepo().GetByID(plan.PoolID); perr == nil && p != nil {
			pool = p
			for _, pi := range p.Items {
				code := normalizeStockCode(pi.StockCode)
				if code != "" {
					poolItemsByCode[code] = pi
				}
			}
		}
	}

	snapshotCache := map[uint][]models.SignalScanHit{}
	snapshotRepo := data.NewSignalSnapshotRepo()

	out := make([]ItemOrigin, 0, len(plan.Items))
	for _, item := range plan.Items {
		out = append(out, projectItemOrigin(plan, pool, item, poolItemsByCode, snapshotCache, snapshotRepo))
	}
	return out, nil
}

func projectItemOrigin(
	plan *models.TradePlan,
	pool *models.CandidatePool,
	item models.TradePlanItem,
	poolItemsByCode map[string]models.CandidatePoolItem,
	snapshotCache map[uint][]models.SignalScanHit,
	snapshotRepo *data.SignalSnapshotRepo,
) ItemOrigin {
	code := normalizeStockCode(item.StockCode)
	origin := ItemOrigin{
		StockCode:       orMissing(code),
		PlanID:          strconv.FormatUint(uint64(plan.ID), 10),
		SignalTime:      Missing,
		SignalPrice:     Missing,
		SignalTag:       Missing,
		SourceReason:    Missing,
		SelectionReason: Missing,
		StrategyName:    Missing,
		Score:           Missing,
	}

	poolItem, hasPoolItem := poolItemsByCode[code]
	strategyName := strings.TrimSpace(item.StrategyName)
	if strategyName == "" && hasPoolItem {
		strategyName = strings.TrimSpace(poolItem.StrategyName)
	}
	if strategyName != "" {
		origin.StrategyName = strategyName
	}

	score := item.Score
	if score <= 0 && hasPoolItem && poolItem.Score > 0 {
		score = poolItem.Score
	}
	if score > 0 {
		origin.Score = formatScore(score)
	}

	var hit *models.SignalScanHit
	signalTag := ""
	if hasPoolItem {
		if poolItem.SignalSnapshotID > 0 {
			origin.SignalSnapshotID = poolItem.SignalSnapshotID
		}
		signalTag = strings.TrimSpace(poolItem.SignalTag)
		if poolItem.SignalSnapshotID > 0 {
			hits, ok := snapshotCache[poolItem.SignalSnapshotID]
			if !ok {
				hits = loadSnapshotHits(poolItem.SignalSnapshotID, snapshotRepo)
				snapshotCache[poolItem.SignalSnapshotID] = hits
			}
			if h := findHitForStock(hits, code); h != nil {
				hit = h
			}
		}
	}
	if signalTag == "" && hit != nil {
		signalTag = strings.TrimSpace(hit.Tag)
	}
	if signalTag != "" {
		origin.SignalTag = signalTag
	}

	if hit != nil {
		if t := strings.TrimSpace(hit.SignalTime); t != "" {
			origin.SignalTime = t
		}
		if hit.SignalPrice > 0 {
			origin.SignalPrice = formatPrice(hit.SignalPrice)
		}
	}

	poolSource := ""
	if pool != nil {
		poolSource = strings.TrimSpace(pool.Source)
	}
	poolReason := ""
	if hasPoolItem {
		poolReason = strings.TrimSpace(poolItem.Reason)
	}
	statusText := ""
	if hit != nil {
		statusText = strings.TrimSpace(hit.StatusText)
	}

	sourceReason := buildSourceReason(sourceReasonInput{
		SignalTag:    signalTag,
		StrategyName: strategyName,
		SignalTime:   strings.TrimSpace(origin.SignalTime),
		PoolReason:   poolReason,
		PoolSource:   poolSource,
		StatusText:   statusText,
	})
	if sourceReason != Missing {
		origin.SourceReason = sourceReason
	}

	poolRank := 0
	poolScore := 0.0
	if hasPoolItem {
		poolRank = poolItem.Rank
		poolScore = poolItem.Score
	}
	inPlan := item.Status != models.TradePlanItemSkipped
	selectionReason := buildSelectionReason(selectionReasonInput{
		PoolRank:    poolRank,
		PoolScore:   poolScore,
		PlanScore:   item.Score,
		MaxNames:    plan.MaxNames,
		ItemStatus:  item.Status,
		RiskCode:    item.RiskCode,
		RiskMessage: item.RiskMessage,
		HasPoolItem: hasPoolItem,
		InPlan:      inPlan,
	})
	if selectionReason != Missing {
		origin.SelectionReason = selectionReason
	}

	return origin
}

func loadSnapshotHits(snapshotID uint, repo *data.SignalSnapshotRepo) []models.SignalScanHit {
	if db.Dao == nil || snapshotID == 0 {
		return nil
	}
	var snap models.SignalScanSnapshot
	if err := db.Dao.First(&snap, snapshotID).Error; err != nil {
		return nil
	}
	return repo.ParseSnapshotHits(&snap)
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

func normalizeStockCode(raw string) string {
	return opportunity.NormalizeReadStockCode(raw)
}

func orMissing(v string) string {
	if strings.TrimSpace(v) == "" {
		return Missing
	}
	return v
}

func formatScore(v float64) string {
	if v <= 0 {
		return Missing
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func formatPrice(v float64) string {
	if v <= 0 {
		return Missing
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
