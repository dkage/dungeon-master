import { readFileSync } from 'node:fs'
import { afterEach, expect, it } from 'vitest'

afterEach(() => {
  document.head.replaceChildren()
  document.body.replaceChildren()
})

it('overrides the legacy viewport black without overriding the themed scrollable element', () => {
  const style = document.createElement('style')
  style.textContent = readFileSync('node_modules/@xterm/xterm/css/xterm.css', 'utf8')
    + readFileSync('src/lib/terminal.css', 'utf8')
  document.head.append(style)
  document.body.innerHTML = `<div class="term"><div class="xterm">
    <div class="xterm-viewport"></div>
    <div class="xterm-scrollable-element" style="background-color: rgb(20, 20, 26)"></div>
  </div></div>`
  expect(getComputedStyle(document.querySelector('.xterm-viewport')!).backgroundColor).toBe('rgba(0, 0, 0, 0)')
  expect(getComputedStyle(document.querySelector('.xterm-scrollable-element')!).backgroundColor).toBe('rgb(20, 20, 26)')
})
