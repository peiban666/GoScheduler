import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'

// Load the ES module without changing the legacy webpack package to type:module.
const source = readFileSync(new URL('../src/utils/schedule.js', import.meta.url))
const {buildSchedule, describeSchedule, parseSchedule, MAX_SPEC_LENGTH, MAX_INTERVAL_MINUTES} = await import(`data:text/javascript;base64,${source.toString('base64')}`)

test('daily times preserve minute/hour pairs rather than cross-product', () => {
  const state = {mode: 'daily', times: ['07:30', '20:00', '03:00']}
  const spec = buildSchedule(state)
  assert.equal(spec, '0 0 3,20 * * *\n0 30 7 * * *')
  assert.deepEqual(parseSchedule(spec).times, ['03:00', '07:30', '20:00'])
  assert.equal(describeSchedule(spec), '每天 03:00、07:30、20:00')
})

test('minute/hour modes and old screenshot expressions', () => {
  assert.equal(buildSchedule({mode: 'minutely'}), '0 * * * * *')
  assert.equal(buildSchedule({mode: 'hourly', minute: 15}), '0 15 * * * *')
  assert.equal(parseSchedule('0 15 * * * *').minute, 15)
  assert.deepEqual(parseSchedule('0 30 13,17 * * *').times, ['13:30', '17:30'])
  assert.deepEqual(parseSchedule('0 50 10 * * *').times, ['10:50'])
})

test('minute intervals round-trip as durations, not minute-field Cron steps', () => {
  for (const intervalMinutes of [1, 5, 7, 10, 30, 90, MAX_INTERVAL_MINUTES]) {
    const spec = `@every ${intervalMinutes}m`
    assert.equal(buildSchedule({mode: 'interval', intervalMinutes}), spec)
    const state = parseSchedule(spec)
    assert.equal(state.mode, 'interval')
    assert.equal(state.intervalMinutes, intervalMinutes)
    assert.equal(buildSchedule(state), spec)
    assert.equal(describeSchedule(spec), `每 ${intervalMinutes} 分钟执行一次`)
  }
  assert.equal(buildSchedule({mode: 'interval', intervalMinutes: '7'}), '@every 7m')
  assert.equal(parseSchedule('0 * * * * *').mode, 'minutely')
})

test('minute intervals reject missing, fractional and out-of-range input', () => {
  for (const intervalMinutes of ['', undefined, null, NaN, Infinity, 0, -1, 1.5, '1.5', '1e2', MAX_INTERVAL_MINUTES + 1]) {
    assert.throws(() => buildSchedule({mode: 'interval', intervalMinutes}), /整数分钟/)
  }
})

test('duplicates are removed and equal-minute times compacted', () => {
  assert.equal(buildSchedule({mode: 'daily', times: ['07:30', '07:30', '20:30']}), '0 30 7,20 * * *')
  assert.deepEqual(parseSchedule('0 30 7 * * *;0 30 7,20 * * *').times, ['07:30', '20:30'])
})

test('advanced expressions are not guessed or silently replaced', () => {
  for (const spec of ['*/5 * * * * *', '0 */7 * * * *', '0 0 7 * * 1-5', '@every 10s', '@every 1h', '@every 5m30s', '@every 0m', '@every 525601m', '30 30 7 * * *', '0 0 24 * * *']) {
    const state = parseSchedule(spec)
    assert.equal(state.mode, 'advanced')
    assert.equal(buildSchedule(state), spec)
  }
})

test('common descriptors are recognized', () => {
  assert.equal(parseSchedule('@hourly').mode, 'hourly')
  assert.deepEqual(parseSchedule('@daily').times, ['00:00'])
})

test('empty/invalid time selections are rejected', () => {
  for (const times of [[], [''], ['24:00'], ['07:60'], ['7:30'], [null]]) {
    assert.throws(() => buildSchedule({mode: 'daily', times}))
  }
  assert.throws(() => buildSchedule({mode: 'hourly', minute: 60}))
  assert.throws(() => buildSchedule({mode: 'advanced', advanced: ''}))
})

test('existing database length limit is enforced before saving', () => {
  const times = Array.from({length: 10}, (_, minute) => `07:${String(minute).padStart(2, '0')}`)
  assert.throws(() => buildSchedule({mode: 'daily', times}), /64/)
  assert.throws(() => buildSchedule({mode: 'advanced', advanced: 'x'.repeat(MAX_SPEC_LENGTH + 1)}), /64/)
})

test('interval rules are standalone to preserve their anchor', () => {
  assert.throws(() => buildSchedule({mode: 'advanced', advanced: '@every 10s;@every 15s'}), /@every/)
  assert.equal(buildSchedule({mode: 'advanced', advanced: '@every 10s;@every 10s'}), '@every 10s')
})

// Verify component feedback and mode switches using Vue's real reactivity.
const {createRequire} = await import('node:module')
const require = createRequire(import.meta.url)
const Vue = require('vue')
const componentSource = readFileSync(new URL('../src/components/task/schedulePicker.vue', import.meta.url), 'utf8')
const script = componentSource.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/m, '').replace('export default', 'return')
const component = new Function('buildSchedule', 'describeSchedule', 'parseSchedule', 'MAX_INTERVAL_MINUTES', script)(buildSchedule, describeSchedule, parseSchedule, MAX_INTERVAL_MINUTES)

