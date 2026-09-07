package holdingdecision_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func ptr(f float64) *float64 { return &f }

func TestObserve_DefaultPolicy_AllHold_PersistFalse(t *testing.T) {
	t.Parallel()
	ret := -0.12
	in := holdingdecision.ObservationInput{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000001", Weight: 0.35, TotalQty: 1000, MarketValue: 350_000},
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {Symbol: "sz000001", CanSell: true, TotalQty: 1000, AvailableQty: 1000},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {
				Symbol: "sz000001", ReturnRate: &ret, CurrentPrice: ptr(10),
				RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong,
			},
		},
		Policy: holdingdecision.DefaultActionPolicy(),
	}
	got := holdingdecision.Observe(in)
	require.False(t, got.PersistSellPlans)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotExecution)
	require.True(t, got.NotSellTradePlan)
	require.True(t, got.NotBuyChain)
	require.Len(t, got.Decisions, 1)
	require.Equal(t, holdingdecision.ActionHold, got.Decisions[0].Action)
	require.Equal(t, 1, got.ByAction[holdingdecision.ActionHold])
	require.False(t, got.Decisions[0].PersistSellPlans)
}

func TestObserve_Deterministic(t *testing.T) {
	t.Parallel()
	in := sampleInput(true)
	in.Policy.ExitEnabled = true
	a := holdingdecision.Observe(in)
	b := holdingdecision.Observe(in)
	require.Equal(t, a.InputsFingerprint, b.InputsFingerprint)
	ja, err := json.Marshal(a)
	require.NoError(t, err)
	jb, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
}

func TestObserve_T1Locked_ActionExit_ExecutableFalse(t *testing.T) {
	t.Parallel()
	in := sampleInput(false) // T+1 locked
	in.Policy.ExitEnabled = true
	in.Policy.PersistSellPlans = true // must still not persist / executable false
	got := holdingdecision.Observe(in)
	require.False(t, got.PersistSellPlans, "observation forces persist false")
	require.Len(t, got.Decisions, 1)
	d := got.Decisions[0]
	require.Equal(t, holdingdecision.ActionExit, d.Action)
	require.False(t, d.ExecutableHint)
	require.Contains(t, d.ReasonCodes, holdingdecision.ReasonT1Locked)
	require.NotNil(t, d.Exit)
	require.Equal(t, holdingdecision.ExitIntentFlattenSellable, d.Exit.Intent)
	require.True(t, d.RecordOnly)
	require.True(t, d.NotAnOrder)
}

func TestObserve_CanSell_ExecutableTrue(t *testing.T) {
	t.Parallel()
	in := sampleInput(true)
	in.Policy.ExitEnabled = true
	got := holdingdecision.Observe(in)
	d := got.Decisions[0]
	require.Equal(t, holdingdecision.ActionExit, d.Action)
	require.True(t, d.ExecutableHint)
}

func TestObserve_ReduceWhenConcentration(t *testing.T) {
	t.Parallel()
	ret := 0.02
	cap := 0.20
	in := holdingdecision.ObservationInput{
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000002", Weight: 0.35, TotalQty: 500},
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000002": {CanSell: true, TotalQty: 500, AvailableQty: 500},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000002": {
				Symbol: "sz000002", ReturnRate: &ret, CurrentPrice: ptr(11),
				RiskState: papertrading.RiskStateNormal, PeriodState: papertrading.HoldingPeriodMid,
			},
		},
		RiskSnapshot: &holdingdecision.RiskSnapshotFact{MaxSingleNamePct: &cap, Found: true},
		Policy: holdingdecision.ActionPolicy{
			ReduceEnabled:         true,
			DefaultReduceFraction: 0.5,
		},
	}
	got := holdingdecision.Observe(in)
	d := got.Decisions[0]
	require.Equal(t, holdingdecision.ActionReduce, d.Action)
	require.Contains(t, d.ReasonCodes, holdingdecision.ReasonConcentration)
	require.NotNil(t, d.Reduce)
	require.True(t, d.ExecutableHint)
}

func TestObserve_NoForbiddenSellVerbs(t *testing.T) {
	t.Parallel()
	in := sampleInput(false)
	in.Policy.ExitEnabled = true
	got := holdingdecision.Observe(in)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	s := strings.ToUpper(string(raw))
	require.NotContains(t, s, "EXIT_NOW")
	require.NotContains(t, s, "FORCE_CLOSE")
	require.NotContains(t, s, "SELL_APPROVED")
}

func TestPackage_NoExecutionOrSellPlanOrBuyChain(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	forbiddenImports := []string{
		"go-stock/backend/execution",
		"go-stock/backend/strategy",
		"go-stock/backend/models",
		"go-stock/backend/data",
		"go-stock/backend/risk",
		"go-stock/backend/decisionprovider",
	}
	forbiddenSrc := []string{
		"BuildDraftTSellTradePlan",
		"CreatePlanWithItems",
		"ExecutePlanItem",
		"FilterPoolForTradePlan",
		"BuildDraftTradePlanFromCandidatePool",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, e.Name()))
		require.NoError(t, err, e.Name())
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, e.Name())
		}
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p, "%s imports %s", e.Name(), bad)
			}
		}
	}
}

func TestStrategyBuyChain_DoesNotImportHoldingDecision(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "strategy")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, e.Name()))
		require.NoError(t, err, e.Name())
		require.NotContains(t, string(src), "go-stock/backend/holdingdecision", e.Name())
	}
}

func sampleInput(canSell bool) holdingdecision.ObservationInput {
	ret := -0.15
	avail := int64(0)
	if canSell {
		avail = 800
	}
	return holdingdecision.ObservationInput{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000001", Weight: 0.25, TotalQty: 1000, MarketValue: 250_000},
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {
				Symbol: "sz000001", CanSell: canSell, TotalQty: 1000,
				AvailableQty: avail, LockedQty: 1000 - avail, State: "S1_NEW_LOCKED",
			},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {
				Symbol: "sz000001", ReturnRate: &ret, CurrentPrice: ptr(9.0),
				RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong,
				ProfitState: papertrading.ProfitStateLoss,
			},
		},
		StrategyScores: map[string]holdingdecision.StrategyScoreFact{
			"sz000001": {Symbol: "sz000001", Score: ptr(42)},
		},
		RiskSnapshot: &holdingdecision.RiskSnapshotFact{
			MaxSingleNamePct: ptr(0.20), Found: true, MarketRegime: "configured_level_3",
		},
		Policy: holdingdecision.DefaultActionPolicy(),
	}
}
