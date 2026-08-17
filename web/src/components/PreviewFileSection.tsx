import { useEffect, useState } from 'react'
import type { FileDiff, PreviewKind, Status } from '../types'
import { filePath } from '../types'
import { client, type PreviewResult } from '../client'
import { Icon } from './Icon'
import { MoIcon } from './MoIcon'

interface Props {
  group: string
  diffId: string
  file: FileDiff
  status: Status | null
  kind: PreviewKind
  /** active gates the fetch: a section far from the viewport has no reason
   * to ask the server (or, worse, mo) to render it yet. Once true it stays
   * true, so scrolling back to an already-loaded file never refetches it. */
  active: boolean
  /** onUserScroll fires when the reader scrolls this section themselves,
   * which is what turns following the diff off. */
  onUserScroll?: () => void
}

/**
 * PreviewFileSection shows one file's Markdown preview.
 *
 * In the live app the preview is rendered by mo. mo forbids framing with
 * "frame-ancestors 'none'", so sbnn serves it through its own loopback proxy,
 * which relaxes that one directive for sbnn's origin. An exported page has no
 * mo behind it and renders the frozen Markdown itself.
 */
export function PreviewFileSection({ group, diffId, file, status, kind, active, onUserScroll }: Props) {
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [openingMo, setOpeningMo] = useState(false)

  const renderHere = kind === 'preview'
  const previewable = file.isMarkdown

  useEffect(() => {
    if (!previewable) {
      setPreview(null)
      setError(null)
      return
    }
    if (!active) return
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
  }, [group, diffId, file, previewable, renderHere, reloadKey, active])

  const openInMo = async () => {
    if (!previewable) return
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

  return (
    <section className="preview-section">
      <div className="preview-header">
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
        {!client.isStatic && (
          <span title="Reload the preview">
            <button
              className="ghost icon-only"
              onClick={() => setReloadKey((k) => k + 1)}
              disabled={!previewable}
            >
              <Icon name="refresh" />
            </button>
          </span>
        )}
        {/* One slot, always present when not static: a direct link once mo's
            frame has actually loaded, a button that fetches and opens it
            otherwise - two elements swapping in and out by mode made the
            header's width jump between "preview" and "mo". */}
        {!client.isStatic &&
          (preview?.kind === 'frame' && preview.moUrl ? (
            <a className="ghost button" href={preview.moUrl} target="_blank" rel="noreferrer">
              <MoIcon small />
              mo
              <Icon name="open_in_new" small />
            </a>
          ) : (
            <button className="ghost" onClick={() => void openInMo()} disabled={!previewable || openingMo}>
              <MoIcon small />
              {openingMo ? 'Opening…' : 'mo'}
              <Icon name="open_in_new" small />
            </button>
          ))}
      </div>

      {!previewable ? (
        <p className="empty">{filePath(file)} is not Markdown, so there is nothing to preview.</p>
      ) : !active || loading ? (
        <p className="empty">{renderHere ? 'Rendering…' : 'Asking mo for a preview…'}</p>
      ) : error ? (
        <div className="preview-error">
          <p className="error">{error}</p>
          {status && !status.moAvailable && (
            <p className="hint">
              mo renders a richer preview than sbnn's own, and it is not installed here. Install it
              with <code>brew install k1LoW/tap/mo</code> or grab a binary from{' '}
              <a href="https://github.com/k1LoW/mo/releases" target="_blank" rel="noreferrer">
                the releases page
              </a>
              , then reload — or switch back to <strong>preview</strong>, which needs nothing.
            </p>
          )}
        </div>
      ) : preview?.kind === 'html' ? (
        <div
          className="markdown"
          onWheel={onUserScroll}
          onTouchMove={onUserScroll}
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
