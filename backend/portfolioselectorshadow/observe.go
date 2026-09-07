package portfolioselectorshadow

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/portfolioselection"
	"go-stock/backend/selection"
)

// Runtime runs the read-only Selector Shadow pipeline.
type Runtime struct {
	enabled bool
	now     func() time.Time
	seq     atomic.Uint64
}

var (
	defaultOnce sync.Once
	defaultRT   *Runtime
)

// Default returns the process singleton (Enabled=false).
func Default() *Runtime {
	defaultOnce.Do(func() {
		defaultRT = NewRuntime(Config{})
	})
	return defaultRT
}

// NewRuntime builds an isolated runtime (tests should not mutate Default).
func NewRuntime(cfg Config) *Runtime {
	rt := &Runtime{
		enabled: cfg.Enabled,
		now:     cfg.Now,
	}
	if rt.now == nil {
		rt.now = time.Now
	}
	// DefaultEnabled is false; Config.Enabled must be explicit true to run.
	if !cfg.Enabled {
		rt.enabled = DefaultEnabled
	}
	return rt
}

// Enabled reports whether observation may run.
func (r *Runtime) Enabled() bool {
	if r == nil {
		return false
	}
	return r.enabled
}

// SetEnabled toggles observation (tests / ops). Does not persist TradePlan or flip Controlled adoption.
func (r *Runtime) SetEnabled(on bool) {
	if r == nil {
		return
	}
	r.enabled = on
}

// Observe runs the Shadow pipeline. Panic is recovered; errors become report.failure.
// Never writes TradePlan or touches Execution.
func (r *Runtime) Observe(in Input) (rep *Report) {
	defer func() {
		if rec := recover(); rec != nil {
			rep = &Report{
				SchemaVersion:      SchemaVersion,
				Enabled:            r != nil && r.enabled,
				Skipped:            false,
				Comparable:         false,
				IncomparableReason: IncomparableBothFailed,
				Failure: ShadowFailure{
					Side:        "runtime",
					ErrorReason: fmt.Sprintf("panic: %v", rec),
				},
				DataGaps:       []string{"runtime_panic"},
				AllocationDiff: AllocationDiff{LegacyMethod: "fixed_amount_uniform", PerName: []AllocationNameDiff{}},
				SectorConcentrationDiff: SectorConcentrationDiff{
					BaselineSectorWeights:      map[string]float64{},
					LegacyPostSectorWeights:    map[string]float64{},
					PortfolioPostSectorWeights: map[string]float64{},
					Degraded:                   true,
					DegradedReason:             "runtime_panic",
				},
				RejectedReasonSummary: RejectedReasonSummary{
					ConstructionRejectNote: "construction_reject (not PlanFilter skipped)",
				},
				RecordedAt: time.Now(),
			}
			if r != nil && r.now != nil {
				rep.RecordedAt = r.now()
			}
		}
	}()

	if r == nil {
		r = NewRuntime(Config{})
	}
	now := r.now()
	if in.DecisionTime.IsZero() {
		in.DecisionTime = now
	}

	if !r.enabled {
		return &Report{
			SchemaVersion: SchemaVersion,
			Enabled:       false,
			Skipped:       true,
			SkipReason:    SkipReasonDisabled,
			RecordedAt:    now,
			TradeDate:     strings.TrimSpace(in.TradeDate),
			PoolID:        in.PoolID,
			Trigger:       strings.TrimSpace(in.Trigger),
			DataGaps:      []string{},
			AllocationDiff: AllocationDiff{
				LegacyMethod:    "fixed_amount_uniform",
				PortfolioMethod: "equal_weight",
				PerName:         []AllocationNameDiff{},
			},
			SectorConcentrationDiff: SectorConcentrationDiff{
				BaselineSectorWeights:      map[string]float64{},
				LegacyPostSectorWeights:    map[string]float64{},
				PortfolioPostSectorWeights: map[string]float64{},
			},
			RejectedReasonSummary: RejectedReasonSummary{
				ConstructionRejectNote: "construction_reject (not PlanFilter skipped)",
			},
			Legacy:    LegacySelectionView{Selected: []LegacySelectedName{}, Waitlist: []string{}, RejectedAnnotate: []LegacyAnnotate{}},
			Portfolio: PortfolioFaceRef{Selected: []PortfolioSelectedRef{}, Rejected: []PortfolioRejectedRef{}, SectorWeights: map[string]float64{}},
		}
	}

	runID := strings.TrimSpace(in.RunID)
	if runID == "" {
		runID = fmt.Sprintf("pss-%d", r.seq.Add(1))
	}

	gaps := []string{}
	legacyView, legacyErr := projectLegacy(in)
	portFace, portErr := runPortfolioSelector(in)

	rep = &Report{
		SchemaVersion: SchemaVersion,
		Enabled:       true,
		Skipped:       false,
		RunID:         runID,
		RecordedAt:    now,
		TradeDate:     strings.TrimSpace(in.TradeDate),
		PoolID:        in.PoolID,
		Trigger:       strings.TrimSpace(in.Trigger),
		Legacy:        legacyView,
		Portfolio:     portFace,
		RejectedReasonSummary: RejectedReasonSummary{
			ConstructionRejectNote: "construction_reject (not PlanFilter skipped)",
		},
	}

	if legacyErr != nil && portErr != nil {
		rep.Comparable = false
		rep.IncomparableReason = IncomparableBothFailed
		rep.Failure = ShadowFailure{Side: "both", ErrorReason: legacyErr.Error() + "; " + portErr.Error()}
		gaps = append(gaps, "legacy_failed", "selector_failed")
	} else if legacyErr != nil {
		rep.Comparable = false
		rep.IncomparableReason = IncomparableLegacyFailed
		rep.Failure = ShadowFailure{Side: "legacy", ErrorReason: legacyErr.Error()}
		gaps = append(gaps, "legacy_failed")
	} else if portErr != nil {
		rep.Comparable = false
		rep.IncomparableReason = IncomparableSelectorFailed
		rep.Failure = ShadowFailure{Side: "portfolio", ErrorReason: portErr.Error()}
		gaps = append(gaps, "selector_failed")
	} else {
		rep.Comparable = true
	}

	if in.Snapshot == nil || !in.Snapshot.Found {
		gaps = append(gaps, "snapshot_missing_or_not_found")
	}
	if in.LegacyAmount <= 0 {
		gaps = append(gaps, "legacy_amount_non_positive")
	}
	if len(in.PoolItems) == 0 {
		gaps = append(gaps, "empty_pool_items")
	}

	rep.LegacySelectedCount = len(legacyView.Selected)
	rep.PortfolioSelectedCount = portFace.SelectedCount
	rep.AllocationDiff = diffAllocation(legacyView, portFace, equityOf(in))
	rep.SectorConcentrationDiff = diffSector(in, legacyView, portFace)
	rep.RejectedReasonSummary = summarizeRejects(legacyView, portFace)
	rep.DataGaps = uniqueStrings(gaps)
	return rep
}

