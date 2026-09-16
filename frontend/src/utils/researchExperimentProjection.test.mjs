import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import {
  projectResearchExperiment,
  createResearchExperiment,
  listResearchExperiments,
  getResearchExperiment,
  clearResearchExperimentStore,
  readFindingIdRef,
} from './researchExperimentProjection.js'

function report() {
  return {
    schema_version: 'research_report_projection.v1',
    sections: {
      batchOutcome: {
        available: true,
        positive_path_share: 0.5,
        returns: { avg_t5: 0.02 },
        mfe: { max: 0.06 },
        mae: { min: -0.03 },
      },
      experiments: { count: 1, items: [{ experimentId: 'sexp:old' }] },
      backtest: {
        available: true,
        strategy_identity: 'srev:prior_close_compare:v1',
        dataset_identity: 'bdsv-mvp',
        evaluator_identity: 'evalimpl:1205d265aa367514be7492d9428faed067348402ffdd753d73f4e40022ef1c0c',
        metrics: {
          total_return: 0.12,
          max_drawdown: 0.02,
          exposure: 0.25,
          trade_count: 2,
          holding_period: { avg_days: 2, closed_rounds: 1 },
        },
      },
    },
  }
}

function result() {
  return {
    schema_version: 'backtest_result.v1',
    return: { total_return: -0.2 },
    drawdown: { max_drawdown: 0.4 },
    trade_count: 4,
    fills: [{ fill_id: 'simfill-1' }],
    orders: [{ order_id: 'simord-1' }],
    account: { initial_cash: 10000, final_equity: 8000 },
  }
}

function input() {
  return {
    hypothesis: '收盘高于前收时，模拟账户是否留下可复述的成交摘要',
    researchReport: report(),
    backtestResult: result(),
  }
}

function finding(id = 'rf:signal_type=BREAKOUT') {
  return {
    schema_version: 'research_finding.v1',
    identity: { finding_id: id, pattern_key: 'signal_type=BREAKOUT' },
    statistics: {
      event_success_rate: 0.6,
      average_path_return_t5: 0.03,
    },
  }
}

test('empty input does not create an experiment', () => {
  for (const value of [undefined, null, {}, { hypothesis: '仅有假设' }, { researchReport: report() }]) {
    const card = projectResearchExperiment(value)
    assert.equal(card.schema_version, 'experiment.v1')
    assert.equal(card.available, false)
    assert.equal(card.experiment_id, null)
    assert.equal(card.finding_id, null)
    assert.equal(card.status, null)
  }
})

test('a hypothesis plus frozen references creates a recorded card', () => {
  const card = projectResearchExperiment(input())
  assert.equal(card.available, true)
  assert.equal(card.schema_version, 'experiment.v1')
  assert.match(card.experiment_id, /^rexp:[0-9a-f]{8}$/)
  assert.equal(card.hypothesis, input().hypothesis)
  assert.equal(card.finding_id, null)
  assert.equal(card.referenced_strategy_identity, 'srev:prior_close_compare:v1')
  assert.equal(card.referenced_dataset_identity, 'bdsv-mvp')
  assert.deepEqual(card.referenced_backtest_result, {
    schema_version: 'backtest_result.v1',
    evaluator_identity: 'evalimpl:1205d265aa367514be7492d9428faed067348402ffdd753d73f4e40022ef1c0c',
  })
  assert.equal(card.status, 'recorded')
  assert.equal(projectResearchExperiment(input()).experiment_id, card.experiment_id)
})

test('hypothesis plus finding_id creates a recorded card without report', () => {
  const card = projectResearchExperiment({
    hypothesis: '该突破分组的路径分布是否值得单独验证',
    finding_id: 'rf:signal_type=BREAKOUT',
  })
  assert.equal(card.available, true)
  assert.equal(card.finding_id, 'rf:signal_type=BREAKOUT')
  assert.equal(card.status, 'recorded')
  assert.match(card.experiment_id, /^rexp:[0-9a-f]{8}$/)
  assert.equal(card.referenced_strategy_identity, null)
  assert.equal(card.referenced_backtest_result, null)
})

test('finding object supplies finding_id only — statistics are not copied', () => {
  const card = projectResearchExperiment({
    hypothesis: '只引用发现身份',
    finding: finding(),
  })
  assert.equal(card.finding_id, 'rf:signal_type=BREAKOUT')
  const blob = JSON.stringify(card)
  assert.equal(blob.includes('event_success_rate'), false)
  assert.equal(blob.includes('average_path_return_t5'), false)
  assert.equal(blob.includes('0.6'), false)
})

test('readFindingIdRef accepts string or finding object', () => {
  assert.equal(readFindingIdRef('rf:a'), 'rf:a')
  assert.equal(readFindingIdRef(finding('rf:b')), 'rf:b')
  assert.equal(readFindingIdRef(null), null)
})

test('session store create / list / get', () => {
  clearResearchExperimentStore()
  const created = createResearchExperiment({
    hypothesis: '会话内可查询',
    finding_id: 'rf:signal_type=BREAKOUT',
  })
  assert.equal(created.available, true)
  assert.equal(listResearchExperiments().length, 1)
  assert.equal(getResearchExperiment(created.experiment_id)?.hypothesis, '会话内可查询')
  assert.equal(getResearchExperiment('missing'), null)
  createResearchExperiment({
    hypothesis: '会话内可查询',
    finding_id: 'rf:signal_type=BREAKOUT',
  })
  assert.equal(listResearchExperiments().length, 1)
  clearResearchExperimentStore()
  assert.equal(listResearchExperiments().length, 0)
})

test('input report and result are not modified', () => {
  const bag = input()
  const before = JSON.parse(JSON.stringify(bag))
  projectResearchExperiment(bag)
  assert.deepEqual(bag, before)
})

test('does not copy metrics or generate a strategy', () => {
  const card = projectResearchExperiment(input())
  const blob = JSON.stringify(card)
  for (const key of [
    'total_return',
    'max_drawdown',
    'trade_count',
    'fills',
    'orders',
    'positive_path_share',
    'parameter_changes',
    'strategy_version',
    'win_rate',
    'score',
    'rank',
    'best',
  ]) {
    assert.equal(blob.includes(`"${key}"`), false, key)
  }
  assert.equal(card.status, 'recorded')
  assert.notEqual(card.status, 'success')
  assert.notEqual(card.status, 'failure')
})

test('module does not create a strategy or call execution', () => {
  const source = readFileSync(fileURLToPath(new URL('./researchExperimentProjection.js', import.meta.url)), 'utf8')
  for (const token of [
    'createStrategyVersion',
    'projectStrategyVersion',
    'Activate',
    'ExecutePlanItem',
    'TradePlan',
    'paper_sim',
    'projectBacktestResult',
    'projectResearchReport',
    'parameter_changes',
    'summarizeOutcomeStats',
    'runResearchBatch',
  ]) {
    assert.equal(source.includes(token), false, token)
  }
})
