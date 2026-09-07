package investmentnarrative

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"
)

// BuildInvestmentNarrative assembles a read-only narrative for one stock.
func BuildInvestmentNarrative(opts BuildOptions) (*InvestmentNarrative, error) {
	code := normalizeStockCode(opts.StockCode)
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}

	out := &InvestmentNarrative{
		StockCode:      code,
		Discovery:      missingDiscovery(),
		PlanOrigin:     missingPlanOrigin(),
		HoldingBasis:   missingHoldingBasis(),
		ExitReview:     missingExitReview(SectionStatusNA),
		PriceStory:     PriceStoryNarrative{Status: SectionStatusMissing},
		AsOf:           asOf,
		SchemaVersion:  SchemaVersion,
		DataSourceNote: dataSourceNote,
	}
	if code == "" {
		return out, nil
	}
	if db.Dao == nil {
		return out, nil
	}

	accountID := opts.AccountID
	if accountID == 0 {
		if acc, err := papertrading.GetDefaultAccount(); err == nil && acc != nil {
			accountID = acc.ID
		}
	}

	out.Discovery = projectDiscovery(code)
	out.PlanOrigin = projectPlanOrigin(code, accountID)
	out.HoldingBasis = projectHoldingBasis(code, accountID)
	out.ExitReview = projectExitReview(code, accountID)
	out.PriceStory = projectPriceStory(code, out.Discovery, out.HoldingBasis, accountID)

	return out, nil
}

func normalizeStockCode(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.Contains(s, ".") {
		return opportunity.SecucodeToStockCode(s)
	}
	return strings.ToLower(s)
}

func projectDiscovery(code string) DiscoveryNarrative {
	missing := missingDiscovery()
	if db.Dao == nil || !db.Dao.Migrator().HasTable(&models.SignalScanSnapshot{}) {
		return missing
	}

	var snaps []models.SignalScanSnapshot
	if err := db.Dao.Where("status = ?", "done").
		Order("trade_date desc, id desc").
		Limit(60).
		Find(&snaps).Error; err != nil || len(snaps) == 0 {
		return missing
	}

	repo := data.NewSignalSnapshotRepo()
	for _, snap := range snaps {
		for _, hit := range repo.ParseSnapshotHits(&snap) {
			if !hitMatchesStock(hit, code) {
				continue
			}
			d := DiscoveryNarrative{
				Exists:     true,
				SnapshotID: snap.ID,
				SignalTime: strings.TrimSpace(hit.SignalTime),
				Reason:     discoveryReason(hit),
			}
			if hit.SignalPrice > 0 {
				p := hit.SignalPrice
				d.SignalPrice = &p
			}
			d.Status = discoveryStatus(d)
			return d
		}
	}
	return missing
}

func hitMatchesStock(hit models.SignalScanHit, code string) bool {
	hitCode := opportunity.SecucodeToStockCode(hit.SECUCODE)
	if hitCode == "" {
		hitCode = strings.ToLower(strings.TrimSpace(hit.SECURITY_CODE))
	}
	return hitCode == code
}

func discoveryReason(hit models.SignalScanHit) string {
	tag := strings.TrimSpace(hit.Tag)
	if tag != "" {
		return tag
	}
	return strings.TrimSpace(hit.StatusText)
}

func discoveryStatus(d DiscoveryNarrative) string {
	if !d.Exists {
		return SectionStatusMissing
	}
	hasTime := d.SignalTime != ""
	hasPrice := d.SignalPrice != nil && *d.SignalPrice > 0
	hasReason := d.Reason != ""
	if hasTime && hasPrice && hasReason {
		return SectionStatusFull
	}
	if hasTime || hasPrice || hasReason {
		return SectionStatusPartial
	}
	return SectionStatusPartial
}

func projectPlanOrigin(code string, accountID uint) PlanOriginNarrative {
	missing := missingPlanOrigin()
	if db.Dao == nil {
		return missing
	}

	var fill papertrading.PaperSimFill
	q := db.Dao.Where("stock_code = ? AND side = ?", code, "buy").Order("filled_at desc, id desc")
	if accountID > 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&fill).Error; err == nil && fill.PlanID > 0 {
		return planOriginFromIDs(fill.PlanID, fill.PlanItemID)
	}

	var item models.TradePlanItem
	iq := db.Dao.Where("stock_code = ? AND side = ?", code, "buy").Order("id desc")
	if err := iq.First(&item).Error; err != nil {
		return missing
	}
	return planOriginFromIDs(item.PlanID, item.ID)
}

func planOriginFromIDs(planID, itemID uint) PlanOriginNarrative {
	var item models.TradePlanItem
	if err := db.Dao.First(&item, itemID).Error; err != nil {
		if planID == 0 {
			return missingPlanOrigin()
		}
		return PlanOriginNarrative{
			Exists: true,
			Status: SectionStatusPartial,
			PlanID: planID,
		}
	}
	po := PlanOriginNarrative{
		Exists:   true,
		PlanID:   item.PlanID,
		Strategy: strings.TrimSpace(item.StrategyName),
		Reason:   strings.TrimSpace(item.Reason),
	}
	po.Status = planOriginStatus(po)
	return po
}

