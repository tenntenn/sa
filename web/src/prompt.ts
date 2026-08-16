import type { Comment, Diff } from './types'

/**
 * buildPrompt renders review comments the same way the sa server does, so an
 * exported page produces the same text as `sa comments`.
 */
export function buildPrompt(
  group: string,
  diffs: Diff[],
  comments: Comment[],
  includeResolved = false,
): string {
  const open = comments.filter((c) => includeResolved || !c.resolved)
  const lines: string[] = [`# Review comments (sa group "${group}")`, '']
  if (open.length === 0) {
    lines.push('No open review comments.')
    return lines.join('\n') + '\n'
  }
  lines.push(`${open.length} comment(s) to address.`)

  const titles = new Map(diffs.map((d) => [d.id, d.title]))
  open.forEach((c, i) => {
    lines.push('', `## ${i + 1}. ${c.path}${lineRange(c)}`)
    const title = titles.get(c.diffId)
    if (title) lines.push('', `Diff: ${title}`)
    if (c.author) lines.push('', `From: ${c.author}`)
    if (c.resolved) lines.push('', 'Status: resolved')
    const snippet = c.snippet.replace(/\n+$/, '')
    if (snippet) {
      const fence = fenceFor(snippet)
      lines.push('', fence, snippet, fence)
    }
    const body = c.body.replace(/\n+$/, '')
    if (body) {
      lines.push('')
      for (const line of body.split('\n')) lines.push(`> ${line}`)
    }
    const suggestion = (c.suggestion ?? '').replace(/\n+$/, '')
    if (suggestion) {
      const fence = fenceFor(suggestion)
      lines.push(
        '',
        `Suggested replacement for ${c.path}${lineRange(c)}:`,
        '',
        `${fence}suggestion`,
        suggestion,
        fence,
      )
    }
  })

  lines.push(
    '',
    '---',
    '',
    'Address every comment above. A suggestion block replaces the lines it names, verbatim. ' +
      'When a comment is not worth acting on, say why instead of changing the code.',
  )
  return lines.join('\n') + '\n'
}

function lineRange(c: Comment): string {
  if (c.startLine <= 0) return ''
  const side = c.side === 'old' ? ' (old)' : ''
  return c.endLine > c.startLine ? `:${c.startLine}-${c.endLine}${side}` : `:${c.startLine}${side}`
}

function fenceFor(content: string): string {
  let longest = 0
  let current = 0
  for (const ch of content) {
    if (ch === '`') {
      current++
      longest = Math.max(longest, current)
    } else {
      current = 0
    }
  }
  return longest < 3 ? '```' : '`'.repeat(longest + 1)
}
