import { describe, expect, it } from 'vitest'
import { renderMarkdown, restrictRawHtml } from './markdown'

const dom = (html: string) => {
  const d = document.createElement('div')
  d.innerHTML = html
  return d
}

describe('renderMarkdown', () => {
  it('renders common GitHub markdown', () => {
    const d = dom(renderMarkdown('## Steps\n\n1. one\n2. two\n\n- [ ] todo\n\n`code` and **bold**\n\n```ts\nlet x = 1\n```\n'))
    expect(d.querySelector('h2')?.textContent).toBe('Steps')
    expect(d.querySelectorAll('ol li')).toHaveLength(2)
    expect(d.querySelector('strong')?.textContent).toBe('bold')
    expect(d.querySelector('pre code')?.textContent).toContain('let x = 1')
  })

  it('turns single newlines into line breaks, like GitHub comments', () => {
    expect(renderMarkdown('a\nb')).toContain('<br>')
  })

  it('renders tables', () => {
    const d = dom(renderMarkdown('| a | b |\n|---|---|\n| 1 | 2 |\n'))
    expect(d.querySelectorAll('td')).toHaveLength(2)
  })

  it('returns an empty string for empty input', () => {
    expect(renderMarkdown('')).toBe('')
    expect(renderMarkdown('  \n ')).toBe('')
  })

  describe('hostile input', () => {
    const payloads: Record<string, string> = {
      'script tag': '<script>window.pwned = 1</script>hello',
      'inline handler': '<b onclick="window.pwned=1">x</b>',
      'img onerror': '<img src=x onerror="window.pwned=1">',
      'markdown image': '![x](https://tracker.test/pixel.png)',
      'iframe': '<iframe src="https://evil.test"></iframe>',
      'svg onload': '<svg onload="window.pwned=1"><circle/></svg>',
      'javascript link': '[click](javascript:window.pwned=1)',
      'JavaScript mixed case': '[click](JaVaScRiPt:window.pwned=1)',
      'data link': '[click](data:text/html;base64,PHNjcmlwdD4=)',
      'vbscript link': '[click](vbscript:msgbox)',
      'html javascript link': '<a href="javascript:window.pwned=1">x</a>',
      'style attribute': '<p style="background:url(https://tracker.test/x)">x</p>',
      'style tag': '<style>body{display:none}</style>x',
      'form': '<form action="https://evil.test"><input name=p></form>',
      'base tag': '<base href="https://evil.test/">',
      'meta refresh': '<meta http-equiv="refresh" content="0;url=https://evil.test">',
      'object': '<object data="https://evil.test"></object>',
    }
    for (const [name, payload] of Object.entries(payloads)) {
      it(`neutralises: ${name}`, () => {
        const html = renderMarkdown(payload)
        const d = dom(html)
        expect(d.querySelector('script, iframe, object, embed, form, style, base, meta, svg, img')).toBeNull()
        for (const el of d.querySelectorAll('*')) {
          for (const attr of Array.from(el.attributes)) {
            expect(attr.name.startsWith('on')).toBe(false)
            expect(attr.name).not.toBe('style')
            if (attr.name === 'href') expect(attr.value).toMatch(/^(https?:|mailto:|#|\/|\.)/i)
          }
        }
        expect((window as unknown as { pwned?: number }).pwned).toBeUndefined()
      })
    }
  })

  it('keeps safe links and makes them open in a new, isolated tab', () => {
    const d = dom(renderMarkdown('[docs](https://example.com/a?b=c) and <https://example.org> and [mail](mailto:a@b.co) and [here](#top)'))
    const links = Array.from(d.querySelectorAll('a'))
    expect(links.map((a) => a.getAttribute('href'))).toEqual(['https://example.com/a?b=c', 'https://example.org', 'mailto:a@b.co', '#top'])
    for (const a of links) {
      expect(a.getAttribute('target')).toBe('_blank')
      expect(a.getAttribute('rel')).toContain('noopener')
      expect(a.getAttribute('rel')).toContain('noreferrer')
    }
  })

  it('drops protocol-relative links, which would leave the page for an unknown host', () => {
    const d = dom(renderMarkdown('[x](//evil.test/path)'))
    expect(d.querySelector('a')?.getAttribute('href') ?? '').not.toMatch(/^\/\//)
  })

  it('does not execute the text of a markdown image, only drops it', () => {
    const d = dom(renderMarkdown('before ![alt text](https://tracker.test/p.png) after'))
    expect(d.querySelector('img')).toBeNull()
    expect(d.textContent).toContain('before')
    expect(d.textContent).toContain('after')
  })

  describe('raw HTML', () => {
    it('keeps the bare tags people commonly write, such as <details>', () => {
      const d = dom(renderMarkdown('<details open>\n<summary>More</summary>\n\nhidden **text**\n\n</details>\n\nline<br>two'))
      expect(d.querySelector('details')?.hasAttribute('open')).toBe(true)
      expect(d.querySelector('summary')?.textContent).toBe('More')
      expect(d.querySelector('details strong')?.textContent).toBe('text')
      expect(d.querySelector('br')).not.toBeNull()
    })

    it('shows tags with attributes as text instead of parsing them', () => {
      const html = renderMarkdown('<b onclick="x()">bold</b> and <a href="https://e.test" onmouseover="y()">link</a>')
      expect(html).not.toMatch(/<[^>]*on(click|mouseover)/i)
      const d = dom(html)
      expect(d.querySelector('b')).toBeNull()
      expect(d.querySelector('a')).toBeNull()
      expect(d.textContent).toContain('<b onclick="x()">')
    })

    it('never lets an event-handler attribute reach the parser, even one that is later stripped', () => {
      for (const payload of ['<img src=x onerror="a()">', '<svg onload="a()">', '<body onload="a()">', '<div onclick="a()">x</div>', '<input autofocus onfocus="a()">']) {
        const html = renderMarkdown(payload)
        // Every "<" that remains is escaped or belongs to a bare allowed tag.
        for (const tag of html.match(/<[^>]*>/g) ?? []) {
          expect(tag).not.toMatch(/\son\w+\s*=/i)
        }
      }
    })

    it('allows `open` only on details', () => {
      expect(restrictRawHtml('<details open>')).toBe('<details open>')
      expect(restrictRawHtml('<p open>')).toBe('&lt;p open&gt;')
    })

    it('rejects look-alike and malformed tags', () => {
      for (const tag of ['<script>', '</script>', '<detailsx>', '<details onclick=a>', '<br onload=a>', '< b>', '<b/onclick=a>', '<a href=x>', '<style>']) {
        expect(restrictRawHtml(tag)).toBe(tag.replace(/</g, '&lt;').replace(/>/g, '&gt;'))
      }
    })

    it('escapes text and stray angle brackets around tags', () => {
      expect(restrictRawHtml('1 < 2 <b>ok</b> 3 > 2')).toBe('1 &lt; 2 <b>ok</b> 3 &gt; 2')
      expect(restrictRawHtml('a <b')).toBe('a &lt;b')
    })

    it('is case-insensitive about allowed tags', () => {
      expect(restrictRawHtml('<DETAILS><Summary>x</Summary></DETAILS>')).toBe('<DETAILS><Summary>x</Summary></DETAILS>')
    })
  })
})
