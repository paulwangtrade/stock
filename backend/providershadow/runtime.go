package providershadow

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"
)

// ObserveInput is the Draft-path sidecar input. Trigger must be after_close_draft.
type ObserveInput struct {
	Trigger       string
	Selection     *selection.CandidateSelectionResult
	UniformAmount float64
	DecisionTime  time.Time
	TradeDate     string
	Ledger        *portfolio.Snapshot
	Constraints   portfoliolayer.ConstraintSet
	Snapshot      *portfoliolayer.PortfolioSnapshot
	Budget        *portfoliolayer.AllocationBudget
	// Risk is optional H0.2 RiskView for Portfolio-side PortfolioRiskSnapshot (read-only).
	Risk *tradingconfig.RiskView
}

// Config constructs a Runtime. Enabled defaults to DefaultEnabled (false).
type Config struct {
	Enabled    bool
	Timeout    time.Duration
	Legacy     decisionprovider.DecisionProvider
	Portfolio  decisionprovider.DecisionProvider
	Store      RecordStore
	Now        func() time.Time
	LoadLedger func() *portfolio.Snapshot
	Risk       *tradingconfig.RiskView
}

// Runtime compares both providers and appends ShadowComparisonRecord.
type Runtime struct {
	enabled    bool
	timeout    time.Duration
	legacy     decisionprovider.DecisionProvider
	portfolio  decisionprovider.DecisionProvider
	store      RecordStore
	now        func() time.Time
	loadLedger func() *portfolio.Snapshot
	risk       *tradingconfig.RiskView
	seq        atomic.Uint64
}

var (
	defaultOnce sync.Once
	defaultRT   *Runtime
)

// Default returns the process singleton. Enabled is false; it does not take over the write chain.
func Default() *Runtime {
	defaultOnce.Do(func() {
		defaultRT = NewRuntime(Config{})
	})
	return defaultRT
}

// NewRuntime builds an isolated runtime (tests should not mutate Default).
func NewRuntime(cfg Config) *Runtime {
	rt := &Runtime{
		enabled:    cfg.Enabled,
		timeout:    cfg.Timeout,
		legacy:     cfg.Legacy,
		portfolio:  cfg.Portfolio,
		store:      cfg.Store,
		now:        cfg.Now,
		loadLedger: cfg.LoadLedger,
		risk:       cfg.Risk,
	}
	if rt.timeout <= 0 {
		rt.timeout = 3 * time.Second
	}
	if rt.legacy == nil {
		rt.legacy = decisionprovider.NewLegacyDecisionProvider(nil)
	}
	if rt.portfolio == nil {
		rt.portfolio = decisionprovider.NewPortfolioDecisionProvider()
	}
	if rt.store == nil {
		rt.store = NewMemoryStore()
	}
	if rt.now == nil {
		rt.now = time.Now
	}
	return rt
}

func (r *Runtime) Enabled() bool {
	return r != nil && r.enabled
}

// Observe is the Draft hook. Unknown triggers and Enabled=false are no-ops.
// Panics, Portfolio failures, and persist errors never propagate.
func (r *Runtime) Observe(in ObserveInput) (rec *ShadowComparisonRecord) {
	if r == nil || !r.enabled {
		return nil
	}
	if in.Trigger != TriggerAfterCloseDraft {
		return nil
	}
	defer func() {
		if recover() != nil {
			rec = nil
		}
	}()
	ctx := r.buildContext(in)
	out, err := r.RunPrepared(ctx, in.TradeDate, PrepareOptions{Risk: in.Risk, Ledger: in.Ledger})
	if err != nil || out == nil {
		return out
	}
	return out
}

// Run compares both providers on ctx and appends a record. Portfolio failure is stored, not returned.
func (r *Runtime) Run(ctx decisionprovider.DecisionContext, tradeDate string) (*ShadowComparisonRecord, error) {
	return r.RunPrepared(ctx, tradeDate, PrepareOptions{Risk: r.risk})
}

