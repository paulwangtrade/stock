import assert from 'node:assert/strict'

// Pure logic mirror of paper lot/fee for offline verify (no Go runtime needed)
function roundLot(vol, lot = 100) {
  return Math.floor(vol / lot) * lot
}
function buyFee(amount) {
  return Math.max(5, amount * 0.00025)
}
function sellFee(amount) {
  return buyFee(amount) + amount * 0.0005
}

assert.equal(roundLot(250), 200)
assert.equal(roundLot(99), 0)
assert.ok(buyFee(100000) >= 5)
assert.ok(sellFee(100000) > buyFee(100000))

let cash = 1_000_000
const px = 10
const qty = 1000
const amount = px * qty
const fee = buyFee(amount)
cash -= amount + fee
assert.ok(cash < 1_000_000)
assert.equal(qty, 1000)

const order = {
  status: 'submitted',
  filledVolume: 0,
  volume: 1000,
}
const fills = []
function applyFill(fillQty, at) {
  assert.ok(fillQty > 0 && order.filledVolume + fillQty <= order.volume)
  fills.push({ sequence: fills.length + 1, quantity: fillQty, filledAt: at })
  order.filledVolume += fillQty
  order.status = order.filledVolume === order.volume ? 'filled' : 'partially_filled'
}
applyFill(400, '2026-07-14T10:00:00.123+08:00')
assert.equal(order.status, 'partially_filled')
applyFill(600, '2026-07-14T10:00:01.456+08:00')
assert.equal(order.status, 'filled')
assert.deepEqual(fills.map((row) => row.sequence), [1, 2])
assert.ok(new Date(fills[0].filledAt).getTime() <= new Date(fills[1].filledAt).getTime())
assert.match(new Date(fills[0].filledAt).toISOString(), /T\d{2}:\d{2}:\d{2}\.\d{3}Z$/)

console.log('模拟盘费用/整手/Fill状态机校验通过')
