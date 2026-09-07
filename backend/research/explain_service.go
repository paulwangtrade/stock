package research

import (
	"fmt"
	"strings"
	"time"
)

// GetExplain returns ResearchExplain for a candidate id (derive + optional overlay).
func GetExplain(candidateID string) (Explain, error) {
	c, err := loadCandidate(candidateID)
	if err != nil {
		return Explain{}, err
	}
	return buildExplainForCandidate(c), nil
}

// GetExplainByExplainID resolves rx:{candidate_id}.
func GetExplainByExplainID(explainID string) (Explain, error) {
	candID, ok := ParseExplainID(explainID)
	if !ok {
		return Explain{}, ErrNotFound{ID: explainID}
	}
	return GetExplain(candID)
}

// UpdateExplain applies manual overlay (summary / research_reason / risk_note).
// Evidence remains derived; no AI / Intent / Promote.
func UpdateExplain(candidateID string, patch ExplainPatch) (Explain, error) {
	candidateID = strings.TrimSpace(candidateID)
	if patch.Summary == nil && patch.ResearchReason == nil && patch.RiskNote == nil && !patch.ClearRiskNote {
		return Explain{}, ErrBadPatch{Message: "empty explain patch: provide summary, research_reason, and/or risk_note"}
	}
	if patch.RiskNote != nil {
		sev := strings.ToLower(strings.TrimSpace(patch.RiskNote.Severity))
		switch sev {
		case "", RiskSeverityInfo, RiskSeverityWarn, RiskSeverityHigh:
			if sev == "" {
				sev = RiskSeverityInfo
			}
			patch.RiskNote.Severity = sev
		default:
			return Explain{}, ErrBadPatch{Message: fmt.Sprintf("invalid risk_note.severity: %q", patch.RiskNote.Severity)}
		}
	}
	if patch.ResearchReason != nil {
		k := strings.ToLower(strings.TrimSpace(patch.ResearchReason.Kind))
		switch k {
		case "", ReasonKindRule, ReasonKindSignalText, ReasonKindAnalystNote, ReasonKindUnknown:
			if k == "" {
				k = ReasonKindAnalystNote
			}
			patch.ResearchReason.Kind = k
		default:
			return Explain{}, ErrBadPatch{Message: fmt.Sprintf("invalid research_reason.kind: %q", patch.ResearchReason.Kind)}
		}
	}

	if _, err := loadCandidate(candidateID); err != nil {
		return Explain{}, err
	}
	if _, err := DefaultExplainStore().Patch(candidateID, patch); err != nil {
		return Explain{}, err
	}
	return GetExplain(candidateID)
}

func buildExplainForCandidate(c Candidate) Explain {
	ex := DeriveExplain(c, time.Now().UTC())
	ov, ok := DefaultExplainStore().Get(c.ID)
	if !ok {
		return ex
	}
	mergeExplainOverlay(&ex, ov)
	return ex
}

func mergeExplainOverlay(ex *Explain, ov ExplainOverlay) {
	if ex == nil {
		return
	}
	if ov.HasSummary {
		ex.Summary = ov.Summary
	}
	if ov.ResearchReason != nil {
		ex.ResearchReason = *ov.ResearchReason
	}
	if ov.ClearRiskNote {
		ex.RiskNote = nil
	} else if ov.HasRiskNote {
		if ov.RiskNote != nil {
			rn := *ov.RiskNote
			ex.RiskNote = &rn
		} else {
			ex.RiskNote = nil
		}
	}
	if !ov.CreatedAt.IsZero() {
		ex.CreatedAt = ov.CreatedAt
	}
	if !ov.UpdatedAt.IsZero() {
		ex.UpdatedAt = ov.UpdatedAt
	}
	// Manual edits → hybrid (still not Intent / AI).
	if ov.HasSummary || ov.ResearchReason != nil || ov.HasRiskNote {
		if ex.Available {
			ex.ExplainType = ExplainTypeHybrid
		}
	}
	ex.StrategyIntentRef = nil
}

func attachExplainMeta(c *Candidate) {
	if c == nil || c.ID == "" {
		return
	}
	ex := buildExplainForCandidate(*c)
	ref := ex.ID
	c.ExplainRef = &ref
	sum := TruncateExplainSummary(ex.Summary, 80)
	c.ExplainSummary = sum
}

func attachExplainMetaAll(items []Candidate) {
	for i := range items {
		attachExplainMeta(&items[i])
	}
}

func explanationShellFrom(ex Explain) ExplanationShell {
	shell := ExplanationShell{
		Available:     ex.Available,
		Summary:       TruncateExplainSummary(ex.Summary, 120),
		MissingReason: ex.MissingReason,
		ExplainRef:    ex.ID,
	}
	if !ex.Available && shell.MissingReason == "" {
		shell.MissingReason = "unavailable"
	}
	return shell
}
