import { describe, expect, it } from 'vitest'
import { DEFAULT_SETTINGS, type UISettings } from './appearance'
import { exportText, planImport, MAX_FILE_BYTES } from './appearanceFile'

const mine: UISettings = { ...DEFAULT_SETTINGS, theme: 'neutral', terminalSize: 18, terminalColors: 'light' }

describe('exportText', () => {
  it('holds a tag, a version and exactly the appearance keys', () => {
    const file = JSON.parse(exportText(mine))
    expect(file.kind).toBe('dungeon-appearance')
    expect(file.version).toBe(1)
    expect(Object.keys(file.settings).sort()).toEqual(Object.keys(DEFAULT_SETTINGS).sort())
    expect(file.settings).toEqual(mine)
  })
  it('leaves out anything that is not an appearance value', () => {
    const polluted = { ...mine, token: 'secret', dir: '/home/me', heroes: [{ env: ['K=V'] }] } as unknown as UISettings
    const text = exportText(polluted)
    for (const word of ['secret', '/home/me', 'heroes', 'K=V']) expect(text).not.toContain(word)
  })
})

describe('planImport', () => {
  it('restores exported settings and lists what changes', () => {
    const plan = planImport(exportText(mine), DEFAULT_SETTINGS)
    expect(plan).toMatchObject({ ok: true, invalid: [], next: mine })
    if (plan.ok) expect(plan.changes.map((c) => c.key)).toEqual(['terminalSize', 'theme', 'terminalColors'])
  })
  it('has no changes for a file that matches the current look', () => {
    const plan = planImport(exportText(mine), mine)
    expect(plan).toMatchObject({ ok: true, changes: [] })
  })
  it('ignores unknown fields, including ones that look dangerous', () => {
    const text = JSON.stringify({ kind: 'dungeon-appearance', version: 1, settings: { theme: 'neutral', token: 'x', __proto__: { a: 1 }, heroes: [1] } })
    const plan = planImport(text, DEFAULT_SETTINGS)
    expect(plan).toMatchObject({ ok: true, invalid: [] })
    if (plan.ok) {
      expect(plan.next).toEqual({ ...DEFAULT_SETTINGS, theme: 'neutral' })
      expect(Object.keys(plan.next).sort()).toEqual(Object.keys(DEFAULT_SETTINGS).sort())
    }
  })
  it('keeps the current value for an invalid one and says which', () => {
    const text = JSON.stringify({ kind: 'dungeon-appearance', settings: { theme: 'neon', terminalSize: 99, terminalLineHeight: 1.2, terminalFont: 'custom', terminalFontName: 'a"b', font: 'alegreya' } })
    const plan = planImport(text, mine)
    expect(plan.ok).toBe(true)
    if (plan.ok) {
      expect(plan.invalid.sort()).toEqual(['terminalFont', 'terminalFontName', 'terminalLineHeight', 'terminalSize', 'theme'].sort())
      expect(plan.next).toEqual({ ...mine, font: 'alegreya' })
    }
  })
  it('refuses text that is not an appearance file', () => {
    for (const bad of ['', 'nope', '[]', 'null', '{}', '{"kind":"other","settings":{}}', '{"kind":"dungeon-appearance"}', '{"kind":"dungeon-appearance","settings":[]}']) {
      expect(planImport(bad, mine).ok, bad).toBe(false)
    }
    expect(planImport('x'.repeat(MAX_FILE_BYTES + 1), mine)).toMatchObject({ ok: false })
  })
  it('never applies anything itself', () => {
    const before = { ...mine }
    planImport(exportText(DEFAULT_SETTINGS), mine)
    expect(mine).toEqual(before)
  })
})
