/**
 * Phase16.21-D — lightweight self-check for statusDisplay (node).
 * Run: node frontend/src/utils/statusDisplay.selftest.mjs
 */
import {
  formatStatus,
  formatFieldTooltip,
  pendingMorningPrepLabel,
} from './statusDisplay.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

assert(formatStatus('READY', 'execution').label === '模拟执行准备', 'READY')
assert(formatStatus('WAITING_INTENT', 'execution').label === '待早盘准备', 'WAITING_INTENT')
assert(formatStatus('degraded', 'explanation').label === '部分可恢复', 'degraded')
assert(formatStatus('missing', 'explanation').label === '暂无历史解释', 'missing')
assert(formatStatus('new', 'research').label === '新建', 'research new')
assert(formatStatus('WATCHING', 'research').label === '观察中', 'watching')
assert(pendingMorningPrepLabel() === '待早盘准备', 'pending label')
assert(formatFieldTooltip('score').includes('不代表收益'), 'score tip')
assert(formatFieldTooltip('ref').includes('价格锚点'), 'ref tip')

console.log('statusDisplay.selftest OK')
