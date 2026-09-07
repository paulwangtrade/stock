import assert from 'node:assert/strict'
import {
  chunkWatchlistVirtualRows,
  nextWatchlistVirtualEnabled,
} from '../src/utils/watchlistVirtualScroll.js'

assert.equal(nextWatchlistVirtualEnabled(false, 49), false)
assert.equal(nextWatchlistVirtualEnabled(false, 80), true)
assert.equal(nextWatchlistVirtualEnabled(true, 60), true) // 滞回保持
assert.equal(nextWatchlistVirtualEnabled(true, 49), false)
assert.equal(nextWatchlistVirtualEnabled(false, 60), false)

const rows = chunkWatchlistVirtualRows(
  [{ '股票代码': 'a' }, { '股票代码': 'b' }, { '股票代码': 'c' }, { '股票代码': 'd' }, { '股票代码': 'e' }],
  2,
)
assert.equal(rows.length, 3)
assert.equal(rows[0].cells.length, 2)
assert.equal(rows[2].cells.length, 1)
assert.equal(rows[0].startIndex, 0)
assert.equal(rows[1].cells[0].cardIndex, 2)

console.log('verify-watchlist-virtual-scroll: ok')
