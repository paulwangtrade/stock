package research

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
)

// ErrBadTradeDate indicates illegal trade_date query.
type ErrBadTradeDate struct {
	Value string
}

func (e ErrBadTradeDate) Error() string {
	return fmt.Sprintf("invalid trade_date: %q", e.Value)
}

// ErrNotFound indicates candidate id missing from current research universe.
type ErrNotFound struct {
	ID string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("research candidate not found: %s", e.ID)
}

// ValidateTradeDate checks YYYY-MM-DD (empty allowed = use snapshot default).
func ValidateTradeDate(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) != 10 || raw[4] != '-' || raw[7] != '-' {
		return ErrBadTradeDate{Value: raw}
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return ErrBadTradeDate{Value: raw}
	}
	return nil
}

// ListCandidates returns research candidates (DTO / snapshot projection + annotation overlay).
func ListCandidates(q ListQuery) (ListResult, error) {
	if err := ValidateTradeDate(q.TradeDate); err != nil {
		return ListResult{}, err
	}

	src := data.ListResearchCandidatesForTradeDate(strings.TrimSpace(q.TradeDate), q.MinScore)
	out := AssembleFromSnapshot(src)
	applyAnnotations(out.Items)
	attachExplainMetaAll(out.Items)
	out.Items = filterItems(out.Items, q.Status, q.Source)
	return out, nil
}

// GetCandidate returns detail for one id (Explain shell projected from ResearchExplain).
func GetCandidate(id string) (DetailResult, error) {
	c, err := loadCandidate(id)
	if err != nil {
		return DetailResult{}, err
	}
	attachExplainMeta(&c)
	ex := buildExplainForCandidate(c)
	return DetailResult{
		Candidate:   c,
		Explanation: explanationShellFrom(ex),
		Links: LinksShell{
			TradePoolID: nil,
			TradePlanID: nil,
		},
	}, nil
}

// UpdateCandidate patches research-only status/note/tags into the sidecar/memory store.
// Does not touch Trade Candidate Pool, TradePlan, Broker, or execution paths.
func UpdateCandidate(id string, patch UpdatePatch) (DetailResult, error) {
	id = strings.TrimSpace(id)
	if patch.Status == nil && patch.Note == nil && patch.Tags == nil {
		return DetailResult{}, ErrBadPatch{Message: "empty patch: provide status, note, and/or tags"}
	}
	if patch.Status != nil {
		norm, err := NormalizeStatus(*patch.Status)
		if err != nil {
			return DetailResult{}, err
		}
		patch.Status = &norm
	}
	if patch.Note != nil {
		n := strings.TrimSpace(*patch.Note)
		patch.Note = &n
	}
	if patch.Tags != nil {
		cleaned := normalizeTags(*patch.Tags)
		patch.Tags = &cleaned
	}

	// Ensure candidate exists in current research universe before writing overlay.
	if _, err := loadCandidate(id); err != nil {
		return DetailResult{}, err
	}

	if _, err := DefaultStore().Patch(id, patch); err != nil {
		return DetailResult{}, err
	}
	return GetCandidate(id)
}

func loadCandidate(id string) (Candidate, error) {
	id = strings.TrimSpace(id)
	tradeDate, stockCode, ok := ParseCandidateID(id)
	if !ok {
		return Candidate{}, ErrNotFound{ID: id}
	}

	src := data.ListResearchCandidatesForTradeDate(tradeDate, 0)
	assembled := AssembleFromSnapshot(src)

	var found *Candidate
	for i := range assembled.Items {
		it := &assembled.Items[i]
		if it.ID == id || it.StockCode == stockCode {
			found = it
			break
		}
	}
	if found == nil {
		return Candidate{}, ErrNotFound{ID: id}
	}
	applyAnnotationOne(found)
	return *found, nil
}

func applyAnnotations(items []Candidate) {
	for i := range items {
		applyAnnotationOne(&items[i])
	}
}

func applyAnnotationOne(c *Candidate) {
	if c == nil {
		return
	}
	ann, ok := DefaultStore().Get(c.ID)
	if !ok {
		return
	}
	if ann.Status != "" {
		c.Status = ann.Status
	}
	c.Note = ann.Note
	if ann.HasTags {
		c.Tags = append([]string(nil), ann.Tags...)
	}
	if !ann.UpdatedAt.IsZero() {
		t := ann.UpdatedAt
		c.UpdatedAt = &t
	}
}

func normalizeTags(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func filterItems(items []Candidate, statusCSV, source string) []Candidate {
	statusSet := map[string]bool{}
	for _, s := range strings.Split(statusCSV, ",") {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "dismissed" {
			s = StatusDiscarded
		}
		if s != "" {
			statusSet[s] = true
		}
	}
	source = strings.TrimSpace(source)
	out := make([]Candidate, 0, len(items))
	for _, it := range items {
		if len(statusSet) > 0 && !statusSet[strings.ToLower(it.Status)] {
			continue
		}
		if source != "" && it.Source != source {
			continue
		}
		out = append(out, it)
	}
	return out
}
