package registry

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
)

// BuildDecisionID aligns with frontend quantDecisionObservability.buildDecisionId.
func BuildDecisionID(d *models.QuantDecision) string {
	if d == nil {
		return ""
	}
	producer := d.Meta.Producer
	if producer == "" {
		producer = models.QuantProducerJSLegacy
	}
	schema := d.Meta.SchemaVersion
	if schema == 0 {
		schema = models.QuantDecisionSchemaVersion
	}
	code := strings.ToLower(strings.TrimSpace(d.Instrument.StockCode))
	if code == "" {
		code = "_"
	}
	asOf := strings.TrimSpace(formatAsOf(d.AsOf))
	if asOf == "" {
		asOf = "_"
	}
	ac := strings.TrimSpace(d.Action.Code)
	if ac == "" {
		ac = "_"
	}
	al := strings.TrimSpace(d.Action.Label)
	if al == "" {
		al = "_"
	}
	return fmt.Sprintf("qd%d:%s:%s:%s:%s:%s", schema, producer, code, asOf, ac, al)
}

func formatAsOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// EnsureDecisionID sets d.ID when empty; returns the id.
func EnsureDecisionID(d *models.QuantDecision) string {
	if d == nil {
		return ""
	}
	if strings.TrimSpace(d.ID) != "" {
		return d.ID
	}
	d.ID = BuildDecisionID(d)
	return d.ID
}
