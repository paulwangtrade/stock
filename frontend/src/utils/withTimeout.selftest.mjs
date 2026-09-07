import { withTimeout } from './withTimeout.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg || 'assert failed')
}

async function main() {
  const ok = await withTimeout(Promise.resolve(42), 1000, 'ok')
  assert(ok === 42, 'resolve passthrough')

  let timedOut = false
  try {
    await withTimeout(new Promise(() => {}), 50, 'hang')
  } catch (e) {
    timedOut = String(e?.message || e).includes('timeout')
  }
  assert(timedOut, 'must timeout')

  console.log('withTimeout.selftest: PASS')
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
