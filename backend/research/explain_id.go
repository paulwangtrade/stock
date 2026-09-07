package research

import (
	"strings"
)

// MakeExplainID builds rx:{candidate_id}.
func MakeExplainID(candidateID string) string {
	return "rx:" + strings.TrimSpace(candidateID)
}

// ParseExplainID extracts candidate_id from explain id.
func ParseExplainID(explainID string) (candidateID string, ok bool) {
	explainID = strings.TrimSpace(explainID)
	const prefix = "rx:"
	if !strings.HasPrefix(explainID, prefix) {
		return "", false
	}
	candidateID = strings.TrimSpace(explainID[len(prefix):])
	if candidateID == "" {
		return "", false
	}
	// Validate candidate id shape when possible.
	if _, _, cok := ParseCandidateID(candidateID); !cok {
		return "", false
	}
	return candidateID, true
}

// TruncateExplainSummary shortens summary for Candidate.explain_summary.
func TruncateExplainSummary(summary string, maxRunes int) string {
	summary = strings.TrimSpace(summary)
	if maxRunes <= 0 {
		maxRunes = 80
	}
	r := []rune(summary)
	if len(r) <= maxRunes {
		return summary
	}
	return string(r[:maxRunes]) + "…"
}
