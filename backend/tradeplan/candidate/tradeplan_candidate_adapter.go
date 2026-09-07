package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/decision/authority"
	"go-stock/backend/models"
	"go-stock/backend/tradeplan/draft"
)

const adapterVersion = "1"

// DraftToTradePlanCandidate maps a VALIDATED TradePlanDraft to a shadow TradePlanCandidate.
// Does not write TradePlan DB, invoke plan builders, or touch order submit paths.
func DraftToTradePlanCandidate(d draft.TradePlanDraft) (TradePlanCandidate, error) {
	auth := authority.Authorize(authority.RoleTradePlanCandidateShadow)
	if !auth.Allowed {
		return TradePlanCandidate{}, fmt.Errorf("%s: %s", CodeAuthorityDenied, auth.Reason)
	}
	perm := authority.PermissionFor(authority.RoleTradePlanCandidateShadow)
	if !perm.CreateCandidate || perm.CreateTrade || perm.WriteDecision || perm.ModifyAction {
		return TradePlanCandidate{}, fmt.Errorf("%s: candidate shadow authority matrix violated", CodeAuthorityDenied)
	}

	if d.Status != draft.DraftValidated {
		return TradePlanCandidate{}, fmt.Errorf("%s: status=%s", CodeDraftNotValidated, d.Status)
	}
	if d.EnableExecute {
		return TradePlanCandidate{}, fmt.Errorf("%s: draft EnableExecute must be false", CodeCandidateExecuteForbidden)
	}

	sourceHash := d.SourceSnapshotHash
	if sourceHash == "" {
		sourceHash = d.SnapshotHash
	}
	if strings.TrimSpace(d.SourceDecisionID) == "" || strings.TrimSpace(sourceHash) == "" {
		return TradePlanCandidate{}, fmt.Errorf("%s: missing decision/hash provenance", CodeSourceInvalid)
	}

	out := TradePlanCandidate{
		CandidateID:        newCandidateID(d.DraftID, d.SourceDecisionID),
		SourceDraftID:      d.DraftID,
		SourceDecisionID:   d.SourceDecisionID,
		SourceSnapshotHash: sourceHash,
		Side:               d.Payload.Side,
		TargetShares:       d.Payload.TargetShares,
		EntryPriceHint:     d.Payload.EntryPriceHint,
		StopPriceHint:      d.Payload.StopPrice,
		IntentKind:         MapActionCodeToIntentKind(d.Payload.ActionCode),
		Executable:         false, // never a live trade plan
		CreatedAt:          time.Now().UTC(),
		StockCode:          d.Payload.StockCode,
		TradeDate:          d.Payload.TradeDate,
		AdapterVersion:     adapterVersion,
		Validation: CandidateValidation{
			OK:      true,
			Message: "OK: shadow candidate projected; Executable=false",
		},
	}
	out.CandidateHash = ComputeCandidateHash(out)
	return out, nil
}

// MapActionCodeToIntentKind projects Action.Code to explanation IntentKind (never Execute).
func MapActionCodeToIntentKind(code string) string {
	switch strings.TrimSpace(code) {
	case models.QuantActionEnter:
		return IntentEnterHint
	case models.QuantActionWaitPullback:
		return IntentWaitHint
	case models.QuantActionScaleIn:
		return IntentScaleInHint
	case models.QuantActionReduce:
		return IntentReduceHint
	case models.QuantActionExitPartial:
		return IntentExitPartialHint
	case models.QuantActionWatch:
		return IntentWatchHint
	case models.QuantActionBlocked, models.QuantActionHold:
		return IntentBlockedHint
	default:
		return IntentOtherHint
	}
}

func newCandidateID(draftID, decisionID string) string {
	sum := sha256.Sum256([]byte(draftID + "|" + decisionID))
	return fmt.Sprintf("tpc:%s", hex.EncodeToString(sum[:8]))
}

// ComputeCandidateHash seals provenance + payload fields used for mutation detection.
func ComputeCandidateHash(c TradePlanCandidate) string {
	canon := map[string]any{
		"candidateId":        c.CandidateID,
		"sourceDraftId":      c.SourceDraftID,
		"sourceDecisionId":   c.SourceDecisionID,
		"sourceSnapshotHash": c.SourceSnapshotHash,
		"side":               c.Side,
		"targetShares":       c.TargetShares,
		"entryPriceHint":     c.EntryPriceHint,
		"stopPriceHint":      c.StopPriceHint,
		"intentKind":         c.IntentKind,
		"executable":         c.Executable,
		"stockCode":          c.StockCode,
		"tradeDate":          c.TradeDate,
		"adapterVersion":     c.AdapterVersion,
	}
	b, err := json.Marshal(canon)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