// SafeObserve is a swallow-friendly entry: recovers panic and always returns a report.
func SafeObserve(r *Runtime, in Input) *Report {
	if r == nil {
		r = Default()
	}
	return r.Observe(in)
}

func projectLegacy(in Input) (LegacySelectionView, error) {
	amount := in.LegacyAmount
	if amount <= 0 {
		amount = 100_000
	}
	maxN := in.LegacyMaxNames
	if maxN <= 0 {
		maxN = selection.DefaultMaxSelectedNames
	}
	cands := make([]selection.Candidate, 0, len(in.PoolItems))
	for _, it := range in.PoolItems {
		cands = append(cands, selection.Candidate{
			StockCode: it.StockCode,
			StockName: it.StockName,
			Industry:  it.Industry,
			Rank:      it.Rank,
			Score:     it.Score,
			Reason:    it.Reason,
		})
	}
	sel := selection.Select(cands, selection.SelectionContext{MaxSelectedNames: maxN})
	view := LegacySelectionView{
		RankedCount:      len(sel.RankedCandidates),
		SelectionLimit:   sel.SelectionLimit,
		Selected:         []LegacySelectedName{},
		Waitlist:         []string{},
		RejectedAnnotate: []LegacyAnnotate{},
		UniformAmount:    amount,
	}
	for _, d := range sel.PrimaryPicks() {
		c := d.Candidate
		view.Selected = append(view.Selected, LegacySelectedName{
			StockCode: strings.ToLower(strings.TrimSpace(c.StockCode)),
			StockName: c.StockName,
			Industry:  c.Industry,
			Rank:      c.Rank,
			Score:     c.Score,
			Amount:    amount,
		})
		view.AllocatedTotal += amount
	}
	for _, c := range sel.Waitlist() {
		view.Waitlist = append(view.Waitlist, strings.ToLower(strings.TrimSpace(c.StockCode)))
	}
	for _, d := range sel.CandidateDecisions {
		if d.Selected {
			continue
		}
		reason := d.SkippedReason
		if reason == "" {
			reason = d.SelectionReason
		}
		if reason == selection.ReasonOverNameLimit {
			continue // waitlist, not annotate reject for histogram primary
		}
		view.RejectedAnnotate = append(view.RejectedAnnotate, LegacyAnnotate{
			StockCode: strings.ToLower(strings.TrimSpace(d.Candidate.StockCode)),
			Reason:    reason,
		})
	}
	return view, nil
}

