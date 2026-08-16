import { useState } from 'react'
import type { Comment } from '../types'
import { client } from '../client'

interface ThreadProps {
  group: string
  comments: Comment[]
  onChanged: () => void
}

/** CommentThread renders the comments anchored to one line range. */
export function CommentThread({ group, comments, onChanged }: ThreadProps) {
  return (
    <div className="thread">
      {comments.map((c) => (
        <CommentItem key={c.id} group={group} comment={c} onChanged={onChanged} />
      ))}
    </div>
  )
}

function rangeLabel(c: Pick<Comment, 'path' | 'side' | 'startLine' | 'endLine'>): string {
  const lines = c.endLine > c.startLine ? `${c.startLine}-${c.endLine}` : `${c.startLine}`
  return `${c.path}:${lines}${c.side === 'old' ? ' (old)' : ''}`
}

function CommentItem({
  group,
  comment,
  onChanged,
}: {
  group: string
  comment: Comment
  onChanged: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [body, setBody] = useState(comment.body)
  const [suggestion, setSuggestion] = useState(comment.suggestion ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      onChanged()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const copySuggestion = async () => {
    if (!comment.suggestion) return
    try {
      await navigator.clipboard.writeText(comment.suggestion)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className={`comment${comment.resolved ? ' resolved' : ''}`}>
      <div className="comment-meta">
        <span className="comment-range">{rangeLabel(comment)}</span>
        {comment.suggestion && !editing && <span className="badge">suggestion</span>}
        {comment.resolved && <span className="badge">resolved</span>}
      </div>

      {editing ? (
        <>
          <textarea
            className="comment-input"
            value={body}
            onChange={(ev) => setBody(ev.target.value)}
            rows={Math.max(3, body.split('\n').length)}
            placeholder="What should change here?"
          />
          <label className="field-label">Suggested replacement (empty to drop it)</label>
          <textarea
            className="comment-input suggestion-input"
            value={suggestion}
            onChange={(ev) => setSuggestion(ev.target.value)}
            rows={Math.max(2, suggestion.split('\n').length)}
          />
          <div className="comment-actions">
            <button
              disabled={busy || (body.trim() === '' && suggestion.trim() === '')}
              onClick={() =>
                run(async () => {
                  await client.updateComment(group, comment.id, { body, suggestion })
                  setEditing(false)
                })
              }
            >
              Save
            </button>
            <button
              className="ghost"
              disabled={busy}
              onClick={() => {
                setBody(comment.body)
                setSuggestion(comment.suggestion ?? '')
                setEditing(false)
              }}
            >
              Cancel
            </button>
          </div>
        </>
      ) : (
        <>
          {comment.body && <div className="comment-body">{comment.body}</div>}
          {comment.suggestion && (
            <div className="suggestion">
              <div className="suggestion-head">
                <span>Replaces {rangeLabel(comment)}</span>
                <button className="ghost" onClick={() => void copySuggestion()}>
                  {copied ? 'Copied' : 'Copy'}
                </button>
              </div>
              <pre>
                <code>{comment.suggestion}</code>
              </pre>
            </div>
          )}
          <div className="comment-actions">
            <button
              className="ghost"
              disabled={busy}
              onClick={() => run(() => client.updateComment(group, comment.id, { resolved: !comment.resolved }))}
            >
              {comment.resolved ? 'Reopen' : 'Resolve'}
            </button>
            <button className="ghost" disabled={busy} onClick={() => setEditing(true)}>
              Edit
            </button>
            <button
              className="ghost danger"
              disabled={busy}
              onClick={() => run(() => client.deleteComment(group, comment.id))}
            >
              Delete
            </button>
          </div>
        </>
      )}
      {error && <div className="error inline">{error}</div>}
    </div>
  )
}

interface FormProps {
  onSubmit: (body: string, suggestion: string) => Promise<void>
  onCancel: () => void
  label: string
  /** seed is the current text of the selected lines, the starting point of a
   * suggested replacement. */
  seed: string
  /** canSuggest is false for lines that only exist in the old file. */
  canSuggest: boolean
  /** hint explains how to grow the selection. */
  hint?: string
}

/** CommentForm writes a new comment, with an optional suggested replacement. */
export function CommentForm({ onSubmit, onCancel, label, seed, canSuggest, hint }: FormProps) {
  const [body, setBody] = useState('')
  const [suggestion, setSuggestion] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async () => {
    const proposed = suggestion ?? ''
    if (body.trim() === '' && proposed.trim() === '') return
    setBusy(true)
    setError(null)
    try {
      await onSubmit(body, proposed)
      setBody('')
      setSuggestion(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="comment-form">
      <div className="comment-meta">
        <span className="comment-range">{label}</span>
        {hint && <span className="hint">{hint}</span>}
      </div>
      <textarea
        className="comment-input"
        autoFocus
        rows={3}
        placeholder="What should change here?"
        value={body}
        onChange={(ev) => setBody(ev.target.value)}
        onKeyDown={(ev) => {
          if (ev.key === 'Escape') onCancel()
          if (ev.key === 'Enter' && (ev.metaKey || ev.ctrlKey)) void submit()
        }}
      />
      {suggestion !== null && (
        <>
          <label className="field-label">Suggested replacement for these lines</label>
          <textarea
            className="comment-input suggestion-input"
            rows={Math.max(2, suggestion.split('\n').length)}
            value={suggestion}
            onChange={(ev) => setSuggestion(ev.target.value)}
            onKeyDown={(ev) => {
              if (ev.key === 'Enter' && (ev.metaKey || ev.ctrlKey)) void submit()
            }}
          />
        </>
      )}
      <div className="comment-actions">
        <button disabled={busy || (body.trim() === '' && (suggestion ?? '').trim() === '')} onClick={() => void submit()}>
          Comment
        </button>
        {canSuggest &&
          (suggestion === null ? (
            <button className="ghost" disabled={busy} onClick={() => setSuggestion(seed)}>
              Suggest a change
            </button>
          ) : (
            <button className="ghost" disabled={busy} onClick={() => setSuggestion(null)}>
              Drop the suggestion
            </button>
          ))}
        <button className="ghost" disabled={busy} onClick={onCancel}>
          Cancel
        </button>
        <span className="hint">⌘/Ctrl + Enter</span>
      </div>
      {error && <div className="error inline">{error}</div>}
    </div>
  )
}
