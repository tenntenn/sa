/**
 * A suggested change lives inside the comment body as a fenced block whose
 * info string is "suggestion", exactly like on GitHub. This module is the
 * one place that knows how to read and write those blocks.
 */

export type Segment =
  | { kind: 'text'; text: string }
  | { kind: 'suggestion'; text: string }

interface Fence {
  fence: string
}

function openingFence(line: string): Fence | null {
  const trimmed = line.replace(/\r$/, '').trim()
  for (const marker of ['`', '~']) {
    let n = 0
    while (n < trimmed.length && trimmed[n] === marker) n++
    if (n < 3) continue
    if (trimmed.slice(n).trim().toLowerCase() === 'suggestion') {
      return { fence: trimmed.slice(0, n) }
    }
  }
  return null
}

function closesFence(line: string, fence: string): boolean {
  const trimmed = line.replace(/\r$/, '').trim()
  if (trimmed.length < fence.length) return false
  const marker = fence[0]
  return trimmed.startsWith(fence) && trimmed.split('').every((c) => c === marker)
}

/** parseBody splits a comment body into prose and suggested changes. */
export function parseBody(body: string): Segment[] {
  const segments: Segment[] = []
  const lines = body.split('\n')
  let text: string[] = []

  const flushText = () => {
    const joined = text.join('\n').replace(/^\n+|\n+$/g, '')
    if (joined !== '') segments.push({ kind: 'text', text: joined })
    text = []
  }

  for (let i = 0; i < lines.length; i++) {
    const open = openingFence(lines[i])
    if (!open) {
      text.push(lines[i])
      continue
    }
    flushText()
    const block: string[] = []
    i++
    for (; i < lines.length; i++) {
      if (closesFence(lines[i], open.fence)) break
      block.push(lines[i].replace(/\r$/, ''))
    }
    segments.push({ kind: 'suggestion', text: block.join('\n') })
  }
  flushText()
  return segments
}

/** suggestions returns just the proposed replacements of a body. */
export function suggestions(body: string): string[] {
  return parseBody(body)
    .filter((s): s is { kind: 'suggestion'; text: string } => s.kind === 'suggestion')
    .map((s) => s.text)
}

/** suggestionBlock writes a suggestion the way a comment body carries it. */
export function suggestionBlock(text: string): string {
  const content = text.replace(/\n+$/, '')
  let fence = '```'
  while (content.includes(fence)) fence += '`'
  return `${fence}suggestion\n${content}\n${fence}`
}

/** withSuggestion appends a suggestion block to a body. */
export function withSuggestion(body: string, text: string): string {
  if (text.trim() === '') return body
  const block = suggestionBlock(text)
  return body.trim() === '' ? block : `${body.replace(/\n+$/, '')}\n\n${block}`
}

/** originalLines are the lines a suggestion would replace, taken from the
 * snippet stored with the comment (its diff markers removed). */
export function originalLines(snippet: string): string[] {
  if (snippet === '') return []
  return snippet
    .split('\n')
    .filter((line) => !line.startsWith('-'))
    .map((line) => (line.startsWith('+') || line.startsWith(' ') ? line.slice(1) : line))
}