function picker (spec) {
  const vm = new Vue({...component, propsData: {value: spec}})
  vm.$on('input', value => { vm.value = value })
  return vm
}

test('component retains selected times when switching to advanced and back', async () => {
  const vm = picker('0 30 7 * * *')
  vm.state.times.push('20:00', '03:00')
  await Vue.nextTick()
  vm.changeMode('advanced')
  await Vue.nextTick()
  assert.equal(vm.state.advanced, '0 0 3,20 * * *\n0 30 7 * * *')
  vm.changeMode('daily')
  await Vue.nextTick()
  assert.deepEqual(Array.from(vm.state.times), ['03:00', '07:30', '20:00'])
  vm.$destroy()
})

test('component keeps daily mode during incomplete selections', async () => {
  const vm = picker('0 30 7 * * *')
  vm.state.times.push('')
  await Vue.nextTick()
  assert.equal(vm.value, '')
  assert.equal(vm.state.mode, 'daily')
  Vue.set(vm.state.times, 1, '20:00')
  await Vue.nextTick()
  assert.equal(vm.value, '0 0 20 * * *\n0 30 7 * * *')
  vm.$destroy()
})

test('component updates hourly minute and retains legacy advanced expressions', async () => {
  const vm = picker('0 0 * * * *')
  vm.state.minute = 15
  await Vue.nextTick()
  assert.equal(vm.value, '0 15 * * * *')
  assert.equal(vm.state.mode, 'hourly')
  vm.value = '@every 10s'
  await Vue.nextTick()
  assert.equal(vm.state.mode, 'advanced')
  assert.equal(vm.result.spec, '@every 10s')
  vm.$destroy()
})

test('component edits intervals and retains them through advanced mode', async () => {
  const vm = picker('0 30 7 * * *')
  vm.changeMode('interval')
  await Vue.nextTick()
  assert.equal(vm.value, '@every 1m')
  vm.intervalMinutes = 7
  await Vue.nextTick()
  assert.equal(vm.value, '@every 7m')
  vm.changeMode('advanced')
  await Vue.nextTick()
  assert.equal(vm.state.advanced, '@every 7m')
  vm.state.advanced = '@every 30m'
  await Vue.nextTick()
  vm.changeMode('interval')
  await Vue.nextTick()
  assert.equal(vm.state.intervalMinutes, 30)
  assert.equal(vm.value, '@every 30m')
  vm.$destroy()
})

test('merged minute control preserves legacy Cron until its interval changes', async () => {
  const vm = picker('0 * * * * *')
  assert.equal(vm.pickerMode, 'interval')
  assert.equal(vm.intervalMinutes, 1)
  assert.equal(vm.result.spec, '0 * * * * *')
  vm.changeMode('advanced')
  await Vue.nextTick()
  vm.changeMode('interval')
  await Vue.nextTick()
  assert.equal(vm.pickerMode, 'interval')
  assert.equal(vm.value, '0 * * * * *')
  vm.intervalMinutes = 5
  await Vue.nextTick()
  assert.equal(vm.value, '@every 5m')
  vm.intervalMinutes = 1
  await Vue.nextTick()
  assert.equal(vm.value, '@every 1m')
  vm.$destroy()
})

test('minute picker exposes one unified radio option', () => {
  const template = componentSource.match(/<template>([\s\S]*?)<\/template>/)[1]
  assert.equal((template.match(/<el-radio-button label="interval">/g) || []).length, 1)
  assert.doesNotMatch(template, /<el-radio-button label="minutely">/)
})

test('visual picker hides generated expressions without removing readable summaries', () => {
  const template = componentSource.match(/<template>([\s\S]*?)<\/template>/)[1]
  assert.doesNotMatch(template, /实际调度规则|<pre\b|{{\s*result\.spec\s*}}/)
  assert.match(template, /{{description}}/)
  assert.match(template, /v-else-if="pickerMode !== 'advanced'"/)
  assert.match(template, /v-model="state\.advanced"/)
  for (const spec of ['@every 1m', '0 * * * * *', '0 15 * * * *', '0 0 3,20 * * *\n0 30 7 * * *']) {
    const vm = picker(spec)
    assert.equal(vm.result.spec, spec)
    assert.ok(vm.description.length > 0)
    vm.$destroy()
  }
})

test('task list retains readable schedules without a raw-expression tooltip', () => {
  const listSource = readFileSync(new URL('../src/components/task/taskTable.vue', import.meta.url), 'utf8')
  assert.doesNotMatch(listSource, /<el-tooltip\b|:content="scope\.row\.spec"/)
  assert.match(listSource, /{{scope\.row\.spec \| formatSchedule}}/)
  assert.match(listSource, /formatSchedule: describeSchedule/)
})

test('component keeps interval mode during empty input and accepts saved intervals', async () => {
  const vm = picker('@every 7m')
  vm.state.intervalMinutes = undefined
  await Vue.nextTick()
  assert.equal(vm.value, '')
  assert.equal(vm.state.mode, 'interval')
  assert.match(vm.result.error, /整数分钟/)
  vm.state.intervalMinutes = 10
  await Vue.nextTick()
  assert.equal(vm.value, '@every 10m')
  vm.value = '@every 90m'
  await Vue.nextTick()
  assert.equal(vm.state.mode, 'interval')
  assert.equal(vm.state.intervalMinutes, 90)
  assert.equal(vm.description, '每 90 分钟执行一次')
  vm.$destroy()
})
