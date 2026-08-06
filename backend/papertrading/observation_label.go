// Observation Label Layer (Phase10-C.3-O.1).
// Read-only derived tags for paper_sim_* rows. Never writes DB / never rewrites fills.

package papertrading

import (
	"strings"
	"time"
)

// Observation quality tags (C.3-O.1).
const (
	QualityOK             = "OK"
	QualityLegacyBaseline = "LEGACY_BASELINE"
	QualityAnomaly        = "ANOMALY"
	QualityIncomplete     = "INCOMPLETE"
)

// SessionSource indicates how derived_session was obtained.
const (
	SessionSourcePersisted = "persisted" // reserved: future run.session column
	SessionSourceDerived   = "derived"   // from filled_at / started_at local clock
)

// C2CCutoverLocal is the Phase10-C.2-C CloseFill cutover boundary (be83490).
// Fills strictly before this instant may be tagged LEGACY_BASELINE via heuristic;
// Session B + market_open at/after cutover is always ANOMALY (never silent legacy).
var C2CCutoverLocal = time.Date(2026, 8, 6, 0, 0, 0, 0, time.Local)

// KnownLegacyExecutionSample mirrors PHASE10_C2_C_LEGACY_EXECUTION_NOTE.md (exact match).
var KnownLegacyExecutionSample = LegacyExecutionSample{
	RunID:     2,
	PlanID:    26,
	TradeDate: "2026-08-05",
	FillIDs:   []uint{6, 7, 8, 9, 10},
}

// LegacyExecutionSample is an exact, read-only exclusion key (no DB rewrite).
type LegacyExecutionSample struct {
	RunID     uint
	PlanID    uint
	TradeDate string
	FillIDs   []uint
}

// ObservationFillLabel is a per-fill derived label (read-only overlay).
type ObservationFillLabel struct {
	FillID            uint              `json:"fillId"`
	OrderID           uint              `json:"orderId,omitempty"`
	PlanID            uint              `json:"planId,omitempty"`
	StockCode         string            `json:"stockCode,omitempty"`
	FillReason        string            `json:"fillReason"`
	Price             float64           `json:"price,omitempty"`
	FilledAt          time.Time         `json:"filledAt"`
	DerivedSession    ExecutionSession  `json:"derivedSession"`
	SessionSource     string            `json:"sessionSource"`
	PriceMode         string            `json:"priceMode"`
	QualityTag        string            `json:"qualityTag"`
	ExcludedBaseline  bool              `json:"excludedBaseline"`
	LegacyExactMatch  bool              `json:"legacyExactMatch,omitempty"`
	LegacyHeuristic   bool              `json:"legacyHeuristic,omitempty"`
}

// ObservationDayBundle is the C.3-O.1 day-scoped observation label aggregate.
// Built in memory from existing paper_sim_* structs; does not persist.
type ObservationDayBundle struct {
	// Gateway / run layer
	RunID     uint   `json:"runId,omitempty"`
	PlanID    uint   `json:"planId,omitempty"`
	TradeDate string `json:"tradeDate"`
	Trigger   string `json:"trigger,omitempty"`
	Actor     string `json:"actor,omitempty"`

	// Session derived
	DerivedSession ExecutionSession `json:"derivedSession"`
	SessionSource  string           `json:"sessionSource"`

	// Fill provider / price policy derived
	PriceMode string `json:"priceMode"` // realtime | close | unknown

	// Order / Fill aggregates
	OrderCount        int            `json:"orderCount"`
	FillCount         int            `json:"fillCount"`
	FillReasonSummary map[string]int `json:"fillReasonSummary,omitempty"`

	// Position
	PositionCount int `json:"positionCount"`

	// Quality
	QualityTag       string `json:"qualityTag"`
	ExcludedBaseline bool   `json:"excludedBaseline"`

	// Per-fill labels (optional detail for O.2)
	Fills []ObservationFillLabel `json:"fills,omitempty"`
}

// ObservationBundleInput is the read-only input to BuildObservationDayBundle.
type ObservationBundleInput struct {
	Run            *PaperSimRun
	Orders         []PaperSimOrder
	Fills          []PaperSimFill
	PositionCount  int
	// PersistedSession, when non-empty, forces session_source=persisted (future).
	PersistedSession ExecutionSession
}

// IsExactLegacyFill reports whether (runID, planID, tradeDate, fillID) hits the known sample.
func IsExactLegacyFill(runID, planID uint, tradeDate string, fillID uint) bool {
	s := KnownLegacyExecutionSample
	if runID != s.RunID || planID != s.PlanID {
		return false
	}
	if strings.TrimSpace(tradeDate) != "" && strings.TrimSpace(tradeDate) != s.TradeDate {
		return false
	}
	for _, id := range s.FillIDs {
		if id == fillID {
			return true
		}
	}
	return false
}

