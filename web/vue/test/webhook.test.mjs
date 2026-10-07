import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {createRequire} from 'node:module'

const source = readFileSync(new URL('../src/utils/webhook.js', import.meta.url))
const {WEBHOOK_PROVIDERS, WEBHOOK_TEMPLATES, webhookPayload} = await import(`data:text/javascript;base64,${source.toString('base64')}`)
const require = createRequire(import.meta.url)
const Vue = require('vue')
const componentSource = readFileSync(new URL('../src/pages/system/notification/webhook.vue', import.meta.url), 'utf8')
const script = componentSource.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '').replace('export default', 'return')

test('robot defaults have platform-specific message fields and no embedded signature', () => {
  assert.equal(JSON.parse(WEBHOOK_TEMPLATES.dingtalk.replace(/{{[^}]+}}/g, 'sample')).msgtype, 'text')
  assert.equal(JSON.parse(WEBHOOK_TEMPLATES.feishu.replace(/{{[^}]+}}/g, 'sample')).msg_type, 'text')
  for (const template of Object.values(WEBHOOK_TEMPLATES)) assert.doesNotMatch(template, /"sign"|"secret"|"timestamp"/)
  const {template} = require('vue-template-compiler').parseComponent(componentSource)
  assert.deepEqual(require('vue-template-compiler').compile(template.content).errors, [])
  assert.match(componentSource, /type="password" show-password/)
})

test('blank secrets preserve existing keys; clearing and platform changes require explicit input', () => {
  const form = {provider: 'dingtalk', sign_enabled: true, secret: '', clear_secret: false}
  assert.throws(() => webhookPayload(form, false), /密钥/)
  assert.equal(webhookPayload(form, true).secret, '')
  assert.equal(webhookPayload({...form, secret: '  fixture-secret  '}, false).secret, 'fixture-secret')
  assert.throws(() => webhookPayload({...form, clear_secret: true}, true), /密钥/)
  assert.equal(webhookPayload({...form, sign_enabled: false, clear_secret: true}, true).clear_secret, 1)
  assert.throws(() => webhookPayload({...form, provider: 'generic'}, true), /钉钉或飞书/)
  assert.throws(() => webhookPayload({...form, provider: '__proto__'}, true), /平台/)
})

test('editing configs does not echo a key and failed saves are retryable', () => {
  const requests = []
  let load
  const service = {
    webhooks (callback) { load = callback },
    storeWebhook (payload, callback, failed) { requests.push({payload, callback, failed}) }
  }
  const component = new Function('notificationService', 'WEBHOOK_PROVIDERS', 'WEBHOOK_TEMPLATES', 'webhookPayload', 'systemSidebar', 'notificationTab', script)(
    service, WEBHOOK_PROVIDERS, WEBHOOK_TEMPLATES, webhookPayload, {}, {})
  const vm = new Vue({...component, beforeCreate () {
    this.$message = {success () {}, error () {}}
    this.$appConfirm = callback => callback()
  }})
  const stored = {id: 9, name: '运维钉钉', url: 'https://example.invalid/robot', template: WEBHOOK_TEMPLATES.dingtalk,
    provider: 'dingtalk', sign_enabled: true, has_secret: true}
  load([stored])
  vm.editWebhook(stored)
  assert.equal(vm.form.secret, '')
  assert.equal(vm.hasUsableSecret, true)
  vm.save()
  requests[0].failed()
  assert.equal(vm.saving, false)
  vm.save()
  assert.equal(requests[1].payload.secret, '')
  requests[1].callback()
  assert.equal(vm.saving, false)
  // Loading a legacy configuration must not accidentally enable signing.
  vm.editWebhook({id: 0, name: '默认 Webhook', url: 'https://example.invalid/legacy', template: WEBHOOK_TEMPLATES.generic})
  assert.equal(vm.form.provider, 'generic')
  assert.equal(vm.form.sign_enabled, false)
  vm.form.provider = 'feishu'
  vm.changeProvider('feishu')
  assert.equal(vm.form.template, WEBHOOK_TEMPLATES.feishu)
  assert.equal(vm.hasUsableSecret, false)
  vm.form.template = '{"custom":true}'
  vm.form.provider = 'dingtalk'
  vm.changeProvider('dingtalk')
  assert.equal(vm.form.template, '{"custom":true}')
  vm.useDefaultTemplate()
  assert.equal(vm.form.template, WEBHOOK_TEMPLATES.dingtalk)
  vm.$destroy()
})

