import { pickField, toNumber } from './tradingFormat.js'

export const TRADING_STREAM_LIMIT = 500

let sequence = 0

function unwrapEvent(raw) {
  if (typeof raw === 'string') {
    try {
      return unwrapEvent(JSON.parse(raw))
    } catch {
      return { message: raw }
    }
  }
  const envelope = raw || {}
  const payload = pickField(envelope, 'data', 'Data', 'payload', 'Payload', 'event', 'Event')
  return payload && typeof payload === 'object'
    ? { ...envelope, ...payload }
    : envelope
}

export function normalizeTradingEvent(raw, source = 'stream') {
  const event = unwrapEvent(raw)
  const timestamp = pickField(
    event,
    'timestamp', 'Timestamp', 'time', 'Time', 'createdAt', 'CreatedAt',
    'updatedAt', 'UpdatedAt', 'tradingTime', 'TradingTime',
  ) || new Date().toISOString()
  const id = pickField(event, 'eventId', 'EventID', 'eventID', 'id', 'ID', 'orderId', 'OrderID')
  const type = String(pickField(event, 'type', 'Type', 'eventType', 'EventType') || '').toLowerCase()

  return {
    ...event,
    _key: id !== undefined
      ? `${source}:${id}:${type}`
      : `${source}:${timestamp}:${++sequence}`,
    source,
    timestamp,
    type: type || (source === 'snapshot' ? 'snapshot-order' : 'trade'),
    venue: pickField(event, 'venue', 'Venue', 'market', 'Market', 'exchange', 'Exchange') || 'PAPER',
    stockCode: String(pickField(event, 'stockCode', 'StockCode', 'code', 'Code', 'symbol', 'Symbol') || ''),
    stockName: String(pickField(event, 'stockName', 'StockName', 'name', 'Name') || ''),
    side: pickField(event, 'side', 'Side', 'direction', 'Direction') || '',
    orderType: pickField(event, 'orderType', 'OrderType', 'businessType', 'BusinessType', 'tradeType', 'TradeType') || '',
    strategy: pickField(event, 'strategyTag', 'StrategyTag', 'strategy', 'Strategy', 'strategyName', 'StrategyName') || '',
    status: pickField(event, 'status', 'Status', 'orderStatus', 'OrderStatus') || '',
    price: toNumber(pickField(event, 'filledPrice', 'FilledPrice', 'price', 'Price')),
    volume: toNumber(pickField(event, 'filledVolume', 'FilledVolume', 'filledVol', 'FilledVol', 'volume', 'Volume', 'quantity', 'Quantity')),
    fee: toNumber(pickField(event, 'fee', 'Fee', 'commission', 'Commission')),
    message: pickField(event, 'message', 'Message', 'reason', 'Reason') || '',
    reasonCode: String(pickField(event, 'reasonCode', 'ReasonCode', 'riskCode', 'RiskCode') || ''),
    currentValue: pickField(event, 'currentValue', 'CurrentValue', 'current', 'Current'),
    threshold: pickField(event, 'threshold', 'Threshold', 'limit', 'Limit'),
  }
}

function eventSignature(event) {
  const id = pickField(event, 'id', 'ID', 'orderId', 'OrderID')
  if (id !== undefined) return `order:${id}`
  return [
    event.stockCode,
    event.orderType || event.side,
    event.status,
    event.timestamp,
    event.price,
    event.volume,
  ].join('|')
}

export function createTradingStream(limit = TRADING_STREAM_LIMIT) {
  const max = Math.max(1, toNumber(limit, TRADING_STREAM_LIMIT))
  let events = []

  const trim = () => {
    if (events.length > max) events = events.slice(0, max)
    return events
  }

  return {
    push(raw) {
      const event = normalizeTradingEvent(raw)
      events = [event, ...events]
      return trim()
    },
    backfill(snapshot) {
      const existing = new Set(events.map(eventSignature))
      const orders = (pickField(snapshot, 'orders', 'Orders') || [])
        .map(order => ({ ...order, type: 'order_updated' }))
      const fills = (pickField(snapshot, 'fills', 'Fills') || [])
        .map(fill => ({ ...fill, type: 'fill' }))
      const additions = [...orders, ...fills]
        .map(item => normalizeTradingEvent(item, 'snapshot'))
        .filter((event) => !existing.has(eventSignature(event)))
      events = [...events, ...additions]
        .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      return trim()
    },
    snapshot() {
      return events.slice()
    },
    clear() {
      events = []
      return events
    },
    get size() {
      return events.length
    },
  }
}
