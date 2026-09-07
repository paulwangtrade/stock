package job_test

import (
	"testing"
	"time"

	"go-stock/backend/job"

	"github.com/stretchr/testify/require"
)

func TestJobRegistered(t *testing.T) {
	r := job.NewRegistry()
	r.Register(job.Definition{
		Name:             job.JobDailyPlan,
		ExpectedSchedule: "0 20 9 * * 1-5",
		CatchUpPolicy:    job.CatchUpDetectOnly,
	})
	require.True(t, r.IsRegistered(job.JobDailyPlan))
	st, ok := r.Get(job.JobDailyPlan)
	require.True(t, ok)
	require.Equal(t, "0 20 9 * * 1-5", st.ExpectedSchedule)
	require.Equal(t, job.CatchUpDetectOnly, st.CatchUpPolicy)
}

func TestSuccessRecord(t *testing.T) {
	r := job.NewRegistry()
	r.Register(job.Definition{Name: "j1", ExpectedSchedule: "0 0 9 * * *"})
	ran := false
	r.Observe("j1", func() { ran = true })()
	require.True(t, ran)
	st, ok := r.Get("j1")
	require.True(t, ok)
	require.Equal(t, job.StatusSuccess, st.LastStatus)
	require.NotNil(t, st.LastRun)
	recs := r.Records("j1", 10)
	require.Len(t, recs, 1)
	require.Equal(t, job.StatusSuccess, recs[0].Status)
	require.Equal(t, "cron", recs[0].Trigger)
	require.False(t, recs[0].FinishedAt.IsZero())
}

func TestFailedRecord(t *testing.T) {
	r := job.NewRegistry()
	r.Register(job.Definition{Name: "j2", ExpectedSchedule: "0 0 9 * * *"})
	r.ObserveErr("j2", func() error {
		return errSample
	})()
	st, ok := r.Get("j2")
	require.True(t, ok)
	require.Equal(t, job.StatusFailed, st.LastStatus)
	require.Contains(t, st.LastError, "boom")
	recs := r.Records("j2", 0)
	require.Len(t, recs, 1)
	require.Equal(t, job.StatusFailed, recs[0].Status)
}

var errSample = errString("boom")

type errString string

func (e errString) Error() string { return string(e) }

func TestMissedDetection(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	r := job.NewRegistry()
	r.SetLocation(loc)

	r.Register(job.Definition{
		Name:             job.JobDailyPlan,
		ExpectedSchedule: "0 20 9 * * 1-5",
		CatchUpPolicy:    job.CatchUpDetectOnly,
	})

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, loc) // well after 09:20
	misses := r.DetectMissed(now, job.DefaultGrace)
	require.Len(t, misses, 1)
	require.Equal(t, job.JobDailyPlan, misses[0].Job)
	require.True(t, misses[0].AllowManualOnly)
	require.Equal(t, time.Date(2026, 8, 10, 9, 20, 0, 0, loc), misses[0].ExpectedAt)

	// Mark — still no auto trade
	wrote := r.MarkMissed(misses)
	require.Len(t, wrote, 1)
	require.Equal(t, job.StatusMissed, wrote[0].Status)
	require.Equal(t, "detect", wrote[0].Trigger)

	st, _ := r.Get(job.JobDailyPlan)
	require.Equal(t, job.StatusMissed, st.LastStatus)

	// Idempotent detect for same slot
	misses2 := r.DetectMissed(now, job.DefaultGrace)
	require.Empty(t, misses2)

	// After a success covering the slot, no miss
	r2 := job.NewRegistry()
	r2.SetLocation(loc)
	r2.Register(job.Definition{Name: job.JobDailyPlan, ExpectedSchedule: "0 20 9 * * 1-5"})
	r2.Record(job.ExecutionRecord{
		Job:        job.JobDailyPlan,
		StartedAt:  time.Date(2026, 8, 10, 9, 20, 1, 0, loc),
		FinishedAt: time.Date(2026, 8, 10, 9, 20, 2, 0, loc),
		Status:     job.StatusSuccess,
		Trigger:    "cron",
	})
	require.Empty(t, r2.DetectMissed(now, job.DefaultGrace))
}

func TestMissedDetection_BeforeGrace_NoMiss(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	r := job.NewRegistry()
	r.SetLocation(loc)
	r.Register(job.Definition{Name: job.JobDailyPlan, ExpectedSchedule: "0 20 9 * * 1-5"})
	now := time.Date(2026, 8, 10, 9, 22, 0, 0, loc) // within 5m grace after 09:20
	require.Empty(t, r.DetectMissed(now, job.DefaultGrace))
}

func TestTradingCatalog_DetectOnly(t *testing.T) {
	for _, def := range job.TradingCatalog() {
		require.Equal(t, job.CatchUpDetectOnly, def.CatchUpPolicy)
		require.NotEmpty(t, def.Name)
		require.NotEmpty(t, def.ExpectedSchedule)
	}
}

func TestTradingCatalog_IncludesPaperTradingT1Unlock(t *testing.T) {
	found := false
	for _, def := range job.TradingCatalog() {
		if def.Name == job.JobPaperTradingT1Unlock {
			found = true
			require.Equal(t, "0 20 9 * * 1-5", def.ExpectedSchedule)
			require.Equal(t, job.CatchUpDetectOnly, def.CatchUpPolicy)
			break
		}
	}
	require.True(t, found, "TradingCatalog must include paper_trading_t1_unlock so PositionUnlockJob is monitored")
}

func TestExpectedFireToday_Weekend(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	// 2026-08-09 is Sunday
	sun := time.Date(2026, 8, 9, 12, 0, 0, 0, loc)
	_, ok, err := job.ExpectedFireToday("0 20 9 * * 1-5", sun, loc)
	require.NoError(t, err)
	require.False(t, ok)
}
