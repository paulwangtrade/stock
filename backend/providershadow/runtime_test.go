package providershadow

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfoliolayer"

	"github.com/stretchr/testify/require"
)

type failPortfolio struct {
	code string
	msg  string
	boom bool
}

func (p failPortfolio) Name() string { return decisionprovider.ProviderPortfolioAllocation }

func (p failPortfolio) Decide(ctx decisionprovider.DecisionContext) (*decisionprovider.DecisionEnvelope, error) {
	if p.boom {
		panic("portfolio boom")
	}
	err := &decisionprovider.DecisionError{Code: p.code, Message: p.msg}
	return &decisionprovider.DecisionEnvelope{
		OK:           false,
		Provider:     p.Name(),
		DecisionTime: ctx.DecisionTime,
		Error:        err,
		Lines:        []decisionprovider.DecisionLine{},
		Rejected:     []decisionprovider.RejectedLine{},
	}, err
}

func TestDefaultEnabledIsFalse(t *testing.T) {
	require.False(t, DefaultEnabled)
	require.False(t, Default().Enabled())
}

func TestRuntime_DisabledDoesNotCallPortfolioOrPersist(t *testing.T) {
	store := NewMemoryStore()
	port := &countingProvider{inner: decisionprovider.NewPortfolioDecisionProvider()}
	rt := NewRuntime(Config{
		Enabled:   false,
		Portfolio: port,
		Store:     store,
		Now:       g12Time,
	})
	sel := g12Sel("sz000002", "sz000003")
	rec, err := rt.Run(g12Ctx(sel, 100_000, nil), "2026-08-21")
	require.NoError(t, err)
	require.Nil(t, rec)
	require.Empty(t, store.List())
	require.Equal(t, 0, port.calls)
}

func TestRuntime_EnabledWritesComparisonOnly(t *testing.T) {
	store := NewMemoryStore()
	rt := NewRuntime(Config{
		Enabled: true,
		Store:   store,
		Now:     g12Time,
	})
	sel := g12Sel("sz000002", "sz000003", "sz000004")
	snap := portfoliolayer.FromLedger(nil)
	rec, err := rt.Run(g12Ctx(sel, 100_000, snap), "2026-08-21")
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, SchemaVersionG61, rec.SchemaVersion)
	require.Equal(t, ProviderVersionStamp, rec.ProviderVersion)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.LegacyProviderVersion)
	require.Equal(t, decisionprovider.ProviderPortfolioAllocation, rec.PortfolioProviderVersion)
	require.Equal(t, ConstraintVersionF21, rec.ConstraintVersion)
	require.Equal(t, decisionprovider.AllocVersionF1Equal, rec.AllocationVersion)
	require.NotEmpty(t, rec.ContextFingerprint)
	require.NotEmpty(t, rec.Fingerprint)
	require.Len(t, store.List(), 1)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.Report.ChainProvider)
	require.True(t, rec.Report.RecordOnly)
	require.True(t, rec.Report.NotATradePlan)
}

func TestRuntime_FingerprintStableOnSameInput(t *testing.T) {
	rt := NewRuntime(Config{Enabled: true, Store: NewMemoryStore(), Now: g12Time})
	sel := g12Sel("sz000002", "sz000003", "sz000004")
	ctx := g12Ctx(sel, 100_000, portfoliolayer.FromLedger(nil))
	a, err := rt.Run(ctx, "2026-08-21")
	require.NoError(t, err)
	b, err := rt.Run(ctx, "2026-08-21")
	require.NoError(t, err)
	require.Equal(t, a.ContextFingerprint, b.ContextFingerprint)
	require.Equal(t, a.Fingerprint, b.Fingerprint)
	require.NotEqual(t, a.RunID, b.RunID)
}

func TestRuntime_PortfolioFailureRecordsShadowFailure(t *testing.T) {
	store := NewMemoryStore()
	rt := NewRuntime(Config{
		Enabled:   true,
		Store:     store,
		Now:       g12Time,
		Portfolio: failPortfolio{code: decisionprovider.ErrCodeZeroAmount, msg: "allocation error"},
	})
	rec, err := rt.Run(g12Ctx(g12Sel("sz000002"), 100_000, nil), "2026-08-21")
	require.NoError(t, err)
	require.True(t, rec.Failure.PortfolioFailed)
	require.Contains(t, rec.Failure.ErrorReason, decisionprovider.ErrCodeZeroAmount)
	require.False(t, rec.Comparable)
	require.Equal(t, IncomparablePortfolioFailed, rec.IncomparableReason)
	require.Equal(t, 0, rec.ComparisonSummary.CommonCount)
}

func TestRuntime_PortfolioPanicIsolated(t *testing.T) {
	rt := NewRuntime(Config{
		Enabled:   true,
		Store:     NewMemoryStore(),
		Now:       g12Time,
		Portfolio: failPortfolio{boom: true},
	})
	rec, err := rt.Run(g12Ctx(g12Sel("sz000002"), 100_000, nil), "2026-08-21")
	require.NoError(t, err)
	require.True(t, rec.Failure.PortfolioFailed)
	require.Contains(t, rec.Failure.ErrorReason, ErrRuntimePanic)
}

func TestRuntime_ObserveRejectsNonDraftTrigger(t *testing.T) {
	store := NewMemoryStore()
	rt := NewRuntime(Config{Enabled: true, Store: store, Now: g12Time})
	got := rt.Observe(ObserveInput{
		Trigger:       "morning_adopt",
		Selection:     g12Sel("sz000001"),
		UniformAmount: 100_000,
		DecisionTime:  g12Time(),
	})
	require.Nil(t, got)
	require.Empty(t, store.List())
}

func TestFileStore_AppendRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shadow.jsonl")
	store := NewFileStore(path)
	rt := NewRuntime(Config{Enabled: true, Store: store, Now: g12Time})
	_, err := rt.Run(g12Ctx(g12Sel("sz000002"), 100_000, nil), "2026-08-21")
	require.NoError(t, err)
	require.Len(t, store.List(), 1)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), SchemaVersionG61)
	require.NotContains(t, string(raw), `"plan_id"`)
}

func TestRuntime_ObserveAfterCloseDraftPersists(t *testing.T) {
	store := NewMemoryStore()
	fixed := time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC)
	rt := NewRuntime(Config{Enabled: true, Store: store, Now: func() time.Time { return fixed }})
	rec := rt.Observe(ObserveInput{
		Trigger:       TriggerAfterCloseDraft,
		Selection:     g12Sel("sz000002", "sz000003"),
		UniformAmount: 100_000,
		DecisionTime:  fixed,
		TradeDate:     "2026-08-21",
		Snapshot:      portfoliolayer.FromLedger(nil),
	})
	require.NotNil(t, rec)
	require.Equal(t, "2026-08-21", rec.TradeDate)
	require.Len(t, store.List(), 1)
}

func TestRuntime_DoesNotSwapChainEnvelopeToPortfolio(t *testing.T) {
	legacy := decisionprovider.NewLegacyDecisionProvider(nil)
	sel := g12Sel("sz000002", "sz000003")
	ctx := g12Ctx(sel, 100_000, nil)
	cmp := CompareProviders(ctx, true, legacy, decisionprovider.NewPortfolioDecisionProvider())
	require.Equal(t, decisionprovider.ProviderFixedAmount, cmp.ChainEnvelope.Provider)
	for _, ln := range cmp.ChainEnvelope.Lines {
		require.Equal(t, 100_000.0, ln.TargetAmount)
	}
}