test('multiple endpoints keep platform, independent IDs and names after reload', () => {
  let loaded
  let saved
  const service = {
    webhooks (callback) { loaded = callback },
    storeWebhook (payload, callback) { saved = {payload, callback} }
  }
  const component = new Function('notificationService', 'WEBHOOK_PROVIDERS', 'WEBHOOK_TEMPLATES', 'webhookPayload', 'systemSidebar', 'notificationTab', script)(
    service, WEBHOOK_PROVIDERS, WEBHOOK_TEMPLATES, webhookPayload, {}, {})
  const vm = new Vue({...component, beforeCreate () { this.$message = {success () {}, error () {}} }})
  const rows = [
    {id: 1, name: '运维', provider: 'dingtalk', url: 'https://example.invalid/a', template: WEBHOOK_TEMPLATES.dingtalk},
    {id: 2, name: '日报', provider: 'feishu', url: 'https://example.invalid/b', template: WEBHOOK_TEMPLATES.feishu}
  ]
  loaded(rows)
  vm.editWebhook(rows[0])
  vm.save()
  assert.equal(saved.payload.id, 1)
  assert.equal(saved.payload.provider, 'dingtalk')
  saved.callback()
  loaded(rows)
  vm.editWebhook(vm.endpoints[0])
  assert.equal(vm.form.provider, 'dingtalk')
  assert.equal(vm.form.id, 1)
  vm.createWebhook()
  assert.equal(vm.form.id, -1)
  assert.equal(vm.form.secret, '')
  assert.equal(vm.hasUsableSecret, false)
  assert.equal(vm.providerLabel('feishu'), '飞书机器人')
  vm.$destroy()
})

test('test-send requires confirmation, handles failure and never saves', async () => {
  const requests = []
  let resolveConfirm
  const service = {
    webhooks () {},
    storeWebhook () { throw new Error('test must not save') },
    testWebhook (payload, callback, failed) { requests.push({payload, callback, failed}) }
  }
  const component = new Function('notificationService', 'WEBHOOK_PROVIDERS', 'WEBHOOK_TEMPLATES', 'webhookPayload', 'systemSidebar', 'notificationTab', script)(
    service, WEBHOOK_PROVIDERS, WEBHOOK_TEMPLATES, webhookPayload, {}, {})
  const vm = new Vue({...component, beforeCreate () {
    this.$message = {success () {}, error () {}}
    this.$confirm = () => new Promise(resolve => { resolveConfirm = resolve })
  }})
  vm.testSaved({id: 3, name: '样本'})
  assert.equal(requests.length, 0)
  resolveConfirm()
  await Promise.resolve()
  assert.equal(vm.testing, true)
  assert.deepEqual(requests[0].payload, {id: 3})
  requests[0].failed()
  assert.equal(vm.testing, false)
  vm.testSaved({id: 3, name: '样本'})
  resolveConfirm()
  await Promise.resolve()
  requests[1].callback()
  assert.equal(vm.testing, false)
  vm.$destroy()
})

test('task editor supports separate webhook selections and removes preview banner', () => {
  const editor = readFileSync(new URL('../src/pages/task/edit.vue', import.meta.url), 'utf8')
  assert.match(editor, /v-model="selectedWebhookIds"/)
  assert.match(editor, /notify_type === 4/)
  assert.match(editor, /selectedWebhookIds\.join\(','\)/)
  const {template} = require('vue-template-compiler').parseComponent(editor)
  assert.deepEqual(require('vue-template-compiler').compile(template.content).errors, [])
  assert.doesNotMatch(readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8'), /本地界面预览|preview-notice/)
})

test('legacy default Webhook name is editable and is submitted without changing its identity', () => {
  assert.doesNotMatch(componentSource, /:disabled="form.id === 0"/)
  let saved
  const service = {
    webhooks () {},
    storeWebhook (payload) { saved = payload }
  }
  const component = new Function('notificationService', 'WEBHOOK_PROVIDERS', 'WEBHOOK_TEMPLATES', 'webhookPayload', 'systemSidebar', 'notificationTab', script)(
    service, WEBHOOK_PROVIDERS, WEBHOOK_TEMPLATES, webhookPayload, {}, {})
  const vm = new Vue({...component, beforeCreate () { this.$message = {success () {}, error () {}} }})
  vm.editWebhook({id: 0, name: '默认 Webhook', url: 'https://example.invalid/robot', template: WEBHOOK_TEMPLATES.dingtalk,
    provider: 'dingtalk', sign_enabled: true, has_secret: true})
  vm.form.name = '运维钉钉群'
  vm.save()
  assert.equal(saved.id, 0)
  assert.equal(saved.name, '运维钉钉群')
  assert.equal(saved.secret, '')
  assert.equal(saved.sign_enabled, 1)
  assert.equal(saved.provider, 'dingtalk')
  vm.$destroy()
})
