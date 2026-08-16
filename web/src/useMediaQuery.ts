import { useEffect, useState } from 'react'

/** useMediaQuery follows a CSS media query from React. */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches)

  useEffect(() => {
    const mql = window.matchMedia(query)
    const onChange = () => setMatches(mql.matches)
    setMatches(mql.matches)
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [query])

  return matches
}

/**
 * NARROW_LAYOUT is the width below which three panes side by side stop making
 * sense — a phone. The layout then shows one pane at a time.
 */
export const NARROW_LAYOUT = '(max-width: 720px)'

/** useNarrowLayout reports whether the window is too narrow for panes. */
export function useNarrowLayout(): boolean {
  return useMediaQuery(NARROW_LAYOUT)
}
