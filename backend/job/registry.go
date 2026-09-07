package job

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/marketstate"

	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Registry tracks job definitions, last status, and execution history (in-memory).
type Registry struct {
	mu      sync.RWMutex
	jobs    map[string]*RuntimeState
	records []ExecutionRecord
	seq     atomic.Uint64
	now     func() time.Time
	loc     *time.Location
}

// DefaultGrace is how long after ExpectedAt before a job is considered MISSED.
const DefaultGrace = 5 * time.Minute

var (
	defaultRegistry = NewRegistry()
)

// Default returns the process-wide Job Registry.
func Default() *Registry {
	return defaultRegistry
}

// ResetDefaultForTest replaces the process registry (tests only).
func ResetDefaultForTest() *Registry {
	defaultRegistry = NewRegistry()
	return defaultRegistry
}

// NewRegistry constructs an empty in-memory registry.
func NewRegistry() *Registry {
	return &Registry{
		jobs:    make(map[string]*RuntimeState),
		records: make([]ExecutionRecord, 0, 64),
		now:     time.Now,
		loc:     time.Local,
	}
}

// SetLocation overrides the timezone used for miss detection (tests).
func (r *Registry) SetLocation(loc *time.Location) {
	if loc == nil {
		loc = time.Local
	}
	r.mu.Lock()
	r.loc = loc
	r.mu.Unlock()
}

// Register adds or updates a job definition. Idempotent by name.
func (r *Registry) Register(def Definition) {
	if def.Name == "" {
		return
	}
	if def.CatchUpPolicy == "" {
		def.CatchUpPolicy = CatchUpDetectOnly
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.jobs[def.Name]; ok {
		existing.ExpectedSchedule = def.ExpectedSchedule
		existing.CatchUpPolicy = def.CatchUpPolicy
		existing.Description = def.Description
		return
	}
	r.jobs[def.Name] = &RuntimeState{
		Name:             def.Name,
		ExpectedSchedule: def.ExpectedSchedule,
		RegisteredAt:     r.now().UTC(),
		CatchUpPolicy:    def.CatchUpPolicy,
		Description:      def.Description,
	}
}

// IsRegistered reports whether name is in the registry.
func (r *Registry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.jobs[name]
	return ok
}

// Get returns a copy of runtime state.
func (r *Registry) Get(name string) (RuntimeState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	st, ok := r.jobs[name]
	if !ok {
		return RuntimeState{}, false
	}
	return *st, true
}

// List returns all registered job states.
func (r *Registry) List() []RuntimeState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RuntimeState, 0, len(r.jobs))
	for _, st := range r.jobs {
		out = append(out, *st)
	}
	return out
}

