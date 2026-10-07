const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const dist = path.resolve(__dirname, '../dist')

test('production CSS resolves both Element UI icon fonts at root and subpath', () => {
  const html = fs.readFileSync(path.join(dist, 'index.html'), 'utf8')
  const stylesheets = Array.from(html.matchAll(/href=["']?([^"' >]+\.css)["']?/g), match => match[1])
  assert.ok(stylesheets.length, 'production index must include extracted CSS')
  let checked = 0
  for (const stylesheet of stylesheets) {
    const cssFile = path.join(dist, stylesheet.replace(/^public\//, ''))
    const css = fs.readFileSync(cssFile, 'utf8')
    const iconFont = css.match(/@font-face\{[^}]*font-family:element-icons[^}]*\}/)
    if (!iconFont) continue
    const fonts = Array.from(iconFont[0].matchAll(/url\(([^)]+)\)/g), match => match[1].replace(/["']/g, ''))
    assert.equal(fonts.length, 2, 'Element UI must expose both woff and ttf fonts')
    for (const prefix of ['', 'scheduler/']) {
      const cssURL = new URL(stylesheet, `http://localhost/${prefix}`)
      for (const font of fonts) {
        const resolved = new URL(font, cssURL)
        assert.ok(resolved.pathname.startsWith(`/${prefix}public/static/fonts/`), `incorrect font URL: ${resolved}`)
        const diskFile = path.join(dist, resolved.pathname.slice(`/${prefix}public/`.length))
        assert.ok(fs.existsSync(diskFile), `missing font: ${diskFile}`)
        assert.ok(fs.statSync(diskFile).size > 0, 'font must not be empty')
        checked++
      }
    }
  }
  assert.equal(checked, 4, 'check two font formats at two mount paths')
})
