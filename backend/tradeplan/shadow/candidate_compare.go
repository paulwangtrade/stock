package shadow

import (
	"fmt"
	"math"
	"strings"

	"go-stock/backend/models"
	"go-stock/backend/tradeplan/candidate"
)

const priceEps = 1e-6

// FieldDiff one compared field difference.
type FieldDiff struct {
	Path  string `json:"path"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
	Kind  string `json:"kind"` // different | weak_match | ignored | forbidden_field
}

// TradePlanShadowDiff single candidate↔plan comparison result.
type TradePlanShadowDiff struct {
	Equal           bool        `json:"equal"`
	MissingFields   []string    `json:"missingFields,omitempty"`
	DifferentFields []FieldDiff `json:"differentFields,omitempty"`
	RiskFlags       []string    `json:"riskFlags,omitempty"`
	StockCode       string      `json:"stockCode,omitempty"`
	TradeDate       string      `json:"tradeDate,omitempty"`
	Summary         string      `json:"summary"`
}

// CompareTradePlanCandidate compares shadow Candidate to an existing TradePlan (+ matching item).
// Lifecycle/status/order/fill fields are ignored or flagged as forbidden — never drive Equal alone.
func CompareTradePlanCandidate(c candidate.TradePlanCandidate, plan models.TradePlan) TradePlanShadowDiff {
	out := TradePlanShadowDiff{
		StockCode: c.StockCode,
		TradeDate: firstNonEmpty(c.TradeDate, plan.TradeDate),
	}

	item, itemOK := findPlanItem(plan, c.StockCode)
	if !itemOK {
		out.Equal = false
		out.MissingFields = append(out.MissingFields, "plan.item(stockCode)")
		out.RiskFlags = append(out.RiskFlags, "plan_item_missing")
		out.Summary = "FAIL: no matching TradePlanItem for stockCode"
		return out
	}

	var diffs []FieldDiff
	var missing []string
	var risks []string

	// --- allowed compares ---
	// Side
	if normalizeSide(c.Side) != normalizeSide(firstNonEmpty(item.Side, plan.Side)) {
		diffs = append(diffs, FieldDiff{
			Path: "Side", Left: c.Side, Right: firstNonEmpty(item.Side, plan.Side), Kind: "different",
		})
	}

	// TargetShares ↔ TargetVolume
	if c.TargetShares != item.TargetVolume {
		diffs = append(diffs, FieldDiff{
			Path: "TargetShares", Left: c.TargetShares, Right: item.TargetVolume, Kind: "different",
		})
	}

	// EntryPriceHint ↔ LimitPrice (weak if one side zero)
	priceDiff := comparePriceHint(c.EntryPriceHint, item.LimitPrice)
	if priceDiff != nil {
		diffs = append(diffs, *priceDiff)
		if priceDiff.Kind == "weak_match" {
			risks = append(risks, "price_weak_match")
		}
	}

	// StopHint ↔ Risk fields (no Stop column on TradePlan → missing)
	if c.StopPriceHint != 0 {
		if item.RiskCode == "" && plan.RiskStatus == "" && item.RiskMessage == "" && plan.RiskSummary == "" {
			missing = append(missing, "StopHint→Risk")
			risks = append(risks, "missing_stop_mapping")
		}
		// Stop has no numeric twin; record structural missing
		missing = appendUnique(missing, "plan.stopPrice")
	} else {
		missing = appendUnique(missing, "plan.stopPrice")
	}

	// IntentKind ↔ Plan Intent (Reason as weak proxy; no IntentKind column)
	if c.IntentKind != "" {
		if item.Reason == "" {
			missing = appendUnique(missing, "plan.intent")
			risks = append(risks, "missing_plan_intent")
		} else if !intentWeakMatch(c.IntentKind, item.Reason) {
			diffs = append(diffs, FieldDiff{
				Path: "IntentKind", Left: c.IntentKind, Right: item.Reason, Kind: "different",
			})
		} else {
			diffs = append(diffs, FieldDiff{
				Path: "IntentKind", Left: c.IntentKind, Right: item.Reason, Kind: "weak_match",
			})
			risks = appendUnique(risks, "intent_weak_match")
		}
	}

	// Provenance missing on existing plan
	missing = appendUnique(missing, "plan.sourceDecisionId")
	missing = appendUnique(missing, "plan.snapshotHash")
	risks = appendUnique(risks, "missing_decision_provenance")

	// --- ignored lifecycle ---
	if plan.Status != "" {
		diffs = append(diffs, FieldDiff{
			Path: "Status", Left: "n/a(candidate)", Right: plan.Status, Kind: "ignored",
		})
	}
	if item.Status != "" {
		diffs = append(diffs, FieldDiff{
			Path: "Item.Status", Left: "n/a(candidate)", Right: item.Status, Kind: "ignored",
		})
	}

	// --- forbidden live-trading-layer fields if present ---
	if plan.Status == models.TradePlanStatusExecuting || plan.ExecutedAt != nil {
		diffs = append(diffs, FieldDiff{
			Path: "Executing", Left: false, Right: true, Kind: "forbidden_field",
		})
		risks = appendUnique(risks, "forbidden_field")
	}
	if item.OrderID != 0 {
		diffs = append(diffs, FieldDiff{
			Path: "OrderID", Left: 0, Right: item.OrderID, Kind: "forbidden_field",
		})
		risks = appendUnique(risks, "forbidden_field")
	}
	if item.FillID != 0 || item.FilledVolume != 0 || item.FilledPrice != 0 {
		diffs = append(diffs, FieldDiff{
			Path: "Fill", Left: nil, Right: fmt.Sprintf("fillId=%d vol=%d px=%v", item.FillID, item.FilledVolume, item.FilledPrice), Kind: "forbidden_field",
		})
		risks = appendUnique(risks, "forbidden_field")
	}

	// Equal: no "different" among allowed semantic fields
	equal := true
	for _, d := range diffs {
		if d.Kind == "different" {
			equal = false
			break
		}
	}

	out.Equal = equal
	out.MissingFields = missing
	out.DifferentFields = diffs
	out.RiskFlags = risks
	if equal {
		out.Summary = "OK: compared fields equal (lifecycle/fill ignored)"
	} else {
		out.Summary = "DIFF: semantic field mismatch"
	}
	return out
}

func findPlanItem(plan models.TradePlan, code string) (models.TradePlanItem, bool) {
	want := strings.ToLower(strings.TrimSpace(code))
	for _, it := range plan.Items {
		if strings.ToLower(strings.TrimSpace(it.StockCode)) == want {
			return it, true
		}
	}
	// single-item plan without code on candidate: use first
	if want == "" && len(plan.Items) == 1 {
		return plan.Items[0], true
	}
	return models.TradePlanItem{}, false
}

func normalizeSide(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "none"
	}
	return s
}

func comparePriceHint(left, right float64) *FieldDiff {
	if approxEq(left, right) {
		return nil
	}
	if left == 0 || right == 0 {
		return &FieldDiff{Path: "EntryPriceHint", Left: left, Right: right, Kind: "weak_match"}
	}
	return &FieldDiff{Path: "EntryPriceHint", Left: left, Right: right, Kind: "different"}
}

func approxEq(a, b float64) bool {
	return math.Abs(a-b) <= priceEps
}

func intentWeakMatch(kind, reason string) bool {
	r := strings.ToLower(reason)
	switch kind {
	case candidate.IntentEnterHint:
		return strings.Contains(r, "enter") || strings.Contains(r, "buy") || strings.Contains(r, "买")
	case candidate.IntentWaitHint:
		return strings.Contains(r, "wait") || strings.Contains(r, "pull") || strings.Contains(r, "回踩")
	case candidate.IntentScaleInHint:
		return strings.Contains(r, "scale") || strings.Contains(r, "加")
	case candidate.IntentReduceHint, candidate.IntentExitPartialHint:
		return strings.Contains(r, "reduce") || strings.Contains(r, "exit") || strings.Contains(r, "减") || strings.Contains(r, "止")
	default:
		return true // OTHER/WATCH/BLOCKED: treat reason present as weak ok
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func appendUnique(ss []string, v string) []string {
	for _, x := range ss {
		if x == v {
			return ss
		}
	}
	return append(ss, v)
}
