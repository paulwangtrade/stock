package papertrading

import "strings"

// Exclusive FillMode values (Phase10-C.4-A). Controls which Fill cron is registered.
const (
	FillModeA = "A" // 09:31 Session A (default)
	FillModeB = "B" // 15:10 Session B observation sampling
)

// Cron actor for Session B fill job (observability).
const ActorCronSessionB = "cron:session_b"

// NormalizeFillMode maps config text to A|B. Empty/unknown → A (safe default).
func NormalizeFillMode(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case FillModeB:
		return FillModeB
	default:
		return FillModeA
	}
}

// FillCronExclusive reports which exclusive Fill cron should register.
// Settle cron is always independent and not represented here.
// Never registers both open and session-B at once (C.4-A).
func FillCronExclusive(fillMode string) (registerOpen, registerSessionB bool) {
	if NormalizeFillMode(fillMode) == FillModeB {
		return false, true
	}
	return true, false
}
