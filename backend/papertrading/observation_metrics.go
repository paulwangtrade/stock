// Observation Metrics (Phase10-C.3-O.2).
// Read-only aggregation over Label Layer (C.3-O.1). Never writes paper_sim_*.

package papertrading

import (
	"fmt"
	"strings"

	"go-stock/backend/db"
)

// ObservationMetrics is the C.3-O.2 read-only metrics snapshot.
type ObservationMetrics struct {
	Enabled   bool   `json:"enabled"`
	TradeDate string `json:"tradeDate,omitempty"` // filter applied; empty = all

	TotalRuns int `json:"totalRuns"`

	SessionDistribution ObservationSessionMetrics `json:"sessionDistribution"`
	FillPolicy          ObservationFillPolicyMetrics `json:"fillPolicy"`
	Quality             ObservationQualityMetrics `json:"quality"`
	Legacy              ObservationLegacyMetrics `json:"legacy"`

	// PricePolicyCompliance is compliantB / max(1, baselineBAllowFills)
	// where baseline excludes excludedBaseline fills. Range [0,1].
	PricePolicyCompliance float64 `json:"pricePolicyCompliance"`
}

// ObservationSessionMetrics counts runs by derived session (from run.started_at via Label Layer).
type ObservationSessionMetrics struct {
	SessionA      int `json:"sessionA"`
	SessionB      int `json:"sessionB"`
	SessionC      int `json:"sessionC"`
	SessionClosed int `json:"sessionClosed"`
}

// ObservationFillPolicyMetrics counts fills / B-window compliance (baseline-filtered).
type ObservationFillPolicyMetrics struct {
	TotalFills        int `json:"totalFills"`
	MarketOpenFills   int `json:"marketOpenFills"`
	MarketCloseFills  int `json:"marketCloseFills"`
	BWindowTotal      int `json:"bWindowTotal"`      // Session B ∧ !excludedBaseline
	BWindowCloseFills int `json:"bWindowCloseFills"` // B ∧ market_close ∧ !excluded
	BWindowOpenFills  int `json:"bWindowOpenFills"`  // B ∧ market_open ∧ !excluded (violations)
	BWindowCompliant  int `json:"bWindowCompliant"`  // alias of BWindowCloseFills for clarity
	BWindowViolation  int `json:"bWindowViolation"`  // alias of BWindowOpenFills
}

// ObservationQualityMetrics counts fill quality tags from Label Layer.
type ObservationQualityMetrics struct {
	OKCount         int `json:"okCount"`
	AnomalyCount    int `json:"anomalyCount"`
	IncompleteCount int `json:"incompleteCount"`
	LegacyCount     int `json:"legacyCount"`
}

// ObservationLegacyMetrics isolates known / heuristic baseline exclusions.
type ObservationLegacyMetrics struct {
	LegacyBaselineCount int `json:"legacyBaselineCount"`
	ExcludedFillCount   int `json:"excludedFillCount"`
}

// AggregateObservationMetrics builds metrics from runs + already-classified fill labels.
// Must not re-derive session/legacy/priceMode rules — labels come from ClassifyFill.
func AggregateObservationMetrics(runs []PaperSimRun, labels []ObservationFillLabel) ObservationMetrics {
	out := ObservationMetrics{
		Enabled: IsEnabled(),
	}
	out.TotalRuns = len(runs)
	for _, r := range runs {
		if r.StartedAt.IsZero() {
			continue
		}
		switch DeriveSessionFromLocalTime(r.StartedAt) {
		case SessionA:
			out.SessionDistribution.SessionA++
		case SessionB:
			out.SessionDistribution.SessionB++
		case SessionC:
			out.SessionDistribution.SessionC++
		case SessionClosed:
			out.SessionDistribution.SessionClosed++
		}
	}

	compliantB := 0
	violationB := 0

	for _, lbl := range labels {
		out.FillPolicy.TotalFills++
		switch strings.TrimSpace(lbl.FillReason) {
		case FillReasonMarketOpen:
			out.FillPolicy.MarketOpenFills++
		case FillReasonMarketClose:
			out.FillPolicy.MarketCloseFills++
		}

		switch lbl.QualityTag {
		case QualityOK:
			out.Quality.OKCount++
		case QualityAnomaly:
			out.Quality.AnomalyCount++
		case QualityIncomplete:
			out.Quality.IncompleteCount++
		case QualityLegacyBaseline:
			out.Quality.LegacyCount++
			out.Legacy.LegacyBaselineCount++
		}

		if lbl.ExcludedBaseline {
			out.Legacy.ExcludedFillCount++
			continue // excluded from B-window compliance denominators
		}

		if lbl.DerivedSession != SessionB {
			continue
		}
		out.FillPolicy.BWindowTotal++
		reason := strings.TrimSpace(lbl.FillReason)
		if reason == FillReasonMarketClose {
			out.FillPolicy.BWindowCloseFills++
			compliantB++
		} else if reason == FillReasonMarketOpen {
			out.FillPolicy.BWindowOpenFills++
			violationB++
		}
	}

	out.FillPolicy.BWindowCompliant = compliantB
	out.FillPolicy.BWindowViolation = violationB

	denom := compliantB + violationB
	if denom <= 0 {
		out.PricePolicyCompliance = 1 // no baseline B fills → vacuously compliant
	} else {
		out.PricePolicyCompliance = float64(compliantB) / float64(denom)
	}
	return out
}

