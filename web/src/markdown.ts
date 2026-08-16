import DOMPurify from 'dompurify'
import { marked } from 'marked'

/**
 * renderMarkdown renders Markdown for pages written with `sbnn export`.
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

// Links open away from the page, and they are the only elements that gain
// an attribute here, so the hook only has to look at them.
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.hasAttribute('href')) {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noreferrer noopener')
  }
})

/**
 * sanitize strips everything executable out of rendered Markdown.
 *
 * This used to be a hand-written pass over the parsed DOM, which cannot be
 * made correct: DOMParser runs with scripting off, where <noscript> holds
 * markup, and the page that renders the result runs with scripting on,
 * where it holds text. An attribute closing the tag inside its own value -
 * <noscript><p title="</noscript><img src=x onerror=...>"> - therefore
 * passed the check as an attribute and came back as an element. Turning
 * markup into a DOM safely is a job with a maintained answer, so sbnn uses it
 * rather than keeping its own.
 */
function sanitize(html: string): string {
  return DOMPurify.sanitize(html, {
    // An exported page is one file with the diff frozen into it; nothing in
    // a preview should be reaching for anything else.
    FORBID_TAGS: ['style', 'form', 'input', 'button', 'link', 'meta', 'base'],
    ALLOW_DATA_ATTR: false,
  })
}
