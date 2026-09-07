package execution

// Phase6.5.7.4.2.1 Safety Gate Runtime Acceptance Harness (tests only).
// Does not modify safetygate production logic, OMS, Order Schema, or connect real brokers.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/execution/safetygate"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// Acceptance aliases (map to production blocker codes; gate.go unchanged).
const (
	AccBlockNoPrice         = safetygate.BlockSpecPriceInvalid    // BLOCK_NO_PRICE
	AccBlockNoVolume        = safetygate.BlockSpecVolumeInvalid   // BLOCK_NO_VOLUME
	AccBlockRuntimeOverride = safetygate.BlockRuntimePriceOverride // + volume override
	AccBlockNotFrozen       = safetygate.BlockPlanNotFrozen
)

// gateAuditRecord is test-level capture only (no DB / no schema change).
type gateAuditRecord struct {
	SubmitAttemptTime time.Time
	PlanID            uint
	OrderID           string
	SpecHash          string
	GateResult        string // allowed | blocked
	Blockers          []string
	PortCalled        bool
	SubmitCalls       int
	LastIntent        SubmitIntent
}

func acceptanceFrozenPlan(id uint) *models.TradePlan {
	now := time.Now()
	return &models.TradePlan{
		ID:         id,
		Status:     models.TradePlanStatusReady,
		ApprovedAt: &now,
		ApprovedBy: "runtime-acceptance",
		FreezeAt:   &now,
		FreezeBy:   "runtime-acceptance",
	}
}

func acceptanceValidItem() models.TradePlanItem {
	return models.TradePlanItem{
		ID:           42,
		PlanID:       7,
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		LimitPrice:   10.0,
		TargetVolume: 1000,
		Status:       models.TradePlanItemPending,
	}
}

func computeSpecHash(planID uint, item models.TradePlanItem, freezeAt *time.Time) string {
	freezeUnix := int64(0)
	if freezeAt != nil {
		freezeUnix = freezeAt.Unix()
	}
	raw := fmt.Sprintf("%d|%d|%s|%s|%.8f|%d|%d",
		planID, item.ID, item.StockCode, item.Side, item.LimitPrice, item.TargetVolume, freezeUnix)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

// runSafetyGateAcceptance drives ExecutePlanItem and captures attempt-level audit.
func runSafetyGateAcceptance(
	t *testing.T,
	port *stubPort,
	item models.TradePlanItem,
	opts ExecutePlanItemOpts,
) (*gateAuditRecord, error) {
	t.Helper()
	attempt := time.Now()
	planID := uint(0)
	var freezeAt *time.Time
	if opts.Plan != nil {
		planID = opts.Plan.ID
		freezeAt = opts.Plan.FreezeAt
	}

	gatePrice := item.LimitPrice
	gateVol := item.TargetVolume
	if opts.Price > 0 {
		gatePrice = opts.Price
	}
	if opts.Volume > 0 {
		gateVol = opts.Volume
	}
	skipFrozen := opts.SkipSafetyFrozen || opts.Plan == nil
	spec := safetygate.SpecFromItem(item)
	gate := safetygate.ValidateBrokerSubmit(spec, safetygate.Context{
		Plan:            opts.Plan,
		SubmitPrice:     gatePrice,
		SubmitVolume:    gateVol,
		SkipFrozenCheck: skipFrozen,
	})

	rec := &gateAuditRecord{
		SubmitAttemptTime: attempt,
		PlanID:            planID,
		SpecHash:          computeSpecHash(planID, item, freezeAt),
		GateResult:        "blocked",
		Blockers:          append([]string(nil), gate.Blockers...),
	}
	if gate.Allowed {
		rec.GateResult = "allowed"
	}

	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	})
	order, err := svc.ExecutePlanItem(context.Background(), item, opts)
	rec.SubmitCalls = port.calls
	rec.PortCalled = port.calls > 0
	rec.LastIntent = port.lastIntent
	if order != nil {
		rec.OrderID = order.ID
	}
	return rec, err
}

