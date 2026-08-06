package tradingcalendar

import (
	"testing"
	"time"
)

func TestNextTradingDay_SkipsWeekend(t *testing.T) {
	t.Parallel()
	cal := Calendar{Location: time.UTC}

	// Thursday → Friday
	thu := time.Date(2026, 7, 16, 15, 30, 0, 0, time.UTC) // Thu
	next, err := cal.NextTradingDay(thu)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Format(DateLayout); got != "2026-07-17" {
		t.Fatalf("Thu→Fri got %s", got)
	}

	// Friday → Monday
	fri := time.Date(2026, 7, 17, 15, 30, 0, 0, time.UTC)
	next, err = cal.NextTradingDay(fri)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Format(DateLayout); got != "2026-07-20" {
		t.Fatalf("Fri→Mon got %s", got)
	}

	// Saturday → Monday
	sat := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	next, err = cal.NextTradingDay(sat)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Format(DateLayout); got != "2026-07-20" {
		t.Fatalf("Sat→Mon got %s", got)
	}
}

func TestIsTradingDay_Weekend(t *testing.T) {
	t.Parallel()
	cal := Calendar{Location: time.UTC}
	sat := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	mon := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	if cal.IsTradingDay(sat) {
		t.Fatal("Saturday should not be trading")
	}
	if !cal.IsTradingDay(mon) {
		t.Fatal("Monday should be trading")
	}
}

func TestHolidayProvider_Extension(t *testing.T) {
	t.Parallel()
	// National Day week example: Wed 2026-09-30 workday, Thu-Fri holiday,
	// plus a fictional make-up Sunday open.
	holidays := StaticHolidays{
		NonTrading: map[string]bool{
			"2026-10-01": true,
			"2026-10-02": true,
		},
		MakeUp: map[string]bool{
			"2026-09-27": true, // Sunday make-up
		},
	}
	cal := Calendar{Holidays: holidays, Location: time.UTC}

	if !cal.IsTradingDay(time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("make-up Sunday should be trading")
	}
	if cal.IsTradingDay(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("Oct 1 holiday should not be trading")
	}

	// From 2026-09-30 (Wed) → skip Oct 1–2 → Oct 5? Wait Oct 3 Sat Oct 4 Sun → Oct 5 Mon
	next, err := cal.NextTradingDay(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Format(DateLayout); got != "2026-10-05" {
		t.Fatalf("after Sep 30 want 2026-10-05 got %s", got)
	}
}

func TestNextTradingDayString(t *testing.T) {
	t.Parallel()
	// Use Default (Local) but fixed Y-M-D parse is location-safe for date-only.
	got, err := NextTradingDayString("2026-07-17")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-07-20" {
		t.Fatalf("got %s want 2026-07-20", got)
	}
}

func TestPrevTradingDay_SkipsWeekend(t *testing.T) {
	t.Parallel()
	cal := Calendar{Location: time.UTC}

	// Monday → previous Friday
	mon := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	prev, err := cal.PrevTradingDay(mon)
	if err != nil {
		t.Fatal(err)
	}
	if got := prev.Format(DateLayout); got != "2026-07-17" {
		t.Fatalf("Mon→Fri got %s", got)
	}

	// Saturday → Friday
	sat := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	prev, err = cal.PrevTradingDay(sat)
	if err != nil {
		t.Fatal(err)
	}
	if got := prev.Format(DateLayout); got != "2026-07-17" {
		t.Fatalf("Sat→Fri got %s", got)
	}

	// Sunday → Friday
	sun := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	prev, err = cal.PrevTradingDay(sun)
	if err != nil {
		t.Fatal(err)
	}
	if got := prev.Format(DateLayout); got != "2026-07-17" {
		t.Fatalf("Sun→Fri got %s", got)
	}
}

func TestPrevTradingDay_RoundTripWithNext(t *testing.T) {
	t.Parallel()
	tradeDate := "2026-08-06" // Thursday
	prev, err := PrevTradingDayString(tradeDate)
	if err != nil {
		t.Fatal(err)
	}
	if prev != "2026-08-05" {
		t.Fatalf("Prev(%s)=%s want 2026-08-05", tradeDate, prev)
	}
	next, err := NextTradingDayString(prev)
	if err != nil {
		t.Fatal(err)
	}
	if next != tradeDate {
		t.Fatalf("Next(Prev(%s))=%s want %s", tradeDate, next, tradeDate)
	}
}

func TestParseDate_Errors(t *testing.T) {
	t.Parallel()
	if _, err := ParseDate(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseDate("2026/07/17"); err == nil {
		t.Fatal("expected error")
	}
}
