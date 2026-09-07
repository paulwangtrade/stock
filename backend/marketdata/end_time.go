package marketdata

import (
	"strings"
	"time"
)

// ParseEndTime converts an upstream end key (YYYYMMDD or YYYYMMDDHHmmss) to local time.
// Empty / LatestEndFlag → zero time (meaning "latest" for GetBars).
func ParseEndTime(end string) time.Time {
	end = strings.TrimSpace(end)
	if end == "" || end == LatestEndFlag {
		return time.Time{}
	}
	for _, layout := range []string{"20060102150405", "20060102"} {
		if t, err := time.ParseInLocation(layout, end, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}
