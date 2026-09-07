package strategyintent

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"go-stock/backend/strategyschema"
)

// SchemaRevisionLookup resolves a pinned schema revision for Intent binding checks.
type SchemaRevisionLookup interface {
	LookupRevision(strategyID, revision string) (status string, found bool, err error)
}

type schemaStoreLookup struct{}

func (schemaStoreLookup) LookupRevision(strategyID, revision string) (status string, found bool, err error) {
	r, err := strategyschema.GetRevision(strategyID, revision)
	if err != nil {
		var nf strategyschema.ErrNotFound
		if errors.As(err, &nf) {
			return "", false, nil
		}
		return "", false, err
	}
	return r.Status, true, nil
}

var (
	schemaRevisionLookupMu sync.RWMutex
	schemaRevisionLookup   SchemaRevisionLookup = schemaStoreLookup{}
)

// SetSchemaRevisionLookupForTest replaces the revision lookup (nil restores production default).
func SetSchemaRevisionLookupForTest(l SchemaRevisionLookup) {
	schemaRevisionLookupMu.Lock()
	defer schemaRevisionLookupMu.Unlock()
	if l == nil {
		schemaRevisionLookup = schemaStoreLookup{}
		return
	}
	schemaRevisionLookup = l
}

func getSchemaRevisionLookup() SchemaRevisionLookup {
	schemaRevisionLookupMu.RLock()
	defer schemaRevisionLookupMu.RUnlock()
	return schemaRevisionLookup
}

// TestSchemaLookup is an injectable revision map for tests (key: strategy_id/revision).
type TestSchemaLookup map[string]string

func (m TestSchemaLookup) LookupRevision(strategyID, revision string) (status string, found bool, err error) {
	st, ok := m[strategyID+"/"+revision]
	return st, ok, nil
}

// DefaultTestSchemaLookup accepts the common seed pin used in intent tests.
func DefaultTestSchemaLookup() SchemaRevisionLookup {
	return TestSchemaLookup{
		"sdef:trend_breakout/v1": strategyschema.RevisionStatusActive,
	}
}

type schemaBindingContext struct {
	priorRef       *SchemaRef
	priorRevision  string
}

func validateSchemaBinding(ref SchemaRef, schemaRevision string, ctx schemaBindingContext) error {
	if err := ValidateSchemaRef(ref, schemaRevision); err != nil {
		return err
	}
	if ref.Unbound {
		return nil
	}
	strategyID := strings.TrimSpace(ref.StrategyID)
	revision := effectiveRevision(ref, schemaRevision)
	if strategyID == "" || revision == "" {
		return nil
	}
	if ctx.priorRef != nil && !ctx.priorRef.Unbound && schemaBindingEqual(*ctx.priorRef, ctx.priorRevision, ref, schemaRevision) {
		return nil
	}
	return checkSchemaRevisionBindable(strategyID, revision)
}

func schemaBindingEqual(aRef SchemaRef, aTop string, bRef SchemaRef, bTop string) bool {
	if strings.TrimSpace(aRef.StrategyID) != strings.TrimSpace(bRef.StrategyID) {
		return false
	}
	return effectiveRevision(aRef, aTop) == effectiveRevision(bRef, bTop)
}

func checkSchemaRevisionBindable(strategyID, revision string) error {
	lookup := getSchemaRevisionLookup()
	if lookup == nil {
		return nil
	}
	status, found, err := lookup.LookupRevision(strategyID, revision)
	if err != nil {
		return err
	}
	if !found {
		return ValidationError{
			Code:    CodeSchemaRevisionNotFound,
			Message: fmt.Sprintf("strategy schema revision not found: %s/%s", strategyID, revision),
		}
	}
	switch strings.TrimSpace(status) {
	case strategyschema.RevisionStatusRetired, strategyschema.RevisionStatusDiscarded:
		return ValidationError{
			Code:    CodeSchemaRevisionNotBindable,
			Message: fmt.Sprintf("cannot bind intent to %s revision %q", status, revision),
		}
	default:
		return nil
	}
}