func blockersContain(blockers []string, code string) bool {
	for _, b := range blockers {
		if strings.Contains(b, code) {
			return true
		}
	}
	return false
}

func TestRuntimeAcceptance_P1_FrozenValid_PortCalled(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "ord-p1", Status: "pending", StockCode: "sz000001"}}
	item := acceptanceValidItem()
	plan := acceptanceFrozenPlan(7)
	opts := ExecutePlanItemOpts{
		Price: item.LimitPrice, Volume: item.TargetVolume,
		Plan: plan, AutoFill: false,
	}

	rec, err := runSafetyGateAcceptance(t, port, item, opts)
	require.NoError(t, err)
	require.Equal(t, "allowed", rec.GateResult)
	require.True(t, rec.PortCalled)
	require.Equal(t, 1, rec.SubmitCalls)
	require.Equal(t, "ord-p1", rec.OrderID)
	require.Equal(t, uint(7), rec.PlanID)
	require.NotEmpty(t, rec.SpecHash)
	require.False(t, rec.SubmitAttemptTime.IsZero())
	require.InDelta(t, item.LimitPrice, rec.LastIntent.Price, 1e-9)
	require.Equal(t, item.TargetVolume, rec.LastIntent.Volume)
	require.Equal(t, item.StockCode, rec.LastIntent.StockCode)
}

func TestRuntimeAcceptance_N1_NotFrozen_PortNotCalled(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "should-not"}}
	item := acceptanceValidItem()
	plan := &models.TradePlan{ID: 8, Status: models.TradePlanStatusDraft}
	opts := ExecutePlanItemOpts{
		Price: item.LimitPrice, Volume: item.TargetVolume, Plan: plan,
	}

	rec, err := runSafetyGateAcceptance(t, port, item, opts)
	require.Error(t, err)
	require.Equal(t, "blocked", rec.GateResult)
	require.True(t, blockersContain(rec.Blockers, AccBlockNotFrozen))
	require.Contains(t, err.Error(), AccBlockNotFrozen)
	require.False(t, rec.PortCalled)
	require.Equal(t, 0, rec.SubmitCalls)
	require.Empty(t, rec.OrderID)
	require.Equal(t, uint(8), rec.PlanID)
	require.NotEmpty(t, rec.SpecHash)
}

func TestRuntimeAcceptance_N2_NoPrice_PortNotCalled(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "x"}}
	item := acceptanceValidItem()
	item.LimitPrice = 0
	plan := acceptanceFrozenPlan(7)
	opts := ExecutePlanItemOpts{Price: 10, Volume: item.TargetVolume, Plan: plan}

	rec, err := runSafetyGateAcceptance(t, port, item, opts)
	require.Error(t, err)
	require.Equal(t, "blocked", rec.GateResult)
	require.True(t, blockersContain(rec.Blockers, AccBlockNoPrice), "BLOCK_NO_PRICE → %s", AccBlockNoPrice)
	require.Contains(t, err.Error(), AccBlockNoPrice)
	require.False(t, rec.PortCalled)
	require.Equal(t, 0, rec.SubmitCalls)
	require.Empty(t, rec.OrderID)
}

func TestRuntimeAcceptance_N3_NoVolume_PortNotCalled(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "x"}}
	item := acceptanceValidItem()
	item.TargetVolume = 0
	plan := acceptanceFrozenPlan(7)
	opts := ExecutePlanItemOpts{Price: item.LimitPrice, Volume: 1000, Plan: plan}

	rec, err := runSafetyGateAcceptance(t, port, item, opts)
	require.Error(t, err)
	require.Equal(t, "blocked", rec.GateResult)
	require.True(t, blockersContain(rec.Blockers, AccBlockNoVolume), "BLOCK_NO_VOLUME → %s", AccBlockNoVolume)
	require.Contains(t, err.Error(), AccBlockNoVolume)
	require.False(t, rec.PortCalled)
	require.Equal(t, 0, rec.SubmitCalls)
	require.Empty(t, rec.OrderID)
}

