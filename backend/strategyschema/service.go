package strategyschema

import (
	"fmt"
	"strings"
	"time"
)

// ErrNotFound indicates missing schema asset.
type ErrNotFound struct {
	ID string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("strategy schema not found: %s", e.ID)
}

// ListSchemas returns definition summaries.
func ListSchemas() (ListResult, error) {
	defs, err := DefaultStore().ListDefinitions()
	if err != nil {
		return ListResult{Items: []ListItem{}, Message: err.Error()}, err
	}
	items := make([]ListItem, 0, len(defs))
	for _, d := range defs {
		items = append(items, ListItem{
			StrategyID:      d.StrategyID,
			Name:            d.Name,
			Description:     d.Description,
			Status:          d.Status,
			CurrentRevision: d.CurrentRevision,
			UpdatedAt:       d.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	out := ListResult{Items: items}
	if len(items) == 0 {
		out.Message = "暂无策略模板（请检查 data/strategy_schemas.json）"
	}
	return out, nil
}

// GetSchema returns definition + revision summaries + current full revision if any.
func GetSchema(strategyID string) (DetailResult, error) {
	strategyID = strings.TrimSpace(strategyID)
	def, ok, err := DefaultStore().GetDefinition(strategyID)
	if err != nil {
		return DetailResult{}, err
	}
	if !ok {
		return DetailResult{}, ErrNotFound{ID: strategyID}
	}
	revs, err := DefaultStore().ListRevisions(strategyID)
	if err != nil {
		return DetailResult{}, err
	}
	summaries := make([]RevisionSummary, 0, len(revs))
	for _, r := range revs {
		summaries = append(summaries, toSummary(r))
	}
	out := DetailResult{Definition: def, Revisions: summaries}
	if def.CurrentRevision != "" {
		if cur, ok2, _ := DefaultStore().GetRevision(strategyID, def.CurrentRevision); ok2 {
			c := cur
			out.Current = &c
		}
	}
	return out, nil
}

// ListRevisions returns revision summaries for a strategy.
func ListRevisions(strategyID string) (RevisionListResult, error) {
	strategyID = strings.TrimSpace(strategyID)
	if _, ok, err := DefaultStore().GetDefinition(strategyID); err != nil {
		return RevisionListResult{}, err
	} else if !ok {
		return RevisionListResult{}, ErrNotFound{ID: strategyID}
	}
	revs, err := DefaultStore().ListRevisions(strategyID)
	if err != nil {
		return RevisionListResult{}, err
	}
	items := make([]RevisionSummary, 0, len(revs))
	for _, r := range revs {
		items = append(items, toSummary(r))
	}
	return RevisionListResult{StrategyID: strategyID, Items: items}, nil
}

// GetRevision returns one full revision by strategy_id + version label.
func GetRevision(strategyID, revision string) (Revision, error) {
	strategyID = strings.TrimSpace(strategyID)
	revision = strings.TrimSpace(revision)
	r, ok, err := DefaultStore().GetRevision(strategyID, revision)
	if err != nil {
		return Revision{}, err
	}
	if !ok {
		return Revision{}, ErrNotFound{ID: strategyID + "/" + revision}
	}
	return r, nil
}

func toSummary(r Revision) RevisionSummary {
	return RevisionSummary{
		RevisionID:   r.RevisionID,
		Revision:     r.Revision,
		Status:       r.Status,
		RevisionNote: r.RevisionNote,
		ParamsHash:   r.Parameters.ParamsHash,
		RevisionHash: r.RevisionHash,
		Source:       r.Source,
		UpdatedAt:    r.UpdatedAt,
	}
}
