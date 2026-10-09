import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {createRequire} from 'node:module'

const source = readFileSync(new URL('../src/utils/taskGroups.js', import.meta.url))
const {groupKey, groupQuery, prepareGroups, normalizeGroupName} = await import(`data:text/javascript;base64,${source.toString('base64')}`)
const Vue = createRequire(import.meta.url)('vue')
const listSource = readFileSync(new URL('../src/pages/task/list.vue', import.meta.url), 'utf8')
const script = listSource.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '').replace('export default', 'return')

function list () {
  const calls = {groups: [], groupTasks: [], assignGroup: [], deleteGroup: [], copyTasks: []}
  const service = {
    hosts (callback) { callback([]) },
    groups (query, callback, failed) { calls.groups.push({query, callback, failed}) },
    groupTasks (query, callback, failed) { calls.groupTasks.push({query, callback, failed}) },
    assignGroup (ids, tag, callback, failed) { calls.assignGroup.push({ids, tag, callback, failed}) },
    copyTasks (ids, tag, callback, failed) { calls.copyTasks.push({ids, tag, callback, failed}) },
    deleteGroup (tag, deleteTasks, expectedCount, callback, failed) { calls.deleteGroup.push({tag, deleteTasks, expectedCount, callback, failed}) }
  }
  const component = new Function('taskService', 'groupKey', 'groupQuery', 'prepareGroups', 'normalizeGroupName', 'taskSidebar', 'taskTable', 'groupPicker', 'groupCreateDialog', 'groupRenameDialog', script)(service, groupKey, groupQuery, prepareGroups, normalizeGroupName, {}, {}, {}, {}, {})
  const vm = new Vue({
    ...component,
    beforeCreate () {
      this.$store = {getters: {user: {isAdmin: true}}}
      this.$route = {query: {}}
      this.$message = {success () {}, error () {}}
      this.$router = {push () {}}
    }
  })
  return {vm, calls}
}

const groups = [{name: '', total: 3}, {name: '备份', total: 52}, {name: '同步', total: 7}]

test('single copy defaults to its source group without changing original tasks', () => {
  const {vm, calls} = list()
  vm.openCopyTasks({id: 42, tag: '备份'})
  assert.equal(vm.copyDialog, true)
  assert.deepEqual(Array.from(vm.copyIDs), [42])
  assert.equal(vm.copyGroup, '备份')
  assert.equal(calls.copyTasks.length, 0)
  vm.copyGroup = '  同步  '
  vm.confirmCopyTasks()
  vm.confirmCopyTasks()
  assert.equal(calls.copyTasks.length, 1)
  assert.deepEqual(Array.from(calls.copyTasks[0].ids), [42])
  assert.equal(calls.copyTasks[0].tag, '同步')
  assert.equal(calls.assignGroup.length, 0)
  calls.copyTasks[0].callback({copied: 1})
  assert.equal(vm.copyDialog, false)
  assert.equal(vm.copyingTasks, false)
  assert.deepEqual(Array.from(vm.expandedGroups), ['group:同步'])
  vm.$destroy()
})

test('batch copy snapshots cross-group selections and new-group creation does not clear them', () => {
  const {vm, calls} = list()
  vm.selectTasks('ungrouped', [{id: 1}, {id: 2}])
  vm.selectTasks('group:备份', [{id: 2}, {id: 3}])
  vm.openCopyTasks()
  assert.deepEqual(Array.from(vm.copyIDs), [1, 2, 3])
  const before = calls.groups.length
  vm.copyGroupCreated('新组')
  assert.equal(calls.groups.length, before + 1)
  assert.deepEqual(Array.from(vm.selectedIDs), [1, 2, 3])
  vm.selection = {}
  vm.copyGroup = '新组'
  vm.confirmCopyTasks()
  assert.deepEqual(Array.from(calls.copyTasks[0].ids), [1, 2, 3])
  calls.copyTasks[0].callback({copied: 3})
  assert.equal(vm.copyIDs.length, 0)
  assert.equal(vm.selectedIDs.length, 0)
  assert.deepEqual(Array.from(vm.expandedGroups), ['group:新组'])
  assert.equal(vm.searchParams.status, '')
  vm.$destroy()
})