func runPortfolioSelector(in Input) (PortfolioFaceRef, error) {
	items := make([]portfolioselection.PoolItem, 0, len(in.PoolItems))
	for _, it := range in.PoolItems {
		items = append(items, portfolioselection.PoolItem{
			StockCode: it.StockCode,
			StockName: it.StockName,
			Industry:  it.Industry,
			Rank:      it.Rank,
			Score:     it.Score,
			Reason:    it.Reason,
		})
	}
	var snap *portfolioselection.Snapshot
	cons := portfolioselection.Constraints{}
	if in.Snapshot != nil {
		snap = &portfolioselection.Snapshot{
			Found:       in.Snapshot.Found,
			Cash:        in.Snapshot.Cash,
			Equity:      in.Snapshot.Equity,
			MarketValue: in.Snapshot.MarketValue,
		}
		for _, p := range in.Snapshot.Positions {
			snap.Positions = append(snap.Positions, portfolioselection.Position{
				StockCode: p.StockCode, MarketValue: p.MarketValue, Industry: p.Industry,
			})
		}
		cons.EquityBase = in.Snapshot.Equity
		cons.AvailableCash = in.Snapshot.Cash * 0.95
		if cons.AvailableCash < 0 {
			cons.AvailableCash = 0
		}
	}
	pol := portfolioselection.DefaultPolicy()
	if in.LegacyMaxNames > 0 {
		pol.MaxNames = in.LegacyMaxNames
	}
	port := portfolioselection.Select(portfolioselection.Input{
		TradeDate:   in.TradeDate,
		PoolID:      in.PoolID,
		Items:       items,
		Snapshot:    snap,
		Policy:      pol,
		Constraints: cons,
	})
	if port == nil {
		return PortfolioFaceRef{}, fmt.Errorf("portfolio selector returned nil")
	}
	face := PortfolioFaceRef{
		SelectedCount: port.Summary.SelectedCount,
		RejectedCount: port.Summary.RejectedCount,
		TotalAmount:   port.Summary.TotalTargetAmount,
		Method:        "equal_weight",
		Selected:      []PortfolioSelectedRef{},
		Rejected:      []PortfolioRejectedRef{},
		SectorWeights: port.Summary.SectorWeights,
		Binding:       port.Summary.Binding,
	}
	if face.SectorWeights == nil {
		face.SectorWeights = map[string]float64{}
	}
	for _, s := range port.Selected {
		face.Selected = append(face.Selected, PortfolioSelectedRef{
			StockCode: s.StockCode, Amount: s.TargetAmount, Weight: s.TargetWeight,
			Industry: s.Industry, AllocationReason: s.AllocationReason,
		})
	}
	for _, r := range port.Rejected {
		face.Rejected = append(face.Rejected, PortfolioRejectedRef{
			StockCode: r.StockCode, RejectReason: r.RejectReason,
		})
	}
	return face, nil
}

func equityOf(in Input) float64 {
	if in.Snapshot != nil && in.Snapshot.Equity > 0 {
		return in.Snapshot.Equity
	}
	return 0
}

func diffAllocation(legacy LegacySelectionView, port PortfolioFaceRef, equity float64) AllocationDiff {
	out := AllocationDiff{
		LegacyMethod:         "fixed_amount_uniform",
		PortfolioMethod:      port.Method,
		LegacyTotalAmount:    legacy.AllocatedTotal,
		PortfolioTotalAmount: port.TotalAmount,
		TotalAmountDelta:     port.TotalAmount - legacy.AllocatedTotal,
		PerName:              []AllocationNameDiff{},
	}
	if out.PortfolioMethod == "" {
		out.PortfolioMethod = "equal_weight"
	}
	leg := map[string]LegacySelectedName{}
	for _, s := range legacy.Selected {
		leg[s.StockCode] = s
	}
	portMap := map[string]PortfolioSelectedRef{}
	for _, s := range port.Selected {
		portMap[s.StockCode] = s
	}
	codes := map[string]struct{}{}
	for c := range leg {
		codes[c] = struct{}{}
	}
	for c := range portMap {
		codes[c] = struct{}{}
	}
	list := make([]string, 0, len(codes))
	for c := range codes {
		list = append(list, c)
	}
	sort.Strings(list)
	for _, c := range list {
		l, lok := leg[c]
		p, pok := portMap[c]
		row := AllocationNameDiff{StockCode: c}
		switch {
		case lok && pok:
			row.Presence = PresenceBoth
			row.LegacyAmount = l.Amount
			row.PortfolioAmount = p.Amount
			row.PortfolioAllocationReason = p.AllocationReason
		case lok:
			row.Presence = PresenceOnlyLegacy
			row.LegacyAmount = l.Amount
		default:
			row.Presence = PresenceOnlyPortfolio
			row.PortfolioAmount = p.Amount
			row.PortfolioAllocationReason = p.AllocationReason
		}
		row.AmountDelta = row.PortfolioAmount - row.LegacyAmount
		if equity > 0 {
			row.LegacyWeight = row.LegacyAmount / equity
			row.PortfolioWeight = row.PortfolioAmount / equity
			row.WeightDelta = row.PortfolioWeight - row.LegacyWeight
		}
		if lok && pok && math.Abs(row.AmountDelta) > AmountEpsilon {
			out.AmountDeltaNameCount++
		}
		out.PerName = append(out.PerName, row)
	}
	return out
}