func TestRuntimeAcceptance_N4_RuntimeOverride_PortNotCalled(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "x"}}
	item := acceptanceValidItem()
	plan := acceptanceFrozenPlan(7)
	opts := ExecutePlanItemOpts{
		Price: 12.5, Volume: 800, // diverge from Spec
		Plan: plan,
	}

	rec, err := runSafetyGateAcceptance(t, port, item, opts)
	require.Error(t, err)
	require.Equal(t, "blocked", rec.GateResult)
	require.True(t, blockersContain(rec.Blockers, AccBlockRuntimeOverride) ||
		blockersContain(rec.Blockers, safetygate.BlockRuntimeVolumeOverride),
		"BLOCK_RUNTIME_OVERRIDE")
	require.False(t, rec.PortCalled)
	require.Equal(t, 0, rec.SubmitCalls)
	require.Empty(t, rec.OrderID)
	require.NotEmpty(t, rec.SpecHash)
	require.False(t, rec.SubmitAttemptTime.IsZero())
}

func TestRuntimeAcceptance_AuditFieldsPresent(t *testing.T) {
	// P1 audit shape; N1 ensures blocked attempts still capture fields without order_id.
	port := &stubPort{order: &broker.TradeOrder{ID: "aud-1"}}
	item := acceptanceValidItem()
	plan := acceptanceFrozenPlan(77)
	rec, err := runSafetyGateAcceptance(t, port, item, ExecutePlanItemOpts{
		Price: item.LimitPrice, Volume: item.TargetVolume, Plan: plan,
	})
	require.NoError(t, err)
	require.False(t, rec.SubmitAttemptTime.IsZero())
	require.Equal(t, uint(77), rec.PlanID)
	require.Equal(t, "aud-1", rec.OrderID)
	require.Len(t, rec.SpecHash, 32) // 16-byte hex
	require.Equal(t, "allowed", rec.GateResult)
}

func TestRuntimeAcceptance_PaperAndRealStub_SameFrozenSpec(t *testing.T) {
	item := acceptanceValidItem()
	plan := acceptanceFrozenPlan(7)
	opts := ExecutePlanItemOpts{
		Price: item.LimitPrice, Volume: item.TargetVolume,
		Plan: plan, AutoFill: false, AccountID: 1,
	}

	paperPort := &stubPort{order: &broker.TradeOrder{ID: "paper-1", Status: "pending"}}
	realPort := &stubPort{order: &broker.TradeOrder{ID: "real-1", Status: "pending"}}

	paperRec, err := runSafetyGateAcceptance(t, paperPort, item, opts)
	require.NoError(t, err)
	// Reset opts plan pointer ok; re-run with fresh real port
	realRec, err := runSafetyGateAcceptance(t, realPort, item, opts)
	require.NoError(t, err)

	require.Equal(t, "allowed", paperRec.GateResult)
	require.Equal(t, "allowed", realRec.GateResult)
	require.Equal(t, paperRec.SpecHash, realRec.SpecHash)
	require.Equal(t, paperRec.LastIntent.Price, realRec.LastIntent.Price)
	require.Equal(t, paperRec.LastIntent.Volume, realRec.LastIntent.Volume)
	require.Equal(t, paperRec.LastIntent.StockCode, realRec.LastIntent.StockCode)
	require.Equal(t, paperRec.LastIntent.Side, realRec.LastIntent.Side)
	require.InDelta(t, item.LimitPrice, paperRec.LastIntent.Price, 1e-9)
	require.Equal(t, item.TargetVolume, paperRec.LastIntent.Volume)

	// Override blocked for both backends equally (Port never called).
	badOpts := opts
	badOpts.Price = 99
	p2 := &stubPort{order: &broker.TradeOrder{ID: "p"}}
	r2 := &stubPort{order: &broker.TradeOrder{ID: "r"}}
	pr, err := runSafetyGateAcceptance(t, p2, item, badOpts)
	require.Error(t, err)
	rr, err := runSafetyGateAcceptance(t, r2, item, badOpts)
	require.Error(t, err)
	require.False(t, pr.PortCalled)
	require.False(t, rr.PortCalled)
	require.Equal(t, pr.SpecHash, rr.SpecHash)
}