// LabelFillsForMetrics classifies fills using C.3-O.1 ClassifyFill (no reimplementation).
// runByPlanDate maps "planID|tradeDate" → run for exact legacy matching.
func LabelFillsForMetrics(fills []PaperSimFill, runs []PaperSimRun, fillTradeDate map[uint]string) []ObservationFillLabel {
	runIndex := indexRunsByPlanDate(runs)
	labels := make([]ObservationFillLabel, 0, len(fills))
	for _, f := range fills {
		td := ""
		if fillTradeDate != nil {
			td = fillTradeDate[f.ID]
		}
		runID, planID, tradeDate := resolveRunForFill(f, td, runIndex, runs)
		labels = append(labels, ClassifyFill(runID, planID, tradeDate, f))
	}
	return labels
}

func indexRunsByPlanDate(runs []PaperSimRun) map[string]*PaperSimRun {
	out := make(map[string]*PaperSimRun, len(runs))
	for i := range runs {
		r := &runs[i]
		key := fmt.Sprintf("%d|%s", r.PlanID, strings.TrimSpace(r.TradeDate))
		// Prefer lowest id for stable legacy match (known sample run_id=2).
		if prev, ok := out[key]; ok && prev.ID < r.ID {
			continue
		}
		out[key] = r
	}
	return out
}

func resolveRunForFill(f PaperSimFill, orderTradeDate string, byPlanDate map[string]*PaperSimRun, runs []PaperSimRun) (runID, planID uint, tradeDate string) {
	planID = f.PlanID
	td := strings.TrimSpace(orderTradeDate)
	if td != "" {
		if r := byPlanDate[fmt.Sprintf("%d|%s", planID, td)]; r != nil {
			return r.ID, r.PlanID, r.TradeDate
		}
	}
	// Fallback: first run with same plan_id (prefer exact legacy run if present).
	for i := range runs {
		r := &runs[i]
		if r.PlanID == planID {
			if IsExactLegacyRun(r.ID, r.PlanID, r.TradeDate) {
				return r.ID, r.PlanID, r.TradeDate
			}
		}
	}
	for i := range runs {
		r := &runs[i]
		if r.PlanID == planID {
			return r.ID, r.PlanID, r.TradeDate
		}
	}
	return 0, planID, td
}

// GetObservationMetrics loads paper_sim runs/fills (read-only) and aggregates via Label Layer.
// tradeDate empty → all dates.
func GetObservationMetrics(tradeDate string) (*ObservationMetrics, error) {
	tradeDate = strings.TrimSpace(tradeDate)
	out := &ObservationMetrics{Enabled: IsEnabled(), TradeDate: tradeDate}
	if db.Dao == nil {
		return out, fmt.Errorf("papertrading: db not initialized")
	}

	runs := []PaperSimRun{}
	if db.Dao.Migrator().HasTable(&PaperSimRun{}) {
		q := db.Dao.Model(&PaperSimRun{})
		if tradeDate != "" {
			q = q.Where("trade_date = ?", tradeDate)
		}
		if err := q.Order("id asc").Find(&runs).Error; err != nil {
			return out, err
		}
	}

	fills := []PaperSimFill{}
	fillTradeDate := map[uint]string{}
	if db.Dao.Migrator().HasTable(&PaperSimFill{}) {
		if tradeDate != "" && db.Dao.Migrator().HasTable(&PaperSimOrder{}) {
			var rows []PaperSimFill
			err := db.Dao.Table("paper_sim_fills AS f").
				Select("f.*").
				Joins("JOIN paper_sim_orders AS o ON o.id = f.order_id").
				Where("o.trade_date = ?", tradeDate).
				Order("f.id asc").
				Find(&rows).Error
			if err != nil {
				return out, err
			}
			fills = rows
			for _, f := range fills {
				fillTradeDate[f.ID] = tradeDate
			}
		} else {
			if err := db.Dao.Order("id asc").Find(&fills).Error; err != nil {
				return out, err
			}
			if db.Dao.Migrator().HasTable(&PaperSimOrder{}) && len(fills) > 0 {
				oids := make([]uint, 0, len(fills))
				for _, f := range fills {
					oids = append(oids, f.OrderID)
				}
				var orders []PaperSimOrder
				_ = db.Dao.Where("id IN ?", oids).Find(&orders).Error
				byID := map[uint]string{}
				for _, o := range orders {
					byID[o.ID] = o.TradeDate
				}
				for _, f := range fills {
					fillTradeDate[f.ID] = byID[f.OrderID]
				}
			}
		}
	}

	labels := LabelFillsForMetrics(fills, runs, fillTradeDate)
	m := AggregateObservationMetrics(runs, labels)
	m.Enabled = IsEnabled()
	m.TradeDate = tradeDate
	return &m, nil
}
