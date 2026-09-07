/**
 * Phase13 Onboarding MVP — FirstLaunchState (local only).
 * No account / cloud / payment. Does not touch trading APIs or strategy writes.
 */

export const FIRST_LAUNCH_STORAGE_KEY = 'go-stock.firstLaunch'
/** Bumped for disclaimerAcceptedAt (Phase13 Onboarding MVP). */
export const FIRST_LAUNCH_STATE_VERSION = 2

/**
 * @typedef {{
 *   version: number,
 *   completed: boolean,
 *   skipped: boolean,
 *   completedAt: string|null,
 *   disclaimerAcceptedAt: string|null,
 *   lastStep: number
 * }} FirstLaunchState
 */

/** @returns {FirstLaunchState} */
export function defaultFirstLaunchState() {
  return {
    version: FIRST_LAUNCH_STATE_VERSION,
    completed: false,
    skipped: false,
    completedAt: null,
    disclaimerAcceptedAt: null,
    lastStep: 0,
  }
}

/**
 * @param {unknown} raw
 * @returns {FirstLaunchState}
 */
export function normalizeFirstLaunchState(raw) {
  const base = defaultFirstLaunchState()
  if (!raw || typeof raw !== 'object') return base
  const o = /** @type {Record<string, unknown>} */ (raw)
  return {
    version: FIRST_LAUNCH_STATE_VERSION,
    completed: Boolean(o.completed),
    skipped: Boolean(o.skipped),
    completedAt: typeof o.completedAt === 'string' ? o.completedAt : null,
    disclaimerAcceptedAt: typeof o.disclaimerAcceptedAt === 'string' ? o.disclaimerAcceptedAt : null,
    lastStep: Number.isFinite(Number(o.lastStep)) ? Math.max(0, Math.floor(Number(o.lastStep))) : 0,
  }
}

/**
 * @param {Pick<Storage, 'getItem'>} [storage]
 * @returns {FirstLaunchState}
 */
export function readFirstLaunchState(storage = typeof localStorage !== 'undefined' ? localStorage : undefined) {
  if (!storage) return defaultFirstLaunchState()
  try {
    const raw = storage.getItem(FIRST_LAUNCH_STORAGE_KEY)
    if (!raw) return defaultFirstLaunchState()
    return normalizeFirstLaunchState(JSON.parse(raw))
  } catch {
    return defaultFirstLaunchState()
  }
}

/**
 * @param {FirstLaunchState} state
 * @param {Pick<Storage, 'setItem'>} [storage]
 */
export function writeFirstLaunchState(state, storage = typeof localStorage !== 'undefined' ? localStorage : undefined) {
  if (!storage) return
  const next = normalizeFirstLaunchState(state)
  storage.setItem(FIRST_LAUNCH_STORAGE_KEY, JSON.stringify(next))
}

/**
 * @param {Pick<Storage, 'getItem'>} [storage]
 */
export function shouldShowFirstLaunch(storage) {
  const s = readFirstLaunchState(storage)
  return !s.completed && !s.skipped
}

/**
 * Complete path requires risk acceptance (disclaimerAcceptedAt).
 * @param {number} [lastStep]
 * @param {Pick<Storage, 'getItem'|'setItem'>} [storage]
 * @param {{ disclaimerAcceptedAt?: string }} [opts]
 */
export function markFirstLaunchCompleted(lastStep = 0, storage, opts = {}) {
  const prev = readFirstLaunchState(storage)
  const acceptedAt =
    typeof opts.disclaimerAcceptedAt === 'string' && opts.disclaimerAcceptedAt
      ? opts.disclaimerAcceptedAt
      : new Date().toISOString()
  writeFirstLaunchState(
    {
      ...prev,
      completed: true,
      skipped: false,
      completedAt: new Date().toISOString(),
      disclaimerAcceptedAt: acceptedAt,
      lastStep,
    },
    storage,
  )
}

/**
 * Skip does not imply risk acceptance (disclaimerAcceptedAt stays unset unless already set).
 * @param {number} [lastStep]
 * @param {Pick<Storage, 'getItem'|'setItem'>} [storage]
 */
export function markFirstLaunchSkipped(lastStep = 0, storage) {
  const prev = readFirstLaunchState(storage)
  writeFirstLaunchState(
    {
      ...prev,
      completed: false,
      skipped: true,
      completedAt: new Date().toISOString(),
      lastStep,
    },
    storage,
  )
}

/**
 * Test helper: clear first-launch flags (does not touch productTier / trading data).
 * @param {Pick<Storage, 'removeItem'>} [storage]
 */
export function resetFirstLaunchState(storage = typeof localStorage !== 'undefined' ? localStorage : undefined) {
  if (!storage) return
  storage.removeItem(FIRST_LAUNCH_STORAGE_KEY)
}
