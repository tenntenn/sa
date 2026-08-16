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
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

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

  return (
    <div className={`comment${comment.resolved ? ' resolved' : ''}`}>
      <div className="comment-meta">
        <span className="comment-range">
          {comment.path}:{comment.startLine}
          {comment.endLine > comment.startLine ? `-${comment.endLine}` : ''}
          {comment.side === 'old' ? ' (old)' : ''}
        </span>
        {comment.resolved && <span className="badge">resolved</span>}
      </div>

      {editing ? (
        <>
          <textarea
            className="comment-input"
            value={body}
            onChange={(ev) => setBody(ev.target.value)}
            rows={Math.max(3, body.split('\n').length)}
          />
          <div className="comment-actions">
            <button
              disabled={busy || body.trim() === ''}
              onClick={() =>
                run(async () => {
                  await client.updateComment(group, comment.id, { body })
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
                setEditing(false)
              }}
            >
              Cancel
            </button>
          </div>
        </>
      ) : (
        <>
          <div className="comment-body">{comment.body}</div>
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
  onSubmit: (body: string) => Promise<void>
  onCancel: () => void
  label: string
}

/** CommentForm writes a new comment for the selected lines. */
export function CommentForm({ onSubmit, onCancel, label }: FormProps) {
  const [body, setBody] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async () => {
    if (body.trim() === '') return
    setBusy(true)
    setError(null)
    try {
      await onSubmit(body)
      setBody('')
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
      </div>
      <textarea
        className="comment-input"
        autoFocus
        rows={4}
        placeholder="What should change here?"
        value={body}
        onChange={(ev) => setBody(ev.target.value)}
        onKeyDown={(ev) => {
          if (ev.key === 'Escape') onCancel()
          if (ev.key === 'Enter' && (ev.metaKey || ev.ctrlKey)) void submit()
        }}
      />
      <div className="comment-actions">
        <button disabled={busy || body.trim() === ''} onClick={() => void submit()}>
          Comment
        </button>
        <button className="ghost" disabled={busy} onClick={onCancel}>
          Cancel
        </button>
        <span className="hint">⌘/Ctrl + Enter</span>
      </div>
      {error && <div className="error inline">{error}</div>}
    </div>
  )
}
