package strategysnapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
)

// CaptureInput is the read-only projection source after TradePlan create / evaluation.
// Missing optional fields are recorded as Unavailable / empty — never invents data.
type CaptureInput struct {
	Plan    *models.TradePlan
	Pool    *models.CandidatePool // optional enrichment
	Trigger string
	Now     time.Time
}

// CaptureResult is the bypass write outcome (references only; does not mutate Plan rows).
type CaptureResult struct {
	PlanRef    *PlanReference
	PlanSnap   *StrategySnapshot
	ItemSnaps  []StrategySnapshot
}

func synthesizeID(planID, itemID uint, planVersion int, scope Scope) string {
	if scope == ScopePlan {
		return fmt.Sprintf("sshot:%d:plan:%d", planID, planVersion)
	}
	return fmt.Sprintf("sshot:%d:%d:%d", planID, itemID, planVersion)
}

func parsePoolConfig(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{"_raw": raw, "_parse_error": true}
	}
	return m
}

func parseRiskSnapshotJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{"_raw": raw, "_parse_error": true}
	}
	return m
}

func hashParams(values map[string]any, poolCfg map[string]any) string {
	payload := map[string]any{"values": values, "pool": poolCfg}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func poolItemByCode(pool *models.CandidatePool) map[string]models.CandidatePoolItem {
	out := make(map[string]models.CandidatePoolItem)
	if pool == nil {
		return out
	}
	for _, it := range pool.Items {
		code := strings.TrimSpace(it.StockCode)
		if code != "" {
			out[code] = it
		}
	}
	return out
}

func buildParameters(plan *models.TradePlan, pool *models.CandidatePool) Parameters {
	values := map[string]any{}
	if plan != nil {
		values["amount_per_stock"] = plan.AmountPerStock
		values["max_names"] = plan.MaxNames
		if plan.DefaultEntryRule != "" {
			values["default_entry_rule"] = plan.DefaultEntryRule
		}
		if plan.DefaultMaxSlippage != nil {
			values["default_max_slippage"] = *plan.DefaultMaxSlippage
		}
		values["source_session"] = plan.SourceSession
		values["enable_execute"] = plan.EnableExecute
	}
	var poolCfg map[string]any
	if pool != nil {
		poolCfg = parsePoolConfig(pool.ConfigJSON)
	}
	p := Parameters{
		Values:        values,
		PoolConfigRef: poolCfg,
		ParamsHash:    hashParams(values, poolCfg),
	}
	return p
}

func buildStrategyVersion(item *models.TradePlanItem, pool *models.CandidatePool, pit *models.CandidatePoolItem) StrategyVersion {
	sv := StrategyVersion{Source: "unknown"}
	if pool != nil {
		if pool.Source != "" {
			sv.Source = pool.Source
		}
		sv.SourceRef = pool.SourceRef
	}
	if item != nil {
		sv.StrategyName = item.StrategyName
		sv.Version = item.StrategyVersion
	}
	if pit != nil {
		if sv.StrategyName == "" {
			sv.StrategyName = pit.StrategyName
		}
		if sv.Version == "" {
			sv.Version = pit.StrategyVersion
		}
	}
	// Do not invent strategy_id from display name.
	return sv
}

func buildMarketDataRef(item *models.TradePlanItem, pricingStage string) MarketDataReference {
	if item == nil {
		return MarketDataReference{Unavailable: true, MissingReason: "no plan item"}
	}
	md := MarketDataReference{
		RefPrice:     item.RefPrice,
		RefSource:    item.RefSource,
		RefAsOf:      item.RefAsOf,
		OpenRefPrice: item.OpenRefPrice,
		LimitPrice:   item.LimitPrice,
		PricingStage: pricingStage,
	}
	if item.RefPrice == 0 && item.OpenRefPrice == 0 && item.LimitPrice == 0 &&
		strings.TrimSpace(item.RefSource) == "" {
		md.Unavailable = true
		md.MissingReason = "no price fields on item"
	}
	return md
}

func buildSignalResult(pit *models.CandidatePoolItem, tradeDate string) SignalResult {
	if pit == nil {
		return SignalResult{Unavailable: true, MissingReason: "no pool item signal binding"}
	}
	sr := SignalResult{
		SignalSnapshotID: pit.SignalSnapshotID,
		TradeDate:        tradeDate,
		Tag:              pit.SignalTag,
		Score:            pit.SignalScore,
	}
	if pit.SignalSnapshotID == 0 && strings.TrimSpace(pit.SignalTag) == "" && pit.SignalScore == 0 {
		sr.Unavailable = true
		sr.MissingReason = "signal fields empty"
	}
	return sr
}

func itemAccepted(status string) *bool {
	st := strings.ToLower(strings.TrimSpace(status))
	v := st != models.TradePlanItemSkipped && st != "rejected" && st != "failed"
	if st == models.TradePlanItemSkipped {
		v = false
	}
	return &v
}

func buildPlanRisk(plan *models.TradePlan) RiskDecision {
	if plan == nil {
		return RiskDecision{Unavailable: true, MissingReason: "no plan"}
	}
	rd := RiskDecision{
		RiskStatus:        plan.RiskStatus,
		MarketLevel:       plan.MarketLevel,
		RiskFilteredCount: plan.RiskFilteredCount,
		RiskAcceptedCount: plan.RiskAcceptedCount,
		RiskSummary:       plan.RiskSummary,
		PlanSnapshot:      parseRiskSnapshotJSON(plan.RiskSnapshotJSON),
		ApprovedAt:        plan.ApprovedAt,
		ApprovedBy:        plan.ApprovedBy,
		ApprovedSource:    plan.ApprovedSource,
	}
	if plan.RiskStatus == "" && plan.RiskSnapshotJSON == "" && plan.RiskAcceptedCount == 0 && plan.RiskFilteredCount == 0 {
		rd.Unavailable = true
		rd.MissingReason = "risk fields empty"
	}
	return rd
}

func buildItemRisk(plan *models.TradePlan, item *models.TradePlanItem) RiskDecision {
	rd := buildPlanRisk(plan)
	if item != nil {
		rd.ItemAccepted = itemAccepted(item.Status)
		rd.RiskCode = item.RiskCode
		rd.RiskMessage = item.RiskMessage
		if rd.Unavailable && (item.RiskCode != "" || item.RiskMessage != "") {
			rd.Unavailable = false
			rd.MissingReason = ""
		}
	}
	return rd
}

// BuildSnapshots projects CaptureInput into immutable snapshot DTOs (no store write).
func BuildSnapshots(in CaptureInput) (*CaptureResult, error) {
	if in.Plan == nil {
		return nil, fmt.Errorf("strategysnapshot: plan is required")
	}
	if in.Plan.ID == 0 {
		return nil, fmt.Errorf("strategysnapshot: plan id is required")
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	trigger := strings.TrimSpace(in.Trigger)
	if trigger == "" {
		trigger = TriggerTradePlanCreate
	}

	poolByCode := poolItemByCode(in.Pool)
	params := buildParameters(in.Plan, in.Pool)
	pricingStage := in.Plan.PricingStage

	planSnap := &StrategySnapshot{
		SnapshotID:    synthesizeID(in.Plan.ID, 0, in.Plan.PlanVersion, ScopePlan),
		Scope:         ScopePlan,
		PlanID:        in.Plan.ID,
		TradeDate:     in.Plan.TradeDate,
		PlanVersion:   in.Plan.PlanVersion,
		CapturedAt:    now,
		SchemaVersion: SchemaVersion,
		Trigger:       trigger,
		StrategyVersion: StrategyVersion{
			Source:    "unknown",
			SourceRef: "",
		},
		Parameters:    params,
		MarketDataRef: MarketDataReference{Unavailable: true, MissingReason: "plan scope has no single price ref"},
		SignalResult:  SignalResult{Unavailable: true, MissingReason: "plan scope aggregates items"},
		RiskDecision:  buildPlanRisk(in.Plan),
	}
	if in.Pool != nil {
		planSnap.StrategyVersion.Source = in.Pool.Source
		planSnap.StrategyVersion.SourceRef = in.Pool.SourceRef
		if in.Pool.Source == "" {
			planSnap.StrategyVersion.Source = "unknown"
		}
	}

	itemSnaps := make([]StrategySnapshot, 0, len(in.Plan.Items))
	itemIDs := make(map[uint]string)
	for i := range in.Plan.Items {
		it := &in.Plan.Items[i]
		var pit *models.CandidatePoolItem
		if p, ok := poolByCode[strings.TrimSpace(it.StockCode)]; ok {
			pit = &p
		}
		sid := synthesizeID(in.Plan.ID, it.ID, in.Plan.PlanVersion, ScopeItem)
		if it.ID != 0 {
			itemIDs[it.ID] = sid
		}
		snap := StrategySnapshot{
			SnapshotID:      sid,
			Scope:           ScopeItem,
			PlanID:          in.Plan.ID,
			PlanItemID:      it.ID,
			TradeDate:       in.Plan.TradeDate,
			PlanVersion:     in.Plan.PlanVersion,
			CapturedAt:      now,
			SchemaVersion:   SchemaVersion,
			Trigger:         trigger,
			StrategyVersion: buildStrategyVersion(it, in.Pool, pit),
			Parameters:      params,
			MarketDataRef:   buildMarketDataRef(it, pricingStage),
			SignalResult:    buildSignalResult(pit, in.Plan.TradeDate),
			RiskDecision:    buildItemRisk(in.Plan, it),
		}
		itemSnaps = append(itemSnaps, snap)
	}

	ref := &PlanReference{
		PlanID:          in.Plan.ID,
		TradeDate:       in.Plan.TradeDate,
		PlanVersion:     in.Plan.PlanVersion,
		PlanSnapshotID:  planSnap.SnapshotID,
		ItemSnapshotIDs: itemIDs,
		CapturedAt:      now,
		Trigger:         trigger,
	}
	return &CaptureResult{PlanRef: ref, PlanSnap: planSnap, ItemSnaps: itemSnaps}, nil
}
