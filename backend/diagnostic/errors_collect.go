package diagnostic

import (
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/job"
)

const maxErrorRing = 20

// ErrorEntry is one sanitized diagnostic error item.
type ErrorEntry struct {
	At          string `json:"at"`
	Source      string `json:"source"`
	Category    string `json:"category"`
	Code        string `json:"code,omitempty"`
	MessageSafe string `json:"message_safe"`
	Page        string `json:"page,omitempty"`
}

// ErrorSummary aggregates recent errors for DiagnosticBundle v2.
type ErrorSummary struct {
	TotalCount      int          `json:"total_count"`
	PrimaryCategory string       `json:"primary_category,omitempty"`
	PrimaryCode     string       `json:"primary_code,omitempty"`
	MessageSafe     string       `json:"message_safe,omitempty"`
	Items           []ErrorEntry `json:"items,omitempty"`
}

const (
	SourceBackend  = "backend"
	SourceFrontend = "frontend"
	SourceJob      = "job"
	SourceCrash    = "crash"
	SourceSafety   = "safety"
)

var (
	errorRingMu sync.Mutex
	errorRing   []ErrorEntry
)

// RecordFrontendError stores a sanitized frontend error for the next export.
func RecordFrontendError(page, message string, lineno int) {
	page = SanitizeMessage(page, 80)
	msg := SanitizeMessage(message, 200)
	if msg == "" || looksForbiddenLine(strings.ToLower(msg)) {
		return
	}
	code := "FRONTEND_ERROR"
	if lineno > 0 {
		code = "FRONTEND_ERROR_L" + itoa(lineno)
	}
	appendError(ErrorEntry{
		At:          time.Now().UTC().Format(time.RFC3339),
		Source:      SourceFrontend,
		Category:    CategoryRuntime,
		Code:        code,
		MessageSafe: msg,
		Page:        page,
	})
}

func appendError(e ErrorEntry) {
	errorRingMu.Lock()
	defer errorRingMu.Unlock()
	errorRing = append(errorRing, e)
	if len(errorRing) > maxErrorRing {
		errorRing = errorRing[len(errorRing)-maxErrorRing:]
	}
}

// ResetErrorRingForTest clears buffered errors.
func ResetErrorRingForTest() {
	errorRingMu.Lock()
	defer errorRingMu.Unlock()
	errorRing = nil
}

func peekErrorRing() []ErrorEntry {
	errorRingMu.Lock()
	defer errorRingMu.Unlock()
	out := make([]ErrorEntry, len(errorRing))
	copy(out, errorRing)
	return out
}

func collectErrors(safetyCategory, safetyCode, safetyMsg string, jobs []JobStatusEntry) []ErrorEntry {
	out := make([]ErrorEntry, 0, maxErrorRing)
	if safetyCategory != "" && safetyCategory != CategoryNone {
		out = append(out, ErrorEntry{
			At:          time.Now().UTC().Format(time.RFC3339),
			Source:      SourceSafety,
			Category:    safetyCategory,
			Code:        safetyCode,
			MessageSafe: safetyMsg,
		})
	}
	if le := peekLastError(); le.Category != "" && le.Category != CategoryNone {
		out = append(out, ErrorEntry{
			At:          time.Now().UTC().Format(time.RFC3339),
			Source:      SourceBackend,
			Category:    le.Category,
			Code:        le.Code,
			MessageSafe: le.MessageSafe,
		})
	}
	out = append(out, peekErrorRing()...)
	for _, j := range jobs {
		if j.LastStatus != string(job.StatusFailed) && j.LastStatus != string(job.StatusMissed) {
			continue
		}
		if strings.TrimSpace(j.LastErrorSafe) == "" {
			continue
		}
		out = append(out, ErrorEntry{
			At:          timeOrNow(j.LastRun),
			Source:      SourceJob,
			Category:    CategoryRuntime,
			Code:        "JOB_" + strings.ToUpper(j.LastStatus),
			MessageSafe: j.LastErrorSafe,
		})
	}
	out = mergeErrorsSorted(out)
	if len(out) > maxErrorRing {
		out = out[len(out)-maxErrorRing:]
	}
	return out
}

func buildErrorSummary(items []ErrorEntry, primaryCategory, primaryCode, primaryMsg string) ErrorSummary {
	sum := ErrorSummary{
		TotalCount:      len(items),
		PrimaryCategory: primaryCategory,
		PrimaryCode:     primaryCode,
		MessageSafe:     primaryMsg,
		Items:           items,
	}
	if sum.PrimaryCategory == CategoryNone {
		sum.PrimaryCategory = ""
	}
	return sum
}

func mergeErrorsSorted(in []ErrorEntry) []ErrorEntry {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]ErrorEntry, 0, len(in))
	for _, e := range in {
		key := e.At + "|" + e.Source + "|" + e.Category + "|" + e.Code + "|" + e.MessageSafe
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].At < out[j].At
	})
	return out
}

func timeOrNow(lastRun string) string {
	if strings.TrimSpace(lastRun) != "" {
		return lastRun
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
