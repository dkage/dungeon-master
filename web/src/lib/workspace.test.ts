import { describe, expect, it } from 'vitest'
import { SPLIT, escapeRestores, expand, expandLabel, restore, toggle } from './workspace'

describe('workspace', () => {
  it('starts split, expands one region and restores', () => {
    expect(SPLIT.expanded).toBeNull()
    const w = expand(SPLIT, 'terminal')
    expect(w.expanded).toBe('terminal')
    expect(restore()).toEqual(SPLIT)
  })
  it('switches targets and toggles the same region back', () => {
    const w = toggle(toggle(SPLIT, 'map'), 'detail')
    expect(w.expanded).toBe('detail')
    expect(toggle(w, 'detail').expanded).toBeNull()
  })
  it('labels name the target', () => {
    expect(expandLabel('terminal', false)).toBe('Expand terminal')
    expect(expandLabel('terminal', true)).toBe('Restore split')
  })
  it('routes escape: not in dialogs, text fields or the terminal', () => {
    const w = expand(SPLIT, 'map')
    const el = (html: string) => {
      document.body.innerHTML = html
      return document.body.firstElementChild
    }
    expect(escapeRestores(SPLIT, document.body, false)).toBe(false)
    expect(escapeRestores(w, document.body, true)).toBe(false)
    expect(escapeRestores(w, el('<input>'), false)).toBe(false)
    expect(escapeRestores(w, el('<div class="xterm"></div>'), false)).toBe(false)
    expect(escapeRestores(w, el('<button></button>'), false)).toBe(true)
  })
})
