import { Fragment, useMemo, useState } from 'react'
import type { Comment, Diff, FileDiff, Hunk, Line, ViewMode } from '../types'
import { filePath } from '../types'
import { client } from '../client'
import { wordDiff } from '../wordDiff'
import { CommentForm, CommentThread } from './CommentThread'

interface Props {
  group: string
  diff: Diff
  file: FileDiff
  comments: Comment[]
  /** narrow is set on a phone, where side by side does not fit. */
  narrow?: boolean
  onChanged: () => void
}

type Side = 'new' | 'old'

interface Selection {
  side: Side
  start: number
  end: number
}

/** anchorKey identifies the row a comment is rendered below. */
function anchorKey(side: Side, line: number): string {
  return `${side}:${line}`
}

function lineSide(line: Line): Side {
  return line.kind === 'delete' ? 'old' : 'new'
}

function lineNumber(line: Line): number {
  return line.kind === 'delete' ? line.oldNumber : line.newNumber
}

function marker(kind: Line['kind']): string {
  switch (kind) {
    case 'add':
      return '+'
    case 'delete':
      return '-'
    default:
      return ' '
  }
}

export function DiffView({ group, diff, file, comments, narrow = false, onChanged }: Props) {
  // A new or deleted file has only one side, so side by side makes no sense
  // for it and the toggle stays locked on unified. A narrow screen has no
  // room for two columns either.
  const locked = narrow || file.status === 'added' || file.status === 'deleted' || file.isBinary
  const [viewMode, setViewMode] = useState<ViewMode>(file.viewMode)
  const [selection, setSelection] = useState<Selection | null>(null)
  const mode: ViewMode = locked ? 'unified' : viewMode

  const commentsByAnchor = useMemo(() => {
    const map = new Map<string, Comment[]>()
    for (const c of comments) {
      const key = anchorKey(c.side, c.endLine)
      const list = map.get(key)
      if (list) list.push(c)
      else map.set(key, [c])
    }
    return map
  }, [comments])

  const select = (side: Side, line: number, extend: boolean) => {
    if (line <= 0) return
    setSelection((current) => {
      if (extend && current && current.side === side) {
        return { side, start: Math.min(current.start, line), end: Math.max(current.end, line) }
      }
      return { side, start: line, end: line }
    })
  }

  const submitComment = async (body: string) => {
    if (!selection) return
    await client.addComment(group, {
      diffId: diff.id,
      fileId: file.id,
      path: filePath(file),
      side: selection.side,
      startLine: selection.start,
      endLine: selection.end,
      body,
      snippet: snippetFor(file, selection),
    })
    setSelection(null)
    onChanged()
  }

  const selectionLabel = selection
    ? `${filePath(file)}:${selection.start}${selection.end > selection.start ? `-${selection.end}` : ''}` +
      `${selection.side === 'old' ? ' (old)' : ''}`
    : ''

  const renderExtras = (side: Side, line: number) => {
    const anchored = commentsByAnchor.get(anchorKey(side, line)) ?? []
    const showForm = selection !== null && selection.side === side && selection.end === line
    if (anchored.length === 0 && !showForm) return null
    return (
      <>
        {anchored.length > 0 && (
          <CommentThread group={group} comments={anchored} onChanged={onChanged} />
        )}
        {showForm && (
          <CommentForm
            label={selectionLabel}
            onSubmit={submitComment}
            onCancel={() => setSelection(null)}
          />
        )}
      </>
    )
  }

  return (
    <div className="diff">
      <div className="diff-header">
        <div className="diff-title">
          <span className={`status status-${file.status}`}>{file.status}</span>
          <span className="path">
            {file.status === 'renamed' || file.status === 'copied'
              ? `${file.oldPath} → ${file.newPath}`
              : filePath(file)}
          </span>
          <span className="stat add">+{file.additions}</span>
          <span className="stat del">-{file.deletions}</span>
        </div>
        <div className="diff-tools">
          {locked ? (
            <span
              className="hint"
              title={
                narrow
                  ? 'Side by side needs a wider window'
                  : 'A file without an old side is always shown unified'
              }
            >
              unified
            </span>
          ) : (
            <div className="toggle">
              <button
                className={mode === 'split' ? 'active' : ''}
                onClick={() => setViewMode('split')}
              >
                split
              </button>
              <button
                className={mode === 'unified' ? 'active' : ''}
                onClick={() => setViewMode('unified')}
              >
                unified
              </button>
            </div>
          )}
        </div>
      </div>

      {file.isBinary ? (
        <p className="empty">Binary file — no diff to show.</p>
      ) : file.hunks.length === 0 ? (
        <p className="empty">
          No content change{file.oldMode && file.newMode ? ` (mode ${file.oldMode} → ${file.newMode})` : ''}.
        </p>
      ) : mode === 'unified' ? (
        <UnifiedTable
          hunks={file.hunks}
          selection={selection}
          onSelect={select}
          renderExtras={renderExtras}
        />
      ) : (
        <SplitTable
          hunks={file.hunks}
          selection={selection}
          onSelect={select}
          renderExtras={renderExtras}
        />
      )}
    </div>
  )
}

