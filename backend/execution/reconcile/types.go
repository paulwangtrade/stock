// Package reconcile provides read-only Broker Reconcile Observation core types
// and a pure comparator (Phase 6.5.9.2.1.1).
//
// It never writes Order / Fill / Position / TradePlan / Frozen Spec / OMS.
package reconcile

import "time"

// Divergence kinds (observation namespace; distinct from audit evidence kinds).
const (
	DivMissingOrder         = "missing_order"
	DivStatusMismatch       = "status_mismatch"
	DivFilledQtyMismatch    = "filled_qty_mismatch"
	DivAvgPriceMismatch     = "avg_price_mismatch"
	DivCancelPendingTimeout = "cancel_pending_timeout"
	DivUnknownTimeout       = "unknown_timeout"
)

// Classification labels for ReconcileResult.
const (
	ClassConsistent   = "CONSISTENT"
	ClassDivergent    = "DIVERGENT"
	ClassUnverifiable = "UNVERIFIABLE"
	ClassStale        = "STALE"
)

// Default thresholds (observation-only; no migration / no schema).
const (
	DefaultCancelPendingTimeout = 5 * time.Minute
	DefaultUnknownTimeout       = 2 * time.Minute
	DefaultAvgPriceEpsilon      = 1e-6
)

// LocalOrderSnapshot is a read-only local OMS projection used for comparison.
type LocalOrderSnapshot struct {
	OrderID       string    `json:"order_id"`
	BrokerOrderID string    `json:"broker_order_id,omitempty"`
	OMSStatus     string    `json:"oms_status,omitempty"`
	BrokerStatus  string    `json:"broker_status"`
	FilledQty     int64     `json:"filled_qty"`
	LeavesQty     int64     `json:"leaves_qty"`
	AvgPrice      float64   `json:"avg_price"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// BrokerSnapshot is a read-only observation of channel state.
// It is never applied to Order / Fill / Position / Spec.
type BrokerSnapshot struct {
	OrderID       string    `json:"order_id"`
	BrokerOrderID string    `json:"broker_order_id"`
	Status        string    `json:"status"`
	BrokerStatus  string    `json:"broker_status"`
	FilledQty     int64     `json:"filled_qty"`
	LeavesQty     int64     `json:"leaves_qty"`
	AvgPrice      float64   `json:"avg_price"`
	CheckedAt     time.Time `json:"checked_at"`
	// Missing marks BrokerAdapter.Query not-found. Observation only.
	Missing bool `json:"missing,omitempty"`
}

// Divergence records one observation mismatch. Report-only; never repaired here.
type Divergence struct {
	Kind        string `json:"kind"`
	Detail      string `json:"detail"`
	LocalValue  string `json:"local_value,omitempty"`
	BrokerValue string `json:"broker_value,omitempty"`
}

// ReconcileResult is the pure compare output.
type ReconcileResult struct {
	Consistent     bool         `json:"consistent"`
	Classification string       `json:"classification"`
	Divergences    []Divergence `json:"divergences"`
}

// HasKind reports whether a divergence kind is present.
func (r ReconcileResult) HasKind(kind string) bool {
	for _, d := range r.Divergences {
		if d.Kind == kind {
			return true
		}
	}
	return false
}

// Options tunes observation thresholds. Zero values use defaults.
type Options struct {
	CancelPendingTimeout time.Duration
	UnknownTimeout       time.Duration
	AvgPriceEpsilon      float64
	// Now overrides wall clock for deterministic tests.
	Now time.Time
}
