/**
 * The palette follows the system by default. This module lets a reader
 * override that for their own browser, and keeps out of the way when they
 * have not: an exported page may be read inside something that sets the
 * theme itself, and "auto" means that host wins.
 */

import { readSetting, writeSetting } from './storage'

export type Theme = 'auto' | 'light' | 'dark'

const STORAGE_KEY = 'sbnn.theme'

// hostTheme is whatever the page was stamped with before sbnn touched it, so
// that going back to "auto" restores it rather than dropping it.
const hostTheme = document.documentElement.getAttribute('data-theme')

export function storedTheme(): Theme {
  const stored = readSetting(STORAGE_KEY)
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
  writeSetting(STORAGE_KEY, theme)
}