test('copy failures preserve the batch and destination for retry', () => {
  const {vm, calls} = list()
  vm.openCopyTasks({id: 42, tag: '备份'})
  vm.copyGroup = 'a\nb'
  vm.confirmCopyTasks()
  assert.ok(vm.copyError)
  assert.equal(calls.copyTasks.length, 0)
  vm.copyGroup = ''
  vm.confirmCopyTasks()
  calls.copyTasks[0].failed(new Error('复制失败'))
  assert.equal(vm.copyDialog, true)
  assert.equal(vm.copyingTasks, false)
  assert.equal(vm.copyError, '复制失败')
  assert.deepEqual(Array.from(vm.copyIDs), [42])
  vm.confirmCopyTasks()
  assert.equal(calls.copyTasks.length, 2)
  assert.equal(calls.copyTasks[1].tag, '')
  vm.$destroy()
})

test('copy controls are admin-only and no selection means no request', () => {
  const {vm, calls} = list()
  vm.openCopyTasks()
  vm.confirmCopyTasks()
  assert.equal(calls.copyTasks.length, 0)
  assert.equal(vm.copyDialog, false)
  vm.isAdmin = false
  vm.openCopyTasks({id: 42})
  vm.copyIDs = [42]
  vm.confirmCopyTasks()
  assert.equal(calls.copyTasks.length, 0)
  assert.equal(vm.copyDialog, false)
  const table = readFileSync(new URL('../src/components/task/taskTable.vue', import.meta.url), 'utf8')
  assert.match(table, /\$emit\('copy', scope\.row\)/)
  assert.match(listSource, /@copy="openCopyTasks"/)
  assert.match(listSource, /新任务默认停用/)
  vm.$destroy()
})

test('default and named groups have distinct, safe query keys', () => {
  assert.notEqual(groupKey(''), groupKey('未分组'))
  assert.notEqual(groupKey(''), groupKey('ungrouped'))
  assert.equal(groupKey('__proto__'), 'group:__proto__')
  assert.deepEqual(groupQuery(''), {tag: '', ungrouped: 0})
  assert.deepEqual(groupQuery('ungrouped'), {tag: '', ungrouped: 1})
  assert.deepEqual(groupQuery(groupKey('同步')), {tag: '同步', ungrouped: 0})
  assert.throws(() => groupQuery('invalid'))
  assert.equal(prepareGroups([{name: '', total: '22'}])[0].total, 22)
})

test('group names are trimmed and bounded by characters, not UTF-8 bytes', () => {
  assert.equal(normalizeGroupName('  备份  '), '备份')
  assert.equal(normalizeGroupName(''), '')
  assert.equal(normalizeGroupName('中'.repeat(32)), '中'.repeat(32))
  for (const name of ['中'.repeat(33), 'a\nb', 'a\tb', 'a\u0000b']) assert.throws(() => normalizeGroupName(name))
})

test('homepage shows complete summaries and fetches no tasks until expanded', () => {
  const {vm, calls} = list()
  calls.groups[1].callback(groups)
  assert.equal(vm.taskTotal, 62)
  assert.equal(vm.groups.length, 3)
  assert.deepEqual(Array.from(vm.expandedGroups), [])
  assert.equal(calls.groupTasks.length, 0)
  vm.expandedGroups = ['group:备份']
  vm.openGroups(vm.expandedGroups)
  assert.equal(calls.groupTasks.length, 1)
  assert.deepEqual(calls.groupTasks[0].query, {id: '', name: '', protocol: '', host_id: '', status: '', tag: '备份', ungrouped: 0, page: 1, page_size: 20})
  calls.groupTasks[0].callback({total: 52, data: [{id: 42}]})
  assert.equal(vm.groupStates['group:备份'].tasks[0].id, 42)
  assert.equal(vm.groupStates['group:备份'].total, 52)
  vm.$destroy()
})

test('groups paginate independently and ungrouped does not mean all tasks', () => {
  const {vm, calls} = list()
  calls.groups[1].callback(groups)
  vm.openGroups(['ungrouped', 'group:备份'])
  assert.equal(calls.groupTasks[0].query.ungrouped, 1)
  vm.changeGroupPage(vm.groups[1], 2)
  assert.equal(calls.groupTasks[2].query.page, 2)
  assert.equal(vm.groupStates.ungrouped.page, 1)
  vm.changeGroupSize(vm.groups[1], 50)
  assert.equal(calls.groupTasks[3].query.page_size, 50)
  assert.equal(calls.groupTasks[3].query.page, 1)
  vm.$destroy()
})