func planOriginStatus(p PlanOriginNarrative) string {
	if !p.Exists {
		return SectionStatusMissing
	}
	if p.PlanID > 0 && p.Strategy != "" && p.Reason != "" {
		return SectionStatusFull
	}
	if p.PlanID > 0 {
		return SectionStatusPartial
	}
	return SectionStatusPartial
}

func projectHoldingBasis(code string, accountID uint) HoldingBasisNarrative {
	missing := missingHoldingBasis()
	if db.Dao == nil || accountID == 0 {
		return missing
	}
	if !db.Dao.Migrator().HasTable(&papertrading.PaperSimPosition{}) {
		return missing
	}
	var pos papertrading.PaperSimPosition
	if err := db.Dao.Where("account_id = ? AND stock_code = ?", accountID, code).First(&pos).Error; err != nil {
		return missing
	}
	if pos.TotalVolume <= 0 {
		return missing
	}
	hb := HoldingBasisNarrative{
		Exists:   true,
		Quantity: pos.TotalVolume,
		AvgCost:  pos.AvgCost,
	}
	if hb.AvgCost > 0 {
		hb.Status = SectionStatusFull
	} else {
		hb.Status = SectionStatusPartial
	}
	return hb
}

func projectExitReview(code string, accountID uint) ExitReviewNarrative {
	na := missingExitReview(SectionStatusNA)
	if accountID == 0 {
		return na
	}

	var pos papertrading.PaperSimPosition
	if err := db.Dao.Where("account_id = ? AND stock_code = ?", accountID, code).First(&pos).Error; err != nil || pos.TotalVolume <= 0 {
		return na
	}

	view, err := papertrading.BuildExitEvaluation(papertrading.ExitEvaluationBuildOptions{
		StockCode: code,
	})
	if err != nil || view == nil {
		return missingExitReview(SectionStatusMissing)
	}
	_ = papertrading.AttachLatestOutcomes(view)

	for _, h := range view.Holdings {
		if normalizeStockCode(h.StockCode) != code {
			continue
		}
		er := ExitReviewNarrative{
			Exists:      true,
			Status:      h.Evaluation.State,
			ReasonCodes: append([]string(nil), h.Evaluation.ReasonCodes...),
			Summary:     h.Evaluation.Summary,
		}
		if h.LatestOutcome != nil {
			er.LatestOutcome = &ExitReviewOutcomeNarrative{
				Decision:           h.LatestOutcome.Decision,
				Reason:             h.LatestOutcome.Reason,
				RelatedTradePlanID: h.LatestOutcome.RelatedTradePlanID,
			}
			if !h.LatestOutcome.ReviewTime.IsZero() {
				t := h.LatestOutcome.ReviewTime
				er.LatestOutcome.ReviewTime = &t
			}
		}
		if er.Status == "" {
			er.Status = SectionStatusMissing
		}
		return er
	}
	return missingExitReview(SectionStatusMissing)
}

func missingExitReview(status string) ExitReviewNarrative {
	if status == "" {
		status = SectionStatusMissing
	}
	return ExitReviewNarrative{Exists: false, Status: status}
}

func projectPriceStory(code string, d DiscoveryNarrative, h HoldingBasisNarrative, accountID uint) PriceStoryNarrative {
	ps := PriceStoryNarrative{Status: SectionStatusMissing}
	if d.SignalPrice != nil && *d.SignalPrice > 0 {
		p := *d.SignalPrice
		ps.SignalPrice = &p
	}

	var current *float64
	if h.Exists && accountID > 0 && db.Dao != nil {
		var pos papertrading.PaperSimPosition
		if err := db.Dao.Where("account_id = ? AND stock_code = ?", accountID, code).First(&pos).Error; err == nil {
			if pos.MarkPrice > 0 {
				cp := pos.MarkPrice
				current = &cp
			}
		}
		if current == nil {
			holding, err := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{
				StockCode: code,
			})
			if err == nil && holding != nil {
				for _, row := range holding.Holdings {
					if normalizeStockCode(row.StockCode) != code {
						continue
					}
					if row.CurrentPrice != nil && *row.CurrentPrice > 0 {
						cp := *row.CurrentPrice
						current = &cp
						break
					}
					if row.MarketPrice != nil && *row.MarketPrice > 0 {
						cp := *row.MarketPrice
						current = &cp
						break
					}
				}
			}
		}
	}

	if current != nil {
		ps.CurrentPrice = current
	}

	if ps.SignalPrice != nil && ps.CurrentPrice != nil && *ps.SignalPrice > 0 {
		pct := ((*ps.CurrentPrice - *ps.SignalPrice) / *ps.SignalPrice) * 100
		ps.VsSignalPct = &pct
		ps.Status = SectionStatusFull
	} else if ps.SignalPrice != nil || ps.CurrentPrice != nil {
		ps.Status = SectionStatusPartial
	}
	return ps
}

func missingDiscovery() DiscoveryNarrative {
	return DiscoveryNarrative{Exists: false, Status: SectionStatusMissing}
}

func missingPlanOrigin() PlanOriginNarrative {
	return PlanOriginNarrative{Exists: false, Status: SectionStatusMissing}
}

func missingHoldingBasis() HoldingBasisNarrative {
	return HoldingBasisNarrative{Exists: false, Status: SectionStatusMissing}
}
