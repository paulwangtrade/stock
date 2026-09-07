package diagnostic

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/job"
)

const maxJobEntries = 40

// JobStatusEntry is a privacy-safe job runtime snapshot.
type JobStatusEntry struct {
	JobName         string `json:"job_name"`
	LastRun         string `json:"last_run,omitempty"`
	LastStatus      string `json:"last_status,omitempty"`
	LastErrorSafe   string `json:"last_error_safe,omitempty"`
	ExpectedSchedule string `json:"expected_schedule,omitempty"`
	CatchUpPolicy   string `json:"catch_up_policy,omitempty"`
}

// CollectJobStatus returns registered job snapshots sorted by job_name.
func CollectJobStatus() []JobStatusEntry {
	states := job.Default().List()
	out := make([]JobStatusEntry, 0, len(states))
	for _, st := range states {
		entry := JobStatusEntry{
			JobName:          strings.TrimSpace(st.Name),
			LastStatus:       string(st.LastStatus),
			ExpectedSchedule: strings.TrimSpace(st.ExpectedSchedule),
			CatchUpPolicy:    string(st.CatchUpPolicy),
		}
		if st.LastRun != nil && !st.LastRun.IsZero() {
			entry.LastRun = st.LastRun.UTC().Format(time.RFC3339)
		}
		if msg := SanitizeMessage(st.LastError, 200); msg != "" && !looksForbiddenLine(strings.ToLower(msg)) {
			entry.LastErrorSafe = msg
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].JobName < out[j].JobName
	})
	if len(out) > maxJobEntries {
		out = out[:maxJobEntries]
	}
	return out
}
