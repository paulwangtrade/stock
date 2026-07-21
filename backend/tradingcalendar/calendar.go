// Package tradingcalendar provides A-share oriented trading-day helpers.
//
// Phase6-A2.1: weekends are always non-trading unless a HolidayProvider marks a
// make-up workday. Statutory holidays are optional via HolidayProvider so callers
// can plug in a static table or remote API later without changing NextTradingDay.
package tradingcalendar

import (
	"fmt"
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

// HolidayProvider extends weekend rules with China holiday / make-up workdays.
// Implementations must be safe for concurrent use.
type HolidayProvider interface {
	// IsNonTradingHoliday reports a weekday (or any date) that is closed for trading
	// due to a holiday break (not merely Saturday/Sunday).
	IsNonTradingHoliday(day time.Time) bool
	// IsMakeUpWorkday reports a weekend (or holiday-adjacent) date that is open for trading.
	IsMakeUpWorkday(day time.Time) bool
}

// StaticHolidays is a simple in-memory HolidayProvider.
// Keys are YYYY-MM-DD in local wall dates (use date-only, ignore clock).
type StaticHolidays struct {
	// NonTrading maps holiday rest days → true.
	NonTrading map[string]bool
	// MakeUp maps调休上班 dates → true.
	MakeUp map[string]bool
}

func (s StaticHolidays) IsNonTradingHoliday(day time.Time) bool {
	if s.NonTrading == nil {
		return false
	}
	return s.NonTrading[FormatDate(day)]
}

func (s StaticHolidays) IsMakeUpWorkday(day time.Time) bool {
	if s.MakeUp == nil {
		return false
	}
	return s.MakeUp[FormatDate(day)]
}

// Calendar evaluates trading days. A nil Holidays provider means weekend-only rules.
type Calendar struct {
	Holidays HolidayProvider
	// Location for date truncation; nil → time.Local.
	Location *time.Location
}

// Default is a package-level calendar with weekend rules only (no holiday table).
var Default = Calendar{}

func (c Calendar) loc() *time.Location {
	if c.Location != nil {
		return c.Location
	}
	return time.Local
}

// TruncateDay returns the calendar date at 00:00 in c's location.
func (c Calendar) TruncateDay(t time.Time) time.Time {
	loc := c.loc()
	t = t.In(loc)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// FormatDate formats as YYYY-MM-DD in the calendar location.
func FormatDate(t time.Time) string {
	return Default.TruncateDay(t).Format(DateLayout)
}

// ParseDate parses YYYY-MM-DD into a truncated local date.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("tradingcalendar: empty date")
	}
	t, err := time.ParseInLocation(DateLayout, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("tradingcalendar: invalid date %q: %w", s, err)
	}
	return t, nil
}

// IsWeekend reports Saturday or Sunday (wall date in calendar location).
func (c Calendar) IsWeekend(day time.Time) bool {
	d := c.TruncateDay(day)
	wd := d.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// IsTradingDay reports whether the market is open on day.
// Order: make-up workday → non-trading holiday → weekend → else true.
func (c Calendar) IsTradingDay(day time.Time) bool {
	d := c.TruncateDay(day)
	if c.Holidays != nil {
		if c.Holidays.IsMakeUpWorkday(d) {
			return true
		}
		if c.Holidays.IsNonTradingHoliday(d) {
			return false
		}
	}
	return !c.IsWeekend(d)
}

// NextTradingDay returns the next trading day strictly after day.
// Searches at most 366 calendar days; returns an error if none found (misconfigured holidays).
func (c Calendar) NextTradingDay(day time.Time) (time.Time, error) {
	cur := c.TruncateDay(day).AddDate(0, 0, 1)
	for i := 0; i < 366; i++ {
		if c.IsTradingDay(cur) {
			return cur, nil
		}
		cur = cur.AddDate(0, 0, 1)
	}
	return time.Time{}, fmt.Errorf("tradingcalendar: no trading day within 366 days after %s", FormatDate(day))
}

// NextTradingDayString parses YYYY-MM-DD and returns the next trading day as YYYY-MM-DD.
func (c Calendar) NextTradingDayString(date string) (string, error) {
	day, err := ParseDate(date)
	if err != nil {
		return "", err
	}
	next, err := c.NextTradingDay(day)
	if err != nil {
		return "", err
	}
	return next.Format(DateLayout), nil
}

// Package-level helpers use Default (weekend-only until Holidays is set).

// IsTradingDay reports whether day is a trading day under Default.
func IsTradingDay(day time.Time) bool { return Default.IsTradingDay(day) }

// NextTradingDay returns the next trading day after day under Default.
func NextTradingDay(day time.Time) (time.Time, error) { return Default.NextTradingDay(day) }

// NextTradingDayString is the string form of NextTradingDay under Default.
func NextTradingDayString(date string) (string, error) { return Default.NextTradingDayString(date) }