test('late pages and old searches cannot overwrite fresh results', () => {
  const {vm, calls} = list()
  calls.groups[1].callback(groups)
  vm.openGroups(['group:备份'])
  vm.changeGroupPage(vm.groups[1], 2)
  calls.groupTasks[1].callback({total: 52, data: [{id: 2}]})
  calls.groupTasks[0].callback({total: 52, data: [{id: 1}]})
  assert.equal(vm.groupStates['group:备份'].tasks[0].id, 2)
  vm.search()
  calls.groupTasks[1].callback({total: 52, data: [{id: 99}]})
  assert.deepEqual(Object.keys(vm.groupStates), [])
  calls.groups[2].callback([{name: '同步', total: 1}])
  calls.groups[1].callback(groups)
  assert.equal(vm.groups.length, 1)
  vm.$destroy()
})

test('selected tasks move together without changing their schedules', () => {
  const {vm, calls} = list()
  calls.groups[1].callback(groups)
  vm.selectTasks('ungrouped', [{id: 1}, {id: 2}])
  vm.selectTasks('group:备份', [{id: 2}, {id: 3}])
  assert.deepEqual(Array.from(vm.selectedIDs), [1, 2, 3])
  vm.moveGroup = '  新分组  '
  vm.assignGroup()
  assert.equal(calls.assignGroup[0].tag, '新分组')
  assert.deepEqual(Array.from(calls.assignGroup[0].ids), [1, 2, 3])
  calls.assignGroup[0].callback()
  assert.equal(vm.savingGroup, false)
  assert.equal(vm.selectedIDs.length, 0)
  assert.deepEqual(Array.from(vm.expandedGroups), ['group:新分组'])
  vm.$destroy()
})

test('failed requests leave loading state retryable', () => {
  const {vm, calls} = list()
  calls.groups[1].failed()
  assert.equal(vm.loadingGroups, false)
  assert.equal(vm.groupError, true)
  vm.loadGroups()
  calls.groups[2].callback(groups)
  vm.openGroups(['ungrouped'])
  calls.groupTasks[0].failed()
  assert.equal(vm.groupStates.ungrouped.loading, false)
  assert.equal(vm.groupStates.ungrouped.error, true)
  vm.selectTasks('ungrouped', [{id: 1}])
  vm.assignGroup()
  calls.assignGroup[0].failed()
  assert.equal(vm.savingGroup, false)
  assert.equal(vm.selectedIDs.length, 1)
  vm.$destroy()
})

test('task editor offers existing or new groups and preserves stored group data', () => {
  const picker = readFileSync(new URL('../src/components/task/groupPicker.vue', import.meta.url), 'utf8')
  const editor = readFileSync(new URL('../src/pages/task/edit.vue', import.meta.url), 'utf8')
  assert.match(picker, /value="create" label="＋ 新建分组"/)
  assert.match(picker, /filterable/)
  assert.match(editor, /v-model="form\.tag"/)
  assert.match(editor, /this\.form\.tag = taskData\.tag \|\| ''/)
  assert.match(editor, /this\.\$route\.query\.group/)
})

test('delete dialog uses the whole-group count and defaults to preserving tasks', () => {
  const {vm, calls} = list()
  calls.groups[1].callback([{name: '备份', total: 1}])
  vm.searchParams.name = '仅搜索到一个任务'
  vm.openDeleteGroup(vm.groups[0])
  assert.equal(vm.deleteTasks, false)
  assert.equal(vm.deleteTarget, null)
  assert.deepEqual(calls.groups[2].query, {tag: '备份'})
  vm.confirmDeleteGroup()
  assert.equal(calls.deleteGroup.length, 0)
  calls.groups[2].callback([{name: '备份', total: 52}])
  assert.equal(vm.deleteTarget.total, 52)
  vm.confirmDeleteGroup()
  assert.equal(calls.deleteGroup[0].expectedCount, 52)
  assert.equal(calls.deleteGroup[0].deleteTasks, false)
  calls.deleteGroup[0].callback()
  assert.equal(vm.deleteDialog, false)
  assert.deepEqual(Array.from(vm.expandedGroups), ['ungrouped'])
  vm.$destroy()
})

