import { Marked } from 'marked'
import DOMPurify from 'dompurify'

// Issue text is untrusted input. Two layers keep it harmless, and only the
// result may ever reach {@html}:
//
//  1. Raw HTML inside the markdown is cut down to a short list of bare tags
//     (<details>, <br>, ...). Anything else, above all anything with
//     attributes, is shown as text, so event handlers and URLs in raw HTML
//     are never even parsed.
//  2. DOMPurify then sanitises whatever markdown produced, as a backstop.

const purify = DOMPurify(window)

// Links open in a new tab and cannot reach back into this window.
purify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer nofollow')
  }
})

const SAFE_URI = /^(?:https?:|mailto:|#|\/(?!\/)|\.{1,2}\/)/i

// Tags GitHub users commonly write by hand. Attributes are not allowed, except
// the bare `open` on <details>.
const ALLOWED_RAW =
  /^<(\/?)(details|summary|br|kbd|sub|sup|b|i|em|strong|code|p|hr|ul|ol|li|blockquote|h[1-6]|table|thead|tbody|tr|th|td|pre|del|ins|mark)(\s+open)?\s*(\/?)>$/i

function escapeText(s: string): string {
  return s.replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/** Keeps allowed bare tags in a chunk of raw HTML and turns the rest into text. */
export function restrictRawHtml(chunk: string): string {
  let out = ''
  let last = 0
  for (const m of chunk.matchAll(/<[^<>]*>/g)) {
    out += escapeText(chunk.slice(last, m.index))
    const tag = m[0]
    const ok = ALLOWED_RAW.exec(tag)
    // `open` only makes sense on details; elsewhere the tag is not allowed.
    out += ok && (!ok[3] || ok[2].toLowerCase() === 'details') ? tag : escapeText(tag)
    last = m.index + tag.length
  }
  return out + escapeText(chunk.slice(last))
}

const md = new Marked({
  gfm: true,
  breaks: true,
  async: false,
  renderer: {
    html: (token) => restrictRawHtml(token.text),
  },
})

export function renderMarkdown(src: string): string {
  if (!src.trim()) return ''
  const html = md.parse(src) as string
  return purify.sanitize(html, {
    // Images would be fetched from third-party hosts (and the page's CSP
    // blocks them anyway); embeds, forms and styles have no place here.
    FORBID_TAGS: ['img', 'picture', 'source', 'iframe', 'object', 'embed', 'form', 'style', 'link', 'meta', 'base', 'svg', 'math'],
    FORBID_ATTR: ['style', 'srcset', 'src'],
    ALLOWED_URI_REGEXP: SAFE_URI,
  })
}
