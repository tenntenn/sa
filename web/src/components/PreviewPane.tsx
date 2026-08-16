import { useEffect, useRef, useState } from 'react'
import type { FileDiff, Status } from '../types'
import { filePath } from '../types'
import { client, type PreviewResult } from '../client'
import { readSetting, writeSetting } from '../storage'

interface Props {
  group: string
  diffId: string | null
  file: FileDiff | null
  status: Status | null
  /** narrow is set on a phone, where mo's own layout has no room. */
  narrow?: boolean
  /** scrollTo is the fraction of the diff that has been scrolled past, or
   * null when the diff is not driving the preview. */
  scrollTo?: number | null
  /** sync is whether the preview is following the diff, and onSync turns
   * that off and on where the reader is looking. */
  sync?: boolean
  onSync?: (on: boolean) => void
}

/** PreviewKind is which of the two previews is showing. */
export type PreviewKind = 'mo' | 'simple'

const RENDERER_KEY = 'sa.preview.renderer'

/**
 * PreviewPane shows the Markdown preview next to the diff.
 *
 * In the live app the preview is rendered by mo. mo forbids framing with
 * "frame-ancestors 'none'", so sa serves it through its own loopback proxy,
 * which relaxes that one directive for sa's origin. An exported page has no
 * mo behind it and renders the frozen Markdown itself.
 *
 * A phone does the same: mo keeps its own sidebar inside the frame, which
 * would leave a column too narrow to read, so the Markdown is rendered here
 * and mo is one tap away in its own window.
 */
export function PreviewPane({
  group,
  diffId,
  file,
  status,
  narrow = false,
  scrollTo = null,
  sync = false,
  onSync,
}: Props) {
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [openingMo, setOpeningMo] = useState(false)

  // Which preview to use is the reader's to choose: mo is the full one, and
  // the simple one is rendered by this page, which is what lets the diff
  // scroll it. A phone and an exported page have no embedded mo to choose.
  const [chosen, setChosen] = useState<PreviewKind>(
    () => (readSetting(RENDERER_KEY) === 'simple' ? 'simple' : 'mo'),
  )
  const forced = narrow || client.isStatic
  const kind: PreviewKind = forced ? 'simple' : chosen
  const previewable = file !== null && file.isMarkdown && diffId !== null
  const renderHere = kind === 'simple'
  const body = useRef<HTMLDivElement>(null)

  useEffect(() => {
    writeSetting(RENDERER_KEY, kind)
  }, [kind])

  // Follow the diff. Only the preview rendered here can be moved: mo is
  // framed from another origin, where a page may not touch its scrolling.
  useEffect(() => {
    if (scrollTo === null || renderHere !== true) return
    const el = body.current
    if (!el) return
    const room = el.scrollHeight - el.clientHeight
    if (room <= 0) return
    el.scrollTop = room * scrollTo
  }, [scrollTo, renderHere, preview])

  useEffect(() => {
    if (!previewable || !file || !diffId) {
      setPreview(null)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    const load = renderHere
      ? client.previewMarkdown(group, diffId, file.id)
      : client.preview(group, diffId, file.id)
    load
      .then((p) => {
        if (!cancelled) setPreview(p)
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setPreview(null)
          setError(err instanceof Error ? err.message : String(err))
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [group, diffId, file, previewable, renderHere, reloadKey])

  // mo is not embedded on a phone, but it is still the full preview: ask the
  // server for it only when someone actually wants it.
  const openInMo = async () => {
    if (!previewable || !file || !diffId) return
    setOpeningMo(true)
    setError(null)
    try {
      const result = await client.preview(group, diffId, file.id)
      if (result.kind === 'frame' && result.moUrl) {
        window.open(result.moUrl, '_blank', 'noreferrer')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setOpeningMo(false)
    }
  }

  if (!file) {
    return (
      <section className="preview">
        <div className="preview-header">
          <span className="preview-title">Preview</span>
        </div>
        <p className="empty">Select a file.</p>
      </section>
    )
  }

  return (
    <section className="preview">
      <div className="preview-header">
        <span className="preview-title">Preview</span>
        <span className="path">{filePath(file)}</span>
        {preview && (
          <>
            <span
              className="badge"
              title={`${preview.path}\n${
                preview.source === 'worktree' ? 'the working tree file' : 'rebuilt from the diff'
              }`}
            >
              {preview.source === 'worktree' ? 'tree' : 'rebuilt'}
            </span>
            {!preview.complete && (
              <span className="badge warn" title="A unified diff only carries the changed hunks">
                partial
              </span>
            )}
          </>
        )}
        <span className="spacer" />
        {!forced && (
          <div className="toggle" title="mo renders the full preview; the simple one is drawn by this page and can follow the diff">
            <button className={kind === 'mo' ? 'active' : ''} onClick={() => setChosen('mo')}>
              mo
            </button>
            <button className={kind === 'simple' ? 'active' : ''} onClick={() => setChosen('simple')}>
              simple
            </button>
          </div>
        )}
        {previewable && onSync && (
          <label
            className="switch"
            title={
              !renderHere
                ? 'Only the simple preview can follow the diff: mo is framed from another origin, where a page may not touch its scrolling'
                : sync
                  ? 'The preview follows the diff; scrolling it yourself stops that'
                  : 'Follow the diff again'
            }
          >
            <input
              type="checkbox"
              checked={sync && renderHere}
              disabled={!renderHere}
              onChange={(ev) => onSync(ev.target.checked)}
            />
            Sync
          </label>
        )}
        {!client.isStatic && (
          <button className="ghost" onClick={() => setReloadKey((k) => k + 1)} disabled={!previewable}>
            Reload
          </button>
        )}
        {preview?.kind === 'frame' && preview.moUrl && (
          <a className="ghost button" href={preview.moUrl} target="_blank" rel="noreferrer">
            Open in mo
          </a>
        )}
        {!client.isStatic && renderHere && (
          <button className="ghost" onClick={() => void openInMo()} disabled={!previewable || openingMo}>
            {openingMo ? 'Opening…' : 'Open in mo'}
          </button>
        )}
      </div>

      {!file.isMarkdown ? (
        <p className="empty">{filePath(file)} is not Markdown, so there is nothing to preview.</p>
      ) : loading ? (
        <p className="empty">{renderHere ? 'Rendering…' : 'Asking mo for a preview…'}</p>
      ) : error ? (
        <div className="preview-error">
          <p className="error">{error}</p>
          {status && !status.moAvailable && (
            <p className="hint">
              The Markdown preview is rendered by mo. Install it with{' '}
              <code>brew install k1LoW/tap/mo</code> or grab a binary from{' '}
              <a href="https://github.com/k1LoW/mo/releases" target="_blank" rel="noreferrer">
                the releases page
              </a>
              , then reload.
            </p>
          )}
        </div>
      ) : preview?.kind === 'html' ? (
        <div
          className="markdown"
          ref={body}
          onWheel={() => onSync?.(false)}
          onTouchMove={() => onSync?.(false)}
          dangerouslySetInnerHTML={{ __html: preview.html }}
        />
      ) : preview?.kind === 'frame' && preview.url ? (
        <iframe className="preview-frame" src={preview.url} title="Markdown preview" />
      ) : preview?.kind === 'frame' && preview.moUrl ? (
        <div className="preview-error">
          <p className="error">The preview cannot be embedded here.</p>
          <p className="hint">
            <a href={preview.moUrl} target="_blank" rel="noreferrer">
              Open it in mo
            </a>{' '}
            instead.
          </p>
        </div>
      ) : (
        <p className="empty">No preview.</p>
      )}
    </section>
  )
}