// IsExactLegacyRun reports whether the run matches the known legacy sample header.
func IsExactLegacyRun(runID, planID uint, tradeDate string) bool {
	s := KnownLegacyExecutionSample
	if runID != s.RunID || planID != s.PlanID {
		return false
	}
	if td := strings.TrimSpace(tradeDate); td != "" && td != s.TradeDate {
		return false
	}
	return true
}

// DeriveSessionFromLocalTime classifies a fill/run clock into A/B/C/closed.
// Delegates to ResolveExecutionSession (same production windows).
func DeriveSessionFromLocalTime(t time.Time) ExecutionSession {
	if t.IsZero() {
		return ""
	}
	return ResolveExecutionSession(t.In(time.Local))
}

// InferPriceModeFromFillReason maps fill_reason → observation price_mode.
func InferPriceModeFromFillReason(fillReason string) string {
	switch strings.TrimSpace(fillReason) {
	case FillReasonMarketClose:
		return PriceModeClose
	case FillReasonMarketOpen:
		return PriceModeRealtime
	default:
		return "unknown"
	}
}

// ClassifyFill labels one fill without writing DB.
//
// Rules (priority):
//  1. Incomplete (zero filled_at / empty reason when needed) → INCOMPLETE
//  2. Exact legacy sample → LEGACY_BASELINE + excludedBaseline
//  3. Session B + market_open + filled_at < C2CCutover → LEGACY_BASELINE + excludedBaseline (heuristic)
//  4. Session B + market_open + filled_at >= C2CCutover → ANOMALY (never silent legacy)
//  5. Session B + market_close → OK + price_mode=close
//  6. Session A + market_open → OK + price_mode=realtime
//  7. Otherwise policy mismatch → ANOMALY; empty/unknown → INCOMPLETE
func ClassifyFill(runID, planID uint, tradeDate string, fill PaperSimFill) ObservationFillLabel {
	out := ObservationFillLabel{
		FillID:         fill.ID,
		OrderID:        fill.OrderID,
		PlanID:         fill.PlanID,
		StockCode:      fill.StockCode,
		FillReason:     fill.FillReason,
		Price:          fill.Price,
		FilledAt:       fill.FilledAt,
		SessionSource:  SessionSourceDerived,
		PriceMode:      InferPriceModeFromFillReason(fill.FillReason),
		QualityTag:     QualityIncomplete,
	}
	if fill.PlanID != 0 {
		planID = fill.PlanID
		out.PlanID = fill.PlanID
	}
	if fill.FilledAt.IsZero() {
		out.QualityTag = QualityIncomplete
		return out
	}

	sess := DeriveSessionFromLocalTime(fill.FilledAt)
	out.DerivedSession = sess
	if sess == "" {
		out.QualityTag = QualityIncomplete
		return out
	}

	reason := strings.TrimSpace(fill.FillReason)
	if reason == "" {
		out.QualityTag = QualityIncomplete
		out.PriceMode = "unknown"
		return out
	}

	// 2) Exact legacy
	if IsExactLegacyFill(runID, planID, tradeDate, fill.ID) {
		out.QualityTag = QualityLegacyBaseline
		out.ExcludedBaseline = true
		out.LegacyExactMatch = true
		return out
	}

	filledLocal := fill.FilledAt.In(time.Local)

	// 3–4) Session B + market_open
	if sess == SessionB && reason == FillReasonMarketOpen {
		if filledLocal.Before(C2CCutoverLocal) {
			out.QualityTag = QualityLegacyBaseline
			out.ExcludedBaseline = true
			out.LegacyHeuristic = true
			return out
		}
		out.QualityTag = QualityAnomaly
		out.ExcludedBaseline = false
		return out
	}

	// 5–6) Policy OK
	if sess == SessionB && reason == FillReasonMarketClose {
		out.QualityTag = QualityOK
		out.PriceMode = PriceModeClose
		return out
	}
	if sess == SessionA && reason == FillReasonMarketOpen {
		out.QualityTag = QualityOK
		out.PriceMode = PriceModeRealtime
		return out
	}

	// Session C/closed should not have fills post-policy; treat as anomaly.
	if sess == SessionC || sess == SessionClosed {
		out.QualityTag = QualityAnomaly
		return out
	}

	// A + close or other mismatches
	out.QualityTag = QualityAnomaly
	return out
}

