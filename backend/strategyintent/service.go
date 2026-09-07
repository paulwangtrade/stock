package strategyintent

import (
	"fmt"
	"strings"
	"time"
)

// ErrNotFound indicates missing intent.
type ErrNotFound struct {
	ID string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("strategy intent not found: %s", e.ID)
}

// ListIntents returns intent summaries (read-only).
func ListIntents() (ListResult, error) {
	items, err := DefaultStore().List()
	if err != nil {
		return ListResult{Items: []ListItem{}, Message: err.Error()}, err
	}
	out := make([]ListItem, 0, len(items))
	for _, in := range items {
		out = append(out, toListItem(in))
	}
	res := ListResult{Items: out}
	if len(out) == 0 {
		res.Message = "暂无策略意图（请检查 data/strategy_intents.json）"
	}
	return res, nil
}

// GetIntent returns one intent by id (read-only).
func GetIntent(id string) (Intent, error) {
	id = strings.TrimSpace(id)
	in, ok, err := DefaultStore().Get(id)
	if err != nil {
		return Intent{}, err
	}
	if !ok {
		return Intent{}, ErrNotFound{ID: id}
	}
	// ensure null execution fields on read path
	in.PromotedPoolID = nil
	in.TradePlanID = nil
	if in.SchemaVersion == "" {
		in.SchemaVersion = SchemaVersion
	}
	return in, nil
}

func toListItem(in Intent) ListItem {
	return ListItem{
		ID:             in.ID,
		CandidateID:    in.CandidateID,
		Status:         in.Status,
		IntentType:     in.IntentType,
		Summary:        in.Summary,
		SchemaRevision: firstNonEmpty(in.SchemaRevision, in.StrategySchemaRef.Revision),
		StrategyID:     in.StrategySchemaRef.StrategyID,
		Unbound:        in.StrategySchemaRef.Unbound,
		UpdatedAt:      in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