func diffSector(in Input, legacy LegacySelectionView, port PortfolioFaceRef) SectorConcentrationDiff {
	out := SectorConcentrationDiff{
		BaselineSectorWeights:      map[string]float64{},
		LegacyPostSectorWeights:    map[string]float64{},
		PortfolioPostSectorWeights: map[string]float64{},
	}
	equity := equityOf(in)
	out.EquityBase = equity
	if equity <= 0 {
		out.Degraded = true
		out.DegradedReason = "equity_unavailable"
		return out
	}
	baselineMV := map[string]float64{}
	if in.Snapshot != nil {
		for _, p := range in.Snapshot.Positions {
			ind := industryOrUnknown(p.Industry)
			baselineMV[ind] += p.MarketValue
		}
	}
	for ind, mv := range baselineMV {
		out.BaselineSectorWeights[ind] = mv / equity
	}
	legMV := copyFloatMap(baselineMV)
	for _, s := range legacy.Selected {
		ind := industryOrUnknown(s.Industry)
		if ind == "unknown" {
			// try match from pool
			ind = industryFromPool(in.PoolItems, s.StockCode)
		}
		legMV[ind] += s.Amount
	}
	portMV := copyFloatMap(baselineMV)
	for _, s := range port.Selected {
		ind := industryOrUnknown(s.Industry)
		portMV[ind] += s.Amount
	}
	out.LegacyPostSectorWeights = weightsFromMV(legMV, equity)
	out.PortfolioPostSectorWeights = weightsFromMV(portMV, equity)
	out.LegacyMaxSectorWeight = maxWeight(out.LegacyPostSectorWeights)
	out.PortfolioMaxSectorWeight = maxWeight(out.PortfolioPostSectorWeights)
	out.MaxSectorDelta = out.PortfolioMaxSectorWeight - out.LegacyMaxSectorWeight
	return out
}

func summarizeRejects(legacy LegacySelectionView, port PortfolioFaceRef) RejectedReasonSummary {
	out := RejectedReasonSummary{
		ConstructionRejectNote:   "construction_reject (not PlanFilter skipped)",
		PortfolioRejectHistogram: []ReasonCount{},
		LegacyAnnotateHistogram:  []ReasonCount{},
	}
	ph := map[string]int{}
	for _, r := range port.Rejected {
		ph[r.RejectReason]++
	}
	out.PortfolioRejectHistogram = histToSlice(ph)
	lh := map[string]int{}
	for _, a := range legacy.RejectedAnnotate {
		lh[a.Reason]++
	}
	out.LegacyAnnotateHistogram = histToSlice(lh)
	return out
}

func histToSlice(m map[string]int) []ReasonCount {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ReasonCount, 0, len(keys))
	for _, k := range keys {
		out = append(out, ReasonCount{Reason: k, Count: m[k]})
	}
	return out
}

func industryOrUnknown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}

func industryFromPool(items []PoolItemView, code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, it := range items {
		if strings.ToLower(strings.TrimSpace(it.StockCode)) == code {
			return industryOrUnknown(it.Industry)
		}
	}
	return "unknown"
}

func copyFloatMap(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func weightsFromMV(mv map[string]float64, equity float64) map[string]float64 {
	out := map[string]float64{}
	if equity <= 0 {
		return out
	}
	for k, v := range mv {
		out[k] = v / equity
	}
	return out
}

func maxWeight(m map[string]float64) float64 {
	max := 0.0
	for _, v := range m {
		if v > max {
			max = v
		}
	}
	return max
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
