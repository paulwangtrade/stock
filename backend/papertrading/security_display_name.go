// Security Display Name — read-only projection (Phase10).
// Does not mutate stock_name in DB; does not fetch external quotes.

package papertrading

import "strings"

// SecurityDisplayName separates snapshot vs current display (no schema change).
type SecurityDisplayName struct {
	SnapshotName string `json:"snapshot_name"`
	CurrentName  string `json:"current_name"` // "UNKNOWN" when no Security Master
	DisplayName  string `json:"display_name"`
	NameChanged  bool   `json:"name_changed"`
}

const securityNameUnknown = "UNKNOWN"

// ProjectSecurityDisplayName builds UI name projection.
// current empty / missing master → current_name=UNKNOWN; display falls back to snapshot.
func ProjectSecurityDisplayName(snapshotName, currentName string) SecurityDisplayName {
	snap := strings.TrimSpace(snapshotName)
	cur := strings.TrimSpace(currentName)
	out := SecurityDisplayName{
		SnapshotName: snap,
		CurrentName:  securityNameUnknown,
	}
	if cur != "" && !strings.EqualFold(cur, securityNameUnknown) {
		out.CurrentName = cur
	}
	if out.CurrentName != securityNameUnknown {
		out.DisplayName = out.CurrentName
	} else if snap != "" {
		out.DisplayName = snap
	} else {
		out.DisplayName = securityNameUnknown
	}
	out.NameChanged = snap != "" && out.CurrentName != securityNameUnknown && snap != out.CurrentName
	return out
}
