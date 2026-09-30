package signalattribution

import (
	"strings"
	"time"
)

// NormalizeDay accepts YYYY-MM-DD, YYYY/MM/DD, YYYYMMDD, and datetime prefixes.
// Invalid input returns empty so callers fail closed instead of guessing a date.
func NormalizeDay(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "/", "-")
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		day := s[:10]
		if validYMD(day) {
			return day
		}
	}
	compact := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(compact) >= 8 {
		day := compact[:4] + "-" + compact[4:6] + "-" + compact[6:8]
		if validYMD(day) {
			return day
		}
	}
	return ""
}

func validYMD(day string) bool {
	_, err := time.Parse("2006-01-02", day)
	return err == nil
}
