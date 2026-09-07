package research

import (
	"fmt"
	"strings"
)

// ValidStatuses is the MVP-2 research status set (not TradePlan statuses).
var ValidStatuses = map[string]bool{
	StatusNew:       true,
	StatusWatching:  true,
	StatusReviewed:  true,
	StatusDiscarded: true,
}

// ErrBadStatus indicates illegal research status.
type ErrBadStatus struct {
	Value string
}

func (e ErrBadStatus) Error() string {
	return fmt.Sprintf("invalid research status: %q", e.Value)
}

// ErrBadPatch indicates empty / invalid update patch.
type ErrBadPatch struct {
	Message string
}

func (e ErrBadPatch) Error() string {
	if e.Message == "" {
		return "invalid research candidate patch"
	}
	return e.Message
}

// NormalizeStatus lowercases and validates research status.
func NormalizeStatus(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	// legacy alias from early A2 wording
	if s == "dismissed" {
		s = StatusDiscarded
	}
	if !ValidStatuses[s] {
		return "", ErrBadStatus{Value: raw}
	}
	return s, nil
}

// IsValidStatus reports whether s is an allowed research status (after normalize).
func IsValidStatus(raw string) bool {
	_, err := NormalizeStatus(raw)
	return err == nil
}
