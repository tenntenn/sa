import { useEffect, useState } from 'react'
import type { FileDiff, Preview, Status } from '../types'
import { filePath } from '../types'
import { getPreview } from '../api'

interface Props {
  group: string
  diffId: string | null
  file: FileDiff | null
  status: Status | null
}

/**
 * PreviewPane shows the Markdown preview rendered by mo next to the diff.
 *
 * mo forbids framing with "frame-ancestors 'none'", so sa serves it through
 * its own loopback proxy, which relaxes that one directive for sa's origin.
 */
export function PreviewPane({ group, diffId, file, status }: Props) {
  const [preview, setPreview] = useState<Preview | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)

  const previewable = file !== null && file.isMarkdown && diffId !== null

  useEffect(() => {
    if (!previewable || !file || !diffId) {
      setPreview(null)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    getPreview(group, diffId, file.id)
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
  }, [group, diffId, file, previewable, reloadKey])

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
        <button className="ghost" onClick={() => setReloadKey((k) => k + 1)} disabled={!previewable}>
          Reload
        </button>
        {preview?.moUrl && (
          <a className="ghost button" href={preview.moUrl} target="_blank" rel="noreferrer">
            Open in mo
          </a>
        )}
      </div>

      {!file.isMarkdown ? (
        <p className="empty">{filePath(file)} is not Markdown, so there is nothing to preview.</p>
      ) : loading ? (
        <p className="empty">Asking mo for a preview…</p>
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
      ) : preview?.url ? (
        <iframe className="preview-frame" src={preview.url} title="Markdown preview" />
      ) : preview?.moUrl ? (
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
