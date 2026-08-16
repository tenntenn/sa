import { useCallback, useEffect, useMemo, useState } from 'react'
import { getGroup, getPrompt, getStatus, groupFromLocation, subscribe } from './api'
import type { Comment, Diff, FileDiff, Status } from './types'
import { DiffView } from './components/DiffView'
import { PreviewPane } from './components/PreviewPane'
import { Sidebar } from './components/Sidebar'
import { SplitPane } from './components/SplitPane'

interface Selected {
  diffId: string
  fileId: string
}

export function App() {
  const group = useMemo(groupFromLocation, [])
  const [diffs, setDiffs] = useState<Diff[]>([])
  const [comments, setComments] = useState<Comment[]>([])
  const [status, setStatus] = useState<Status | null>(null)
  const [selected, setSelected] = useState<Selected | null>(null)
  const [showPreview, setShowPreview] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const reload = useCallback(async () => {
    try {
      const [g, st] = await Promise.all([getGroup(group), getStatus()])
      setDiffs(g.diffs ?? [])
      setComments(g.comments ?? [])
      setStatus(st)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [group])

  useEffect(() => {
    void reload()
    return subscribe(group, () => {
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

  const copyPrompt = async () => {
    try {
      const text = await getPrompt(group)
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">sa</span>
        <span className="group">{group}</span>
        <span className="hint">
          {diffs.length} diff(s) · {comments.length} comment(s)
          {openComments > 0 ? ` · ${openComments} open` : ''}
        </span>
        <span className="spacer" />
        <button className="ghost" onClick={() => void copyPrompt()} disabled={comments.length === 0}>
          {copied ? 'Copied' : 'Copy prompt'}
        </button>
        <label className="switch">
          <input
            type="checkbox"
            checked={showPreview}
            onChange={(ev) => setShowPreview(ev.target.checked)}
          />
          Preview
        </label>
      </header>

      {error && <div className="error banner">{error}</div>}

      <div className="body">
        <Sidebar
          group={group}
          diffs={diffs}
          comments={comments}
          status={status}
          selected={selected}
          onSelect={(diffId, fileId) => setSelected({ diffId, fileId })}
          onChanged={() => void reload()}
        />

        <main className="content">
          {diffs.length === 0 ? (
            <div className="welcome">
              <h1>Waiting for a diff</h1>
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
          ) : (
            <SplitPane
              showRight={showPreview}
              left={
                selectedDiff && selectedFile ? (
                  <DiffView
                    group={group}
                    diff={selectedDiff}
                    file={selectedFile}
                    comments={fileComments}
                    onChanged={() => void reload()}
                  />
                ) : (
                  <p className="empty">Select a file.</p>
                )
              }
              right={
                <PreviewPane
                  group={group}
                  diffId={selectedDiff?.id ?? null}
                  file={selectedFile}
                  status={status}
                />
              }
            />
          )}
        </main>
      </div>
    </div>
  )
}
