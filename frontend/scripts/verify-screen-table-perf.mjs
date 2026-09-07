import assert from 'node:assert/strict'
import {
  SCREEN_TABLE_PAGE_SIZES,
  SCREEN_TABLE_DEFAULT_PAGE_SIZE,
  SCREEN_VIRTUAL_ON,
  nextScreenVirtualEnabled,
  clampScreenPageSize,
} from '../src/utils/screenTablePerf.js'

assert.deepEqual(SCREEN_TABLE_PAGE_SIZES, [20, 30, 50])
assert.equal(clampScreenPageSize(500), SCREEN_TABLE_DEFAULT_PAGE_SIZE)
assert.equal(clampScreenPageSize(30), 30)

let v = false
v = nextScreenVirtualEnabled(v, 50)
assert.equal(v, false)
v = nextScreenVirtualEnabled(v, SCREEN_VIRTUAL_ON)
assert.equal(v, true)
v = nextScreenVirtualEnabled(v, 80)
assert.equal(v, true, 'hysteresis keep on')
v = nextScreenVirtualEnabled(v, 50)
assert.equal(v, false)

console.log('screenTablePerf: ok')
