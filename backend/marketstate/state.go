// Package marketstate provides a unified A-share oriented Market Session state machine.
//
// It only classifies calendar + wall-clock time. It does not place orders, fill,
// settle, or run strategies. PaperTrading Session A/B policy remains separate;
// this package answers higher-level PRE_OPEN / OPEN / MIDDAY / CLOSE / AFTER_CLOSE.
package marketstate

import (
	"time"

	"go-stock/backend/tradingcalendar"
)

// State is the market session state.
type State string

const (
	StatePreOpen    State = "PRE_OPEN"    // trading day, before 09:30
	StateOpen       State = "OPEN"        // continuous auction windows
	StateMidday     State = "MIDDAY"      // lunch break
	StateClose      State = "CLOSE"       // 15:00–15:30 close window
	StateAfterClose State = "AFTER_CLOSE" // after 15:30 on a trading day
	StateClosed     State = "CLOSED"      // weekend / holiday (calendar closed)
)

// Snapshot is a point-in-time market session evaluation.
type Snapshot struct {
	State        State     `json:"state"`
	IsTradingDay bool      `json:"isTradingDay"`
	LocalTime    time.Time `json:"localTime"`
	Reason       string    `json:"reason,omitempty"`
}

// Clock abstracts time for tests.
type Clock func() time.Time

// Service evaluates market session state.
type Service struct {
	clock    Clock
	loc      *time.Location
	calendar tradingcalendar.Calendar
}

// Default is the process-wide service (Local + tradingcalendar.Default).
var Default = New(nil, nil, tradingcalendar.Default)

// New constructs a Service. nil clock → time.Now; nil loc → time.Local.
func New(clock Clock, loc *time.Location, cal tradingcalendar.Calendar) *Service {
	if clock == nil {
		clock = time.Now
	}
	if loc == nil {
		loc = time.Local
	}
	return &Service{clock: clock, loc: loc, calendar: cal}
}

// GetCurrentMarketState returns the session state for "now".
func GetCurrentMarketState() State {
	return Default.GetCurrentMarketState()
}

// CanExecute reports whether the current session allows execution windows (OPEN/CLOSE).
func CanExecute() bool {
	return Default.CanExecute()
}

// CanGeneratePlan reports whether plan generation windows are open (PRE_OPEN / AFTER_CLOSE).
func CanGeneratePlan() bool {
	return Default.CanGeneratePlan()
}

// SnapshotNow returns a full snapshot at "now".
func SnapshotNow() Snapshot {
	return Default.SnapshotAt(Default.clock())
}

// GetCurrentMarketState implements Service.
func (s *Service) GetCurrentMarketState() State {
	return s.SnapshotAt(s.clock()).State
}

// CanExecute is true only in OPEN or CLOSE on a trading day.
func (s *Service) CanExecute() bool {
	st := s.GetCurrentMarketState()
	return st == StateOpen || st == StateClose
}

// CanGeneratePlan is true in PRE_OPEN (morning prep) or AFTER_CLOSE (draft workflow).
func (s *Service) CanGeneratePlan() bool {
	st := s.GetCurrentMarketState()
	return st == StatePreOpen || st == StateAfterClose
}

// SnapshotAt evaluates state at an explicit instant.
func (s *Service) SnapshotAt(now time.Time) Snapshot {
	if s == nil {
		return Default.SnapshotAt(now)
	}
	loc := s.loc
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	cal := s.calendar
	cal.Location = loc

	snap := Snapshot{LocalTime: now}
	if !cal.IsTradingDay(now) {
		snap.State = StateClosed
		snap.IsTradingDay = false
		if cal.IsWeekend(now) {
			snap.Reason = "weekend"
		} else {
			snap.Reason = "holiday_or_non_trading"
		}
		return snap
	}

	snap.IsTradingDay = true
	mins := now.Hour()*60 + now.Minute()
	switch {
	case mins < 9*60+30:
		snap.State = StatePreOpen
		snap.Reason = "before_continuous_auction"
	case mins >= 9*60+30 && mins < 11*60+30:
		snap.State = StateOpen
		snap.Reason = "morning_continuous"
	case mins >= 11*60+30 && mins < 13*60:
		snap.State = StateMidday
		snap.Reason = "lunch_break"
	case mins >= 13*60 && mins < 15*60:
		snap.State = StateOpen
		snap.Reason = "afternoon_continuous"
	case mins >= 15*60 && mins < 15*60+30:
		snap.State = StateClose
		snap.Reason = "close_window"
	default:
		snap.State = StateAfterClose
		snap.Reason = "after_close"
	}
	return snap
}

// CanExecuteAt / CanGeneratePlanAt evaluate gates at an explicit time (tests / jobs).
func (s *Service) CanExecuteAt(now time.Time) bool {
	st := s.SnapshotAt(now).State
	return st == StateOpen || st == StateClose
}

func (s *Service) CanGeneratePlanAt(now time.Time) bool {
	st := s.SnapshotAt(now).State
	return st == StatePreOpen || st == StateAfterClose
}

// IsTradingDay reports calendar trading-day status for now.
func (s *Service) IsTradingDay(now time.Time) bool {
	return s.SnapshotAt(now).IsTradingDay
}
