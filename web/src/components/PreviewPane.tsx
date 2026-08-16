import { useEffect, useState } from 'react'
import type { FileDiff, Status } from '../types'
import { filePath } from '../types'
import { client, type PreviewResult } from '../client'

interface Props {
  group: string
  diffId: string | null
  file: FileDiff | null
  status: Status | null
  /** narrow is set on a phone, where mo's own layout has no room. */
  narrow?: boolean
}

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
export function PreviewPane({ group, diffId, file, status, narrow = false }: Props) {
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [openingMo, setOpeningMo] = useState(false)

  const previewable = file !== null && file.isMarkdown && diffId !== null
  const renderHere = narrow || client.isStatic

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
            <span className="badge" title={preview.path}>
              {preview.source === 'worktree' ? 'working tree' : 'rebuilt from the diff'}
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
        <div className="markdown" dangerouslySetInnerHTML={{ __html: preview.html }} />
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
