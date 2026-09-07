/**
 * Unit tests: opportunityListPagination (Phase16.26-B).
 * Run: node frontend/src/utils/opportunityListPagination.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const {
  OPPORTUNITY_LIST_PAGINATE_THRESHOLD,
  formatOpportunityListCountLabel,
  shouldPaginateOpportunityList,
  sliceOpportunityPage,
} = await import(pathToFileURL(join(__dir, 'opportunityListPagination.js')).href)

assert.equal(OPPORTUNITY_LIST_PAGINATE_THRESHOLD, 50)
assert.equal(shouldPaginateOpportunityList(29), false)
assert.equal(shouldPaginateOpportunityList(50), false)
assert.equal(shouldPaginateOpportunityList(51), true)
assert.equal(formatOpportunityListCountLabel(29), '共 29 只股票')

{
  const rows = Array.from({ length: 29 }, (_, i) => i)
  assert.equal(sliceOpportunityPage(rows, { page: 1, pageSize: 20, paginate: false }).length, 29)
  assert.deepEqual(sliceOpportunityPage(rows, { page: 2, pageSize: 20, paginate: true }), [
    20, 21, 22, 23, 24, 25, 26, 27, 28,
  ])
}

console.log('opportunityListPagination.test.mjs: all passed')
