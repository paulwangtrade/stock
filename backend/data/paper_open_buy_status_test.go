package data

import (
	"testing"
	"time"
)

func TestIsWeekdayLocal(t *testing.T) {
	mon := time.Date(2026, 7, 13, 9, 0, 0, 0, time.Local) // Monday
	sat := time.Date(2026, 7, 18, 9, 0, 0, 0, time.Local) // Saturday
	if !IsWeekdayLocal(mon) {
		t.Fatal("monday should be weekday")
	}
	if IsWeekdayLocal(sat) {
		t.Fatal("saturday should not be weekday")
	}
}
