/** Phase14-H1: signal scan task poll + table loading helpers (pure, testable). */

export const SIGNAL_SCAN_TASK_TERMINAL_STATUSES = new Set([
  'completed',
  'failed',
  'cancelled',
])

export const SIGNAL_SCAN_TASK_ACTIVE_STATUSES = new Set(['running', 'pending'])

export function normalizeSignalScanTaskStatus(status) {
  return String(status ?? '').trim().toLowerCase()
}

/** True when poll should stop (completed / failed / cancelled / empty). */
export function isSignalScanTaskTerminalStatus(status) {
  const st = normalizeSignalScanTaskStatus(status)
  if (!st) return true
  return SIGNAL_SCAN_TASK_TERMINAL_STATUSES.has(st)
}

export function isSignalScanTaskActiveStatus(status) {
  const st = normalizeSignalScanTaskStatus(status)
  return SIGNAL_SCAN_TASK_ACTIVE_STATUSES.has(st)
}

/**
 * Outcome for one scan-task poll tick.
 * releaseSignalScanLoading is true for all terminal states (including completed).
 */
export function resolveSignalScanTaskPollOutcome(taskStatus) {
  const st = normalizeSignalScanTaskStatus(taskStatus)
  if (!isSignalScanTaskTerminalStatus(st)) {
    return {
      shouldStopPoll: false,
      releaseBackendLoading: false,
      releaseSignalScanLoading: false,
      clearScanStatus: false,
    }
  }
  const stillActive = st === 'running' || st === 'pending'
  return {
    shouldStopPoll: true,
    releaseBackendLoading: !stillActive,
    releaseSignalScanLoading: true,
    clearScanStatus: true,
  }
}

/** Stock screen table :loading — user-initiated fetch only; not backend/page signal scan. */
export function resolveStockScreenTableLoading(loadingRef) {
  return !!loadingRef
}