// Records returns recent execution records (newest last), optionally filtered by job.
func (r *Registry) Records(jobName string, limit int) []ExecutionRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExecutionRecord, 0)
	for i := range r.records {
		if jobName != "" && r.records[i].Job != jobName {
			continue
		}
		out = append(out, r.records[i])
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Begin starts a running record; returns id for Complete.
func (r *Registry) Begin(jobName, trigger string) string {
	now := r.now().UTC()
	id := fmt.Sprintf("jer-%d", r.seq.Add(1))
	rec := ExecutionRecord{
		ID:        id,
		Job:       jobName,
		StartedAt: now,
		Status:    StatusRunning,
		Trigger:   trigger,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	if st, ok := r.jobs[jobName]; ok {
		t := now
		st.LastRun = &t
		st.LastStatus = StatusRunning
		st.LastError = ""
	}
	return id
}

// Complete finishes a running record by id.
func (r *Registry) Complete(id string, status Status, errMsg string) {
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.records) - 1; i >= 0; i-- {
		if r.records[i].ID != id {
			continue
		}
		r.records[i].FinishedAt = now
		r.records[i].Status = status
		r.records[i].Error = errMsg
		if st, ok := r.jobs[r.records[i].Job]; ok {
			t := now
			st.LastRun = &t
			st.LastStatus = status
			st.LastError = errMsg
		}
		return
	}
}

// Record appends a finished execution in one shot (tests / detect).
func (r *Registry) Record(rec ExecutionRecord) {
	if rec.ID == "" {
		rec.ID = fmt.Sprintf("jer-%d", r.seq.Add(1))
	}
	if rec.FinishedAt.IsZero() {
		rec.FinishedAt = r.now().UTC()
	}
	if rec.StartedAt.IsZero() {
		rec.StartedAt = rec.FinishedAt
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	if st, ok := r.jobs[rec.Job]; ok {
		t := rec.FinishedAt
		st.LastRun = &t
		st.LastStatus = rec.Status
		st.LastError = rec.Error
	}
}

// Observe wraps fn to record success/failure without changing fn semantics.
// Panics are re-thrown after failed record (caller PanicHandler still applies outside).
func (r *Registry) Observe(jobName string, fn func()) func() {
	return func() {
		id := r.Begin(jobName, "cron")
		defer func() {
			if p := recover(); p != nil {
				r.Complete(id, StatusFailed, fmt.Sprintf("panic: %v", p))
				panic(p)
			}
		}()
		fn()
		r.Complete(id, StatusSuccess, "")
	}
}

// ObserveErr wraps a func returning error.
func (r *Registry) ObserveErr(jobName string, fn func() error) func() {
	return func() {
		id := r.Begin(jobName, "cron")
		defer func() {
			if p := recover(); p != nil {
				r.Complete(id, StatusFailed, fmt.Sprintf("panic: %v", p))
				panic(p)
			}
		}()
		if err := fn(); err != nil {
			r.Complete(id, StatusFailed, err.Error())
			return
		}
		r.Complete(id, StatusSuccess, "")
	}
}

// ExpectedFireToday returns today's scheduled fire time in registry location, if any.
func ExpectedFireToday(schedule string, now time.Time, loc *time.Location) (time.Time, bool, error) {
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	sched, err := cronParser.Parse(schedule)
	if err != nil {
		return time.Time{}, false, err
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	next := sched.Next(start.Add(-time.Second))
	if next.Year() != now.Year() || next.Month() != now.Month() || next.Day() != now.Day() {
		return time.Time{}, false, nil
	}
	return next, true, nil
}

// DetectMissed finds jobs whose today's expected fire is overdue without a success/skip after it.
// Phase11-A: detection only — does not invoke trading jobs.
func (r *Registry) DetectMissed(now time.Time, grace time.Duration) []MissedInfo {
	if grace < 0 {
		grace = DefaultGrace
	}
	if now.IsZero() {
		now = r.now()
	}
	r.mu.RLock()
	loc := r.loc
	defs := make([]RuntimeState, 0, len(r.jobs))
	for _, st := range r.jobs {
		defs = append(defs, *st)
	}
	records := append([]ExecutionRecord(nil), r.records...)
	r.mu.RUnlock()

	out := make([]MissedInfo, 0)
	for _, st := range defs {
		if st.ExpectedSchedule == "" {
			continue
		}
		expected, ok, err := ExpectedFireToday(st.ExpectedSchedule, now, loc)
		if err != nil || !ok {
			continue
		}
		// Phase11-B: no MISSED on calendar-closed days (weekend / holiday).
		if !marketstate.Default.IsTradingDay(expected) {
			continue
		}
		if now.Before(expected.Add(grace)) {
			continue
		}
		if hasCoverage(records, st.Name, expected) {
			continue
		}
		if alreadyMissed(records, st.Name, expected) {
			continue
		}
		msg := fmt.Sprintf("job %s expected at %s but no success/skip recorded (detect_only; manual trigger allowed)",
			st.Name, expected.Format(time.RFC3339))
		info := MissedInfo{
			Job:              st.Name,
			ExpectedSchedule: st.ExpectedSchedule,
			ExpectedAt:       expected,
			DetectedAt:       now,
			LastRun:          st.LastRun,
			LastStatus:       st.LastStatus,
			Message:          msg,
			AllowManualOnly:  true,
		}
		out = append(out, info)
	}
	return out
}

// MarkMissed persists MISSED detection records for DetectMissed results (still no auto-run).
func (r *Registry) MarkMissed(misses []MissedInfo) []ExecutionRecord {
	wrote := make([]ExecutionRecord, 0, len(misses))
	for _, m := range misses {
		rec := ExecutionRecord{
			Job:        m.Job,
			StartedAt:  m.ExpectedAt,
			FinishedAt: m.DetectedAt,
			Status:     StatusMissed,
			Error:      m.Message,
			Trigger:    "detect",
			ExpectedAt: m.ExpectedAt,
		}
		r.Record(rec)
		wrote = append(wrote, rec)
	}
	return wrote
}

// DetectAndMark runs DetectMissed then MarkMissed.
func (r *Registry) DetectAndMark(now time.Time, grace time.Duration) []MissedInfo {
	misses := r.DetectMissed(now, grace)
	if len(misses) > 0 {
		r.MarkMissed(misses)
	}
	return misses
}

func hasCoverage(records []ExecutionRecord, job string, expected time.Time) bool {
	for i := range records {
		rec := records[i]
		if rec.Job != job {
			continue
		}
		if rec.Status != StatusSuccess && rec.Status != StatusSkipped {
			continue
		}
		// Covered if finished at/after expected (cron fired), or started at/after expected.
		t := rec.FinishedAt
		if t.IsZero() {
			t = rec.StartedAt
		}
		if !t.Before(expected) {
			return true
		}
	}
	return false
}

func alreadyMissed(records []ExecutionRecord, job string, expected time.Time) bool {
	for i := range records {
		rec := records[i]
		if rec.Job != job || rec.Status != StatusMissed {
			continue
		}
		if rec.ExpectedAt.Equal(expected) {
			return true
		}
	}
	return false
}
