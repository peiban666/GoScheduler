'use strict'

const path = require('node:path')
const {createRequire} = require('node:module')

// Vue 2.5 eagerly creates a MessageChannel for its macro-task scheduler.
// Its template compiler contains the same eager initialization. Modern Node
// exposes that browser API, leaving MessagePorts alive after tests or builds.
// Initialize both using their built-in timer fallback in this DOM-free tooling,
// then restore the global unchanged. This never changes browser-bundled code.
// Resolve from cwd so release tooling can also initialize older tagged sources.
const messageChannel = Object.getOwnPropertyDescriptor(globalThis, 'MessageChannel')
try {
  Object.defineProperty(globalThis, 'MessageChannel', {
    configurable: true,
    writable: true,
    value: undefined
  })
  const fromFrontend = createRequire(path.resolve('package.json'))
  fromFrontend('vue')
  fromFrontend('vue-template-compiler')
} finally {
  if (messageChannel) {
    Object.defineProperty(globalThis, 'MessageChannel', messageChannel)
  } else {
    delete globalThis.MessageChannel
  }
}
