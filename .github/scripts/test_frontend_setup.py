"""Regression-test natural process exit against the installed Vue version."""

import os
from pathlib import Path
import shutil
import subprocess
import unittest


class FrontendSetupTest(unittest.TestCase):
    def test_vue_next_tick_and_global_restoration_exit_without_force(self):
        root = Path(__file__).resolve().parents[2]
        frontend = Path(os.environ.get("FRONTEND_TEST_DIR", root / "web/vue")).resolve()
        if not (frontend / "node_modules/vue").exists():
            self.skipTest("Install frontend dependencies before this regression test")
        node = shutil.which("node")
        self.assertIsNotNone(node)
        setup = root / "web/vue/build/node-compat.cjs"
        script = """
const assert = require('node:assert/strict')
const before = Object.getOwnPropertyDescriptor(globalThis, 'MessageChannel')
require(process.argv[1])
assert.deepEqual(Object.getOwnPropertyDescriptor(globalThis, 'MessageChannel'), before)
const Vue = require('vue')
const compiler = require('vue-template-compiler')
assert.deepEqual(compiler.compile('<div>fixture</div>').errors, [])
const vm = new Vue({data: {value: 1}})
let observed
vm.$watch('value', value => { observed = value })
vm.value = 2
Vue.nextTick(() => {
  assert.equal(observed, 2)
  vm.$destroy()
  assert.ok(!process.getActiveResourcesInfo().includes('MessagePort'))
  console.log('Vue', Vue.version, 'nextTick passed; no referenced MessagePort')
})
"""
        result = subprocess.run([node, "-e", script, str(setup)], cwd=frontend,
                                text=True, encoding="utf-8", capture_output=True,
                                timeout=10, check=True)
        self.assertIn("nextTick passed", result.stdout)


if __name__ == "__main__":
    unittest.main()
