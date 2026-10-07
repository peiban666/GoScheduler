import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'

const source = readFileSync(new URL('../src/utils/responseData.js', import.meta.url))
const {normalizePageResponse, requireObjectResponse} = await import(`data:text/javascript;base64,${source.toString('base64')}`)

test('list responses retain records and normalize total and a valid empty slice', () => {
  assert.deepEqual(normalizePageResponse({total: '1', data: [{id: 1}]}), {total: 1, data: [{id: 1}]})
  assert.deepEqual(normalizePageResponse({total: 0, data: null}), {total: 0, data: []})
  assert.deepEqual(normalizePageResponse({total: 25, data: []}), {total: 25, data: []})
})

test('missing list payloads produce a readable service error, not a null dereference', () => {
  for (const value of [null, undefined, [], 'invalid', {}, {total: -1, data: []}, {total: 1, data: null}, {total: 0}]) {
    assert.throws(() => normalizePageResponse(value), /列表接口/)
  }
  for (const value of [null, undefined, [], 'invalid']) assert.throws(() => requireObjectResponse(value), /配置接口/)
  assert.deepEqual(requireObjectResponse({url: '', template: ''}), {url: '', template: ''})
})

test('all list APIs validate page shape and user pagination is forwarded', () => {
  for (const file of ['user', 'host', 'taskLog', 'system']) {
    const api = readFileSync(new URL(`../src/api/${file}.js`, import.meta.url), 'utf8')
    assert.match(api, /httpClient\.getPage\('[^']+', query, callback\)/)
  }
  const notification = readFileSync(new URL('../src/api/notification.js', import.meta.url), 'utf8')
  assert.equal((notification.match(/httpClient\.getObject/g) || []).length, 3)
})

test('task creation fetches choices without requesting an undefined task', () => {
  const source = readFileSync(new URL('../src/api/task.js', import.meta.url), 'utf8')
  const script = source.replace(/^import .*$/gm, '').replace('export default', 'return')
  const calls = []
  const client = {batchGet (requests, callback) { calls.push({requests, callback}) }}
  const service = new Function('httpClient', script)(client)
  let result
  service.detail(undefined, (...values) => { result = values })
  assert.deepEqual(calls[0].requests.map(item => item.uri), ['/host/all', '/task/groups'])
  calls[0].callback([{id: 1}], [{name: '备份'}])
  assert.deepEqual(result, [null, [{id: 1}], [{name: '备份'}]])
  service.detail(1, () => {})
  assert.deepEqual(calls[1].requests.map(item => item.uri), ['/task/1', '/host/all', '/task/groups'])
})

test('switching create/edit routes recreates forms instead of retaining previous values', () => {
  const app = readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')
  assert.match(app, /router-view :key="\$route\.fullPath"/)
})
