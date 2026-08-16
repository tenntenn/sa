import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { groupFromLocation } from './api'
import { client } from './client'
import { readSetting, writeSetting } from './storage'
import type { Comment, Diff, FileDiff, Status } from './types'
import { DiffView } from './components/DiffView'
import { Divider } from './components/Divider'
import { PreviewPane } from './components/PreviewPane'
import { Sidebar } from './components/Sidebar'
import { clampRatio, SplitPane, SPLIT_DEFAULT } from './components/SplitPane'
import { useNarrowLayout } from './useMediaQuery'
import { applyTheme, nextTheme, storedTheme, themeLabel, type Theme } from './theme'

interface Selected {
  diffId: string
  fileId: string
}

/** Pane names the three panes, which a phone shows one at a time. */
type Pane = 'files' | 'diff' | 'preview'

// The file list is a pane like the others: it can be dragged narrow, and
// pulling it past the snapping point puts it away entirely.
const SIDEBAR_DEFAULT = 280
const SIDEBAR_MAX = 720
const SIDEBAR_SNAP = 48
const SIDEBAR_STEP = 24
const SIDEBAR_KEY = 'sa.sidebar.width'
const SPLIT_KEY = 'sa.split'

function storedSplitRatio(): number {
  const stored = readSetting(SPLIT_KEY)
  if (stored === null) return SPLIT_DEFAULT
  const ratio = Number(stored)
  if (!Number.isFinite(ratio) || ratio < 0 || ratio > 1) return SPLIT_DEFAULT
  return ratio === 0 || ratio === 1 ? ratio : clampRatio(ratio)
}

function storedSidebarWidth(): number {
  // An unset entry reads as null, which Number() would happily turn into a
  // collapsed sidebar, so the absence is checked before the value.
  const stored = readSetting(SIDEBAR_KEY)
  if (stored === null) return SIDEBAR_DEFAULT
  const width = Number(stored)
  return Number.isFinite(width) && width >= 0 && width <= SIDEBAR_MAX ? width : SIDEBAR_DEFAULT
}

