import { useEffect, useState, type RefObject } from 'react'
import type { Comment, Diff, FileDiff, Status } from '../types'
import { filePath } from '../types'
import { client } from '../client'
import { readSetting, writeSetting } from '../storage'

/** Layout is how the rounds are shown: stacked, or one tab at a time. */
type Layout = 'list' | 'tabs'

const LAYOUT_KEY = 'sbnn.sidebar.layout'

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

  // A shut group still says how much is waiting inside it.
  const groupComments = (diff: Diff): number =>
    comments.filter((c) => c.diffId === diff.id && !c.resolved).length

  // Rounds pile up: a review of four diffs is four headings and everything
  // under them. A group can be shut, and the whole list can be turned into
  // tabs, which shows one round at a time.
  const [layout, setLayout] = useState<Layout>(
    () => (readSetting(LAYOUT_KEY) === 'tabs' ? 'tabs' : 'list'),
  )
  const [shutGroups, setShutGroups] = useState<Set<string>>(() => new Set())
  const [tab, setTab] = useState<string | null>(null)

  useEffect(() => {
    writeSetting(LAYOUT_KEY, layout)
  }, [layout])

  const shown = (diff: Diff): FileDiff[] => diff.files.filter((f) => matchesPath(filePath(f), query))
  const total = diffs.reduce((n, d) => n + d.files.length, 0)
  const found = diffs.reduce((n, d) => n + shown(d).length, 0)

  const searching = query !== ''

  // A search is about the whole review, not about one round of it, so the
  // tabs are searched too: a round with nothing matching drops out of the
  // strip, and its count says how much it holds. Losing the tabs during a
  // search - which is what flattening them into a list did - takes the
  // reader out of the layout they chose the moment they look for
  // something.
  const tabbed = diffs.filter((d) => !searching || shown(d).length > 0)

  // The tab in front is the one holding the selected file, so picking a
  // file from anywhere - a keyboard step, a comment - brings its round
  // forward rather than leaving the reader on a tab that shows nothing.
  // When a search empties the current tab, the first one with a match
  // takes over, since a tab strip with nothing behind it is a dead end.
  const preferred = diffs.find((d) => d.id === selected?.diffId)?.id ?? tab ?? diffs[0]?.id ?? null
  const activeTab =
    tabbed.some((d) => d.id === preferred) ? preferred : (tabbed[0]?.id ?? null)

  const visible = (diff: Diff): boolean => {
    if (shown(diff).length === 0) return false
    if (layout === 'tabs') return diff.id === activeTab
    return true
  }
  // A search opens every round it matched: a match hidden inside a shut
  // group is a match nobody sees.
  const isShut = (diff: Diff): boolean =>
    layout === 'list' && !searching && shutGroups.has(diff.id)

  const toggleGroup = (id: string) =>
    setShutGroups((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

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
          {diffs.length > 1 && (
            <div className="toggle sm" title="Stack the rounds, or show one at a time">
              <button
                className={layout === 'list' ? 'active' : ''}
                onClick={() => setLayout('list')}
              >
                list
              </button>
              <button
                className={layout === 'tabs' ? 'active' : ''}
                onClick={() => setLayout('tabs')}
              >
                tabs
              </button>
            </div>
          )}
        </div>
      )}

      {layout === 'tabs' && diffs.length > 1 && (
        <div className="diff-tabs" role="tablist">
          {tabbed.map((diff) => (
            <button
              key={diff.id}
              role="tab"
              aria-selected={diff.id === activeTab}
              className={`diff-tab${diff.id === activeTab ? ' active' : ''}`}
              title={new Date(diff.createdAt).toLocaleString()}
              onClick={() => {
                setTab(diff.id)
                const first = shown(diff)[0]
                if (first) onSelect(diff.id, first.id)
              }}
            >
              {diff.title}
              {searching && tabbed.length > 1 && (
                <span className="hint" title="paths matching in this round">
                  {shown(diff).length}
                </span>
              )}
              {groupComments(diff) > 0 && <span className="badge sm warn">{groupComments(diff)}</span>}
              {!client.isStatic && diff.id === activeTab && (
                <span
                  className="tab-remove"
                  role="button"
                  title="Remove this diff"
                  onClick={(ev) => {
                    ev.stopPropagation()
                    void client.deleteDiff(group, diff.id).then(onChanged)
                  }}
                >
                  ×
                </span>
              )}
            </button>
          ))}
        </div>
      )}

      {searching && found === 0 && <p className="empty">No path contains that.</p>}

      {diffs.map((diff) => (
        <div className="diff-group" key={diff.id} hidden={!visible(diff)}>
          <div className="diff-group-header" hidden={layout === 'tabs'}>
            {layout === 'list' ? (
              <button
                className="diff-group-title as-button"
                title={new Date(diff.createdAt).toLocaleString()}
                aria-expanded={!isShut(diff)}
                onClick={() => toggleGroup(diff.id)}
              >
                <span className="disclosure">{isShut(diff) ? '▸' : '▾'}</span>
                {diff.title}
                <span className="hint">{shown(diff).length}</span>
                {groupComments(diff) > 0 && (
                  <span className="badge sm warn">{groupComments(diff)}</span>
                )}
              </button>
            ) : (
              <span className="diff-group-title" title={new Date(diff.createdAt).toLocaleString()}>
                {diff.title}
              </span>
            )}
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
          <ul className="file-list" hidden={isShut(diff)}>
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

      {status && status.groups.some((g) => g.name !== group) && (
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
