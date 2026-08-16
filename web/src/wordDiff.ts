export interface Segment {
  text: string
  changed: boolean
}

const tokenPattern = /(\s+|[A-Za-z0-9_$]+|.)/g

function tokenize(s: string): string[] {
  return s.match(tokenPattern) ?? []
}

/**
 * wordDiff highlights what actually changed between a removed and an added
 * line by trimming the common head and tail at token boundaries. It is
 * deliberately simple: a full diff of every line would cost more than it is
 * worth while reviewing.
 */
export function wordDiff(oldLine: string, newLine: string): [Segment[], Segment[]] {
  const a = tokenize(oldLine)
  const b = tokenize(newLine)

  let head = 0
  while (head < a.length && head < b.length && a[head] === b[head]) head++

  let tail = 0
  while (
    tail < a.length - head &&
    tail < b.length - head &&
    a[a.length - 1 - tail] === b[b.length - 1 - tail]
  ) {
    tail++
  }

  const build = (tokens: string[]): Segment[] => {
    const segments: Segment[] = []
    const push = (text: string, changed: boolean) => {
      if (!text) return
      const last = segments[segments.length - 1]
      if (last && last.changed === changed) last.text += text
      else segments.push({ text, changed })
    }
    push(tokens.slice(0, head).join(''), false)
    push(tokens.slice(head, tokens.length - tail).join(''), true)
    push(tokens.slice(tokens.length - tail).join(''), false)
    return segments
  }

  return [build(a), build(b)]
}
