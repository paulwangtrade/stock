package papertrading

import "time"

// ExecutionSession is the intraday window for track-B Fill (Phase10-C.2-B.1).
// Price selection (realtime vs close) is intentionally out of scope for B.1.
type ExecutionSession string

const (
	SessionA      ExecutionSession = "A"      // 09:30-11:30, 13:00-15:00 — allow
	SessionB      ExecutionSession = "B"      // 15:00-15:30 — allow (close price later in C.2-C)
	SessionC      ExecutionSession = "C"      // 15:30+ — reject immediate fill
	SessionClosed ExecutionSession = "closed" // <09:30 or 11:30-13:00 — reject
)

// Policy decisions.
const (
	PolicyAllow  = "allow"
	PolicyReject = "reject"
)

// Run ledger / result status when Session Policy blocks Fill.
const RunStatusSkippedOutsideSession = "skipped_outside_session"

// SessionPolicyDecision is the Gateway-facing outcome of time-window evaluation.
type SessionPolicyDecision struct {
	Session  ExecutionSession `json:"session"`
	Allow    bool             `json:"allow"`
	Decision string           `json:"decision"` // allow | reject
	Reason   string           `json:"reason"`
}

// ResolveExecutionSession classifies local clock into A/B/C/closed.
// Boundaries (half-open):
//
//	A:  [09:30, 11:30) ∪ [13:00, 15:00)
//	B:  [15:00, 15:30)
//	C:  [15:30, 24:00)
//	closed: [00:00, 09:30) ∪ [11:30, 13:00)
//
// Does NOT inspect weekday/calendar — that remains SkipWeekdayCheck / IsWeekdayLocal.
func ResolveExecutionSession(now time.Time) ExecutionSession {
	mins := now.Hour()*60 + now.Minute()
	switch {
	case mins >= 9*60+30 && mins < 11*60+30:
		return SessionA
	case mins >= 11*60+30 && mins < 13*60:
		return SessionClosed
	case mins >= 13*60 && mins < 15*60:
		return SessionA
	case mins >= 15*60 && mins < 15*60+30:
		return SessionB
	case mins >= 15*60+30:
		return SessionC
	default:
		return SessionClosed
	}
}

// EvaluateSessionPolicy returns allow/reject for Fill entry.
// Session Policy cannot be skipped; weekday bypass is orthogonal.
func EvaluateSessionPolicy(now time.Time) SessionPolicyDecision {
	sess := ResolveExecutionSession(now)
	switch sess {
	case SessionA:
		return SessionPolicyDecision{
			Session:  SessionA,
			Allow:    true,
			Decision: PolicyAllow,
			Reason:   "session_A_continuous_auction",
		}
	case SessionB:
		return SessionPolicyDecision{
			Session:  SessionB,
			Allow:    true,
			Decision: PolicyAllow,
			Reason:   "session_B_close_window",
		}
	case SessionC:
		return SessionPolicyDecision{
			Session:  SessionC,
			Allow:    false,
			Decision: PolicyReject,
			Reason:   "session_C_after_close_no_immediate_fill",
		}
	default:
		return SessionPolicyDecision{
			Session:  SessionClosed,
			Allow:    false,
			Decision: PolicyReject,
			Reason:   "session_closed_no_immediate_fill",
		}
	}
}