test('delete-all requires explicit mode and clears the deleted group filter', () => {
  const {vm, calls} = list()
  calls.groups[1].callback(groups)
  vm.selectedGroup = 'group:备份'
  vm.moveGroup = '备份'
  vm.openDeleteGroup(vm.groups[1])
  calls.groups[2].callback([{name: '备份', total: 52}])
  vm.deleteTasks = true
  vm.confirmDeleteGroup()
  vm.confirmDeleteGroup()
  assert.equal(calls.deleteGroup.length, 1)
  assert.equal(calls.deleteGroup[0].deleteTasks, true)
  assert.equal(calls.deleteGroup[0].expectedCount, 52)
  calls.deleteGroup[0].callback()
  assert.equal(vm.selectedGroup, '')
  assert.equal(vm.moveGroup, '')
  assert.deepEqual(Object.keys(vm.groupStates), [])
  vm.$destroy()
})

test('built-in ungrouped has no deletion operation and failed deletion refreshes', () => {
  const {vm, calls} = list()
  calls.groups[1].callback(groups)
  vm.openDeleteGroup(vm.groups[0])
  assert.equal(calls.groups.length, 2)
  vm.openDeleteGroup(vm.groups[1])
  calls.groups[2].callback([{name: '备份', total: 52}])
  vm.confirmDeleteGroup()
  calls.deleteGroup[0].failed()
  assert.equal(vm.deletingGroup, false)
  assert.equal(vm.deleteDialog, false)
  assert.equal(vm.deleteTarget, null)
  assert.match(listSource, /v-if="isAdmin && group\.name"/)
  vm.$destroy()
})

const require = createRequire(import.meta.url)
const compiler = require('vue-template-compiler')
const pickerSource = readFileSync(new URL('../src/components/task/groupPicker.vue', import.meta.url), 'utf8')
const pickerScript = pickerSource.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '').replace('export default', 'return')
const pickerComponent = new Function('groupCreateDialog', pickerScript)({})

test('group creation components compile as valid single-root Vue templates', () => {
  for (const file of ['groupPicker.vue', 'groupCreateDialog.vue', 'groupRenameDialog.vue']) {
    const componentSource = readFileSync(new URL('../src/components/task/' + file, import.meta.url), 'utf8')
    const {template} = compiler.parseComponent(componentSource)
    assert.deepEqual(compiler.compile(template.content).errors, [], file)
  }
})

test('dropdown keeps the plus entry last and does not confuse real group names with actions', async () => {
  const vm = new Vue({...pickerComponent, propsData: {value: 'create', groups: [{name: 'create'}, {name: '每日备份'}]}})
  vm.$refs.select = {blur () {}}
  const inputs = []
  vm.$on('input', name => inputs.push(name))
  assert.equal(vm.selectedValue, 'group:create')
  vm.filterGroups('备份')
  assert.deepEqual(Array.from(vm.namedGroups), ['每日备份'])
  vm.selectValue('create')
  assert.equal(vm.createDialog, true)
  assert.equal(inputs.length, 0)
  vm.groupCreated('新分组')
  assert.equal(inputs[0], '新分组')
  vm.selectValue('group:create')
  assert.equal(inputs[1], 'create')
  vm.selectValue('ungrouped')
  assert.equal(inputs[2], '')
  assert.ok(pickerSource.indexOf('v-for="name in namedGroups"') < pickerSource.indexOf('value="create"'))
  await Vue.nextTick()
  vm.$destroy()
})

