package scheduler

// Phase11-A Job Runtime Reliability lives in backend/job (JobRegistry / ExecutionRecord / MISSED detect).
// This priority-queue Scheduler is unchanged: trading App crons remain on robfig cron with job.Observe wrappers.
// Future catch-up runners may enqueue manual-only tasks here — never auto-trading from MISSED detection alone.
