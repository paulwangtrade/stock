package strategyintent

import "strings"

// IntentIDForCandidate builds the MVP one-intent-per-candidate id.
func IntentIDForCandidate(candidateID string) string {
	return "si:" + strings.TrimSpace(candidateID)
}
