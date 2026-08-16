import type { RefObject } from 'react'
import type { Comment, Diff, FileDiff, Status } from '../types'
import { filePath } from '../types'
import { client } from '../client'

/**
 * matchesPath reports whether a path answers a search.
 *
 * Every whitespace-separated term has to appear somewhere in the path,
 * ignoring case - so "server go" and "internal/server" both find
 * internal/server/server.go. Nothing turns up that does not contain what
 * was typed. A looser match (the letters in order, anywhere)
 * would find more, and would also find things the reader did not ask for,
 * which in a list you are scanning is worse than finding nothing.
 */
export function matchesPath(path: string, query: string): boolean {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (terms.length === 0) return true
  const haystack = path.toLowerCase()
  return terms.every((term) => haystack.includes(term))
}

interface Props {
  /** width in pixels; 0 collapses the file list out of the way and null lets
   * it fill the space, which is what a phone does. */
  width: number | null
  group: string
  diffs: Diff[]
  comments: Comment[]
  status: Status | null
  selected: { diffId: string; fileId: string } | null
  onSelect: (diffId: string, fileId: string) => void
  onChanged: () => void
  /** query narrows the list to the paths that contain it. */
  query: string
  onQuery: (query: string) => void
  searchRef?: RefObject<HTMLInputElement | null>
}

export function Sidebar({
  width,
  group,
  diffs,
  comments,
  status,
  selected,
  onSelect,
  onChanged,
  query,
  onQuery,
  searchRef,
}: Props) {
  const commentCount = (diffId: string, fileId: string) =>
    comments.filter((c) => c.diffId === diffId && c.fileId === fileId && !c.resolved).length

  const shown = (diff: Diff): FileDiff[] => diff.files.filter((f) => matchesPath(filePath(f), query))
  const total = diffs.reduce((n, d) => n + d.files.length, 0)
  const found = diffs.reduce((n, d) => n + shown(d).length, 0)

  // Enter takes you to the first path still standing, which is the whole
  // point of typing into a list.
  const openFirst = () => {
    for (const diff of diffs) {
      const first = shown(diff)[0]
      if (first) {
        onSelect(diff.id, first.id)
        return
      }
    }
  }

  return (
    <aside
      className={`sidebar${width === 0 ? ' collapsed' : ''}${width === null ? ' fill' : ''}`}
      style={width === null ? undefined : { width }}
      aria-hidden={width === 0}
    >
      {diffs.length === 0 && <p className="empty">No diff yet.</p>}

      {total > 0 && (
        <div className="file-search">
          <input
            ref={searchRef}
            type="search"
            className="file-search-input"
            value={query}
            placeholder="Filter paths ( / )"
            aria-label="Filter files by path"
            onChange={(ev) => onQuery(ev.target.value)}
            onKeyDown={(ev) => {
              if (ev.key === 'Escape') {
                if (query === '') ev.currentTarget.blur()
                else onQuery('')
              }
              if (ev.key === 'Enter') openFirst()
            }}
          />
          {query !== '' && (
            <span className="hint">
              {found} of {total}
            </span>
          )}
        </div>
      )}

      {query !== '' && found === 0 && <p className="empty">No path contains that.</p>}

      {diffs.map((diff) => (
        <div className="diff-group" key={diff.id} hidden={shown(diff).length === 0}>
          <div className="diff-group-header">
            <span className="diff-group-title" title={new Date(diff.createdAt).toLocaleString()}>
              {diff.title}
            </span>
            {!client.isStatic && (
              <button
                className="ghost danger"
                title="Remove this diff"
                onClick={() => {
                  void client.deleteDiff(group, diff.id).then(onChanged)
                }}
              >
                ×
              </button>
            )}
          </div>
          <ul className="file-list">
            {shown(diff).map((file) => {
              const active = selected?.diffId === diff.id && selected.fileId === file.id
              const count = commentCount(diff.id, file.id)
              // A folded file is still listed - the point is that it is out
              // of the way, not out of sight.
              const folded = Boolean(file.folded) && count === 0
              return (
                <li key={file.id}>
                  <button
                    className={`file-item${active ? ' active' : ''}${folded ? ' folded' : ''}`}
                    onClick={() => onSelect(diff.id, file.id)}
                  >
                    <span className={`dot status-${file.status}`} title={file.status} />
                    <span className="file-path" title={filePath(file)}>
                      {filePath(file)}
                    </span>
                    {folded && (
                      <span className="badge sm" title={file.foldReason}>
                        folded
                      </span>
                    )}
                    {file.isMarkdown && <span className="badge sm" title="Previewable with mo">md</span>}
                    {count > 0 && <span className="badge sm warn">{count}</span>}
                    <span className="stat add">+{file.additions}</span>
                    <span className="stat del">-{file.deletions}</span>
                  </button>
                </li>
              )
            })}
          </ul>
        </div>
      ))}

      {status && status.groups.length > 1 && (
        <div className="groups">
          <div className="groups-title">Groups</div>
          <ul>
            {status.groups.map((g) => (
              <li key={g.name}>
                <a className={g.name === group ? 'active' : ''} href={g.url}>
                  {g.name}
                  <span className="hint">
                    {g.diffs} diff(s){g.unresolved > 0 ? `, ${g.unresolved} open` : ''}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </div>
      )}
    </aside>
  )
}
