package marketstate_test

import (
	"testing"
	"time"

	"go-stock/backend/marketstate"
	"go-stock/backend/tradingcalendar"

	"github.com/stretchr/testify/require"
)

func cst() *time.Location {
	return time.FixedZone("CST", 8*3600)
}

func at(h, m int) time.Time {
	// 2026-08-10 Monday
	return time.Date(2026, 8, 10, h, m, 0, 0, cst())
}

func newSvc(cal tradingcalendar.Calendar) *marketstate.Service {
	cal.Location = cst()
	return marketstate.New(func() time.Time { return at(10, 0) }, cst(), cal)
}

func TestOpenTime(t *testing.T) {
	svc := newSvc(tradingcalendar.Calendar{})
	require.Equal(t, marketstate.StateOpen, svc.SnapshotAt(at(9, 30)).State)
	require.Equal(t, marketstate.StateOpen, svc.SnapshotAt(at(10, 0)).State)
	require.Equal(t, marketstate.StateOpen, svc.SnapshotAt(at(14, 0)).State)
	require.True(t, svc.CanExecuteAt(at(10, 0)))
	require.False(t, svc.CanGeneratePlanAt(at(10, 0)))
}

func TestCloseTime(t *testing.T) {
	svc := newSvc(tradingcalendar.Calendar{})
	require.Equal(t, marketstate.StateClose, svc.SnapshotAt(at(15, 0)).State)
	require.Equal(t, marketstate.StateClose, svc.SnapshotAt(at(15, 29)).State)
	require.Equal(t, marketstate.StateAfterClose, svc.SnapshotAt(at(15, 30)).State)
	require.True(t, svc.CanExecuteAt(at(15, 10)))
	require.True(t, svc.CanGeneratePlanAt(at(15, 30)))
	require.False(t, svc.CanExecuteAt(at(15, 30)))
}

func TestPreOpenMidday(t *testing.T) {
	svc := newSvc(tradingcalendar.Calendar{})
	require.Equal(t, marketstate.StatePreOpen, svc.SnapshotAt(at(9, 20)).State)
	require.True(t, svc.CanGeneratePlanAt(at(9, 20)))
	require.False(t, svc.CanExecuteAt(at(9, 20)))

	require.Equal(t, marketstate.StateMidday, svc.SnapshotAt(at(12, 0)).State)
	require.False(t, svc.CanExecuteAt(at(12, 0)))
	require.False(t, svc.CanGeneratePlanAt(at(12, 0)))
}

func TestWeekend(t *testing.T) {
	svc := newSvc(tradingcalendar.Calendar{})
	sun := time.Date(2026, 8, 9, 10, 0, 0, 0, cst()) // Sunday
	snap := svc.SnapshotAt(sun)
	require.Equal(t, marketstate.StateClosed, snap.State)
	require.False(t, snap.IsTradingDay)
	require.Equal(t, "weekend", snap.Reason)
	require.False(t, svc.CanExecuteAt(sun))
	require.False(t, svc.CanGeneratePlanAt(sun))
}

func TestHoliday(t *testing.T) {
	cal := tradingcalendar.Calendar{
		Location: cst(),
		Holidays: tradingcalendar.StaticHolidays{
			NonTrading: map[string]bool{"2026-10-01": true},
			MakeUp:     map[string]bool{"2026-10-10": true}, // Saturday make-up
		},
	}
	svc := marketstate.New(time.Now, cst(), cal)

	national := time.Date(2026, 10, 1, 10, 0, 0, 0, cst()) // Thu holiday
	snap := svc.SnapshotAt(national)
	require.Equal(t, marketstate.StateClosed, snap.State)
	require.False(t, snap.IsTradingDay)
	require.Contains(t, snap.Reason, "holiday")

	makeup := time.Date(2026, 10, 10, 10, 0, 0, 0, cst()) // Sat make-up → OPEN window
	snap2 := svc.SnapshotAt(makeup)
	require.True(t, snap2.IsTradingDay)
	require.Equal(t, marketstate.StateOpen, snap2.State)
	require.True(t, svc.CanExecuteAt(makeup))
}

func TestPackageLevelHelpers(t *testing.T) {
	// Smoke: helpers delegate to Default (wall clock).
	_ = marketstate.GetCurrentMarketState()
	_ = marketstate.CanExecute()
	_ = marketstate.CanGeneratePlan()
	_ = marketstate.SnapshotNow()
}
