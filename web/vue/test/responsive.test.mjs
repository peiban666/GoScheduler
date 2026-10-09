import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {createRequire} from 'node:module'

const require = createRequire(import.meta.url)
const compiler = require('vue-template-compiler')
const css = readFileSync(new URL('../src/styles/responsive.css', import.meta.url), 'utf8')
const component = path => readFileSync(new URL('../src/' + path, import.meta.url), 'utf8')

test('responsive navigation does not shrink labels into fixed grid columns', () => {
  const source = component('components/common/navMenu.vue')
  assert.doesNotMatch(source, /el-col|:span|2vh/)
  assert.match(source, /class="app-brand"/)
  assert.match(source, /class="app-account"/)
  assert.match(css, /@media \(max-width: 767px\)/)
  assert.match(css, /flex-wrap: wrap/)
  assert.match(css, /\.app-brand[\s\S]*?box-sizing: border-box/)
})

test('mobile login uses full-width inputs and a viewport-bounded dialog', () => {
  const source = component('pages/user/login.vue')
  assert.match(source, /custom-class="login-dialog"/)
  assert.doesNotMatch(source, /width="40%"|:span="16"/)
  assert.equal((source.match(/:span="24"/g) || []).length, 2)
  assert.match(css, /max-width: calc\(100vw - 24px\)/)
  assert.match(css, /\.login-dialog \.el-form-item__content[\s\S]*?margin-left: 0 !important/)
})

test('task filters, sidebar, tables and pagination have constrained responsive containers', () => {
  assert.match(component('pages/task/list.vue'), /class="task-filters"/)
  for (const path of ['pages/task/sidebar.vue', 'pages/system/sidebar.vue']) {
    assert.match(component(path), /class="app-sidebar"/)
    assert.doesNotMatch(component(path), /width="8%"/)
  }
  assert.match(css, /grid-template-columns: minmax\(0, 1fr\)/)
  assert.match(css, /#main-container \.el-pagination[\s\S]*?flex-wrap: wrap/)
  assert.match(component('App.vue'), /import '\.\/styles\/responsive\.css'/)
})

test('all modified responsive templates compile', () => {
  for (const path of ['App.vue', 'components/common/navMenu.vue', 'pages/task/list.vue',
    'pages/task/edit.vue', 'pages/task/sidebar.vue', 'pages/system/sidebar.vue', 'pages/user/login.vue',
    'components/task/schedulePicker.vue', 'components/task/taskTable.vue']) {
    const {template} = compiler.parseComponent(component(path))
    assert.deepEqual(compiler.compile(template.content).errors, [], path)
  }
  assert.doesNotThrow(() => require('postcss').parse(css))
})