// BuildObservationDayBundle derives an in-memory ObservationDayBundle (read-only).
func BuildObservationDayBundle(in ObservationBundleInput) ObservationDayBundle {
	out := ObservationDayBundle{
		PositionCount:     in.PositionCount,
		OrderCount:        len(in.Orders),
		FillCount:         len(in.Fills),
		FillReasonSummary: map[string]int{},
		SessionSource:     SessionSourceDerived,
		PriceMode:         "unknown",
		QualityTag:        QualityIncomplete,
	}

	var runID, planID uint
	if in.Run != nil {
		runID = in.Run.ID
		planID = in.Run.PlanID
		out.RunID = in.Run.ID
		out.PlanID = in.Run.PlanID
		out.TradeDate = in.Run.TradeDate
		out.Trigger = in.Run.Trigger
		out.Actor = in.Run.Actor
	}

	if in.PersistedSession != "" {
		out.DerivedSession = in.PersistedSession
		out.SessionSource = SessionSourcePersisted
	}

	labels := make([]ObservationFillLabel, 0, len(in.Fills))
	for _, f := range in.Fills {
		lbl := ClassifyFill(runID, planID, out.TradeDate, f)
		labels = append(labels, lbl)
		r := strings.TrimSpace(f.FillReason)
		if r == "" {
			r = "(empty)"
		}
		out.FillReasonSummary[r]++
	}
	out.Fills = labels

	// Bundle session: prefer persisted; else majority/first fill; else run.started_at
	if out.SessionSource != SessionSourcePersisted {
		if len(labels) > 0 {
			out.DerivedSession = labels[0].DerivedSession
			// If fills disagree, keep first but quality will reflect worst tag.
			for _, lbl := range labels[1:] {
				if lbl.DerivedSession != "" && lbl.DerivedSession != out.DerivedSession {
					// keep first session; anomaly aggregation handles mismatch
					break
				}
			}
		} else if in.Run != nil && !in.Run.StartedAt.IsZero() {
			out.DerivedSession = DeriveSessionFromLocalTime(in.Run.StartedAt)
		}
	}

	out.PriceMode = aggregatePriceMode(labels)
	out.QualityTag, out.ExcludedBaseline = aggregateQuality(labels, in.Run)

	return out
}

func aggregatePriceMode(labels []ObservationFillLabel) string {
	if len(labels) == 0 {
		return "unknown"
	}
	mode := labels[0].PriceMode
	for _, lbl := range labels[1:] {
		if lbl.PriceMode != mode {
			return "unknown"
		}
	}
	if mode == "" {
		return "unknown"
	}
	return mode
}

func aggregateQuality(labels []ObservationFillLabel, run *PaperSimRun) (tag string, excluded bool) {
	if len(labels) == 0 {
		if run != nil && IsExactLegacyRun(run.ID, run.PlanID, run.TradeDate) {
			return QualityLegacyBaseline, true
		}
		if run == nil {
			return QualityIncomplete, false
		}
		return QualityIncomplete, false
	}

	hasAnomaly := false
	hasIncomplete := false
	allLegacy := true
	anyLegacy := false
	allExcluded := true

	for _, lbl := range labels {
		switch lbl.QualityTag {
		case QualityAnomaly:
			hasAnomaly = true
			allLegacy = false
		case QualityIncomplete:
			hasIncomplete = true
			allLegacy = false
		case QualityLegacyBaseline:
			anyLegacy = true
		case QualityOK:
			allLegacy = false
		default:
			allLegacy = false
			hasIncomplete = true
		}
		if !lbl.ExcludedBaseline {
			allExcluded = false
		}
	}

	// Worst-first: ANOMALY > INCOMPLETE > LEGACY_BASELINE > OK
	if hasAnomaly {
		return QualityAnomaly, false
	}
	if hasIncomplete {
		return QualityIncomplete, false
	}
	if anyLegacy && allLegacy {
		return QualityLegacyBaseline, allExcluded
	}
	if anyLegacy {
		// mix of OK + legacy → still surface legacy exclusion for baseline filters,
		// but tag ANOMALY? Design: bundle for pure legacy sample is LEGACY.
		// Mixed day: prefer ANOMALY only if non-legacy bad; if OK+legacy, use INCOMPLETE? 
		// Safer: QualityAnomaly=false, use OK for compliant fills but ExcludedBaseline if all legacy excluded.
		// User Case 1 is pure legacy. For mixed, mark ANOMALY is wrong.
		// Use LEGACY_BASELINE only when all fills legacy; else OK if remaining ok and set excluded if any legacy for metrics later.
		return QualityOK, false
	}
	return QualityOK, false
}