export function App() {
  const group = useMemo(() => (client.isStatic ? staticGroupName() : groupFromLocation()), [])
  const narrow = useNarrowLayout()
  const [diffs, setDiffs] = useState<Diff[]>([])
  const [comments, setComments] = useState<Comment[]>([])
  const [reviewedAt, setReviewedAt] = useState<string | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [selected, setSelected] = useState<Selected | null>(null)
  const [splitRatio, setSplitRatio] = useState(storedSplitRatio)
  const [pane, setPane] = useState<Pane>('diff')
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [closing, setClosing] = useState(false)
  const [reviewNote, setReviewNote] = useState<string | null>(null)
  const [sidebarWidth, setSidebarWidth] = useState(storedSidebarWidth)
  const [theme, setTheme] = useState<Theme>(storedTheme)
  const bodyRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  useEffect(() => {
    writeSetting(SIDEBAR_KEY, String(sidebarWidth))
  }, [sidebarWidth])

  const resizeSidebar = useCallback((clientX: number) => {
    const rect = bodyRef.current?.getBoundingClientRect()
    if (!rect) return
    const next = clientX - rect.left
    setSidebarWidth(next < SIDEBAR_SNAP ? 0 : Math.min(SIDEBAR_MAX, next))
  }, [])

  const toggleSidebar = () => setSidebarWidth((w) => (w === 0 ? SIDEBAR_DEFAULT : 0))

  useEffect(() => {
    writeSetting(SPLIT_KEY, String(splitRatio))
  }, [splitRatio])

  // Minimising one pane hands its room to the other; bringing it back splits
  // the room again rather than restoring a width nobody remembers.
  const toggleDiff = () => setSplitRatio((r) => (r === 0 ? SPLIT_DEFAULT : 0))
  const togglePreview = () => setSplitRatio((r) => (r === 1 ? SPLIT_DEFAULT : 1))

  const reload = useCallback(async () => {
    try {
      const data = await client.load(group)
      setDiffs(data.diffs)
      setComments(data.comments)
      setReviewedAt(data.reviewedAt ?? null)
      setStatus(data.status)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [group])

  useEffect(() => {
    void reload()
    return client.subscribe(group, () => {
      void reload()
    })
  }, [group, reload])

  // Keep a file selected: the newest diff is what the reviewer just sent.
  useEffect(() => {
    if (diffs.length === 0) {
      setSelected(null)
      return
    }
    setSelected((current) => {
      if (current) {
        const diff = diffs.find((d) => d.id === current.diffId)
        if (diff && diff.files.some((f) => f.id === current.fileId)) return current
      }
      const last = diffs[diffs.length - 1]
      const file = last.files[0]
      return file ? { diffId: last.id, fileId: file.id } : null
    })
  }, [diffs])

  const selectedDiff: Diff | null = selected
    ? (diffs.find((d) => d.id === selected.diffId) ?? null)
    : null
  const selectedFile: FileDiff | null =
    selectedDiff && selected
      ? (selectedDiff.files.find((f) => f.id === selected.fileId) ?? null)
      : null
  const fileComments = useMemo(
    () =>
      selected
        ? comments.filter((c) => c.diffId === selected.diffId && c.fileId === selected.fileId)
        : [],
    [comments, selected],
  )
  const openComments = comments.filter((c) => !c.resolved).length
  const fileCount = diffs.reduce((total, diff) => total + diff.files.length, 0)

  const copyPrompt = async () => {
    try {
      const text = await client.prompt(group)
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  // The review is what anything waiting on sa is waiting for, so saying "I
  // am done" is an explicit act rather than a guess from the last comment.
  const summary = status?.groups.find((g) => g.name === group)
  const reviewed = summary ? summary.reviewed : reviewedAt !== null
  const hooks = summary?.hooks ?? 0

  const submitReview = async () => {
    setSubmitting(true)
    setError(null)
    try {
      await client.submitReview(group, reviewNote ?? '')
      setReviewNote(null)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSubmitting(false)
    }
  }

  // Closing is the end of a review: the diffs, the comments and the hooks
  // go, and the page is back to waiting for the next one.
  const closeReview = async () => {
    const open = comments.filter((c) => !c.resolved).length
    const question =
      open > 0
        ? `Close this review? ${open} comment(s) are still open and will go with it.`
        : 'Close this review? Its diffs and comments will go.'
    if (!window.confirm(question)) return
    setClosing(true)
    setError(null)
    try {
      await client.closeReview(group)
      await reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setClosing(false)
    }
  }

  const selectFile = (diffId: string, fileId: string) => {
    setSelected({ diffId, fileId })
    // On a phone the file list covers the screen, so picking a file means
    // "show me this diff".
    if (narrow) setPane('diff')
  }

  const sidebar = (
    <Sidebar
      width={narrow ? null : sidebarWidth}
      group={group}
      diffs={diffs}
      comments={comments}
      status={status}
      selected={selected}
      onSelect={selectFile}
      onChanged={() => void reload()}
    />
  )

  const diffPane =
    selectedDiff && selectedFile ? (
      <DiffView
        // The server picks split or unified per file, and that choice is
        // the initial state of the view; without a key React keeps the
        // first file's state for every file after it.
        key={selectedFile.id}
        group={group}
        diff={selectedDiff}
        file={selectedFile}
        comments={fileComments}
        narrow={narrow}
        onChanged={() => void reload()}
      />
    ) : (
      <p className="empty">Select a file.</p>
    )

  const previewPane = (
    <PreviewPane
      group={group}
      diffId={selectedDiff?.id ?? null}
      file={selectedFile}
      status={status}
      narrow={narrow}
    />
  )

  const welcome = (
    <div className="welcome">
      <h1>{client.isStatic ? 'This page carries no diff' : 'Waiting for a diff'}</h1>
      <p>Pipe one in — sa adds it to this page:</p>
      <pre>
        <code>
          git diff | sa{group === 'default' ? '' : ` --target ${group}`}
          {'\n'}diff -u old.md new.md | sa{group === 'default' ? '' : ` --target ${group}`}
        </code>
      </pre>
      <p className="hint">
        Comments you leave here are readable from the command line with{' '}
        <code>sa comments{group === 'default' ? '' : ` -t ${group}`}</code>.
      </p>
    </div>
  )

  return (
    <div className={`app${narrow ? ' narrow' : ''}`}>
      <header className="topbar">
        <span className="brand">sa</span>
        <span className="group">{group}</span>
        <span className="hint counts">
          {diffs.length} diff(s) · {comments.length} comment(s)
          {openComments > 0 ? ` · ${openComments} open` : ''}
        </span>
        {client.isStatic && (
          <span
            className="badge"
            title={
              'This page was written with `sa export`. The diff is frozen and ' +
              'comments are kept in this browser.'
            }
          >
            exported
            {client.exportedAt && (
              <span className="exported-at"> {new Date(client.exportedAt).toLocaleString()}</span>
            )}
          </span>
        )}
        <span className="spacer" />
        <button className="ghost" onClick={() => void copyPrompt()} disabled={comments.length === 0}>
          {copied ? 'Copied' : 'Copy prompt'}
        </button>
        {!client.isStatic && (
          <button
            className={reviewed ? 'ghost' : ''}
            disabled={submitting || diffs.length === 0}
            onClick={() => setReviewNote((note) => (note === null ? '' : null))}
            title={
              hooks > 0
                ? `Submitting runs ${hooks} hook(s) on the sa server`
                : 'Tell whoever is waiting that the review is done'
            }
          >
            {reviewed ? 'Reviewed' : 'Submit review'}
          </button>
        )}
        <button
          className="ghost"
          onClick={() => setTheme(nextTheme(theme))}
          title="Colours: follow the system, or pick one"
        >
          {themeLabel(theme)}
        </button>
        {!client.isStatic && diffs.length > 0 && (
          <button
            className="ghost danger"
            disabled={closing}
            onClick={() => void closeReview()}
            title="Drop this review: its diffs, comments and hooks"
          >
            {closing ? 'Closing…' : 'Close'}
          </button>
        )}
        {!narrow && (
          <>
            <label className="switch">
              <input type="checkbox" checked={sidebarWidth > 0} onChange={toggleSidebar} />
              Files
            </label>
            <label className="switch">
              <input
                type="checkbox"
                checked={splitRatio > 0}
                onChange={toggleDiff}
                disabled={diffs.length === 0}
              />
              Diff
            </label>
            <label className="switch">
              <input
                type="checkbox"
                checked={splitRatio < 1}
                onChange={togglePreview}
                disabled={diffs.length === 0}
              />
              Preview
            </label>
          </>
        )}
      </header>

      {narrow && diffs.length > 0 && (
        <nav className="tabs" aria-label="Panes">
          <button
            className={pane === 'files' ? 'active' : ''}
            aria-pressed={pane === 'files'}
            onClick={() => setPane('files')}
          >
            Files<span className="tab-count">{fileCount}</span>
          </button>
          <button
            className={pane === 'diff' ? 'active' : ''}
            aria-pressed={pane === 'diff'}
            onClick={() => setPane('diff')}
          >
            Diff
            {fileComments.length > 0 && <span className="tab-count">{fileComments.length}</span>}
          </button>
          <button
            className={pane === 'preview' ? 'active' : ''}
            aria-pressed={pane === 'preview'}
            onClick={() => setPane('preview')}
            disabled={!selectedFile?.isMarkdown}
            title={selectedFile?.isMarkdown ? undefined : 'Only Markdown files have a preview'}
          >
            Preview
          </button>
        </nav>
      )}

      {reviewNote !== null && (
        <div className="review-form">
          <label className="field-label" htmlFor="sa-review-note">
            Anything to say about the change as a whole? (optional)
          </label>
          <textarea
            id="sa-review-note"
            className="comment-input"
            autoFocus
            rows={3}
            value={reviewNote}
            placeholder="Looks good apart from the two comments"
            onChange={(ev) => setReviewNote(ev.target.value)}
            onKeyDown={(ev) => {
              if (ev.key === 'Escape') setReviewNote(null)
              if (ev.key === 'Enter' && (ev.metaKey || ev.ctrlKey)) void submitReview()
            }}
          />
          <div className="comment-actions">
            <button disabled={submitting} onClick={() => void submitReview()}>
              {submitting ? 'Submitting…' : `Submit review (${openComments} open)`}
            </button>
            <button className="ghost" disabled={submitting} onClick={() => setReviewNote(null)}>
              Cancel
            </button>
            <span className="hint">
              {hooks > 0
                ? `${hooks} hook(s) will run on the sa server`
                : 'Anything waiting with `sa wait` carries on'}
            </span>
          </div>
        </div>
      )}

      {reviewed && reviewNote === null && (
        <div className="review-banner">
          Review submitted{reviewedAt ? ` ${new Date(reviewedAt).toLocaleString()}` : ''}. Send
          another diff to start the next round.
        </div>
      )}

      {error && <div className="error banner">{error}</div>}

      {narrow ? (
        <div className="body">
          {diffs.length === 0 ? (
            <main className="content">{welcome}</main>
          ) : pane === 'files' ? (
            sidebar
          ) : (
            <main className="content">{pane === 'preview' ? previewPane : diffPane}</main>
          )}
        </div>
      ) : (
        <div className="body" ref={bodyRef}>
          {sidebar}
          <Divider
            label="Resize the file list"
            onDrag={resizeSidebar}
            onReset={toggleSidebar}
            onNudge={(direction) =>
              setSidebarWidth((w) => Math.min(SIDEBAR_MAX, Math.max(0, w + direction * SIDEBAR_STEP)))
            }
          />
          <main className="content">
            {diffs.length === 0 ? (
              welcome
            ) : (
              <SplitPane
                ratio={splitRatio}
                onRatioChange={setSplitRatio}
                left={diffPane}
                right={previewPane}
              />
            )}
          </main>
        </div>
      )}
    </div>
  )
}

/** staticGroupName reads the group an exported page was written for. */
function staticGroupName(): string {
  return window.__SA_DATA__?.group ?? 'default'
}
