/**
 * The keys the review page answers to.
 *
 * Reviewing is reading, and reading is done with both hands on the
 * keyboard: the next file, the next comment, fold this away, submit. Every
 * key here is a single unmodified press, which is only safe because none of
 * them fires while something is being typed into - see typingInto below.
 *
 * The list is the documentation: the help overlay is drawn from it, so a
 * shortcut cannot exist without a line explaining it.
 */
export interface Shortcut {
  keys: string[]
  what: string
}

export const shortcuts: Shortcut[] = [
  { keys: ['j'], what: 'Next file' },
  { keys: ['k'], what: 'Previous file' },
  { keys: ['/'], what: 'Filter the file list by path' },
  { keys: ['n'], what: 'Next comment' },
  { keys: ['p'], what: 'Previous comment' },
  { keys: ['f'], what: 'Fold or unfold this file' },
  { keys: ['v'], what: 'Split or unified' },
  { keys: ['s'], what: 'Follow the diff with the preview' },
  { keys: ['r'], what: 'Submit review' },
  { keys: ['?'], what: 'This list' },
  { keys: ['Esc'], what: 'Close what is open' },
]

/**
 * typingInto reports whether a key belongs to whatever the reader is
 * writing in. A comment full of the letter "f" would be unusable if every
 * one of them folded the file away.
 */
export function typingInto(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  if (el.isContentEditable) return true
  return /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName)
}

/** plainKey reports whether an event is an unmodified key press. */
export function plainKey(ev: KeyboardEvent): boolean {
  return !ev.metaKey && !ev.ctrlKey && !ev.altKey
}
