package reconcile

// API View DTOs (Phase 6.5.9.2.1.3.2).
//
// These are HTTP-facing observation views. They must not expose
// BrokerSnapshot, LocalOrderSnapshot, or internal OMS models.

import "time"

// Aggregate status labels for BrokerReconcileView (observation-only; never blocks trading).
const (
	ViewStatusReady             = "READY"
	ViewStatusDegraded          = "DEGRADED"
	ViewStatusAttention         = "ATTENTION"
	ViewStatusObservationError  = "OBSERVATION_ERROR"
)

// BrokerReconcileView is the snake_case HTTP DTO for GET /api/execution/broker-reconcile.
type BrokerReconcileView struct {
	Status          string           `json:"status"`
	CheckedAt       time.Time        `json:"checked_at"`
	TotalOrders     int              `json:"total_orders"`
	DivergenceCount int              `json:"divergence_count"`
	Divergences     []DivergenceView `json:"divergences"`
	// ObservationError is set when the candidate source or provider path failed
	// as an observation problem (HTTP still 200). Never triggers order mutation.
	ObservationError string `json:"observation_error,omitempty"`
}

// DivergenceView is one flattened observation divergence for the API surface.
type DivergenceView struct {
	Kind     string `json:"kind"`
	OrderID  string `json:"order_id"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
}

// OrderObservation pairs a local order key with its ReconcileResult for view mapping.
// It is an internal mapping aid; not an HTTP DTO and not a BrokerSnapshot.
type OrderObservation struct {
	OrderID string
	Result  ReconcileResult
}

// MapBrokerReconcileView builds the public view from observation results.
// It does not call Compare, Query, or mutate any runtime state.
func MapBrokerReconcileView(checkedAt time.Time, observations []OrderObservation) BrokerReconcileView {
	divs := make([]DivergenceView, 0)
	for _, obs := range observations {
		for _, d := range obs.Result.Divergences {
			divs = append(divs, DivergenceView{
				Kind:     d.Kind,
				OrderID:  obs.OrderID,
				Detail:   d.Detail,
				Severity: severityForKind(d.Kind),
			})
		}
	}
	if divs == nil {
		divs = []DivergenceView{}
	}
	return BrokerReconcileView{
		Status:          aggregateViewStatus(observations),
		CheckedAt:       checkedAt,
		TotalOrders:     len(observations),
		DivergenceCount: len(divs),
		Divergences:     divs,
	}
}

// EmptyBrokerReconcileView returns a READY empty observation (no auto-created snapshots).
func EmptyBrokerReconcileView(checkedAt time.Time) BrokerReconcileView {
	return BrokerReconcileView{
		Status:          ViewStatusReady,
		CheckedAt:       checkedAt,
		TotalOrders:     0,
		DivergenceCount: 0,
		Divergences:     []DivergenceView{},
	}
}

// ObservationErrorView returns an observation-error envelope without mutating orders.
func ObservationErrorView(checkedAt time.Time, msg string) BrokerReconcileView {
	return BrokerReconcileView{
		Status:           ViewStatusObservationError,
		CheckedAt:        checkedAt,
		TotalOrders:      0,
		DivergenceCount:  0,
		Divergences:      []DivergenceView{},
		ObservationError: msg,
	}
}

func severityForKind(kind string) string {
	switch kind {
	case DivMissingOrder, DivFilledQtyMismatch, DivCancelPendingTimeout, DivUnknownTimeout:
		return "attention"
	case DivStatusMismatch, DivAvgPriceMismatch:
		return "warn"
	default:
		return "info"
	}
}

func aggregateViewStatus(observations []OrderObservation) string {
	if len(observations) == 0 {
		return ViewStatusReady
	}
	hasAttention := false
	hasAny := false
	for _, obs := range observations {
		for _, d := range obs.Result.Divergences {
			hasAny = true
			switch d.Kind {
			case DivMissingOrder, DivFilledQtyMismatch, DivCancelPendingTimeout, DivUnknownTimeout:
				hasAttention = true
			}
		}
	}
	switch {
	case hasAttention:
		return ViewStatusAttention
	case hasAny:
		return ViewStatusDegraded
	default:
		return ViewStatusReady
	}
}
