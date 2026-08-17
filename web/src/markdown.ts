import DOMPurify from 'dompurify'
import { marked } from 'marked'

/**
 * renderMarkdown renders the Markdown of a preview.
 *
 * The live app can leave Markdown to mo instead, but its own renderer is
 * also what an exported page uses, since that has no server behind it. The
 * result is sanitised: the Markdown comes from a diff, and a diff is not
 * trusted input.
 *
 * Each top level block is wrapped in an element carrying the source line
 * range it came from, in `data-ln`, so the preview can turn a text selection
 * back into the line numbers a comment anchors to.
 */
export function renderMarkdown(source: string): string {
  const { frontmatter, body, bodyStartLine } = splitFrontmatter(source)
  const rendered = sanitize(renderBody(body, bodyStartLine))
  if (frontmatter === '') return rendered
  return `<pre class="frontmatter">${escapeHTML(frontmatter)}</pre>` + rendered
}

/** renderBody renders each top level block of body on its own, so the exact
 * source lines it consumed can be attached to it before anything is joined
 * back into one string. startLine is where body itself starts in the file
 * that was split into frontmatter and body. */
function renderBody(body: string, startLine: number): string {
  const tokens = marked.lexer(body, { gfm: true, breaks: false })
  const options = { gfm: true, breaks: false }
  const parts: string[] = []
  let line = startLine
  for (const token of tokens) {
    const raw = token.raw ?? ''
    const newlines = countChar(raw, '\n')
    const start = line
    const end = raw.endsWith('\n') ? start + newlines - 1 : start + newlines
    line = start + newlines
    const html = marked.parser([token], options)
    if (typeof html !== 'string' || html.trim() === '') continue
    parts.push(`<div data-ln="${start}-${Math.max(end, start)}">${html}</div>`)
  }
  return parts.join('')
}

function countChar(s: string, ch: string): number {
  let n = 0
  for (let i = 0; i < s.length; i++) if (s[i] === ch) n++
  return n
}

/** splitFrontmatter peels off the YAML metadata block mo shows separately.
 * bodyStartLine is the line body starts on in the original file, which is 1
 * unless a frontmatter block pushed it down. */
function splitFrontmatter(source: string): { frontmatter: string; body: string; bodyStartLine: number } {
  const match = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?/.exec(source)
  if (!match) return { frontmatter: '', body: source, bodyStartLine: 1 }
  return {
    frontmatter: match[1],
    body: source.slice(match[0].length),
    bodyStartLine: 1 + countChar(match[0], '\n'),
  }
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
    // data-ln is not user content, sbnn put it there itself, and the preview
    // needs it to survive sanitising.
    ADD_ATTR: ['data-ln'],
  })
}