interface TableProps {
  hunks: Hunk[]
  selection: Selection | null
  onSelect: (side: Side, line: number, extend: boolean) => void
  renderExtras: (side: Side, line: number) => React.ReactNode
}

function isSelected(selection: Selection | null, side: Side, line: number): boolean {
  return (
    selection !== null &&
    selection.side === side &&
    line >= selection.start &&
    line <= selection.end
  )
}

function UnifiedTable({ hunks, selection, onSelect, renderExtras }: TableProps) {
  return (
    <table className="diff-table unified">
      <colgroup>
        <col className="col-num" />
        <col className="col-num" />
        <col className="col-marker" />
        <col />
      </colgroup>
      <tbody>
        {hunks.map((hunk, hi) => (
          <Fragment key={hi}>
            <tr className="hunk">
              <td className="num" />
              <td className="num" />
              <td className="code" colSpan={2}>
                {hunk.header}
              </td>
            </tr>
            {hunk.lines.map((line, li) => {
              const side = lineSide(line)
              const num = lineNumber(line)
              const extras = renderExtras(side, num)
              return (
                <Fragment key={li}>
                  <tr className={`line ${line.kind}${isSelected(selection, side, num) ? ' selected' : ''}`}>
                    <td
                      className="num clickable"
                      onClick={(ev) => onSelect('old', line.oldNumber, ev.shiftKey)}
                    >
                      {line.oldNumber > 0 ? line.oldNumber : ''}
                    </td>
                    <td
                      className="num clickable"
                      onClick={(ev) => onSelect('new', line.newNumber, ev.shiftKey)}
                    >
                      {line.newNumber > 0 ? line.newNumber : ''}
                    </td>
                    <td className="marker">{marker(line.kind)}</td>
                    <td className="code" onClick={(ev) => onSelect(side, num, ev.shiftKey)}>
                      {line.content || ' '}
                      {line.noNewline && <span className="hint"> (no newline at end of file)</span>}
                    </td>
                  </tr>
                  {extras && (
                    <tr className="extras">
                      <td colSpan={4}>{extras}</td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
          </Fragment>
        ))}
      </tbody>
    </table>
  )
}

interface SplitRow {
  left?: Line
  right?: Line
  /** paired marks a removed/added pair, which gets word level highlighting. */
  paired: boolean
}

/** buildSplitRows lays the lines of a hunk out in two columns. */
function buildSplitRows(lines: Line[]): SplitRow[] {
  const rows: SplitRow[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.kind === 'context') {
      rows.push({ left: line, right: line, paired: false })
      i++
      continue
    }
    const removed: Line[] = []
    const added: Line[] = []
    while (i < lines.length && lines[i].kind === 'delete') removed.push(lines[i++])
    while (i < lines.length && lines[i].kind === 'add') added.push(lines[i++])
    const count = Math.max(removed.length, added.length)
    for (let k = 0; k < count; k++) {
      rows.push({
        left: removed[k],
        right: added[k],
        paired: removed[k] !== undefined && added[k] !== undefined,
      })
    }
  }
  return rows
}

function SplitTable({ hunks, selection, onSelect, renderExtras }: TableProps) {
  return (
    <table className="diff-table split">
      <colgroup>
        <col className="col-num" />
        <col className="col-side" />
        <col className="col-num" />
        <col className="col-side" />
      </colgroup>
      <tbody>
        {hunks.map((hunk, hi) => (
          <Fragment key={hi}>
            <tr className="hunk">
              <td className="num" />
              <td className="code" colSpan={3}>
                {hunk.header}
              </td>
            </tr>
            {buildSplitRows(hunk.lines).map((row, ri) => {
              const [oldSegments, newSegments] = row.paired
                ? wordDiff(row.left?.content ?? '', row.right?.content ?? '')
                : [null, null]
              const leftExtras = row.left ? renderExtras('old', row.left.oldNumber) : null
              const rightExtras = row.right ? renderExtras('new', row.right.newNumber) : null
              const hasExtras = Boolean(leftExtras || rightExtras)
              return (
                <Fragment key={ri}>
                  <tr className="line">
                    <td
                      className={`num clickable${isSelected(selection, 'old', row.left?.oldNumber ?? -1) ? ' selected' : ''}`}
                      onClick={(ev) => row.left && onSelect('old', row.left.oldNumber, ev.shiftKey)}
                    >
                      {row.left && row.left.oldNumber > 0 ? row.left.oldNumber : ''}
                    </td>
                    <td
                      className={`code side ${row.left ? row.left.kind : 'empty'}${
                        isSelected(selection, 'old', row.left?.oldNumber ?? -1) ? ' selected' : ''
                      }`}
                      onClick={(ev) => row.left && onSelect('old', row.left.oldNumber, ev.shiftKey)}
                    >
                      {row.left ? renderSegments(row.left.content, oldSegments) : ''}
                    </td>
                    <td
                      className={`num clickable${isSelected(selection, 'new', row.right?.newNumber ?? -1) ? ' selected' : ''}`}
                      onClick={(ev) => row.right && onSelect('new', row.right.newNumber, ev.shiftKey)}
                    >
                      {row.right && row.right.newNumber > 0 ? row.right.newNumber : ''}
                    </td>
                    <td
                      className={`code side ${row.right ? row.right.kind : 'empty'}${
                        isSelected(selection, 'new', row.right?.newNumber ?? -1) ? ' selected' : ''
                      }`}
                      onClick={(ev) => row.right && onSelect('new', row.right.newNumber, ev.shiftKey)}
                    >
                      {row.right ? renderSegments(row.right.content, newSegments) : ''}
                    </td>
                  </tr>
                  {hasExtras && (
                    <tr className="extras">
                      <td colSpan={4}>
                        {leftExtras}
                        {rightExtras}
                      </td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
          </Fragment>
        ))}
      </tbody>
    </table>
  )
}

function renderSegments(content: string, segments: { text: string; changed: boolean }[] | null) {
  if (!segments) return content || ' '
  return (
    <>
      {segments.map((seg, i) =>
        seg.changed ? (
          <mark key={i}>{seg.text}</mark>
        ) : (
          <Fragment key={i}>{seg.text}</Fragment>
        ),
      )}
      {content === '' ? ' ' : null}
    </>
  )
}

/** snippetFor collects the reviewed lines so the comment stays readable
 * outside the browser, for instance in `sa comments`. */
function snippetFor(file: FileDiff, selection: Selection): string {
  const out: string[] = []
  for (const hunk of file.hunks) {
    for (const line of hunk.lines) {
      const num = selection.side === 'old' ? line.oldNumber : line.newNumber
      if (num < selection.start || num > selection.end) continue
      if (selection.side === 'old' && line.kind === 'add') continue
      if (selection.side === 'new' && line.kind === 'delete') continue
      out.push(`${marker(line.kind)}${line.content}`)
    }
  }
  return out.join('\n')
}
