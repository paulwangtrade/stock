package draft

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/decision/authority"
	"go-stock/backend/decision/registry"
)

// DecisionToTradePlanDraft projects sealed DecisionSnapshot → TradePlanDraft (Status=CREATED).
// EnableExecute is always false. Does not write Decision/Candidate/Rank; no Execution/Broker.
func DecisionToTradePlanDraft(snap registry.Snapshot) (*TradePlanDraft, error) {
	auth := authority.Authorize(authority.RoleTradePlanDraftFuture)
	if !auth.Allowed {
		return nil, fmt.Errorf("draft: consumer not allowed: %s", auth.Reason)
	}
	if snap.Decision == nil {
		return nil, fmt.Errorf("draft: nil decision in snapshot")
	}
	if strings.TrimSpace(snap.DecisionID) == "" {
		return nil, fmt.Errorf("draft: empty sourceDecisionId")
	}
	if strings.TrimSpace(snap.Hash) == "" {
		return nil, fmt.Errorf("draft: empty snapshot hash")
	}

	d := snap.Decision
	entryHint := d.Size.EntryPrice
	if entryHint == 0 && d.EntryZone != nil {
		entryHint = d.EntryZone.InstantPrice
	}
	side := d.Action.Side
	if side == "" {
		side = "none"
	}

	out := &TradePlanDraft{
		DraftID:            newDraftID(snap.DecisionID),
		SourceDecisionID:   snap.DecisionID,
		SourceSnapshotHash: snap.Hash,
		SnapshotHash:       snap.Hash,
		Status:             DraftCreated,
		CreatedAt:          time.Now().UTC(),
		EnableExecute:      false, // AllowDraft must never imply execute
		Payload: DraftPayload{
			TradeDate:      d.TradeDate,
			StockCode:      d.Instrument.StockCode,
			StockName:      d.Instrument.StockName,
			Side:           side,
			ActionCode:     d.Action.Code,
			ActionLabel:    d.Action.Label,
			AllowDraft:     d.Action.AllowDraft,
			TargetShares:   d.Size.TargetShares,
			AddShares:      d.Size.AddShares,
			TargetAmount:   d.Size.TargetAmount,
			PositionPct:    d.Size.PositionPct,
			EntryPriceHint: entryHint,
			StopPrice:      d.Size.StopPrice,
			MarketLevel:    d.Regime.Level,
			GateReady:      d.Gate.Ready,
		},
		ConsumerRole: string(authority.RoleTradePlanDraftFuture),
		Message:      "Phase4-B draft CREATED; not an executable TradePlan",
	}
	out.DraftHash = ComputeDraftHash(out)
	return out, nil
}

// DecisionToTradePlanDraftFromRegistry loads sealed snapshot then projects.
func DecisionToTradePlanDraftFromRegistry(reg *registry.Registry, decisionID string) (*TradePlanDraft, error) {
	if reg == nil {
		return nil, fmt.Errorf("draft: nil registry")
	}
	snap, ok := reg.GetSnapshot(decisionID)
	if !ok {
		return nil, fmt.Errorf("draft: snapshot not found for %s", decisionID)
	}
	return DecisionToTradePlanDraft(snap)
}

// ComputeDraftHash seals SourceDecisionID, SourceSnapshotHash, Payload, Status, EnableExecute.
func ComputeDraftHash(d *TradePlanDraft) string {
	if d == nil {
		return ""
	}
	canon := map[string]any{
		"sourceDecisionId":   d.SourceDecisionID,
		"sourceSnapshotHash": d.SourceSnapshotHash,
		"status":             string(d.Status),
		"enableExecute":      d.EnableExecute,
		"payload":            d.Payload,
	}
	b, err := json.Marshal(canon)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
