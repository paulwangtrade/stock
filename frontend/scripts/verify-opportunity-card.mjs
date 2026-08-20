import assert from 'node:assert/strict'
import { UNKNOWN_STOCK_NAME } from '../src/utils/stockDisplay.js'
import {
  collectOpportunityNameHints,
  toOpportunityCards,
} from '../src/utils/opportunityCard.js'

/** Audit fixture: attention.stock_code was the holding; candidate lived only on opportunity_attention. */
const holdingCode = 'sz000021'
const candidateCode = 'sz000858'
const opp = {
  candidate_code: candidateCode,
  candidate_score: 53,
  user_action: 'WATCH',
  highlights: [
    {
      holding_code: holdingCode,
      holding_score: 44,
      reason: '候选分 53 高于持仓分 44（分差 9）',
    },
  ],
}

const attentionRow = {
  item_type: 'OPPORTUNITY',
  stock_code: holdingCode,
  title: '机会对比',
  reason: opp.highlights[0].reason,
}

{
  const cards = toOpportunityCards(opp)
  assert.equal(cards.length, 1)
  const card = cards[0]
  assert.equal(card.kind, 'opportunity_compare')

  // Old UI would show attention.stock_code (holding) as the opportunity ticker.
  assert.equal(attentionRow.stock_code, holdingCode)
  assert.notEqual(card.candidate_stock.code, attentionRow.stock_code)
  assert.equal(card.candidate_stock.code, candidateCode)
  assert.equal(card.holding_stock.code, holdingCode)

  assert.equal(card.candidate_stock.display.display_code, '000858.SZ')
  assert.equal(card.holding_stock.display.display_code, '000021.SZ')
  assert.equal(card.candidate_stock.display.display_name, UNKNOWN_STOCK_NAME)
  assert.equal(card.holding_stock.display.display_name, UNKNOWN_STOCK_NAME)

  assert.equal(card.candidate_score, 53)
  assert.equal(card.holding_score, 44)
  assert.equal(card.score_gap, 9)
  assert.equal(card.user_label, '关注')
}

{
  const cards = toOpportunityCards(opp, { sz000858: '五粮液', sz000021: '深科技' })
  assert.equal(cards[0].candidate_stock.name, '五粮液')
  assert.equal(cards[0].holding_stock.display.display_name, '深科技')
  assert.equal(cards[0].candidate_stock.display.display_name, '五粮液')
}

{
  const hints = collectOpportunityNameHints({
    daily_summary: {
      position_attention: [{ stock_code: 'sz000021', stock_name: '深科技' }],
    },
    daily_attention: {
      items: [{ stock_code: holdingCode, stock_name: holdingCode, item_type: 'OPPORTUNITY' }],
    },
  })
  assert.equal(hints.sz000021, '深科技')
  const cards = toOpportunityCards(opp, hints)
  assert.equal(cards[0].holding_stock.display.display_name, '深科技')
  assert.equal(cards[0].candidate_stock.display.display_name, UNKNOWN_STOCK_NAME)
}

assert.deepEqual(toOpportunityCards(null), [])
assert.deepEqual(toOpportunityCards({ candidate_code: candidateCode, candidate_score: 53, highlights: [] }), [])
assert.deepEqual(
  toOpportunityCards({
    candidate_code: candidateCode,
    candidate_score: 53,
    highlights: [{ holding_code: holdingCode }],
  }),
  [],
)

{
  const many = {
    candidate_code: candidateCode,
    candidate_score: 53,
    user_action: 'REVIEW',
    highlights: [
      { holding_code: 'sz000021', holding_score: 44 },
      { holding_code: 'sz000002', holding_score: 40 },
      { holding_code: 'sz000001', holding_score: 30 },
      { holding_code: 'sz000004', holding_score: 20 },
    ],
  }
  const cards = toOpportunityCards(many, {}, 3)
  assert.equal(cards.length, 3)
  assert.equal(cards[0].user_label, '建议研究')
  for (const card of cards) {
    assert.equal(card.candidate_stock.code, candidateCode)
    assert.notEqual(card.holding_stock.code, candidateCode)
  }
}

console.log('verify-opportunity-card: ok')