test('create dialog validates input and selects only after successful persistence', () => {
  const source = readFileSync(new URL('../src/components/task/groupCreateDialog.vue', import.meta.url), 'utf8')
  const script = source.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '').replace('export default', 'return')
  const requests = []
  const service = {createGroup (name, callback, failed) { requests.push({name, callback, failed}) }}
  const component = new Function('taskService', 'normalizeGroupName', script)(service, normalizeGroupName)
  const vm = new Vue({...component, propsData: {value: true}})
  vm.$message = {success () {}}
  const created = []
  vm.$on('created', name => created.push(name))
  vm.name = ''
  vm.submit()
  assert.equal(requests.length, 0)
  assert.match(vm.error, /不能为空/)
  vm.name = '  新分组  '
  vm.submit()
  vm.submit()
  assert.equal(requests.length, 1)
  assert.equal(requests[0].name, '新分组')
  assert.deepEqual(created, [])
  requests[0].failed(new Error('分组已存在'))
  assert.equal(vm.saving, false)
  assert.equal(vm.error, '分组已存在')
  vm.name = '另一个分组'
  vm.submit()
  requests[1].callback()
  assert.deepEqual(created, ['另一个分组'])
  assert.equal(vm.saving, false)
  vm.$destroy()
})

test('rename updates active filters and expanded keys, clearing old caches and selections', () => {
  const {vm, calls} = list()
  calls.groups[0].callback(groups)
  calls.groups[1].callback(groups)
  vm.selectedGroup = 'group:备份'
  vm.moveGroup = '备份'
  vm.expandedGroups = ['group:备份', 'group:同步']
  vm.selectTasks('group:备份', [{id: 42}])
  vm.openRenameGroup(vm.groups[0])
  assert.equal(vm.renameDialog, false)
  vm.openRenameGroup(vm.groups[1])
  assert.equal(vm.renameTarget, '备份')
  assert.equal(vm.renameDialog, true)
  vm.groupRenamed({oldName: '备份', name: '每日备份'})
  assert.equal(vm.selectedGroup, 'group:每日备份')
  assert.equal(vm.moveGroup, '每日备份')
  assert.deepEqual(Array.from(vm.expandedGroups), ['group:每日备份', 'group:同步'])
  assert.equal(vm.selectedIDs.length, 0)
  assert.deepEqual(Object.keys(vm.groupStates), [])
  assert.equal(calls.groups[3].query.tag, '每日备份')
  calls.groups[2].callback([{name: '每日备份', total: 52}])
  calls.groups[0].callback(groups)
  assert.equal(vm.groupChoices[0].name, '每日备份')
  vm.isAdmin = false
  vm.renameDialog = false
  vm.openRenameGroup({name: '同步'})
  assert.equal(vm.renameDialog, false)
  vm.$destroy()
})

test('rename dialog pre-fills, validates, preserves failed input and saves exactly once', async () => {
  const source = readFileSync(new URL('../src/components/task/groupRenameDialog.vue', import.meta.url), 'utf8')
  const script = source.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '').replace('export default', 'return')
  const requests = []
  const service = {renameGroup (tag, newTag, callback, failed) { requests.push({tag, newTag, callback, failed}) }}
  const component = new Function('taskService', 'normalizeGroupName', script)(service, normalizeGroupName)
  const vm = new Vue({...component, propsData: {value: true, groupName: '旧名称'}})
  vm.$message = {success () {}}
  const renamed = []
  const visible = []
  vm.$on('renamed', value => renamed.push(value))
  vm.$on('input', value => visible.push(value))
  vm.reset()
  assert.equal(vm.name, '旧名称')
  vm.name = ' 旧名称 '
  vm.submit()
  for (const invalid of ['', '  ', '中'.repeat(33), 'a\nb']) {
    vm.name = invalid
    vm.submit()
    assert.ok(vm.error)
  }
  assert.equal(requests.length, 0)
  vm.name = '  同步  '
  vm.submit()
  vm.submit()
  assert.equal(requests.length, 1)
  assert.equal(requests[0].tag, '旧名称')
  assert.equal(requests[0].newTag, '同步')
  assert.deepEqual(renamed, [])
  requests[0].failed(new Error('分组名称已存在'))
  assert.equal(vm.saving, false)
  assert.equal(vm.name, '  同步  ')
  assert.equal(vm.error, '分组名称已存在')
  assert.deepEqual(visible, [])
  vm.name = '  新名称  '
  vm.submit()
  requests[1].callback()
  assert.deepEqual(renamed, [{oldName: '旧名称', name: '新名称'}])
  assert.deepEqual(visible, [false])
  assert.equal(vm.saving, false)
  await Vue.nextTick()
  vm.$destroy()
})
