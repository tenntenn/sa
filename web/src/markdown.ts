import { marked } from 'marked'

/**
 * renderMarkdown renders Markdown for pages written with `sa export`.
 *
 * The live app leaves Markdown to mo; an exported page has no server behind
 * it, so it renders the frozen content itself. The result is sanitised: the
 * Markdown comes from a diff, and a diff is not trusted input.
 */
export function renderMarkdown(source: string): string {
  const { frontmatter, body } = splitFrontmatter(source)
  const html = marked.parse(body, { async: false, gfm: true, breaks: false })
  const rendered = sanitize(typeof html === 'string' ? html : '')
  if (frontmatter === '') return rendered
  return `<pre class="frontmatter">${escapeHTML(frontmatter)}</pre>` + rendered
}

/** splitFrontmatter peels off the YAML metadata block mo shows separately. */
function splitFrontmatter(source: string): { frontmatter: string; body: string } {
  const match = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?/.exec(source)
  if (!match) return { frontmatter: '', body: source }
  return { frontmatter: match[1], body: source.slice(match[0].length) }
}

function escapeHTML(s: string): string {
  return s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c] ?? c)
}

const allowedProtocols = new Set(['http:', 'https:', 'mailto:', 'data:'])

function sanitize(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll('script, style, iframe, object, embed, form, link, meta').forEach((el) => {
    el.remove()
  })
  doc.querySelectorAll('*').forEach((el) => {
    for (const attr of Array.from(el.attributes)) {
      const name = attr.name.toLowerCase()
      if (name.startsWith('on')) {
        el.removeAttribute(attr.name)
        continue
      }
      if (name === 'href' || name === 'src') {
        if (!isSafeURL(attr.value, name)) el.removeAttribute(attr.name)
      }
    }
    if (el.tagName === 'A') {
      el.setAttribute('target', '_blank')
      el.setAttribute('rel', 'noreferrer noopener')
    }
  })
  return doc.body.innerHTML
}

function isSafeURL(value: string, attr: string): boolean {
  const url = value.trim()
  if (url === '') return false
  if (url.startsWith('#') || url.startsWith('/') || url.startsWith('./') || url.startsWith('../')) {
    return true
  }
  try {
    const parsed = new URL(url, window.location.href)
    if (!allowedProtocols.has(parsed.protocol)) return false
    // data: is only ever useful for inline images.
    if (parsed.protocol === 'data:') return attr === 'src' && url.startsWith('data:image/')
    return true
  } catch {
    return false
  }
}
