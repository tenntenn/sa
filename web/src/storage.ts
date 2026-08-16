/**
 * Remembered settings - pane sizes, the theme - kept where the browser lets
 * us keep them.
 *
 * Reaching for localStorage is not always allowed: a sandboxed iframe throws
 * on the property access itself, and so does a browser told to block site
 * data. An exported page is meant to be embedded in exactly such a frame
 * (`sa export --fragment`, an artifact), where an unguarded read during
 * render leaves the reader with a blank page instead of the review. A
 * forgotten pane width is not worth that, so every access is guarded and a
 * refusal simply means the defaults.
 */
export function readSetting(key: string): string | null {
  try {
    return window.localStorage.getItem(key)
  } catch {
    return null
  }
}

export function writeSetting(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value)
  } catch {
    // The setting lasts for this page view, which is the best on offer.
  }
}