// RunPrepared is Run with explicit Portfolio risk prepare options (H0.2 + H.3).
func (r *Runtime) RunPrepared(ctx decisionprovider.DecisionContext, tradeDate string, prepOpts PrepareOptions) (*ShadowComparisonRecord, error) {
	if r == nil || !r.enabled {
		return nil, nil
	}
	if ctx.DecisionTime.IsZero() {
		ctx.DecisionTime = r.now()
	}
	if ctx.Version.Contract == "" {
		ctx.Version.Contract = decisionprovider.ContractG21
	}
	if ctx.Version.Constraint == "" {
		ctx.Version.Constraint = ConstraintVersionF21
	}
	if ctx.Version.Allocation == "" {
		ctx.Version.Allocation = decisionprovider.AllocVersionF1Equal
	}
	if ctx.Version.Selection == "" {
		ctx.Version.Selection = SelectionVersionE61
	}
	if prepOpts.Risk == nil {
		prepOpts.Risk = r.risk
	}

	cmp := r.compareWithTimeout(ctx, prepOpts)
	report := cmp.Report
	rec := ShadowComparisonRecord{
		SchemaVersion:            SchemaVersionG61,
		RunID:                    r.nextRunID(),
		RecordedAt:               r.now(),
		DecisionTime:             ctx.DecisionTime,
		TradeDate:                tradeDate,
		ContextFingerprint:       contextFingerprint(ctx),
		ProviderVersion:          ProviderVersionStamp,
		LegacyProviderVersion:    decisionprovider.ProviderFixedAmount,
		PortfolioProviderVersion: decisionprovider.ProviderPortfolioAllocation,
		ConstraintVersion:        constraintVersionOf(ctx),
		AllocationVersion:        allocationVersionOf(ctx),
		SelectionVersion:         SelectionVersionE61,
		ComparatorVersion:        ComparatorVersionH31,
		ContractVersion:          decisionprovider.ContractG21,
		Report:                   report,
	}
	if report != nil {
		stampReportFlags(report)
		rec.Comparable = report.Comparable
		rec.IncomparableReason = report.IncomparableReason
		rec.ComparisonSummary = summaryFromReport(report)
		rec.LegacySummary = report.Legacy
		rec.PortfolioSummary = report.Portfolio
		fail := failureFromReport(report)
		rec.Failure = fail
		rec.FailureSummary = fail
	}
	enrichAllocationShadowRecord(&rec, report, cmp.RiskAdjustment)
	rec.Fingerprint = reportFingerprint(rec, report)
	if err := r.store.Append(rec); err != nil {
		return &rec, err
	}
	return &rec, nil
}

func (r *Runtime) compareWithTimeout(ctx decisionprovider.DecisionContext, prepOpts PrepareOptions) CompareResult {
	type box struct{ cmp CompareResult }
	ch := make(chan box, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				fail := closedFail(ctx, decisionprovider.ProviderPortfolioAllocation, ErrRuntimePanic, panicText(rec))
				legacyCtx := ctx
				legacyCtx.Version.Provider = decisionprovider.ProviderFixedAmount
				legacyEnv := decideSafe(r.legacy, legacyCtx, decisionprovider.ProviderFixedAmount)
				report := DiffEnvelopes(legacyEnv, fail, ctx.DecisionTime, rankedOrNil(ctx))
				prep := PreparePortfolioSide(ctx, prepOpts)
				stampReportFlags(&report)
				report.RiskAdjustment = prep.Trace
				ch <- box{CompareResult{
					ChainEnvelope:  legacyEnv,
					Report:         &report,
					RiskAdjustment: prep.Trace,
				}}
			}
		}()
		ch <- box{CompareProvidersPrepared(ctx, true, r.legacy, r.portfolio, prepOpts)}
	}()
	select {
	case got := <-ch:
		return got.cmp
	case <-time.After(r.timeout):
		legacyCtx := ctx
		legacyCtx.Version.Provider = decisionprovider.ProviderFixedAmount
		legacyEnv := decideSafe(r.legacy, legacyCtx, decisionprovider.ProviderFixedAmount)
		fail := closedFail(ctx, decisionprovider.ProviderPortfolioAllocation, ErrRuntimeTimeout, "portfolio shadow timed out")
		report := DiffEnvelopes(legacyEnv, fail, ctx.DecisionTime, rankedOrNil(ctx))
		prep := PreparePortfolioSide(ctx, prepOpts)
		stampReportFlags(&report)
		report.RiskAdjustment = prep.Trace
		return CompareResult{
			ChainEnvelope:  legacyEnv,
			Report:         &report,
			RiskAdjustment: prep.Trace,
		}
	}
}

func rankedOrNil(ctx decisionprovider.DecisionContext) []selection.Candidate {
	if ctx.Selection == nil {
		return nil
	}
	return ctx.Selection.RankedCandidates
}

func (r *Runtime) buildContext(in ObserveInput) decisionprovider.DecisionContext {
	now := in.DecisionTime
	if now.IsZero() {
		now = r.now()
	}
	snap := in.Snapshot
	if snap == nil {
		ledger := in.Ledger
		if ledger == nil && r.loadLedger != nil {
			func() {
				defer func() { _ = recover() }()
				ledger = r.loadLedger()
			}()
		}
		if ledger == nil {
			func() {
				defer func() { _ = recover() }()
				got, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
				if err == nil {
					ledger = got
				}
			}()
		}
		snap = portfoliolayer.FromLedger(ledger)
	}
	return decisionprovider.DecisionContext{
		Selection:     in.Selection,
		DecisionTime:  now,
		UniformAmount: in.UniformAmount,
		Snapshot:      snap,
		Constraints:   in.Constraints,
		Budget:        in.Budget,
		Version: decisionprovider.DecisionVersion{
			Contract:   decisionprovider.ContractG21,
			Selection:  SelectionVersionE61,
			Constraint: ConstraintVersionF21,
			Allocation: decisionprovider.AllocVersionF1Equal,
		},
	}
}

func (r *Runtime) nextRunID() string {
	n := r.seq.Add(1)
	return fmt.Sprintf("shadow-%d-%d", r.now().UTC().UnixNano(), n)
}

// Store exposes the record sink for tests.
func (r *Runtime) Store() RecordStore {
	if r == nil {
		return nil
	}
	return r.store
}
