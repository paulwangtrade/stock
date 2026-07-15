import assert from 'node:assert/strict'
import { createTradingStream, normalizeTradingEvent } from '../src/utils/tradingStream.js'
import { formatEventTime } from '../src/utils/tradingFormat.js'

const stream = createTradingStream(3)
stream.push({ eventId: 'e1', type: 'order_submitted', timestamp: '2026-07-14T10:01:02.123+08:00' })
stream.push({ eventId: 'e2', type: 'fill', timestamp: '2026-07-14T10:01:03.234+08:00' })
stream.push({ eventId: 'e3', type: 'position_changed', timestamp: '2026-07-14T10:01:04.345+08:00' })
stream.push({ eventId: 'e4', type: 'account_changed', timestamp: '2026-07-14T10:01:05.456+08:00' })
assert.equal(stream.size, 3)
assert.deepEqual(stream.snapshot().map(event => event.type), ['account_changed', 'position_changed', 'fill'])

const fill = normalizeTradingEvent({
  EventID: 'f1',
  Type: 'fill',
  Timestamp: '2026-07-14T10:01:03.234+08:00',
  Data: {
    StockCode: '600519.SH',
    Side: 'buy',
    FilledPrice: 1500,
    FilledVolume: 100,
  },
})
assert.equal(fill.stockCode, '600519.SH')
assert.equal(fill.price, 1500)
assert.equal(fill.volume, 100)
assert.match(formatEventTime(fill.timestamp), /^\d{2}:\d{2}:\d{2}\.\d{3}$/)

console.log('量化交易事件队列与毫秒时间格式校验通过')
