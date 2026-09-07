// Package job provides Job Runtime Reliability: registry, execution records,
// and MISSED detection (detect-only; no automatic trading catch-up).
//
// Phase11-A observes existing robfig cron registrations via wrappers.
// It does not replace App schedulers or change TradePlan / Broker / Gateway behavior.
package job

import "time"

// Status is an execution or detection outcome.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusRunning Status = "running"
	StatusMissed  Status = "missed"
	StatusSkipped Status = "skipped" // weekday gate etc.; still counts as observed run
)

// CatchUpPolicy controls post-miss behavior. Phase11-A only detects.
type CatchUpPolicy string

const (
	// CatchUpDetectOnly records MISSED and allows manual trigger later — never auto-trades.
	CatchUpDetectOnly CatchUpPolicy = "detect_only"
)

// Definition declares a monitored job (schedule metadata for miss detection).
type Definition struct {
	Name             string
	ExpectedSchedule string // robfig cron with seconds, e.g. "0 20 9 * * 1-5"
	CatchUpPolicy    CatchUpPolicy
	Description      string
}

// RuntimeState is the registry snapshot for one job.
type RuntimeState struct {
	Name             string         `json:"jobName"`
	ExpectedSchedule string         `json:"expectedSchedule"`
	LastRun          *time.Time     `json:"lastRun,omitempty"`
	LastStatus       Status         `json:"lastStatus,omitempty"`
	LastError        string         `json:"lastError,omitempty"`
	RegisteredAt     time.Time      `json:"registeredAt"`
	CatchUpPolicy    CatchUpPolicy  `json:"catchUpPolicy"`
	Description      string         `json:"description,omitempty"`
}

// ExecutionRecord is one observed run or miss detection row.
type ExecutionRecord struct {
	ID          string    `json:"id"`
	Job         string    `json:"job"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	Status      Status    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Trigger     string    `json:"trigger,omitempty"` // cron | manual | detect
	ExpectedAt  time.Time `json:"expected_at,omitempty"`
}

// MissedInfo is a detect-only miss finding (no auto execution).
type MissedInfo struct {
	Job              string    `json:"job"`
	ExpectedSchedule string    `json:"expectedSchedule"`
	ExpectedAt       time.Time `json:"expectedAt"`
	DetectedAt       time.Time `json:"detectedAt"`
	LastRun          *time.Time `json:"lastRun,omitempty"`
	LastStatus       Status    `json:"lastStatus,omitempty"`
	Message          string    `json:"message"`
	AllowManualOnly  bool      `json:"allowManualOnly"` // always true in Phase11-A
}
