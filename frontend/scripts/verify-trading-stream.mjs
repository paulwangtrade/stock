import assert from 'node:assert/strict'
import { createTradingStream, normalizeTradingEvent } from '../src/utils/tradingStream.js'
import { formatEventTime } from '../src/utils/tradingFormat.js'

const stream = createTradingStream(3)
stream.push({ eventId: 'e1', type: 'order_submitted', timestamp: '2026-07-14T10:01:02.123+08:00', stockCode: 'sz000001', side: 'buy', price: 10, volume: 100 })
stream.push({ eventId: 'e2', type: 'fill', timestamp: '2026-07-14T10:01:03.234+08:00', stockCode: 'sz000001', side: 'buy', price: 10, volume: 100 })
stream.push({ eventId: 'e3', type: 'position_changed', timestamp: '2026-07-14T10:01:04.345+08:00' })
stream.push({ eventId: 'e4', type: 'account_changed', timestamp: '2026-07-14T10:01:05.456+08:00' })
stream.push({ eventId: 'e5', type: 'order_submitted', timestamp: '2026-07-14T10:01:06.567+08:00', stockCode: 'sh600519', side: 'buy', price: 1, volume: 100 })
assert.equal(stream.size, 3)
assert.deepEqual(stream.snapshot().map(event => event.type), ['order_submitted', 'fill', 'order_submitted'])

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

// EventHub TradingEvent 信封：字段在 fill 嵌套对象
const hubFill = normalizeTradingEvent({
  sequence: 1,
  type: 'fill',
  source: 'paper',
  accountId: '1',
  orderId: '1',
  occurredAt: '2026-07-16T23:40:00+08:00',
  fill: {
    id: '1',
    orderId: '1',
    accountId: '1',
    stockCode: 'sz000001',
    stockName: '平安银行',
    side: 'buy',
    price: 10.77,
    volume: 9200,
    fee: 24.771,
    strategyTag: 'open_buy_mvp',
    filledAt: '2026-07-16T23:40:00+08:00',
  },
})
assert.equal(hubFill.stockCode, 'sz000001')
assert.equal(hubFill.stockName, '平安银行')
assert.equal(hubFill.side, 'buy')
assert.equal(hubFill.price, 10.77)
assert.equal(hubFill.volume, 9200)
assert.equal(hubFill.fee, 24.771)
assert.equal(hubFill.strategy, 'open_buy_mvp')

const hubOrder = normalizeTradingEvent({
  type: 'order_submitted',
  orderId: '2',
  order: {
    id: '2',
    stockCode: 'sh600519',
    stockName: '贵州茅台',
    side: 'buy',
    status: 'submitted',
    price: 1500,
    volume: 100,
    strategyTag: 'open_buy_mvp',
  },
})
assert.equal(hubOrder.stockCode, 'sh600519')
assert.equal(hubOrder.strategy, 'open_buy_mvp')
assert.equal(hubOrder.volume, 100)

console.log('量化交易事件队列与毫秒时间格式校验通过')
