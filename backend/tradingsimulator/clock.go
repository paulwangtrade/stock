package tradingsimulator

import "time"

// Clock is a request-local simulation clock. It is never installed globally.
type Clock struct {
	t time.Time
}

func NewClock(date, hhmmss string, loc *time.Location) *Clock {
	if loc == nil {
		loc = time.Local
	}
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		day = time.Date(2026, 8, 17, 0, 0, 0, 0, loc)
	}
	return &Clock{t: parseOnDay(day, hhmmss)}
}

func (c *Clock) Now() time.Time {
	if c == nil {
		return time.Now()
	}
	return c.t
}

func (c *Clock) SetTime(hhmmss string) {
	if c == nil {
		return
	}
	c.t = parseOnDay(c.t, hhmmss)
}

// AdvanceTo moves the clock forward on the same calendar day. Backward moves are ignored.
func (c *Clock) AdvanceTo(hhmmss string) {
	if c == nil {
		return
	}
	next := parseOnDay(c.t, hhmmss)
	if next.Before(c.t) {
		return
	}
	c.t = next
}

func parseOnDay(day time.Time, hhmmss string) time.Time {
	loc := day.Location()
	t, err := time.ParseInLocation("15:04:05", hhmmss, loc)
	if err != nil {
		t, err = time.ParseInLocation("15:04", hhmmss, loc)
		if err != nil {
			return day
		}
	}
	y, m, d := day.Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, loc)
}
