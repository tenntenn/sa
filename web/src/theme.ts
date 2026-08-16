/**
 * The palette follows the system by default. This module lets a reader
 * override that for their own browser, and keeps out of the way when they
 * have not: an exported page may be read inside something that sets the
 * theme itself, and "auto" means that host wins.
 */

export type Theme = 'auto' | 'light' | 'dark'

const STORAGE_KEY = 'sa.theme'

// hostTheme is whatever the page was stamped with before sa touched it, so
// that going back to "auto" restores it rather than dropping it.
const hostTheme = document.documentElement.getAttribute('data-theme')

export function storedTheme(): Theme {
  const stored = window.localStorage.getItem(STORAGE_KEY)
  return stored === 'light' || stored === 'dark' ? stored : 'auto'
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement
  if (theme === 'auto') {
    if (hostTheme === null) root.removeAttribute('data-theme')
    else root.setAttribute('data-theme', hostTheme)
  } else {
    root.setAttribute('data-theme', theme)
  }
  try {
    if (theme === 'auto') window.localStorage.removeItem(STORAGE_KEY)
    else window.localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // The choice holds for this page view.
  }
}

/** nextTheme cycles auto → light → dark → auto. */
export function nextTheme(theme: Theme): Theme {
  switch (theme) {
    case 'auto':
      return 'light'
    case 'light':
      return 'dark'
    default:
      return 'auto'
  }
}

export function themeLabel(theme: Theme): string {
  switch (theme) {
    case 'light':
      return 'Light'
    case 'dark':
      return 'Dark'
    default:
      return 'Auto'
  }
}
